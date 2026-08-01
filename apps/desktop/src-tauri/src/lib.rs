use serde::{Deserialize, Serialize};
use std::{
    borrow::Cow,
    fs::File,
    io::{BufReader, Read, Write},
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    sync::{
        atomic::{AtomicU8, Ordering},
        Arc, Mutex,
    },
    time::{Duration, SystemTime, UNIX_EPOCH},
};

mod platform;
#[cfg(windows)]
mod native_windows;
#[cfg(not(any(target_os = "macos", windows)))]
mod native_linux;
use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager,
};

const CONTROL_PROTOCOL_VERSION: u8 = 1;
const MAX_CONTROL_RESPONSE: u64 = 64 << 10;
const CONTROL_EXCHANGE_TIMEOUT: Duration = Duration::from_secs(20);
const CONTROL_STATUS_TIMEOUT: Duration = Duration::from_millis(800);
const CONTROL_STARTUP_PROBE_TIMEOUT: Duration = Duration::from_millis(250);
const SIDECAR_STARTUP_LOG_MAX_BYTES: u64 = 8 << 10;
const SECURITY_SETTINGS_VERSION: u8 = 1;
const DEFAULT_HTTP_PORT: u16 = 4768;
const DEFAULT_SSH_PORT: u16 = 4770;
const MIN_UNPRIVILEGED_PORT: u16 = 1024;
const DEVELOPER_WEBSITE_URL: &str = "https://0o0.site";
const DEVELOPER_GITHUB_URL: &str = "https://github.com/LouisonH";
static UI_LOCALE: AtomicU8 = AtomicU8::new(0);

fn ui_locale() -> &'static str {
    match UI_LOCALE.load(Ordering::Relaxed) {
        1 => "en",
        2 => "ja",
        _ => "zh-CN",
    }
}

#[tauri::command]
fn set_ui_locale(locale: String) -> Result<(), String> {
    let value = match locale.as_str() {
        "zh-CN" => 0,
        "en" => 1,
        "ja" => 2,
        _ => return Err("unsupported interface locale".to_string()),
    };
    UI_LOCALE.store(value, Ordering::Relaxed);
    Ok(())
}

fn native_text(source: &str) -> Cow<'_, str> {
    let translated = match (ui_locale(), source) {
        ("en", "Airlock 安全录入") => "Airlock secure entry",
        ("ja", "Airlock 安全录入") => "Airlock セキュア入力",
        ("en", "取消") => "Cancel",
        ("ja", "取消") => "キャンセル",
        ("en", "继续") => "Continue",
        ("ja", "继续") => "続行",
        ("en", "保存") => "Save",
        ("ja", "保存") => "保存",
        ("en", "不设置") => "Skip",
        ("ja", "不设置") => "設定しない",
        ("en", "完成") => "Done",
        ("ja", "完成") => "完了",
        ("en", "应用并重启") => "Apply and restart",
        ("ja", "应用并重启") => "適用して再起動",
        ("en", "Airlock 安全设置") => "Airlock security settings",
        ("ja", "Airlock 安全设置") => "Airlock セキュリティ設定",
        ("en", "局域网设备将能连接 Airlock 的 HTTP/SSH 入口，仍需要每条路由的凭据。") => "Devices on the private LAN will be able to connect to Airlock's HTTP/SSH endpoints. Each route credential is still required.",
        ("ja", "局域网设备将能连接 Airlock 的 HTTP/SSH 入口，仍需要每条路由的凭据。") => "プライベート LAN 上のデバイスから Airlock の HTTP/SSH 入口に接続できるようになります。各ルートの認証情報は引き続き必要です。",
        ("en", "上游地址和凭据将保存在仅当前用户可读的 0600 文件中，不再由 macOS Keychain 加密保护。") => "Upstream addresses and credentials will be stored in a 0600 file readable only by the current user, without macOS Keychain encryption.",
        ("ja", "上游地址和凭据将保存在仅当前用户可读的 0600 文件中，不再由 macOS Keychain 加密保护。") => "上流アドレスと認証情報は、現在のユーザーだけが読み取れる 0600 ファイルに保存され、macOS Keychain では暗号化されません。",
        ("en", "上游地址和凭据将保存在仅当前用户可读的受保护文件中，不再由系统凭据库加密保护。") => "Upstream addresses and credentials will be stored in a protected file readable only by the current user, without system credential-store encryption.",
        ("ja", "上游地址和凭据将保存在仅当前用户可读的受保护文件中，不再由系统凭据库加密保护。") => "上流アドレスと認証情報は、現在のユーザーだけが読み取れる保護されたファイルに保存され、システムの資格情報ストアでは暗号化されません。",
        ("en", "复制") => "Copy",
        ("ja", "复制") => "コピー",
        ("en", "应用设置会短暂重启本地转发核心。") => "Applying these settings briefly restarts the local relay core.",
        ("ja", "应用设置会短暂重启本地转发核心。") => "設定を適用すると、ローカル転送コアが短時間再起動します。",
        ("en", "高风险 SSH 权限") => "High-risk SSH permissions",
        ("ja", "高风险 SSH 权限") => "高リスク SSH 権限",
        ("en", "允许所有 exec") => "Allow all exec",
        ("ja", "允许所有 exec") => "すべての exec を許可",
        ("en", "确认 SSH Host Key") => "Confirm SSH Host Key",
        ("ja", "确认 SSH Host Key") => "SSH Host Key を確認",
        ("en", "指纹一致") => "Fingerprint matches",
        ("ja", "指纹一致") => "フィンガープリントが一致",
        ("en", "输入完整目标 URL。该内容仅发送到本机 airlockd，并保存进当前选择的受保护凭据存储。") => "Enter the complete target URL. It is sent only to the local airlockd and saved in the selected protected credential store.",
        ("ja", "输入完整目标 URL。该内容仅发送到本机 airlockd，并保存进当前选择的受保护凭据存储。") => "完全な対象 URL を入力します。ローカルの airlockd だけに送信され、選択中の保護された認証情報ストアに保存されます。",
        ("en", "输入上游 Authorization 值，例如 Bearer token。无需认证可选择“不设置”。") => "Enter the upstream Authorization value, such as a Bearer token. Choose Skip when authentication is not required.",
        ("ja", "输入上游 Authorization 值，例如 Bearer token。无需认证可选择“不设置”。") => "Bearer token などの上流 Authorization 値を入力します。認証が不要な場合は「設定しない」を選択します。",
        ("en", "LLM 设置 1/3 · 上游 Base URL") => "LLM setup 1/3 · Upstream Base URL",
        ("ja", "LLM 设置 1/3 · 上游 Base URL") => "LLM 設定 1/3 · 上流 Base URL",
        ("en", "输入兼容供应商的 Base URL。真实地址只会发送到本机 airlockd。") => "Enter the compatible provider Base URL. The real address is sent only to the local airlockd.",
        ("ja", "输入兼容供应商的 Base URL。真实地址只会发送到本机 airlockd。") => "互換プロバイダーの Base URL を入力します。実アドレスはローカルの airlockd だけに送信されます。",
        ("en", "LLM 设置 2/3 · 上游 API Key") => "LLM setup 2/3 · Upstream API key",
        ("ja", "LLM 设置 2/3 · 上游 API Key") => "LLM 設定 2/3 · 上流 API Key",
        ("en", "输入真实上游 API Key。调用者不会看到该 Key。") => "Enter the real upstream API key. Callers never see this key.",
        ("ja", "输入真实上游 API Key。调用者不会看到该 Key。") => "実際の上流 API Key を入力します。呼び出し元にこの Key は表示されません。",
        ("en", "SSH 设置 · 上游地址") => "SSH setup · Upstream address",
        ("ja", "SSH 设置 · 上游地址") => "SSH 設定 · 上流アドレス",
        ("en", "输入上游 SSH 地址，可使用 host、host:port 或 IP。地址只会发送到本机 airlockd。") => "Enter the upstream SSH address as host, host:port, or IP. It is sent only to the local airlockd.",
        ("ja", "输入上游 SSH 地址，可使用 host、host:port 或 IP。地址只会发送到本机 airlockd。") => "host、host:port、または IP 形式で上流 SSH アドレスを入力します。アドレスはローカルの airlockd だけに送信されます。",
        ("en", "SSH 设置 · 上游账号") => "SSH setup · Upstream account",
        ("ja", "SSH 设置 · 上游账号") => "SSH 設定 · 上流アカウント",
        ("en", "输入上游 SSH 用户名。调用者只会看到本地 SSH 用户名，不会看到上游账号。") => "Enter the upstream SSH username. Callers see only the local SSH username, never the upstream account.",
        ("ja", "输入上游 SSH 用户名。调用者只会看到本地 SSH 用户名，不会看到上游账号。") => "上流 SSH ユーザー名を入力します。呼び出し元にはローカル SSH ユーザー名だけが表示され、上流アカウントは表示されません。",
        ("en", "SSH 设置 · 上游密码") => "SSH setup · Upstream password",
        ("ja", "SSH 设置 · 上游密码") => "SSH 設定 · 上流パスワード",
        ("en", "输入上游 SSH 密码。密码只会交给本机核心，并按设置中的凭据保护方式保存。") => "Enter the upstream SSH password. It is passed only to the local core and stored using the configured credential protection.",
        ("ja", "输入上游 SSH 密码。密码只会交给本机核心，并按设置中的凭据保护方式保存。") => "上流 SSH パスワードを入力します。ローカルコアだけに渡され、設定中の認証情報保護方式で保存されます。",
        ("en", "SSH 设置 · 本地登录密码") => "SSH setup · Local login password",
        ("ja", "SSH 设置 · 本地登录密码") => "SSH 設定 · ローカルログインパスワード",
        ("en", "可选：输入至少 12 个字节的本地 SSH 密码。它与上游密码完全隔离，Airlock 只保存摘要。选择“不设置”将自动生成高强度 Capability。") => "Optional: enter a local SSH password of at least 12 bytes. It is isolated from the upstream password and Airlock stores only its digest. Choose Skip to generate a strong capability.",
        ("ja", "可选：输入至少 12 个字节的本地 SSH 密码。它与上游密码完全隔离，Airlock 只保存摘要。选择“不设置”将自动生成高强度 Capability。") => "任意：12 バイト以上のローカル SSH パスワードを入力します。上流パスワードとは完全に分離され、Airlock はダイジェストのみ保存します。「設定しない」で強力な Capability を自動生成します。",
        ("en", "SSH 设置 · 确认本地密码") => "SSH setup · Confirm local password",
        ("ja", "SSH 设置 · 确认本地密码") => "SSH 設定 · ローカルパスワードを確認",
        ("en", "再次输入本地 SSH 密码。") => "Enter the local SSH password again.",
        ("ja", "再次输入本地 SSH 密码。") => "ローカル SSH パスワードをもう一度入力します。",
        ("en", "输入 Clash 或其他本地代理 URL，例如 http://127.0.0.1:7890 或 socks5://127.0.0.1:7890。认证信息可写在 URL 中，内容仅进入当前选择的受保护凭据存储。") => "Enter a Clash or other local proxy URL, such as http://127.0.0.1:7890 or socks5://127.0.0.1:7890. Authentication may be included in the URL and is stored only in the selected protected credential store.",
        ("ja", "输入 Clash 或其他本地代理 URL，例如 http://127.0.0.1:7890 或 socks5://127.0.0.1:7890。认证信息可写在 URL 中，内容仅进入当前选择的受保护凭据存储。") => "http://127.0.0.1:7890 または socks5://127.0.0.1:7890 などの Clash またはローカルプロキシ URL を入力します。認証情報は URL に含められ、選択中の保護ストアにのみ保存されます。",
        ("en", "LLM 设置 3/3 · 二次 API Key") => "LLM setup 3/3 · Secondary API key",
        ("ja", "LLM 设置 3/3 · 二次 API Key") => "LLM 設定 3/3 · 二次 API Key",
        ("en", "为调用者创建一把独立的二次 API Key。它只用于访问 Airlock，真实上游 Key 不会暴露。\n\n随机生成提供 256-bit 强度并仅显示一次；自定义 Key 会要求隐藏输入两次。") => "Create an independent secondary API key for callers. It accesses only Airlock and never exposes the real upstream key.\n\nRandom generation provides 256-bit strength and is shown once. A custom key requires two hidden entries.",
        ("ja", "为调用者创建一把独立的二次 API Key。它只用于访问 Airlock，真实上游 Key 不会暴露。\n\n随机生成提供 256-bit 强度并仅显示一次；自定义 Key 会要求隐藏输入两次。") => "呼び出し元用の独立した二次 API Key を作成します。Airlock へのアクセスだけに使用され、実際の上流 Key は公開されません。\n\nランダム生成は 256-bit 強度で一度だけ表示されます。カスタム Key は非表示で 2 回入力します。",
        ("en", "自定义 Key") => "Custom key",
        ("ja", "自定义 Key") => "カスタム Key",
        ("en", "随机生成（推荐）") => "Generate randomly (recommended)",
        ("ja", "随机生成（推荐）") => "ランダム生成（推奨）",
        ("en", "请通过可信渠道核对上游 SSH Host Key 指纹。指纹不一致时请取消。") => "Verify the upstream SSH Host Key fingerprint through a trusted channel. Cancel if it does not match.",
        ("ja", "请通过可信渠道核对上游 SSH Host Key 指纹。指纹不一致时请取消。") => "信頼できる経路で上流 SSH Host Key のフィンガープリントを確認してください。一致しない場合はキャンセルします。",
        ("en", "路由已安全保存。Capability 仅显示这一次，请交给需要访问该路由的客户端。") => "The route was saved securely. This capability is shown once; give it only to the client that needs this route.",
        ("ja", "路由已安全保存。Capability 仅显示这一次，请交给需要访问该路由的客户端。") => "ルートは安全に保存されました。Capability は一度だけ表示されます。このルートが必要なクライアントだけに渡してください。",
        ("en", "Airlock 路由已创建") => "Airlock route created",
        ("ja", "Airlock 路由已创建") => "Airlock ルート作成済み",
        ("en", "Airlock LLM 路由已创建") => "Airlock LLM route created",
        ("ja", "Airlock LLM 路由已创建") => "Airlock LLM ルート作成済み",
        ("en", "Airlock SSH 路由已创建") => "Airlock SSH route created",
        ("ja", "Airlock SSH 路由已创建") => "Airlock SSH ルート作成済み",
        _ => return Cow::Borrowed(source),
    };
    Cow::Borrowed(translated)
}

#[derive(Clone)]
struct ControlClient {
    endpoint: String,
    token: Arc<SecretToken>,
}

struct SecretToken(String);

impl Drop for SecretToken {
    fn drop(&mut self) {
        clear_string(&mut self.0);
    }
}

struct ManagedDaemon {
    child: Child,
    startup_log: PathBuf,
}

#[derive(Clone)]
struct DaemonProcess(Arc<Mutex<Option<ManagedDaemon>>>);

impl DaemonProcess {
    fn stop(&self) {
        if let Ok(mut guard) = self.0.lock() {
            if let Some(mut daemon) = guard.take() {
                let _ = daemon.child.kill();
                let _ = daemon.child.wait();
            }
        }
    }

    fn replace(&self, child: Child, startup_log: PathBuf) {
        if let Ok(mut guard) = self.0.lock() {
            *guard = Some(ManagedDaemon { child, startup_log });
        }
    }

    fn startup_log(&self) -> Option<PathBuf> {
        self.0
            .lock()
            .ok()
            .and_then(|guard| guard.as_ref().map(|daemon| daemon.startup_log.clone()))
    }

    fn managed_pid(&self) -> Option<u32> {
        self.0
            .lock()
            .ok()
            .and_then(|guard| guard.as_ref().map(|daemon| daemon.child.id()))
    }
}

#[derive(Clone, Default)]
struct CoreStartupState(Arc<Mutex<Option<String>>>);

impl CoreStartupState {
    fn clear(&self) {
        if let Ok(mut state) = self.0.lock() {
            *state = None;
        }
    }

    fn set(&self, message: impl Into<String>) {
        if let Ok(mut state) = self.0.lock() {
            *state = Some(message.into());
        }
    }

    fn message(&self) -> Option<String> {
        self.0.lock().ok().and_then(|state| state.clone())
    }
}

impl Drop for DaemonProcess {
    fn drop(&mut self) {
        if Arc::strong_count(&self.0) == 1 {
            self.stop();
        }
    }
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct SecuritySettings {
    version: u8,
    network_scope: String,
    secret_store: String,
    #[serde(default = "default_http_port")]
    http_port: u16,
    #[serde(default = "default_ssh_port")]
    ssh_port: u16,
}

impl Default for SecuritySettings {
    fn default() -> Self {
        Self {
            version: SECURITY_SETTINGS_VERSION,
            network_scope: "loopback".to_string(),
            secret_store: "local_file".to_string(),
            http_port: default_http_port(),
            ssh_port: default_ssh_port(),
        }
    }
}

fn default_http_port() -> u16 {
    DEFAULT_HTTP_PORT
}

fn default_ssh_port() -> u16 {
    DEFAULT_SSH_PORT
}

fn allowed_external_url(url: &str) -> Option<&'static str> {
    match url {
        DEVELOPER_WEBSITE_URL | "https://0o0.site/" => Some(DEVELOPER_WEBSITE_URL),
        DEVELOPER_GITHUB_URL | "https://github.com/LouisonH/" => Some(DEVELOPER_GITHUB_URL),
        _ => None,
    }
}

fn external_open_command(target: &str) -> Command {
    #[cfg(target_os = "macos")]
    let mut command = Command::new("open");
    #[cfg(target_os = "windows")]
    let mut command = {
        let mut command = Command::new("cmd");
        command.args(["/C", "start", ""]);
        command
    };
    #[cfg(all(unix, not(target_os = "macos")))]
    let mut command = Command::new("xdg-open");
    command.arg(target);
    command
}

#[tauri::command]
fn open_external_url(url: String) -> Result<(), String> {
    let target =
        allowed_external_url(&url).ok_or_else(|| "拒绝打开未授权的外部链接".to_string())?;
    external_open_command(target)
        .spawn()
        .map(|_| ())
        .map_err(|_| "无法打开默认浏览器".to_string())
}

#[derive(Clone)]
struct SecurityConfiguration {
    path: PathBuf,
    settings: Arc<Mutex<SecuritySettings>>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
struct RouteSummary {
    id: String,
    name: String,
    alias: String,
    #[serde(default)]
    local_username: String,
    kind: String,
    status: String,
    local_endpoint: String,
    permission_summary: String,
    egress: String,
    health: String,
    last_used: String,
    current_connections: u32,
    allow_all_commands: bool,
    record_commands: bool,
    #[serde(default)]
    allowed_command: String,
    #[serde(default)]
    provider: String,
    #[serde(default)]
    allowed_models: Vec<String>,
    #[serde(default)]
    max_output_tokens: u32,
    #[serde(default)]
    requests_per_minute: u32,
    #[serde(default)]
    max_concurrent: u32,
    #[serde(default)]
    track_usage: bool,
    #[serde(default)]
    total_requests: u64,
    #[serde(default)]
    input_tokens: u64,
    #[serde(default)]
    output_tokens: u64,
    #[serde(default)]
    authentication_timeout_seconds: u32,
}

#[derive(Serialize)]
struct ControlRequest<'a> {
    version: u8,
    token: &'a str,
    action: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    create: Option<CreateHTTPRoute<'a>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    create_ssh: Option<CreateSSHRoute<'a>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    probe_ssh: Option<ProbeSSHHostKey<'a>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    ssh_policy: Option<SSHPolicyUpdate<'a>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    alias: Option<&'a str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enabled: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    proxy_url: Option<&'a str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    capability: Option<&'a str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    command: Option<&'a str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    secret_store_mode: Option<&'a str>,
}

#[derive(Serialize)]
struct CreateHTTPRoute<'a> {
    name: &'a str,
    alias: &'a str,
    base_url: &'a str,
    #[serde(skip_serializing_if = "str::is_empty")]
    authorization: &'a str,
    egress: &'a str,
    #[serde(skip_serializing_if = "str::is_empty")]
    provider: &'a str,
    #[serde(skip_serializing_if = "slice_is_empty")]
    models: &'a [String],
    #[serde(skip_serializing_if = "is_zero_u32")]
    max_output_tokens: u32,
    #[serde(skip_serializing_if = "is_zero_u32")]
    requests_per_minute: u32,
    #[serde(skip_serializing_if = "is_zero_u32")]
    max_concurrent: u32,
    track_usage: bool,
    #[serde(skip_serializing_if = "str::is_empty")]
    local_api_key: &'a str,
}

fn is_zero_u32(value: &u32) -> bool {
    *value == 0
}

fn slice_is_empty<T>(value: &[T]) -> bool {
    value.is_empty()
}

#[derive(Serialize)]
struct CreateSSHRoute<'a> {
    name: &'a str,
    alias: &'a str,
    local_username: &'a str,
    address: &'a str,
    username: &'a str,
    password: &'a str,
    #[serde(skip_serializing_if = "str::is_empty")]
    local_password: &'a str,
    expected_host_key: &'a str,
    allowed_command: &'a str,
    allow_all_commands: bool,
    record_commands: bool,
    authentication_timeout_seconds: u32,
    egress: &'a str,
}

#[derive(Serialize)]
struct SSHPolicyUpdate<'a> {
    name: &'a str,
    local_username: &'a str,
    allowed_command: &'a str,
    allow_all_commands: bool,
    record_commands: bool,
    authentication_timeout_seconds: u32,
    egress: &'a str,
}

#[derive(Serialize)]
struct ProbeSSHHostKey<'a> {
    address: &'a str,
    egress: &'a str,
}

#[derive(Deserialize)]
struct ControlResponse {
    ok: bool,
    #[serde(default)]
    error: String,
    #[serde(default)]
    warning: String,
    #[serde(default)]
    running: bool,
    #[serde(default)]
    routes: Vec<RouteSummary>,
    created: Option<CreatedRoute>,
    #[serde(default)]
    proxy_configured: bool,
    #[serde(default)]
    ssh_ready: bool,
    ssh_host_key_probe: Option<SSHHostKeyProbe>,
    health_check: Option<HealthCheckSummary>,
    #[serde(default)]
    activity: Vec<ActivityEvent>,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
struct ActivityEvent {
    id: String,
    time: String,
    route_name: String,
    caller: String,
    action: String,
    result: String,
    latency: String,
    egress: String,
    #[serde(default)]
    category: String,
    #[serde(default)]
    event_type: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct HealthCheckSummary {
    alias: String,
    status: String,
    message: String,
    latency: String,
    checked_at: String,
}

#[derive(Deserialize)]
struct SSHHostKeyProbe {
    host_key: String,
    fingerprint: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct SSHHostKeyProbeResult {
    host_key: String,
    fingerprint: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct SSHRouteCreationResult {
    route: RouteSummary,
    local_credential: String,
    generated_credential: bool,
}

#[derive(Deserialize)]
struct CreatedRoute {
    route: RouteSummary,
    capability: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct ControlState {
    connected: bool,
    running: bool,
    routes: Vec<RouteSummary>,
    message: Option<String>,
    proxy_configured: bool,
    ssh_ready: bool,
    activity: Vec<ActivityEvent>,
    security_settings: SecuritySettings,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct ControlUpdate {
    routes: Vec<RouteSummary>,
    message: Option<String>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct SecurityUpdate {
    security_settings: SecuritySettings,
    message: Option<String>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct PlatformInfo {
    os: String,
    arch: String,
    control_transport: String,
    secret_store: String,
    desktop_release: bool,
}

impl ControlClient {
    fn request(&self, request: &ControlRequest<'_>) -> Result<ControlResponse, String> {
        self.request_with_timeout(request, CONTROL_EXCHANGE_TIMEOUT)
    }

    fn request_with_timeout(
        &self,
        request: &ControlRequest<'_>,
        timeout: Duration,
    ) -> Result<ControlResponse, String> {
        let authenticated = ControlRequest {
            version: request.version,
            token: &self.token.0,
            action: request.action,
            create: request.create.as_ref().map(|create| CreateHTTPRoute {
                name: create.name,
                alias: create.alias,
                base_url: create.base_url,
                authorization: create.authorization,
                egress: create.egress,
                provider: create.provider,
                models: create.models,
                max_output_tokens: create.max_output_tokens,
                requests_per_minute: create.requests_per_minute,
                max_concurrent: create.max_concurrent,
                track_usage: create.track_usage,
                local_api_key: create.local_api_key,
            }),
            create_ssh: request.create_ssh.as_ref().map(|create| CreateSSHRoute {
                name: create.name,
                alias: create.alias,
                local_username: create.local_username,
                address: create.address,
                username: create.username,
                password: create.password,
                local_password: create.local_password,
                expected_host_key: create.expected_host_key,
                allowed_command: create.allowed_command,
                allow_all_commands: create.allow_all_commands,
                record_commands: create.record_commands,
                authentication_timeout_seconds: create.authentication_timeout_seconds,
                egress: create.egress,
            }),
            probe_ssh: request.probe_ssh.as_ref().map(|probe| ProbeSSHHostKey {
                address: probe.address,
                egress: probe.egress,
            }),
            ssh_policy: request.ssh_policy.as_ref().map(|policy| SSHPolicyUpdate {
                name: policy.name,
                local_username: policy.local_username,
                allowed_command: policy.allowed_command,
                allow_all_commands: policy.allow_all_commands,
                record_commands: policy.record_commands,
                authentication_timeout_seconds: policy.authentication_timeout_seconds,
                egress: policy.egress,
            }),
            alias: request.alias,
            enabled: request.enabled,
            proxy_url: request.proxy_url,
            capability: request.capability,
            command: request.command,
            secret_store_mode: request.secret_store_mode,
        };
        let mut payload =
            serde_json::to_vec(&authenticated).map_err(|_| "无法编码控制请求".to_string())?;
        payload.push(b'\n');
        let result = self.exchange(&payload, timeout);
        payload.fill(0);
        result
    }

    fn exchange(&self, payload: &[u8], timeout: Duration) -> Result<ControlResponse, String> {
        let mut raw = String::new();
        let response = platform::exchange_control(&self.endpoint, payload, timeout)
            .map_err(|_| "无法连接 airlockd 控制通道".to_string())?;
        raw.push_str(&response);
        if raw.len() > MAX_CONTROL_RESPONSE as usize {
            clear_string(&mut raw);
            return Err("控制响应过大".to_string());
        }
        let response = serde_json::from_str::<ControlResponse>(&raw)
            .map_err(|_| "控制响应格式无效".to_string());
        clear_string(&mut raw);
        let response = response?;
        if !response.ok {
            return Err(if response.error.is_empty() {
                "airlockd 拒绝了控制请求".to_string()
            } else {
                response.error
            });
        }
        Ok(response)
    }
}

#[tauri::command]
fn get_platform_info() -> PlatformInfo {
    let os = if cfg!(target_os = "macos") {
        "macos"
    } else if cfg!(target_os = "windows") {
        "windows"
    } else if cfg!(target_os = "linux") {
        "linux"
    } else {
        "other"
    };
    PlatformInfo {
        os: os.to_string(),
        arch: std::env::consts::ARCH.to_string(),
        control_transport: if cfg!(windows) {
            "named-pipe".to_string()
        } else {
            "unix-socket".to_string()
        },
        secret_store: if cfg!(target_os = "macos") {
            "keychain".to_string()
        } else if cfg!(windows) {
            "credential-manager".to_string()
        } else {
            "secret-service".to_string()
        },
        desktop_release: cfg!(all(
            target_os = "macos",
            target_arch = "aarch64",
            not(debug_assertions)
        )),
    }
}

fn read_control_state(
    client: &ControlClient,
    security: &SecurityConfiguration,
    startup: &CoreStartupState,
) -> ControlState {
    let security_settings = security
        .settings
        .lock()
        .map(|settings| settings.clone())
        .unwrap_or_default();
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "status",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: None,
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    match client.request_with_timeout(&request, CONTROL_STATUS_TIMEOUT) {
        Ok(response) => ControlState {
            connected: true,
            running: response.running,
            routes: response.routes,
            message: None,
            proxy_configured: response.proxy_configured,
            ssh_ready: response.ssh_ready,
            activity: response.activity,
            security_settings: security_settings.clone(),
        },
        Err(message) => ControlState {
            connected: false,
            running: false,
            routes: Vec::new(),
            message: startup.message().or(Some(message)),
            proxy_configured: false,
            ssh_ready: false,
            activity: Vec::new(),
            security_settings,
        },
    }
}

#[tauri::command]
async fn get_control_state(
    client: tauri::State<'_, ControlClient>,
    security: tauri::State<'_, SecurityConfiguration>,
    startup: tauri::State<'_, CoreStartupState>,
) -> Result<ControlState, String> {
    let client = client.inner().clone();
    let security = security.inner().clone();
    let startup = startup.inner().clone();
    Ok(tauri::async_runtime::spawn_blocking(move || {
        read_control_state(&client, &security, &startup)
    })
    .await
    .unwrap_or_else(|_| ControlState {
        connected: false,
        running: false,
        routes: Vec::new(),
        message: Some("无法读取本地核心状态，请重试".to_string()),
        proxy_configured: false,
        ssh_ready: false,
        activity: Vec::new(),
        security_settings: SecuritySettings::default(),
    }))
}

#[tauri::command]
fn set_route_enabled(
    client: tauri::State<'_, ControlClient>,
    alias: String,
    enabled: bool,
) -> Result<Vec<RouteSummary>, String> {
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "set_route_enabled",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(&alias),
        enabled: Some(enabled),
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    client.request(&request).map(|response| response.routes)
}

#[tauri::command]
fn stop_all_routes(client: tauri::State<'_, ControlClient>) -> Result<Vec<RouteSummary>, String> {
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "stop_all",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: None,
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    client.request(&request).map(|response| response.routes)
}

#[tauri::command]
fn delete_route(
    client: tauri::State<'_, ControlClient>,
    alias: String,
) -> Result<ControlUpdate, String> {
    let request = delete_route_request(&alias);
    client.request(&request).map(|response| ControlUpdate {
        routes: response.routes,
        message: if response.warning.is_empty() {
            None
        } else {
            Some("路由已删除，但旧凭据副本清理需要检查".to_string())
        },
    })
}

#[tauri::command]
async fn create_http_route(
    client: tauri::State<'_, ControlClient>,
    name: String,
    alias: String,
    egress: String,
) -> Result<RouteSummary, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        create_http_route_blocking(client, name, alias, egress)
    })
    .await
    .map_err(|_| "原生安全录入意外终止".to_string())?
}

fn create_http_route_blocking(
    client: ControlClient,
    name: String,
    alias: String,
    egress: String,
) -> Result<RouteSummary, String> {
    if name.trim().is_empty() || name.len() > 80 {
        return Err("请输入 1 到 80 个字符的路由名称".to_string());
    }
    if alias.is_empty()
        || alias.len() > 63
        || !alias.bytes().enumerate().all(|(index, byte)| {
            byte.is_ascii_lowercase() || byte.is_ascii_digit() || (byte == b'-' && index > 0)
        })
    {
        return Err("别名只能包含小写字母、数字和连字符".to_string());
    }
    if egress != "Direct" && egress != "Proxy" && egress != "Auto" {
        return Err("出口策略无效".to_string());
    }

    let mut base_url = prompt_protected_value(
        "输入完整目标 URL。该内容仅发送到本机 airlockd，并保存进当前选择的受保护凭据存储。",
        false,
    )?;
    let mut authorization = prompt_protected_value(
        "输入上游 Authorization 值，例如 Bearer token。无需认证可选择“不设置”。",
        true,
    )?;
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "create_http_route",
        create: Some(CreateHTTPRoute {
            name: name.trim(),
            alias: &alias,
            base_url: &base_url,
            authorization: &authorization,
            egress: &egress,
            provider: "",
            models: &[],
            max_output_tokens: 0,
            requests_per_minute: 0,
            max_concurrent: 0,
            track_usage: false,
            local_api_key: "",
        }),
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: None,
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    let result = client.request(&request);
    clear_string(&mut base_url);
    clear_string(&mut authorization);
    let mut created = result?
        .created
        .ok_or_else(|| "airlockd 未返回新路由".to_string())?;

    if let Err(error) = present_capability(&created.route.local_endpoint, &created.capability) {
        clear_string(&mut created.capability);
        return Err(error);
    }
    clear_string(&mut created.capability);

    let enable = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "set_route_enabled",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(&alias),
        enabled: Some(true),
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    client
        .request(&enable)?
        .routes
        .into_iter()
        .find(|route| route.alias == alias)
        .ok_or_else(|| "新路由未出现在控制状态中".to_string())
}

struct CreateLlmRouteInput {
    name: String,
    alias: String,
    egress: String,
    provider: String,
    models: Vec<String>,
    max_output_tokens: u32,
    requests_per_minute: u32,
    max_concurrent: u32,
    track_usage: bool,
}

// Tauri maps these parameters directly from the stable frontend IPC contract.
#[allow(clippy::too_many_arguments)]
#[tauri::command]
async fn create_llm_route(
    client: tauri::State<'_, ControlClient>,
    name: String,
    alias: String,
    egress: String,
    provider: String,
    models: Vec<String>,
    max_output_tokens: u32,
    requests_per_minute: u32,
    max_concurrent: u32,
    track_usage: bool,
) -> Result<RouteSummary, String> {
    let client = client.inner().clone();
    let input = CreateLlmRouteInput {
        name,
        alias,
        egress,
        provider,
        models,
        max_output_tokens,
        requests_per_minute,
        max_concurrent,
        track_usage,
    };
    tauri::async_runtime::spawn_blocking(move || create_llm_route_blocking(client, input))
        .await
        .map_err(|_| "LLM 原生安全录入意外终止".to_string())?
}

fn create_llm_route_blocking(
    client: ControlClient,
    input: CreateLlmRouteInput,
) -> Result<RouteSummary, String> {
    let CreateLlmRouteInput {
        name,
        alias,
        egress,
        provider,
        models,
        max_output_tokens,
        requests_per_minute,
        max_concurrent,
        track_usage,
    } = input;
    validate_route_identity(&name, &alias, &egress)?;
    if provider != "openai" && provider != "anthropic" {
        return Err("LLM 供应商预设无效".to_string());
    }
    if models.is_empty()
        || models.len() > 32
        || models.iter().any(|model| {
            model.trim() != model
                || model.is_empty()
                || model.len() > 200
                || model.contains(['\0', '\r', '\n', '\t'])
        })
        || max_output_tokens == 0
        || max_output_tokens > 1_000_000
        || requests_per_minute == 0
        || requests_per_minute > 60_000
        || max_concurrent == 0
        || max_concurrent > 1_024
    {
        return Err("请设置有效的模型白名单、输出、速率与并发上限".to_string());
    }
    let default_url = if provider == "anthropic" {
        "https://api.anthropic.com"
    } else {
        "https://api.openai.com"
    };
    let mut base_url = prompt_native_value_with_title(
        "LLM 设置 1/3 · 上游 Base URL",
        "输入兼容供应商的 Base URL。真实地址只会发送到本机 airlockd。",
        false,
        false,
        default_url,
    )?;
    let mut upstream_api_key = match prompt_native_value_with_title(
        "LLM 设置 2/3 · 上游 API Key",
        "输入真实上游 API Key。调用者不会看到该 Key。",
        false,
        true,
        "",
    ) {
        Ok(value) => value,
        Err(error) => {
            clear_string(&mut base_url);
            return Err(error);
        }
    };
    let (mut local_api_key, use_custom_local_key) = match prompt_llm_local_api_key() {
        Ok(value) => value,
        Err(error) => {
            clear_string(&mut base_url);
            clear_string(&mut upstream_api_key);
            return Err(error);
        }
    };

    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "create_llm_route",
        create: Some(CreateHTTPRoute {
            name: name.trim(),
            alias: &alias,
            base_url: &base_url,
            authorization: &upstream_api_key,
            egress: &egress,
            provider: &provider,
            models: &models,
            max_output_tokens,
            requests_per_minute,
            max_concurrent,
            track_usage,
            local_api_key: &local_api_key,
        }),
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: None,
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    let result = client.request(&request);
    clear_string(&mut base_url);
    clear_string(&mut upstream_api_key);
    let custom_local_key = use_custom_local_key;
    clear_string(&mut local_api_key);
    let mut created = result?
        .created
        .ok_or_else(|| "airlockd 未返回新 LLM 路由".to_string())?;

    if let Err(error) = present_llm_access(
        &provider,
        &created.route.local_endpoint,
        &created.capability,
        custom_local_key,
    ) {
        clear_string(&mut created.capability);
        return Err(error);
    }
    clear_string(&mut created.capability);

    let enable = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "set_route_enabled",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(&alias),
        enabled: Some(true),
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    client
        .request(&enable)?
        .routes
        .into_iter()
        .find(|route| route.alias == alias)
        .ok_or_else(|| "新 LLM 路由未出现在控制状态中".to_string())
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct CreateSSHRouteInput {
    name: String,
    alias: String,
    local_username: String,
    address: String,
    username: String,
    password: String,
    local_password: String,
    expected_host_key: String,
    egress: String,
    allowed_command: String,
    allow_all_commands: bool,
    allow_all_confirmed: bool,
    record_commands: bool,
    authentication_timeout_seconds: u32,
}

#[tauri::command]
async fn probe_ssh_host_key(
    client: tauri::State<'_, ControlClient>,
    address: String,
    egress: String,
) -> Result<SSHHostKeyProbeResult, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        validate_ssh_upstream(&address, "probe-user", "probe-password")?;
        validate_egress(&egress)?;
        let request = ControlRequest {
            version: CONTROL_PROTOCOL_VERSION,
            token: "",
            action: "probe_ssh_host_key",
            create: None,
            create_ssh: None,
            probe_ssh: Some(ProbeSSHHostKey {
                address: address.trim(),
                egress: &egress,
            }),
            ssh_policy: None,
            alias: None,
            enabled: None,
            proxy_url: None,
            capability: None,
            command: None,
            secret_store_mode: None,
        };
        let probe = client
            .request(&request)?
            .ssh_host_key_probe
            .ok_or_else(|| "airlockd 未返回 SSH Host Key".to_string())?;
        Ok(SSHHostKeyProbeResult {
            host_key: probe.host_key,
            fingerprint: probe.fingerprint,
        })
    })
    .await
    .map_err(|_| "SSH Host Key 检测意外终止".to_string())?
}

#[tauri::command]
async fn create_ssh_route(
    client: tauri::State<'_, ControlClient>,
    input: CreateSSHRouteInput,
) -> Result<SSHRouteCreationResult, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || create_ssh_route_blocking(client, input))
        .await
        .map_err(|_| "SSH 路由创建意外终止".to_string())?
}

fn create_ssh_route_blocking(
    client: ControlClient,
    input: CreateSSHRouteInput,
) -> Result<SSHRouteCreationResult, String> {
    let CreateSSHRouteInput {
        name,
        alias,
        local_username,
        mut address,
        mut username,
        mut password,
        mut local_password,
        mut expected_host_key,
        egress,
        allowed_command,
        allow_all_commands,
        allow_all_confirmed,
        record_commands,
        authentication_timeout_seconds,
    } = input;
    let validation = validate_route_identity(&name, &alias, &egress)
        .and_then(|_| validate_ssh_local_username(&local_username))
        .and_then(|_| validate_ssh_command(&allowed_command, allow_all_commands))
        .and_then(|_| validate_ssh_upstream(&address, &username, &password))
        .and_then(|_| validate_ssh_authentication_timeout(authentication_timeout_seconds));
    if let Err(error) = validation {
        clear_ssh_credentials(
            &mut address,
            &mut username,
            &mut password,
            &mut local_password,
        );
        clear_string(&mut expected_host_key);
        return Err(error);
    }
    if expected_host_key.is_empty() || expected_host_key.len() > 16 << 10 {
        clear_ssh_credentials(
            &mut address,
            &mut username,
            &mut password,
            &mut local_password,
        );
        clear_string(&mut expected_host_key);
        return Err("请先检测并确认上游 SSH Host Key".to_string());
    }
    if !local_password.is_empty()
        && (local_password.len() < 12
            || local_password.len() > 1024
            || local_password.contains(['\0', '\r', '\n']))
    {
        clear_ssh_credentials(
            &mut address,
            &mut username,
            &mut password,
            &mut local_password,
        );
        clear_string(&mut expected_host_key);
        return Err("本地 SSH 密码需要 12 到 1024 个字节，且不能包含换行".to_string());
    }
    if allow_all_commands && !allow_all_confirmed {
        clear_ssh_credentials(
            &mut address,
            &mut username,
            &mut password,
            &mut local_password,
        );
        clear_string(&mut expected_host_key);
        return Err("请确认所有 SSH exec 命令的高风险权限".to_string());
    }
    let mut command = if allow_all_commands {
        "printf airlock-ok".to_string()
    } else {
        allowed_command
    };

    let create_request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "create_ssh_route",
        create: None,
        create_ssh: Some(CreateSSHRoute {
            name: name.trim(),
            alias: &alias,
            local_username: &local_username,
            address: &address,
            username: &username,
            password: &password,
            local_password: &local_password,
            expected_host_key: &expected_host_key,
            allowed_command: &command,
            allow_all_commands,
            record_commands,
            authentication_timeout_seconds,
            egress: &egress,
        }),
        probe_ssh: None,
        ssh_policy: None,
        alias: None,
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    let create_result = client.request(&create_request);
    clear_string(&mut expected_host_key);
    clear_string(&mut address);
    clear_string(&mut username);
    clear_string(&mut password);
    let custom_local_password = !local_password.is_empty();
    let mut created = match create_result {
        Ok(response) => match response.created {
            Some(created) => created,
            None => {
                clear_string(&mut local_password);
                clear_string(&mut command);
                return Err("airlockd 未返回新 SSH 路由".to_string());
            }
        },
        Err(error) => {
            clear_string(&mut local_password);
            clear_string(&mut command);
            return Err(error);
        }
    };
    if custom_local_password == !created.capability.is_empty() {
        clear_string(&mut local_password);
        clear_string(&mut created.capability);
        clear_string(&mut command);
        return Err("airlockd 返回了不一致的本地 SSH 凭据".to_string());
    }
    clear_string(&mut command);
    let local_credential = if custom_local_password {
        clear_string(&mut local_password);
        String::new()
    } else {
        std::mem::take(&mut created.capability)
    };
    Ok(SSHRouteCreationResult {
        route: created.route,
        local_credential,
        generated_credential: !custom_local_password,
    })
}

fn validate_route_identity(name: &str, alias: &str, egress: &str) -> Result<(), String> {
    if name.trim().is_empty() || name.len() > 80 {
        return Err("请输入 1 到 80 个字符的路由名称".to_string());
    }
    if alias.is_empty()
        || alias.len() > 63
        || !alias.bytes().enumerate().all(|(index, byte)| {
            byte.is_ascii_lowercase() || byte.is_ascii_digit() || (byte == b'-' && index > 0)
        })
    {
        return Err("别名只能包含小写字母、数字和连字符".to_string());
    }
    validate_egress(egress)
}

fn validate_ssh_authentication_timeout(seconds: u32) -> Result<(), String> {
    if !(3..=120).contains(&seconds) {
        return Err("SSH 认证预算需要在 3 到 120 秒之间".to_string());
    }
    Ok(())
}

fn validate_egress(egress: &str) -> Result<(), String> {
    if matches!(egress, "Direct" | "Proxy" | "Auto") {
        Ok(())
    } else {
        Err("出口策略无效".to_string())
    }
}

fn validate_ssh_upstream(address: &str, username: &str, password: &str) -> Result<(), String> {
    if address.trim().is_empty()
        || address.len() > 512
        || address.chars().any(char::is_whitespace)
        || address.contains('\0')
    {
        return Err("请输入有效的上游 SSH 地址".to_string());
    }
    if username.is_empty() || username.len() > 255 || username.contains(['\0', '\r', '\n']) {
        return Err("请输入有效的上游 SSH 用户名".to_string());
    }
    if password.is_empty() || password.len() > 8 << 10 || password.contains('\0') {
        return Err("请输入有效的上游 SSH 密码".to_string());
    }
    Ok(())
}

fn validate_ssh_command(command: &str, allow_all_commands: bool) -> Result<(), String> {
    if allow_all_commands {
        return Ok(());
    }
    if command.trim().is_empty() || command.len() > 4096 || command.contains(['\0', '\r', '\n']) {
        return Err("允许命令必须是 1 到 4096 个字节的单行完整命令".to_string());
    }
    Ok(())
}

fn validate_ssh_local_username(username: &str) -> Result<(), String> {
    if username.is_empty()
        || username.len() > 64
        || !username.bytes().enumerate().all(|(index, byte)| {
            byte.is_ascii_lowercase()
                || byte.is_ascii_digit()
                || (index > 0 && matches!(byte, b'.' | b'_' | b'-'))
        })
    {
        return Err("本地 SSH 用户名只能包含小写字母、数字、点、下划线和连字符".to_string());
    }
    Ok(())
}

fn delete_route_request(alias: &str) -> ControlRequest<'_> {
    ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "delete_route",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(alias),
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    }
}

fn clear_ssh_credentials(
    address: &mut String,
    username: &mut String,
    password: &mut String,
    local_password: &mut String,
) {
    clear_string(address);
    clear_string(username);
    clear_string(password);
    clear_string(local_password);
}

#[tauri::command]
#[allow(clippy::too_many_arguments)]
async fn set_ssh_policy(
    client: tauri::State<'_, ControlClient>,
    alias: String,
    name: String,
    local_username: String,
    allowed_command: String,
    allow_all_commands: bool,
    record_commands: bool,
    authentication_timeout_seconds: u32,
    egress: String,
) -> Result<Vec<RouteSummary>, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        set_ssh_policy_blocking(
            client,
            alias,
            name,
            local_username,
            allowed_command,
            allow_all_commands,
            record_commands,
            authentication_timeout_seconds,
            egress,
        )
    })
    .await
    .map_err(|_| "SSH 权限设置意外终止".to_string())?
}

#[allow(clippy::too_many_arguments)]
fn set_ssh_policy_blocking(
    client: ControlClient,
    alias: String,
    name: String,
    local_username: String,
    allowed_command: String,
    allow_all_commands: bool,
    record_commands: bool,
    authentication_timeout_seconds: u32,
    egress: String,
) -> Result<Vec<RouteSummary>, String> {
    if name.trim().is_empty() || name.len() > 80 {
        return Err("请输入有效的 SSH 映射名称".to_string());
    }
    validate_ssh_local_username(&local_username)?;
    validate_ssh_command(&allowed_command, allow_all_commands)?;
    validate_ssh_authentication_timeout(authentication_timeout_seconds)?;
    validate_egress(&egress)?;
    let mut command = if allow_all_commands {
        confirm_allow_all_commands(&alias)?;
        String::new()
    } else {
        allowed_command
    };
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "set_ssh_policy",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: Some(SSHPolicyUpdate {
            name: name.trim(),
            local_username: &local_username,
            allowed_command: &command,
            allow_all_commands,
            record_commands,
            authentication_timeout_seconds,
            egress: &egress,
        }),
        alias: Some(&alias),
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    let result = client.request(&request).map(|response| response.routes);
    clear_string(&mut command);
    result
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct UpdateSSHHostInput {
    alias: String,
    address: String,
    username: String,
    password: String,
    expected_host_key: String,
    egress: String,
}

#[tauri::command]
async fn update_ssh_target(
    client: tauri::State<'_, ControlClient>,
    input: UpdateSSHHostInput,
) -> Result<Vec<RouteSummary>, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || update_ssh_target_blocking(client, input))
        .await
        .map_err(|_| "SSH 宿主机更新意外终止".to_string())?
}

fn update_ssh_target_blocking(
    client: ControlClient,
    input: UpdateSSHHostInput,
) -> Result<Vec<RouteSummary>, String> {
    let UpdateSSHHostInput {
        alias,
        mut address,
        mut username,
        mut password,
        mut expected_host_key,
        egress,
    } = input;
    if let Err(error) =
        validate_ssh_upstream(&address, &username, &password).and_then(|_| validate_egress(&egress))
    {
        clear_string(&mut address);
        clear_string(&mut username);
        clear_string(&mut password);
        clear_string(&mut expected_host_key);
        return Err(error);
    }
    if expected_host_key.is_empty() || expected_host_key.len() > 16 << 10 {
        clear_string(&mut address);
        clear_string(&mut username);
        clear_string(&mut password);
        clear_string(&mut expected_host_key);
        return Err("请先检测并确认新的 SSH Host Key".to_string());
    }
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "update_ssh_target",
        create: None,
        create_ssh: Some(CreateSSHRoute {
            name: "",
            alias: "",
            local_username: "",
            address: &address,
            username: &username,
            password: &password,
            local_password: "",
            expected_host_key: &expected_host_key,
            allowed_command: "",
            allow_all_commands: false,
            record_commands: false,
            authentication_timeout_seconds: 0,
            egress: &egress,
        }),
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(&alias),
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    let result = client.request(&request).map(|response| response.routes);
    clear_string(&mut address);
    clear_string(&mut username);
    clear_string(&mut password);
    clear_string(&mut expected_host_key);
    result
}

#[tauri::command]
async fn rotate_ssh_credential(
    client: tauri::State<'_, ControlClient>,
    alias: String,
    mut local_password: String,
) -> Result<SSHRouteCreationResult, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        if !local_password.is_empty()
            && (local_password.len() < 12
                || local_password.len() > 1024
                || local_password.contains(['\0', '\r', '\n']))
        {
            clear_string(&mut local_password);
            return Err("本地 SSH 密码需要 12 到 1024 个字节，且不能包含换行".to_string());
        }
        let custom = !local_password.is_empty();
        let request = ControlRequest {
            version: CONTROL_PROTOCOL_VERSION,
            token: "",
            action: "rotate_ssh_credential",
            create: None,
            create_ssh: Some(CreateSSHRoute {
                name: "",
                alias: "",
                local_username: "",
                address: "",
                username: "",
                password: "",
                local_password: &local_password,
                expected_host_key: "",
                allowed_command: "",
                allow_all_commands: false,
                record_commands: false,
                authentication_timeout_seconds: 0,
                egress: "",
            }),
            probe_ssh: None,
            ssh_policy: None,
            alias: Some(&alias),
            enabled: None,
            proxy_url: None,
            capability: None,
            command: None,
            secret_store_mode: None,
        };
        let result = client.request(&request);
        clear_string(&mut local_password);
        let mut created = result?
            .created
            .ok_or_else(|| "airlockd 未返回轮换后的 SSH 凭据".to_string())?;
        if custom == !created.capability.is_empty() {
            clear_string(&mut created.capability);
            return Err("airlockd 返回了不一致的 SSH 凭据".to_string());
        }
        Ok(SSHRouteCreationResult {
            route: created.route,
            local_credential: std::mem::take(&mut created.capability),
            generated_credential: !custom,
        })
    })
    .await
    .map_err(|_| "SSH 本地凭据轮换意外终止".to_string())?
}

#[tauri::command]
fn test_proxy_health(client: tauri::State<'_, ControlClient>) -> Result<ControlUpdate, String> {
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "test_proxy_health",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: None,
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    let response = client.request(&request)?;
    let check = response
        .health_check
        .ok_or_else(|| "airlockd 未返回代理健康检查结果".to_string())?;
    Ok(ControlUpdate {
        routes: response.routes,
        message: Some(format!(
            "{} · {} · {}",
            check.message, check.latency, check.checked_at
        )),
    })
}

#[tauri::command]
async fn set_llm_policy(
    client: tauri::State<'_, ControlClient>,
    alias: String,
    models: Vec<String>,
    max_output_tokens: u32,
    requests_per_minute: u32,
    max_concurrent: u32,
    track_usage: bool,
) -> Result<Vec<RouteSummary>, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        set_llm_policy_blocking(
            client,
            alias,
            models,
            max_output_tokens,
            requests_per_minute,
            max_concurrent,
            track_usage,
        )
    })
    .await
    .map_err(|_| "LLM 策略设置意外终止".to_string())?
}

fn set_llm_policy_blocking(
    client: ControlClient,
    alias: String,
    models: Vec<String>,
    max_output_tokens: u32,
    requests_per_minute: u32,
    max_concurrent: u32,
    track_usage: bool,
) -> Result<Vec<RouteSummary>, String> {
    if models.is_empty()
        || models.len() > 32
        || models.iter().any(|model| {
            model.trim() != model
                || model.is_empty()
                || model.len() > 200
                || model.contains(['\0', '\r', '\n', '\t'])
        })
        || max_output_tokens == 0
        || max_output_tokens > 1_000_000
        || requests_per_minute == 0
        || requests_per_minute > 60_000
        || max_concurrent == 0
        || max_concurrent > 1_024
    {
        return Err("请设置有效的模型白名单、输出、速率与并发上限".to_string());
    }
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "set_llm_policy",
        create: Some(CreateHTTPRoute {
            name: "",
            alias: "",
            base_url: "",
            authorization: "",
            egress: "",
            provider: "",
            models: &models,
            max_output_tokens,
            requests_per_minute,
            max_concurrent,
            track_usage,
            local_api_key: "",
        }),
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(&alias),
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    client.request(&request).map(|response| response.routes)
}

#[tauri::command]
async fn rotate_llm_api_key(
    client: tauri::State<'_, ControlClient>,
    alias: String,
) -> Result<RouteSummary, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || rotate_llm_api_key_blocking(client, alias))
        .await
        .map_err(|_| "LLM 二次 API Key 轮换意外终止".to_string())?
}

fn rotate_llm_api_key_blocking(
    client: ControlClient,
    alias: String,
) -> Result<RouteSummary, String> {
    let (mut local_api_key, custom_local_key) = prompt_llm_local_api_key()?;
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "rotate_llm_api_key",
        create: Some(CreateHTTPRoute {
            name: "",
            alias: "",
            base_url: "",
            authorization: "",
            egress: "",
            provider: "",
            models: &[],
            max_output_tokens: 0,
            requests_per_minute: 0,
            max_concurrent: 0,
            track_usage: false,
            local_api_key: &local_api_key,
        }),
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(&alias),
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    let result = client.request(&request);
    clear_string(&mut local_api_key);
    let mut created = result?
        .created
        .ok_or_else(|| "airlockd 未返回轮换后的 LLM 路由".to_string())?;
    if let Err(error) = present_llm_access(
        &created.route.provider,
        &created.route.local_endpoint,
        &created.capability,
        custom_local_key,
    ) {
        clear_string(&mut created.capability);
        return Err(format!("Key 已轮换，但连接信息未显示：{error}"));
    }
    clear_string(&mut created.capability);
    Ok(created.route)
}

#[tauri::command]
fn reset_llm_usage(
    client: tauri::State<'_, ControlClient>,
    alias: String,
) -> Result<Vec<RouteSummary>, String> {
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "reset_llm_usage",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(&alias),
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    client.request(&request).map(|response| response.routes)
}

#[tauri::command]
async fn test_route_health(
    client: tauri::State<'_, ControlClient>,
    alias: String,
    authentication_timeout_seconds: Option<u32>,
) -> Result<ControlUpdate, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        test_route_health_blocking(client, alias, authentication_timeout_seconds)
    })
    .await
    .map_err(|_| "上游健康检查意外终止".to_string())?
}

fn test_route_health_blocking(
    client: ControlClient,
    alias: String,
    authentication_timeout_seconds: Option<u32>,
) -> Result<ControlUpdate, String> {
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "test_route_health",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(&alias),
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    let seconds = authentication_timeout_seconds.unwrap_or(20).clamp(3, 120);
    let response =
        client.request_with_timeout(&request, Duration::from_secs(u64::from(seconds) + 5))?;
    let check = response
        .health_check
        .ok_or_else(|| "airlockd 未返回健康检查结果".to_string())?;
    if check.alias != alias || (check.status != "healthy" && check.status != "degraded") {
        return Err("airlockd 返回了无效健康检查结果".to_string());
    }
    let state = if check.status == "healthy" {
        "健康"
    } else {
        "需要检查"
    };
    Ok(ControlUpdate {
        routes: response.routes,
        message: Some(format!(
            "{state} · {} · {} · {}",
            check.message, check.latency, check.checked_at
        )),
    })
}

#[tauri::command]
async fn configure_proxy(client: tauri::State<'_, ControlClient>) -> Result<bool, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || configure_proxy_blocking(client))
        .await
        .map_err(|_| "原生代理录入意外终止".to_string())?
}

fn configure_proxy_blocking(client: ControlClient) -> Result<bool, String> {
    let mut proxy_url = prompt_protected_value(
        "输入 Clash 或其他本地代理 URL，例如 http://127.0.0.1:7890 或 socks5://127.0.0.1:7890。认证信息可写在 URL 中，内容仅进入当前选择的受保护凭据存储。",
        false,
    )?;
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "configure_proxy",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: None,
        enabled: None,
        proxy_url: Some(&proxy_url),
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    let result = client.request(&request);
    clear_string(&mut proxy_url);
    result.map(|response| response.proxy_configured)
}

#[tauri::command]
fn clear_proxy(client: tauri::State<'_, ControlClient>) -> Result<bool, String> {
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "clear_proxy",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: None,
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    };
    client
        .request(&request)
        .map(|response| response.proxy_configured)
}

#[cfg(target_os = "macos")]
fn prompt_protected_value(message: &str, optional: bool) -> Result<String, String> {
    prompt_native_value(message, optional, true, "")
}

#[cfg(target_os = "macos")]
fn prompt_native_value(
    message: &str,
    optional: bool,
    hidden: bool,
    default_value: &str,
) -> Result<String, String> {
    prompt_native_value_with_title("Airlock 安全录入", message, optional, hidden, default_value)
}

#[cfg(target_os = "macos")]
fn prompt_native_value_with_title(
    title: &str,
    message: &str,
    optional: bool,
    hidden: bool,
    default_value: &str,
) -> Result<String, String> {
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const raw = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const payload = JSON.parse(raw);
const app = Application.currentApplication();
app.includeStandardAdditions = true;
const options = {
  withTitle: payload.title,
  defaultAnswer: payload.defaultValue,
  hiddenAnswer: payload.hidden,
  buttons: payload.optional ? [payload.skip, payload.save] : [payload.cancel, payload.continue],
  defaultButton: payload.optional ? payload.save : payload.continue
};
if (!payload.optional) options.cancelButton = payload.cancel;
const result = app.displayDialog(payload.message, options);
payload.optional && result.buttonReturned !== payload.save ? '' : result.textReturned;
"#;
    let mut payload = serde_json::to_vec(&serde_json::json!({
        "title": native_text(title),
        "message": native_text(message),
        "optional": optional,
        "hidden": hidden,
        "defaultValue": default_value,
        "cancel": native_text("取消"),
        "continue": native_text("继续"),
        "skip": native_text("不设置"),
        "save": native_text("保存"),
    }))
    .map_err(|_| "无法打开 macOS 安全录入窗口".to_string())?;
    let mut child = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", SCRIPT])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .spawn()
        .map_err(|_| "无法打开 macOS 安全录入窗口".to_string())?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(&payload)
            .map_err(|_| "无法向安全录入窗口传递内容".to_string())?;
    }
    payload.fill(0);
    let output = child
        .wait_with_output()
        .map_err(|_| "macOS 安全录入窗口意外终止".to_string())?;
    if !output.status.success() {
        return Err("已取消安全录入".to_string());
    }
    let mut value =
        String::from_utf8(output.stdout).map_err(|_| "安全录入返回了无效内容".to_string())?;
    while value.ends_with(['\r', '\n']) {
        value.pop();
    }
    if !optional && value.is_empty() {
        return Err("安全录入内容不能为空".to_string());
    }
    Ok(value)
}

#[cfg(target_os = "macos")]
fn choose_llm_local_api_key_mode() -> Result<bool, String> {
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const raw = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const payload = JSON.parse(raw);
const app = Application.currentApplication();
app.includeStandardAdditions = true;
const result = app.displayDialog(payload.message, {
  withTitle: payload.title,
  buttons: [payload.cancel, payload.custom, payload.random],
  defaultButton: payload.random,
  cancelButton: payload.cancel
});
result.buttonReturned === payload.custom ? 'custom' : 'random';
"#;
    let mut payload = serde_json::to_vec(&serde_json::json!({
        "title": native_text("LLM 设置 3/3 · 二次 API Key"),
        "message": native_text("为调用者创建一把独立的二次 API Key。它只用于访问 Airlock，真实上游 Key 不会暴露。\n\n随机生成提供 256-bit 强度并仅显示一次；自定义 Key 会要求隐藏输入两次。"),
        "cancel": native_text("取消"),
        "custom": native_text("自定义 Key"),
        "random": native_text("随机生成（推荐）"),
    })).map_err(|_| "无法编码二次 API Key 选择窗口".to_string())?;
    let mut child = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", SCRIPT])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .spawn()
        .map_err(|_| "无法打开二次 API Key 选择窗口".to_string())?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(&payload)
            .map_err(|_| "无法展示二次 API Key 选择".to_string())?;
    }
    payload.fill(0);
    let output = child
        .wait_with_output()
        .map_err(|_| "二次 API Key 选择窗口意外终止".to_string())?;
    if !output.status.success() {
        return Err("已取消二次 API Key 设置".to_string());
    }
    let choice =
        String::from_utf8(output.stdout).map_err(|_| "二次 API Key 选择无效".to_string())?;
    Ok(choice.trim() == "custom")
}

#[cfg(target_os = "macos")]
fn prompt_llm_local_api_key() -> Result<(String, bool), String> {
    if !choose_llm_local_api_key_mode()? {
        return Ok((String::new(), false));
    }
    let mut local_api_key = prompt_native_value_with_title(
        "自定义二次 API Key",
        "输入 16 到 1024 字节且不含空白的本地 API Key。它只用于访问 Airlock，不会发送给上游。",
        false,
        true,
        "",
    )?;
    if local_api_key.len() < 16
        || local_api_key.len() > 1024
        || local_api_key
            .chars()
            .any(|character| character.is_whitespace() || character == '\0')
    {
        clear_string(&mut local_api_key);
        return Err("本地 API Key 需要 16 到 1024 个字节，且不能包含空白字符".to_string());
    }
    let mut confirmation = match prompt_native_value_with_title(
        "确认本地 API Key",
        "再次输入本地 API Key。Airlock 只保存摘要，之后无法回显。",
        false,
        true,
        "",
    ) {
        Ok(value) => value,
        Err(error) => {
            clear_string(&mut local_api_key);
            return Err(error);
        }
    };
    let matches = local_api_key == confirmation;
    clear_string(&mut confirmation);
    if !matches {
        clear_string(&mut local_api_key);
        return Err("两次输入的本地 API Key 不一致".to_string());
    }
    Ok((local_api_key, true))
}

#[cfg(windows)]
fn prompt_protected_value(message: &str, optional: bool) -> Result<String, String> {
    native_windows::prompt_protected_value(message, optional)
}

#[cfg(windows)]
fn prompt_native_value(
    message: &str,
    optional: bool,
    hidden: bool,
    default_value: &str,
) -> Result<String, String> {
    native_windows::prompt_native_value(message, optional, hidden, default_value)
}

#[cfg(windows)]
fn prompt_native_value_with_title(
    title: &str,
    message: &str,
    optional: bool,
    hidden: bool,
    default_value: &str,
) -> Result<String, String> {
    native_windows::prompt_native_value_with_title(
        title,
        message,
        optional,
        hidden,
        default_value,
    )
}

#[cfg(windows)]
fn choose_llm_local_api_key_mode() -> Result<bool, String> {
    native_windows::choose_llm_local_api_key_mode()
}

#[cfg(windows)]
fn prompt_llm_local_api_key() -> Result<(String, bool), String> {
    native_windows::prompt_llm_local_api_key()
}

#[cfg(not(any(target_os = "macos", windows)))]
fn prompt_protected_value(message: &str, optional: bool) -> Result<String, String> {
    native_linux::prompt_protected_value(message, optional)
}

#[cfg(not(any(target_os = "macos", windows)))]
fn prompt_native_value(
    message: &str,
    optional: bool,
    hidden: bool,
    default_value: &str,
) -> Result<String, String> {
    native_linux::prompt_native_value(message, optional, hidden, default_value)
}

#[cfg(not(any(target_os = "macos", windows)))]
fn prompt_native_value_with_title(
    title: &str,
    message: &str,
    optional: bool,
    hidden: bool,
    default_value: &str,
) -> Result<String, String> {
    native_linux::prompt_native_value_with_title(title, message, optional, hidden, default_value)
}

#[cfg(not(any(target_os = "macos", windows)))]
fn choose_llm_local_api_key_mode() -> Result<bool, String> {
    native_linux::choose_llm_local_api_key_mode()
}

#[cfg(not(any(target_os = "macos", windows)))]
fn prompt_llm_local_api_key() -> Result<(String, bool), String> {
    native_linux::prompt_llm_local_api_key()
}

#[cfg(target_os = "macos")]
fn confirm_allow_all_commands(alias: &str) -> Result<(), String> {
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const raw = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const payload = JSON.parse(raw);
const app = Application.currentApplication();
app.includeStandardAdditions = true;
app.displayDialog(payload.message.replace('{alias}', payload.alias), {
  withTitle: payload.title,
  buttons: [payload.cancel, payload.allow], defaultButton: payload.cancel, cancelButton: payload.cancel,
  withIcon: 'caution'
});
true;
"#;
    let (message, title, allow) = match ui_locale() {
        "en" => ("Route “{alias}” will allow callers to run any non-interactive exec command.\n\nShell, PTY, SFTP, and port forwarding remain denied, but commands may read or modify anything available to the upstream account. Use a dedicated least-privilege account.", "High-risk SSH permissions", "Allow all exec"),
        "ja" => ("ルート「{alias}」は、呼び出し元に任意の非対話 exec コマンドを許可します。\n\nShell、PTY、SFTP、ポート転送は引き続き拒否されますが、コマンドは上流アカウントがアクセスできるデータを読み取りまたは変更できます。専用の最小権限アカウントを使用してください。", "高リスク SSH 権限", "すべての exec を許可"),
        _ => ("路由 “{alias}” 将允许调用者执行任意非交互 exec 命令。\n\nShell、PTY、SFTP 与端口转发仍会被拒绝，但上游账号能访问的数据和操作都可能被命令读取或修改。请仅配合低权限专用账号使用。", "高风险 SSH 权限", "允许所有 exec"),
    };
    let mut payload = serde_json::to_vec(&serde_json::json!({ "alias": alias, "message": message, "title": title, "cancel": native_text("取消"), "allow": allow })).map_err(|_| "无法编码 SSH 风险确认".to_string())?;
    let mut child = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", SCRIPT])
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .spawn()
        .map_err(|_| "无法打开 SSH 风险确认窗口".to_string())?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(&payload)
            .map_err(|_| "无法展示 SSH 风险确认".to_string())?;
    }
    payload.fill(0);
    if !child
        .wait()
        .map_err(|_| "SSH 风险确认窗口意外终止".to_string())?
        .success()
    {
        return Err("已取消允许所有 SSH 命令".to_string());
    }
    Ok(())
}

#[cfg(windows)]
fn confirm_allow_all_commands(alias: &str) -> Result<(), String> {
    let (message, title, _allow) = match ui_locale() {
        "en" => ("Route “{alias}” will allow callers to run any non-interactive exec command.\n\nShell, PTY, SFTP, and port forwarding remain denied, but commands may read or modify anything available to the upstream account. Use a dedicated least-privilege account.", "High-risk SSH permissions", "Allow all exec"),
        "ja" => ("ルート「{alias}」は、呼び出し元に任意の非対話 exec コマンドを許可します。\n\nShell、PTY、SFTP、ポート転送は引き続き拒否されますが、コマンドは上流アカウントがアクセスできるデータを読み取りまたは変更できます。専用の最小権限アカウントを使用してください。", "高リスク SSH 権限", "すべての exec を許可"),
        _ => ("路由 “{alias}” 将允许调用者执行任意非交互 exec 命令。\n\nShell、PTY、SFTP 与端口转发仍会被拒绝，但上游账号能访问的数据和操作都可能被命令读取或修改。请仅配合低权限专用账号使用。", "高风险 SSH 权限", "允许所有 exec"),
    };
    native_windows::confirm_yes_no(title, &message.replace("{alias}", alias))
}

#[cfg(all(not(target_os = "macos"), not(windows)))]
fn confirm_allow_all_commands(alias: &str) -> Result<(), String> {
    let (message, title, _allow) = match ui_locale() {
        "en" => ("Route “{alias}” will allow callers to run any non-interactive exec command.\n\nShell, PTY, SFTP, and port forwarding remain denied, but commands may read or modify anything available to the upstream account. Use a dedicated least-privilege account.", "High-risk SSH permissions", "Allow all exec"),
        "ja" => ("ルート「{alias}」は、呼び出し元に任意の非対話 exec コマンドを許可します。\n\nShell、PTY、SFTP、ポート転送は引き続き拒否されますが、コマンドは上流アカウントがアクセスできるデータを読み取りまたは変更できます。専用の最小権限アカウントを使用してください。", "高リスク SSH 権限", "すべての exec を許可"),
        _ => ("路由 “{alias}” 将允许调用者执行任意非交互 exec 命令。\n\nShell、PTY、SFTP 与端口转发仍会被拒绝，但上游账号能访问的数据和操作都可能被命令读取或修改。请仅配合低权限专用账号使用。", "高风险 SSH 权限", "允许所有 exec"),
    };
    native_linux::confirm_yes_no(title, &message.replace("{alias}", alias))
}

#[cfg(target_os = "macos")]
fn present_capability(endpoint: &str, capability: &str) -> Result<(), String> {
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const raw = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const payload = JSON.parse(raw);
const app = Application.currentApplication();
app.includeStandardAdditions = true;
app.displayDialog(payload.message, {
  withTitle: payload.title,
  defaultAnswer: payload.endpoint + '\n' + payload.capability,
  buttons: [payload.done], defaultButton: payload.done
});
true;
"#;
    let mut payload = serde_json::to_vec(&serde_json::json!({
        "endpoint": endpoint,
        "capability": capability,
        "message": native_text("路由已安全保存。Capability 仅显示这一次，请交给需要访问该路由的客户端。"),
        "title": native_text("Airlock 路由已创建"),
        "done": native_text("完成"),
    }))
    .map_err(|_| "无法展示一次性 Capability".to_string())?;
    let mut child = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", SCRIPT])
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .spawn()
        .map_err(|_| "无法打开 Capability 窗口".to_string())?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(&payload)
            .map_err(|_| "无法向 Capability 窗口传递内容".to_string())?;
    }
    payload.fill(0);
    let status = child
        .wait()
        .map_err(|_| "Capability 窗口意外终止".to_string())?;
    if !status.success() {
        return Err("Capability 未确认，路由已安全保存但保持停用".to_string());
    }
    Ok(())
}

#[cfg(target_os = "macos")]
fn present_llm_access(
    provider: &str,
    endpoint: &str,
    capability: &str,
    custom_local_key: bool,
) -> Result<(), String> {
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const raw = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const payload = JSON.parse(raw);
const app = Application.currentApplication();
app.includeStandardAdditions = true;
const openai = payload.provider === 'openai';
const prefix = openai ? 'OPENAI' : 'ANTHROPIC';
const baseURL = openai ? payload.endpoint.replace(/\/$/, '') + '/v1' : payload.endpoint;
const apiKey = payload.customLocalKey ? payload.customPlaceholder : payload.capability;
const details = prefix + '_BASE_URL=' + baseURL + '\n' + prefix + '_API_KEY=' + apiKey;
const message = payload.customLocalKey ? payload.customMessage : payload.randomMessage;
app.displayDialog(message, {
  withTitle: payload.title,
  defaultAnswer: details,
  buttons: [payload.done], defaultButton: payload.done
});
true;
"#;
    let (custom_message, random_message, custom_placeholder) = match ui_locale() {
        "en" => ("The LLM route is enabled. Airlock will not reveal the custom local API key.", "The LLM route is enabled. The random local API key is shown only once.", "<use the local API key set earlier>"),
        "ja" => ("LLM ルートが有効になりました。Airlock はカスタムのローカル API Key を再表示しません。", "LLM ルートが有効になりました。ランダム生成されたローカル API Key は一度だけ表示されます。", "<先ほど設定したローカル API Key を使用>"),
        _ => ("LLM 路由已启用。Airlock 不会回显自定义的本地 API Key。", "LLM 路由已启用。随机生成的本地 API Key 仅显示这一次。", "<使用刚才设置的本地 API Key>"),
    };
    let mut payload = serde_json::to_vec(&serde_json::json!({
        "provider": provider,
        "endpoint": endpoint,
        "capability": capability,
        "customLocalKey": custom_local_key,
        "customMessage": custom_message,
        "randomMessage": random_message,
        "customPlaceholder": custom_placeholder,
        "title": native_text("Airlock LLM 路由已创建"),
        "done": native_text("完成"),
    }))
    .map_err(|_| "无法展示 LLM 连接信息".to_string())?;
    let mut child = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", SCRIPT])
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .spawn()
        .map_err(|_| "无法打开 LLM 连接信息窗口".to_string())?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(&payload)
            .map_err(|_| "无法传递 LLM 连接信息".to_string())?;
    }
    payload.fill(0);
    if !child
        .wait()
        .map_err(|_| "LLM 连接信息窗口意外终止".to_string())?
        .success()
    {
        return Err("LLM 连接信息未确认，路由已保存但保持停用".to_string());
    }
    Ok(())
}

#[cfg(windows)]
fn present_capability(endpoint: &str, capability: &str) -> Result<(), String> {
    native_windows::present_text(
        &native_text("Airlock 路由已创建"),
        &native_text("路由已安全保存。Capability 仅显示这一次，请交给需要访问该路由的客户端。"),
        &format!("{endpoint}\n{capability}"),
    )
}

#[cfg(windows)]
fn present_llm_access(
    provider: &str,
    endpoint: &str,
    capability: &str,
    custom_local_key: bool,
) -> Result<(), String> {
    let (custom_message, random_message, custom_placeholder) = match ui_locale() {
        "en" => (
            "The LLM route is enabled. Airlock will not reveal the custom local API key.",
            "The LLM route is enabled. The random local API key is shown only once.",
            "<use the local API key set earlier>",
        ),
        "ja" => (
            "LLM ルートが有効になりました。Airlock はカスタムのローカル API Key を再表示しません。",
            "LLM ルートが有効になりました。ランダム生成されたローカル API Key は一度だけ表示されます。",
            "<先ほど設定したローカル API Key を使用>",
        ),
        _ => (
            "LLM 路由已启用。Airlock 不会回显自定义的本地 API Key。",
            "LLM 路由已启用。随机生成的本地 API Key 仅显示这一次。",
            "<使用刚才设置的本地 API Key>",
        ),
    };
    let openai = provider == "openai";
    let prefix = if openai { "OPENAI" } else { "ANTHROPIC" };
    let base_url = if openai {
        format!("{}/v1", endpoint.trim_end_matches('/'))
    } else {
        endpoint.to_string()
    };
    let api_key = if custom_local_key {
        custom_placeholder
    } else {
        capability
    };
    let details = format!("{prefix}_BASE_URL={base_url}\n{prefix}_API_KEY={api_key}");
    let message = if custom_local_key {
        custom_message
    } else {
        random_message
    };
    native_windows::present_text(&native_text("Airlock LLM 路由已创建"), message, &details)
}

#[cfg(all(not(target_os = "macos"), not(windows)))]
fn present_capability(endpoint: &str, capability: &str) -> Result<(), String> {
    native_linux::present_text(
        &native_text("Airlock 路由已创建"),
        &native_text("路由已安全保存。Capability 仅显示这一次，请交给需要访问该路由的客户端。"),
        &format!("{endpoint}\n{capability}"),
    )
}

#[cfg(all(not(target_os = "macos"), not(windows)))]
fn present_llm_access(
    provider: &str,
    endpoint: &str,
    capability: &str,
    custom_local_key: bool,
) -> Result<(), String> {
    let (custom_message, random_message, custom_placeholder) = match ui_locale() {
        "en" => (
            "The LLM route is enabled. Airlock will not reveal the custom local API key.",
            "The LLM route is enabled. The random local API key is shown only once.",
            "<use the local API key set earlier>",
        ),
        "ja" => (
            "LLM ルートが有効になりました。Airlock はカスタムのローカル API Key を再表示しません。",
            "LLM ルートが有効になりました。ランダム生成されたローカル API Key は一度だけ表示されます。",
            "<先ほど設定したローカル API Key を使用>",
        ),
        _ => (
            "LLM 路由已启用。Airlock 不会回显自定义的本地 API Key。",
            "LLM 路由已启用。随机生成的本地 API Key 仅显示这一次。",
            "<使用刚才设置的本地 API Key>",
        ),
    };
    let openai = provider == "openai";
    let prefix = if openai { "OPENAI" } else { "ANTHROPIC" };
    let base_url = if openai {
        format!("{}/v1", endpoint.trim_end_matches('/'))
    } else {
        endpoint.to_string()
    };
    let api_key = if custom_local_key {
        custom_placeholder
    } else {
        capability
    };
    let details = format!("{prefix}_BASE_URL={base_url}\n{prefix}_API_KEY={api_key}");
    let message = if custom_local_key {
        custom_message
    } else {
        random_message
    };
    native_linux::present_text(&native_text("Airlock LLM 路由已创建"), message, &details)
}

fn clear_string(value: &mut String) {
    unsafe { value.as_bytes_mut().fill(0) };
    value.clear();
}

fn generate_control_token() -> Result<Arc<SecretToken>, String> {
    let mut random = [0_u8; 32];
    getrandom::getrandom(&mut random).map_err(|_| "无法生成本地控制令牌".to_string())?;
    const HEX: &[u8; 16] = b"0123456789abcdef";
    let mut token = String::with_capacity(80);
    token.push_str("airlock_control_");
    for byte in &random {
        token.push(HEX[(byte >> 4) as usize] as char);
        token.push(HEX[(byte & 0x0f) as usize] as char);
    }
    random.fill(0);
    Ok(Arc::new(SecretToken(token)))
}

fn validate_security_settings(settings: &SecuritySettings) -> Result<(), String> {
    if settings.version != SECURITY_SETTINGS_VERSION
        || (settings.network_scope != "loopback" && settings.network_scope != "lan")
        || (settings.secret_store != "keychain" && settings.secret_store != "local_file")
        || settings.http_port < MIN_UNPRIVILEGED_PORT
        || settings.ssh_port < MIN_UNPRIVILEGED_PORT
        || settings.http_port == settings.ssh_port
    {
        return Err("安全设置无效".to_string());
    }
    Ok(())
}

fn listener_host(settings: &SecuritySettings) -> &'static str {
    if settings.network_scope == "lan" {
        "0.0.0.0"
    } else {
        "127.0.0.1"
    }
}

fn listener_address(settings: &SecuritySettings, port: u16) -> String {
    format!("{}:{port}", listener_host(settings))
}

fn configured_listener_description(settings: &SecuritySettings) -> String {
    format!(
        "{} 或 {}",
        listener_address(settings, settings.http_port),
        listener_address(settings, settings.ssh_port)
    )
}

fn is_configured_listener_port(settings: &SecuritySettings, port: u16) -> bool {
    port == settings.http_port || port == settings.ssh_port
}

fn load_security_settings(path: &PathBuf) -> Result<SecuritySettings, String> {
    let parent = path
        .parent()
        .ok_or_else(|| "安全设置路径无效".to_string())?;
    platform::protect_directory(parent)
        .map_err(|_| "无法创建或保护安全设置目录".to_string())?;
    let metadata = match std::fs::symlink_metadata(path) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            let settings = SecuritySettings::default();
            save_security_settings(path, &settings)?;
            return Ok(settings);
        }
        Err(_) => return Err("无法读取安全设置".to_string()),
    };
    if !metadata.is_file() || metadata.file_type().is_symlink() || metadata.len() > 4096 {
        return Err("安全设置文件权限或类型无效".to_string());
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;

        if metadata.permissions().mode() & 0o077 != 0 {
            return Err("安全设置文件权限或类型无效".to_string());
        }
    }
    let raw = std::fs::read(path).map_err(|_| "无法读取安全设置".to_string())?;
    let settings = serde_json::from_slice::<SecuritySettings>(&raw)
        .map_err(|_| "安全设置格式无效".to_string())?;
    validate_security_settings(&settings)?;
    Ok(settings)
}

fn save_security_settings(path: &PathBuf, settings: &SecuritySettings) -> Result<(), String> {
    validate_security_settings(settings)?;
    let parent = path
        .parent()
        .ok_or_else(|| "安全设置路径无效".to_string())?;
    platform::protect_directory(parent)
        .map_err(|_| "无法创建或保护安全设置目录".to_string())?;
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| "无法生成安全设置临时路径".to_string())?
        .as_nanos();
    let temporary = parent.join(format!(
        ".security-settings-{}-{nonce}.tmp",
        std::process::id()
    ));
    let mut file = platform::open_private_file(&temporary, true, false)
        .map_err(|_| "无法创建安全设置临时文件".to_string())?;
    let result = (|| {
        let payload =
            serde_json::to_vec_pretty(settings).map_err(|_| "无法编码安全设置".to_string())?;
        file.write_all(&payload)
            .map_err(|_| "无法写入安全设置".to_string())?;
        file.sync_all()
            .map_err(|_| "无法同步安全设置".to_string())?;
        drop(file);
        platform::replace_file(&temporary, path)
            .map_err(|_| "无法安装安全设置".to_string())?;
        Ok(())
    })();
    if result.is_err() {
        let _ = std::fs::remove_file(&temporary);
    }
    result
}

fn status_request<'a>() -> ControlRequest<'a> {
    ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "status",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: None,
        enabled: None,
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    }
}

fn secret_store_request<'a>(action: &'a str, mode: &'a str) -> ControlRequest<'a> {
    let mut request = status_request();
    request.action = action;
    request.secret_store_mode = Some(mode);
    request
}

fn wait_for_control(client: &ControlClient) -> bool {
    for _ in 0..40 {
        if client
            .request_with_timeout(&status_request(), CONTROL_STARTUP_PROBE_TIMEOUT)
            .is_ok()
        {
            return true;
        }
        std::thread::sleep(Duration::from_millis(100));
    }
    false
}

#[cfg(target_os = "macos")]
fn confirm_security_change(
    current: &SecuritySettings,
    next: &SecuritySettings,
) -> Result<(), String> {
    let mut risks: Vec<Cow<'_, str>> = Vec::new();
    if current.network_scope != "lan" && next.network_scope == "lan" {
        risks.push(native_text(
            "局域网设备将能连接 Airlock 的 HTTP/SSH 入口，仍需要每条路由的凭据。",
        ));
    }
    if current.secret_store != "local_file" && next.secret_store == "local_file" {
        risks.push(native_text(
            "上游地址和凭据将保存在仅当前用户可读的 0600 文件中，不再由 macOS Keychain 加密保护。",
        ));
    }
    if risks.is_empty() {
        return Ok(());
    }
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const raw = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const payload = JSON.parse(raw);
const app = Application.currentApplication();
app.includeStandardAdditions = true;
app.displayDialog(payload.message + '\n\n' + payload.restart, {
  withTitle: payload.title,
  buttons: [payload.cancel, payload.apply], defaultButton: payload.apply, cancelButton: payload.cancel,
  withIcon: 'caution'
});
true;
"#;
    let mut payload = serde_json::to_vec(&serde_json::json!({
        "message": risks.iter().map(Cow::as_ref).collect::<Vec<_>>().join("\n\n"),
        "restart": native_text("应用设置会短暂重启本地转发核心。"),
        "title": native_text("Airlock 安全设置"),
        "cancel": native_text("取消"),
        "apply": native_text("应用并重启"),
    }))
    .map_err(|_| "无法编码安全设置确认".to_string())?;
    let mut child = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", SCRIPT])
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .spawn()
        .map_err(|_| "无法打开安全设置确认".to_string())?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(&payload)
            .map_err(|_| "无法展示安全设置风险".to_string())?;
    }
    payload.fill(0);
    if !child
        .wait()
        .map_err(|_| "安全设置确认意外终止".to_string())?
        .success()
    {
        return Err("已取消安全设置更改".to_string());
    }
    Ok(())
}

#[cfg(windows)]
fn confirm_security_change(
    current: &SecuritySettings,
    next: &SecuritySettings,
) -> Result<(), String> {
    let mut risks: Vec<Cow<'_, str>> = Vec::new();
    if current.network_scope != "lan" && next.network_scope == "lan" {
        risks.push(native_text(
            "局域网设备将能连接 Airlock 的 HTTP/SSH 入口，仍需要每条路由的凭据。",
        ));
    }
    if current.secret_store != "local_file" && next.secret_store == "local_file" {
        risks.push(native_text(
            "上游地址和凭据将保存在仅当前用户可读的受保护文件中，不再由系统凭据库加密保护。",
        ));
    }
    if risks.is_empty() {
        return Ok(());
    }
    let message = format!(
        "{}\n\n{}",
        risks.iter().map(Cow::as_ref).collect::<Vec<_>>().join("\n\n"),
        native_text("应用设置会短暂重启本地转发核心。")
    );
    native_windows::confirm_yes_no(&native_text("Airlock 安全设置"), &message)
}

#[cfg(all(not(target_os = "macos"), not(windows)))]
fn confirm_security_change(
    current: &SecuritySettings,
    next: &SecuritySettings,
) -> Result<(), String> {
    let mut risks: Vec<Cow<'_, str>> = Vec::new();
    if current.network_scope != "lan" && next.network_scope == "lan" {
        risks.push(native_text(
            "局域网设备将能连接 Airlock 的 HTTP/SSH 入口，仍需要每条路由的凭据。",
        ));
    }
    if current.secret_store != "local_file" && next.secret_store == "local_file" {
        risks.push(native_text(
            "上游地址和凭据将保存在仅当前用户可读的受保护文件中，不再由系统凭据库加密保护。",
        ));
    }
    if risks.is_empty() {
        return Ok(());
    }
    let message = format!(
        "{}\n\n{}",
        risks.iter().map(Cow::as_ref).collect::<Vec<_>>().join("\n\n"),
        native_text("应用设置会短暂重启本地转发核心。")
    );
    native_linux::confirm_yes_no(&native_text("Airlock 安全设置"), &message)
}

fn locate_sidecar(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    if let Some(path) = std::env::var_os("AIRLOCKD_BIN").map(PathBuf::from) {
        if path.is_file() {
            return Ok(path);
        }
    }
    let binary_name = platform::sidecar_binary_name();
    let triple_binary_name = platform::sidecar_bundle_name();
    let mut candidates = Vec::new();
    if let Ok(resource_dir) = app.path().resource_dir() {
        candidates.push(resource_dir.join(binary_name));
    }
    if let Ok(executable) = std::env::current_exe() {
        if let Some(directory) = executable.parent() {
            candidates.push(directory.join(binary_name));
        }
    }
    if let Some(triple_name) = triple_binary_name {
        candidates.push(
            PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                .join("binaries")
                .join(triple_name),
        );
    }
    candidates.push(PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("binaries").join(binary_name));
    candidates.push(PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../bin").join(binary_name));
    candidates
        .into_iter()
        .find(|path| path.is_file())
        .ok_or_else(|| "找不到 airlockd sidecar，请先运行 npm run sidecar:build".to_string())
}

fn create_sidecar_startup_log(app: &tauri::AppHandle) -> Result<(PathBuf, File), String> {
    let directory = app
        .path()
        .app_config_dir()
        .map_err(|_| "无法读取 Airlock 配置目录".to_string())?;
    platform::protect_directory(&directory)
        .map_err(|_| "无法创建或保护 Airlock 配置目录".to_string())?;
    let path = directory.join("airlockd-startup.log");
    let file = platform::open_private_file(&path, false, true)
        .map_err(|_| "无法创建本地核心启动日志".to_string())?;
    Ok((path, file))
}

fn sidecar_start_failure(log_path: Option<&Path>, settings: &SecuritySettings) -> String {
    let log = log_path
        .and_then(|path| File::open(path).ok())
        .and_then(|file| {
            let mut output = String::new();
            BufReader::new(file)
                .take(SIDECAR_STARTUP_LOG_MAX_BYTES)
                .read_to_string(&mut output)
                .ok()
                .map(|_| output)
        })
        .unwrap_or_default()
        .to_ascii_lowercase();
    if log.contains("address already in use") {
        format!(
            "本地核心未能启动：{} 已被其他程序占用。请释放端口、改用其他端口，或退出其他 Airlock 副本后重试。",
            configured_listener_description(settings)
        )
    } else if log.contains("control socket") || log.contains("control channel") {
        "本地核心未能启动：当前用户的受保护控制通道不可用。请退出其他 Airlock 副本后重试。"
            .to_string()
    } else {
        format!(
            "本地核心未能在 4 秒内准备就绪。请重试；若问题持续，请检查 {}。",
            configured_listener_description(settings)
        )
    }
}

fn spawn_sidecar(
    app: &tauri::AppHandle,
    token: &SecretToken,
    settings: &SecuritySettings,
) -> Result<(Child, PathBuf), String> {
    let binary = locate_sidecar(app)?;
    let (startup_log, stderr) = create_sidecar_startup_log(app)?;
    let http_address = listener_address(settings, settings.http_port);
    let ssh_address = listener_address(settings, settings.ssh_port);
    let mut command = Command::new(binary);
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;

        command.creation_flags(0x08000000); // CREATE_NO_WINDOW
    }
    command
        .arg("--control-token-stdin")
        .args(["--listen", &http_address])
        .args(["--ssh-listen", &ssh_address])
        .args(["--network-scope", &settings.network_scope])
        .args(["--secret-store", &settings.secret_store])
        .stderr(Stdio::from(stderr));
    let mut child = command
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .spawn()
        .map_err(|_| "无法启动 airlockd sidecar".to_string())?;
    let mut payload = token.0.as_bytes().to_vec();
    payload.push(b'\n');
    let write_result = child
        .stdin
        .take()
        .ok_or_else(|| "airlockd 控制管道不可用".to_string())?
        .write_all(&payload)
        .map_err(|_| "无法初始化 airlockd 控制通道".to_string());
    payload.fill(0);
    if let Err(error) = write_result {
        let _ = child.kill();
        let _ = child.wait();
        return Err(error);
    }
    Ok((child, startup_log))
}

fn start_configured_sidecar(
    app: &tauri::AppHandle,
    client: &ControlClient,
    process: &DaemonProcess,
    startup: &CoreStartupState,
    settings: &SecuritySettings,
) -> Result<(), String> {
    startup.set("正在启动本地核心");
    let (child, startup_log) = match spawn_sidecar(app, client.token.as_ref(), settings) {
        Ok(result) => result,
        Err(error) => {
            startup.set(error.clone());
            return Err(error);
        }
    };
    process.replace(child, startup_log);
    if wait_for_control(client) {
        startup.clear();
        return Ok(());
    }
    let error = sidecar_start_failure(process.startup_log().as_deref(), settings);
    process.stop();
    startup.set(error.clone());
    Err(error)
}

fn monitor_initial_sidecar(
    client: ControlClient,
    process: DaemonProcess,
    startup: CoreStartupState,
    settings: SecuritySettings,
) {
    std::thread::spawn(move || {
        if wait_for_control(&client) {
            startup.clear();
            return;
        }
        let error = sidecar_start_failure(process.startup_log().as_deref(), &settings);
        process.stop();
        startup.set(error);
    });
}

fn apply_security_settings_blocking(
    app: tauri::AppHandle,
    client: ControlClient,
    process: DaemonProcess,
    security: SecurityConfiguration,
    startup: CoreStartupState,
    network_scope: String,
    secret_store: String,
    http_port: u16,
    ssh_port: u16,
) -> Result<SecurityUpdate, String> {
    let next = SecuritySettings {
        version: SECURITY_SETTINGS_VERSION,
        network_scope,
        secret_store,
        http_port,
        ssh_port,
    };
    validate_security_settings(&next)?;
    let current = security
        .settings
        .lock()
        .map_err(|_| "无法读取当前安全设置".to_string())?
        .clone();
    if current == next {
        return Ok(SecurityUpdate {
            security_settings: current,
            message: None,
        });
    }
    confirm_security_change(&current, &next)?;
    let store_changed = current.secret_store != next.secret_store;
    if store_changed {
        client.request(&secret_store_request(
            "migrate_secret_store",
            &next.secret_store,
        ))?;
    }
    if let Err(error) = save_security_settings(&security.path, &next) {
        if store_changed {
            let _ = client.request(&secret_store_request(
                "cleanup_secret_store",
                &next.secret_store,
            ));
        }
        return Err(error);
    }

    process.stop();
    let restart_result = start_configured_sidecar(&app, &client, &process, &startup, &next);
    if let Err(error) = restart_result {
        let _ = save_security_settings(&security.path, &current);
        let rollback = start_configured_sidecar(&app, &client, &process, &startup, &current);
        if rollback.is_ok() && store_changed {
            let _ = client.request(&secret_store_request(
                "cleanup_secret_store",
                &next.secret_store,
            ));
        }
        return Err(if rollback.is_ok() {
            format!("{error}，已恢复上一个运行设置")
        } else {
            format!("{error}，且旧设置恢复失败，请重启 Airlock")
        });
    }
    if let Ok(mut settings) = security.settings.lock() {
        *settings = next.clone();
    } else {
        return Err("安全设置已应用，但桌面状态更新失败".to_string());
    }
    let message = if store_changed {
        match client.request(&secret_store_request(
            "cleanup_secret_store",
            &current.secret_store,
        )) {
            Ok(cleanup) if cleanup.warning.is_empty() => {
                Some("凭据已迁移，本地核心已重启".to_string())
            }
            Ok(cleanup) => Some(format!("凭据已迁移，本地核心已重启；{}", cleanup.warning)),
            Err(_) => {
                Some("凭据已迁移，本地核心已重启；旧凭据副本清理失败，需要手动处理".to_string())
            }
        }
    } else {
        Some("网络范围已更新，本地核心已重启".to_string())
    };
    Ok(SecurityUpdate {
        security_settings: next,
        message,
    })
}

#[tauri::command]
async fn apply_security_settings(
    app: tauri::AppHandle,
    client: tauri::State<'_, ControlClient>,
    process: tauri::State<'_, DaemonProcess>,
    security: tauri::State<'_, SecurityConfiguration>,
    startup: tauri::State<'_, CoreStartupState>,
    network_scope: String,
    secret_store: String,
    http_port: u16,
    ssh_port: u16,
) -> Result<SecurityUpdate, String> {
    let client = client.inner().clone();
    let process = process.inner().clone();
    let security = security.inner().clone();
    let startup = startup.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        apply_security_settings_blocking(
            app,
            client,
            process,
            security,
            startup,
            network_scope,
            secret_store,
            http_port,
            ssh_port,
        )
    })
    .await
    .map_err(|_| "安全设置更新意外终止".to_string())?
}

fn restart_local_core_blocking(
    app: tauri::AppHandle,
    client: ControlClient,
    process: DaemonProcess,
    security: SecurityConfiguration,
    startup: CoreStartupState,
) -> Result<String, String> {
    let settings = security
        .settings
        .lock()
        .map_err(|_| "无法读取本地核心设置".to_string())?
        .clone();
    process.stop();
    start_configured_sidecar(&app, &client, &process, &startup, &settings)?;
    Ok("本地核心已启动".to_string())
}

#[tauri::command]
async fn restart_local_core(
    app: tauri::AppHandle,
    client: tauri::State<'_, ControlClient>,
    process: tauri::State<'_, DaemonProcess>,
    security: tauri::State<'_, SecurityConfiguration>,
    startup: tauri::State<'_, CoreStartupState>,
) -> Result<String, String> {
    let client = client.inner().clone();
    let process = process.inner().clone();
    let security = security.inner().clone();
    let startup = startup.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        restart_local_core_blocking(app, client, process, security, startup)
    })
    .await
    .map_err(|_| "本地核心重启意外终止".to_string())?
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
struct PortOwner {
    port: u16,
    pid: u32,
    command: String,
}

#[cfg(unix)]
#[derive(Default)]
struct RawPortOwner {
    pid: Option<u32>,
    command: String,
    uid: Option<u32>,
}

#[cfg(unix)]
fn append_port_owner(
    owners: &mut Vec<PortOwner>,
    raw: &RawPortOwner,
    port: u16,
    current_uid: u32,
    managed_pid: Option<u32>,
) {
    let Some(pid) = raw.pid else {
        return;
    };
    if raw.uid != Some(current_uid) || Some(pid) == managed_pid {
        return;
    }
    let command = raw
        .command
        .chars()
        .filter(|character| !character.is_control())
        .take(96)
        .collect::<String>();
    owners.push(PortOwner {
        port,
        pid,
        command: if command.is_empty() {
            "unknown".to_string()
        } else {
            command
        },
    });
}

#[cfg(unix)]
fn listening_port_owners(port: u16, managed_pid: Option<u32>) -> Result<Vec<PortOwner>, String> {
    let selection = format!("-iTCP:{port}");
    let output = Command::new("/usr/sbin/lsof")
        .args(["-nP", "-l", "-a", &selection, "-sTCP:LISTEN", "-Fpcu"])
        .output()
        .map_err(|_| "无法读取端口占用情况".to_string())?;
    if output.stdout.len() > 32 << 10 {
        return Err("端口占用信息异常".to_string());
    }
    if !output.status.success() && output.stdout.is_empty() {
        return Ok(Vec::new());
    }

    let current_uid = unsafe { libc::geteuid() } as u32;
    let mut owners = Vec::new();
    let mut raw = RawPortOwner::default();
    for line in String::from_utf8_lossy(&output.stdout).lines() {
        let Some((field, value)) = line.split_at_checked(1) else {
            continue;
        };
        match field {
            "p" => {
                append_port_owner(&mut owners, &raw, port, current_uid, managed_pid);
                raw = RawPortOwner {
                    pid: value.parse::<u32>().ok(),
                    ..RawPortOwner::default()
                };
            }
            "c" => raw.command = value.to_string(),
            "u" => raw.uid = value.parse::<u32>().ok(),
            _ => {}
        }
    }
    append_port_owner(&mut owners, &raw, port, current_uid, managed_pid);
    owners.sort_by_key(|owner| owner.pid);
    owners.dedup_by_key(|owner| owner.pid);
    Ok(owners)
}

#[cfg(windows)]
fn listening_port_owners(port: u16, managed_pid: Option<u32>) -> Result<Vec<PortOwner>, String> {
    let script = r#"
$port = [int]$env:AIRLOCK_PORT
$rows = netstat -ano | Select-String -Pattern 'LISTENING'
$owners = foreach ($row in $rows) {
  $parts = ($row.Line -split '\s+') | Where-Object { $_ -ne '' }
  if ($parts.Count -lt 5) { continue }
  $local = $parts[1]
  $state = $parts[3]
  $pidText = $parts[4]
  if ($state -ne 'LISTENING') { continue }
  if ($local -notmatch (':' + $port + '$')) { continue }
  $pid = 0
  if (-not [int]::TryParse($pidText, [ref]$pid)) { continue }
  if ($pid -le 0) { continue }
  $proc = Get-CimInstance Win32_Process -Filter "ProcessId = $pid" -ErrorAction SilentlyContinue
  if (-not $proc) { continue }
  $owner = Invoke-CimMethod -InputObject $proc -MethodName GetOwner -ErrorAction SilentlyContinue
  if (-not $owner -or $owner.User -ne $env:USERNAME) { continue }
  [pscustomobject]@{ port = $port; pid = $pid; command = $proc.Name }
}
$owners | Sort-Object pid -Unique | ConvertTo-Json -Compress
"#;
    let output = Command::new("powershell.exe")
        .args(["-NoProfile", "-NonInteractive", "-Command", script])
        .env("AIRLOCK_PORT", port.to_string())
        .output()
        .map_err(|_| "无法读取端口占用情况".to_string())?;
    if output.stdout.len() > 64 << 10 {
        return Err("端口占用信息异常".to_string());
    }
    if output.stdout.iter().all(|byte| byte.is_ascii_whitespace()) {
        return Ok(Vec::new());
    }
    let mut owners: Vec<PortOwner> =
        serde_json::from_slice(&output.stdout).map_err(|_| "端口占用信息无效".to_string())?;
    owners.retain(|owner| Some(owner.pid) != managed_pid);
    owners.sort_by_key(|owner| owner.pid);
    owners.dedup_by_key(|owner| owner.pid);
    Ok(owners)
}

fn validate_listener_port_request(settings: &SecuritySettings, port: u16) -> Result<(), String> {
    validate_security_settings(settings)?;
    if !is_configured_listener_port(settings, port) {
        return Err("只能管理当前 Airlock 配置的 HTTP 或 SSH 端口".to_string());
    }
    Ok(())
}

#[tauri::command]
async fn list_listener_port_owners(
    port: u16,
    process: tauri::State<'_, DaemonProcess>,
    security: tauri::State<'_, SecurityConfiguration>,
) -> Result<Vec<PortOwner>, String> {
    let settings = security
        .settings
        .lock()
        .map_err(|_| "无法读取本地核心设置".to_string())?
        .clone();
    let managed_pid = process.managed_pid();
    tauri::async_runtime::spawn_blocking(move || {
        validate_listener_port_request(&settings, port)?;
        listening_port_owners(port, managed_pid)
    })
    .await
    .map_err(|_| "端口占用检查意外终止".to_string())?
}

#[tauri::command]
async fn terminate_listener_port_owner(
    port: u16,
    pid: u32,
    process: tauri::State<'_, DaemonProcess>,
    security: tauri::State<'_, SecurityConfiguration>,
) -> Result<String, String> {
    let settings = security
        .settings
        .lock()
        .map_err(|_| "无法读取本地核心设置".to_string())?
        .clone();
    let managed_pid = process.managed_pid();
    tauri::async_runtime::spawn_blocking(move || {
        validate_listener_port_request(&settings, port)?;
        if managed_pid == Some(pid) {
            return Err("不能结束当前 Airlock 本地核心".to_string());
        }
        let owners = listening_port_owners(port, managed_pid)?;
        if !owners.iter().any(|owner| owner.pid == pid) {
            return Err("该进程不再监听所选端口，未执行结束操作".to_string());
        }
        let status = {
            #[cfg(unix)]
            {
                Command::new("/bin/kill")
                    .args(["-TERM", &pid.to_string()])
                    .status()
            }
            #[cfg(windows)]
            {
                Command::new("taskkill")
                    .args(["/PID", &pid.to_string()])
                    .status()
            }
        }
        .map_err(|_| "无法向该进程发送结束请求".to_string())?;
        if !status.success() {
            return Err("系统拒绝结束该进程".to_string());
        }
        for _ in 0..20 {
            if !listening_port_owners(port, managed_pid)?
                .iter()
                .any(|owner| owner.pid == pid)
            {
                return Ok(format!("已结束占用端口 {port} 的进程（PID {pid}）"));
            }
            std::thread::sleep(Duration::from_millis(100));
        }
        Err("已发送结束请求，但该进程仍在监听端口；请在系统活动监视器中处理".to_string())
    })
    .await
    .map_err(|_| "结束端口进程意外终止".to_string())?
}

fn show_main_window(app: &tauri::AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.show();
        let _ = window.set_focus();
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            let config_directory = app.path().app_config_dir()?;
            let security_path = config_directory.join("security-settings.json");
            let security_settings =
                load_security_settings(&security_path).map_err(std::io::Error::other)?;
            let token = generate_control_token().map_err(std::io::Error::other)?;
            let client = ControlClient {
                endpoint: platform::local_control_endpoint(&config_directory),
                token,
            };
            let process = DaemonProcess(Arc::new(Mutex::new(None)));
            let startup = CoreStartupState::default();
            startup.set("正在启动本地核心");
            match spawn_sidecar(app.handle(), client.token.as_ref(), &security_settings) {
                Ok((child, startup_log)) => {
                    process.replace(child, startup_log);
                    monitor_initial_sidecar(
                        client.clone(),
                        process.clone(),
                        startup.clone(),
                        security_settings.clone(),
                    );
                }
                Err(error) => startup.set(error),
            }
            app.manage(client);
            app.manage(process);
            app.manage(startup);
            app.manage(SecurityConfiguration {
                path: security_path,
                settings: Arc::new(Mutex::new(security_settings)),
            });

            let show = MenuItem::with_id(app, "show", "显示 Airlock", true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&show, &quit])?;

            TrayIconBuilder::new()
                .icon(
                    app.default_window_icon()
                        .expect("Airlock bundle icon is required")
                        .clone(),
                )
                .tooltip("Airlock")
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "show" => show_main_window(app),
                    "quit" => {
                        if let Some(process) = app.try_state::<DaemonProcess>() {
                            process.stop();
                        }
                        app.exit(0);
                    }
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        ..
                    } = event
                    {
                        show_main_window(tray.app_handle());
                    }
                })
                .build(app)?;

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            get_control_state,
            set_route_enabled,
            stop_all_routes,
            delete_route,
            create_http_route,
            create_llm_route,
            probe_ssh_host_key,
            create_ssh_route,
            update_ssh_target,
            rotate_ssh_credential,
            set_llm_policy,
            rotate_llm_api_key,
            reset_llm_usage,
            test_route_health,
            test_proxy_health,
            set_ssh_policy,
            configure_proxy,
            clear_proxy,
            apply_security_settings,
            restart_local_core,
            list_listener_port_owners,
            terminate_listener_port_owner,
            open_external_url,
            set_ui_locale,
            get_platform_info
        ])
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .build(tauri::generate_context!())
        .expect("error while building Airlock Desktop")
        .run(|app, event| {
            if matches!(event, tauri::RunEvent::Exit) {
                if let Some(process) = app.try_state::<DaemonProcess>() {
                    process.stop();
                }
            }
        });
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;

    struct TestDirectory(PathBuf);

    impl TestDirectory {
        fn new() -> Self {
            let nonce = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("system clock should be valid")
                .as_nanos();
            let path = std::env::temp_dir().join(format!(
                "airlock-security-settings-{}-{nonce}",
                std::process::id()
            ));
            std::fs::create_dir(&path).expect("test directory should be created");
            Self(path)
        }

        fn path(&self) -> &Path {
            &self.0
        }
    }

    impl Drop for TestDirectory {
        fn drop(&mut self) {
            let _ = std::fs::remove_dir_all(&self.0);
        }
    }

    #[test]
    fn security_settings_validation_rejects_unknown_values() {
        let valid = SecuritySettings::default();
        assert_eq!(valid.network_scope, "loopback");
        assert_eq!(valid.secret_store, "local_file");
        assert_eq!(valid.http_port, DEFAULT_HTTP_PORT);
        assert_eq!(valid.ssh_port, DEFAULT_SSH_PORT);
        assert!(validate_security_settings(&valid).is_ok());

        for invalid in [
            SecuritySettings {
                version: SECURITY_SETTINGS_VERSION + 1,
                ..valid.clone()
            },
            SecuritySettings {
                network_scope: "public".to_string(),
                ..valid.clone()
            },
            SecuritySettings {
                secret_store: "plaintext".to_string(),
                ..valid.clone()
            },
            SecuritySettings {
                http_port: MIN_UNPRIVILEGED_PORT - 1,
                ..valid.clone()
            },
            SecuritySettings {
                ssh_port: valid.http_port,
                ..valid.clone()
            },
        ] {
            assert!(validate_security_settings(&invalid).is_err());
        }
    }

    #[test]
    fn external_links_are_limited_to_developer_destinations() {
        assert_eq!(
            allowed_external_url("https://0o0.site/"),
            Some(DEVELOPER_WEBSITE_URL)
        );
        assert_eq!(
            allowed_external_url("https://github.com/LouisonH"),
            Some(DEVELOPER_GITHUB_URL)
        );
        assert_eq!(allowed_external_url("https://example.com"), None);
        assert_eq!(
            allowed_external_url("https://0o0.site.attacker.example"),
            None
        );
    }

    #[test]
    fn ssh_exact_command_validation_enforces_single_line_policy() {
        assert!(validate_ssh_command("uptime", false).is_ok());
        assert!(validate_ssh_command("", false).is_err());
        assert!(validate_ssh_command("printf ok\nuname -a", false).is_err());
        assert!(validate_ssh_command(&"x".repeat(4097), false).is_err());
        assert!(validate_ssh_command("", true).is_ok());
    }

    #[test]
    fn ssh_local_username_validation_matches_gateway_contract() {
        for valid in ["build", "release.bot", "runner_2", "host-a"] {
            assert!(validate_ssh_local_username(valid).is_ok(), "{valid}");
        }
        for invalid in ["", "Build", "-build", "build user", "build@host"] {
            assert!(validate_ssh_local_username(invalid).is_err(), "{invalid}");
        }
        assert!(validate_ssh_local_username(&"a".repeat(64)).is_ok());
        assert!(validate_ssh_local_username(&"a".repeat(65)).is_err());
    }

    #[test]
    fn embedded_ssh_entry_validates_upstream_fields() {
        assert!(validate_ssh_upstream("host.internal:22", "deploy", "secret").is_ok());
        assert!(validate_ssh_upstream("", "deploy", "secret").is_err());
        assert!(validate_ssh_upstream("host internal", "deploy", "secret").is_err());
        assert!(validate_ssh_upstream("host.internal", "", "secret").is_err());
        assert!(validate_ssh_upstream("host.internal", "deploy", "").is_err());
        assert!(validate_ssh_upstream("host.internal", "deploy", "bad\0secret").is_err());
    }

    #[test]
    fn sidecar_startup_diagnostic_identifies_port_collisions_without_echoing_logs() {
        let directory = TestDirectory::new();
        let path = directory.path().join("airlockd-startup.log");
        std::fs::write(
            &path,
            "time=... level=ERROR msg=\"airlockd stopped\" error=\"listen tcp 127.0.0.1:4770: bind: address already in use\"",
        )
        .expect("test startup log should be written");
        let message = sidecar_start_failure(Some(&path), &SecuritySettings::default());
        assert!(message.contains("127.0.0.1:4768"));
        assert!(message.contains("占用"));
        assert!(!message.contains("level=ERROR"));
    }

    #[test]
    fn sidecar_startup_diagnostic_falls_back_to_a_safe_generic_message() {
        let message = sidecar_start_failure(None, &SecuritySettings::default());
        assert!(message.contains("4 秒"));
        assert!(!message.contains("airlockd-startup.log"));
    }

    #[test]
    fn ssh_host_key_probe_uses_frontend_camel_case_contract() {
        let payload = serde_json::to_value(SSHHostKeyProbeResult {
            host_key: "public-key".to_string(),
            fingerprint: "SHA256:fingerprint".to_string(),
        })
        .expect("SSH probe result should serialize");
        assert_eq!(payload["hostKey"], "public-key");
        assert_eq!(payload["fingerprint"], "SHA256:fingerprint");
        assert!(payload.get("host_key").is_none());
    }

    #[cfg(unix)]
    #[test]
    fn authenticated_control_request_preserves_ssh_local_usernames() {
        use std::io::BufRead;
        use std::os::unix::net::UnixListener;
        use std::thread;

        let nonce = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("system clock should be valid")
            .as_nanos();
        let socket_path = PathBuf::from("/tmp").join(format!(
            "airlock-control-test-{}-{nonce}.sock",
            std::process::id()
        ));
        let listener = UnixListener::bind(&socket_path).expect("test control socket should bind");
        let server = thread::spawn(move || {
            let (mut stream, _) = listener
                .accept()
                .expect("test control client should connect");
            let mut raw = String::new();
            BufReader::new(stream.try_clone().expect("test stream should clone"))
                .read_line(&mut raw)
                .expect("test control request should be readable");
            stream
                .write_all(b"{\"ok\":true}\n")
                .expect("test control response should write");
            raw
        });
        let client = ControlClient {
            endpoint: socket_path.to_string_lossy().into_owned(),
            token: Arc::new(SecretToken("authenticated-test-token".to_string())),
        };
        let request = ControlRequest {
            version: CONTROL_PROTOCOL_VERSION,
            token: "",
            action: "create_ssh_route",
            create: None,
            create_ssh: Some(CreateSSHRoute {
                name: "Build",
                alias: "build",
                local_username: "builder",
                address: "upstream.invalid:22",
                username: "upstream-user",
                password: "upstream-password",
                local_password: "shared-local-password",
                expected_host_key: "host-key",
                allowed_command: "uptime",
                allow_all_commands: false,
                record_commands: true,
                authentication_timeout_seconds: 20,
                egress: "Auto",
            }),
            probe_ssh: None,
            ssh_policy: Some(SSHPolicyUpdate {
                name: "Release host",
                local_username: "release",
                allowed_command: "uptime",
                allow_all_commands: false,
                record_commands: true,
                authentication_timeout_seconds: 20,
                egress: "Auto",
            }),
            alias: None,
            enabled: None,
            proxy_url: None,
            capability: None,
            command: None,
            secret_store_mode: None,
        };
        client
            .request(&request)
            .expect("authenticated request should succeed");
        let raw = server.join().expect("test control server should finish");
        let payload: serde_json::Value =
            serde_json::from_str(&raw).expect("control request should be JSON");
        assert_eq!(payload["token"], "authenticated-test-token");
        assert_eq!(payload["create_ssh"]["local_username"], "builder");
        assert_eq!(payload["ssh_policy"]["local_username"], "release");
        let _ = std::fs::remove_file(PathBuf::from(&client.endpoint));
    }

    #[cfg(unix)]
    #[test]
    fn security_settings_persist_with_user_only_permissions() {
        use std::os::unix::fs::PermissionsExt;

        let directory = TestDirectory::new();
        let path = directory.path().join("security-settings.json");
        let settings = SecuritySettings {
            version: SECURITY_SETTINGS_VERSION,
            network_scope: "lan".to_string(),
            secret_store: "local_file".to_string(),
            http_port: 8484,
            ssh_port: 8585,
        };

        save_security_settings(&path, &settings).expect("settings should be saved");
        assert_eq!(
            std::fs::metadata(&path)
                .expect("settings metadata should be readable")
                .permissions()
                .mode()
                & 0o777,
            0o600
        );
        assert_eq!(
            std::fs::metadata(directory.path())
                .expect("directory metadata should be readable")
                .permissions()
                .mode()
                & 0o777,
            0o700
        );
        assert_eq!(
            load_security_settings(&path).expect("settings should load"),
            settings
        );
    }

    #[cfg(unix)]
    #[test]
    fn older_security_settings_receive_default_listener_ports() {
        use std::os::unix::fs::PermissionsExt;

        let directory = TestDirectory::new();
        let path = directory.path().join("security-settings.json");
        std::fs::write(
            &path,
            r#"{"version":1,"networkScope":"loopback","secretStore":"local_file"}"#,
        )
        .expect("legacy settings should be written");
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600))
            .expect("legacy settings should be protected");
        let settings = load_security_settings(&path).expect("legacy settings should load");
        assert_eq!(settings.http_port, DEFAULT_HTTP_PORT);
        assert_eq!(settings.ssh_port, DEFAULT_SSH_PORT);
    }

    #[cfg(unix)]
    #[test]
    fn port_owner_filter_keeps_only_external_current_user_processes() {
        let current_uid = unsafe { libc::geteuid() } as u32;
        let mut owners = Vec::new();
        append_port_owner(
            &mut owners,
            &RawPortOwner {
                pid: Some(100),
                command: "listener\nname".to_string(),
                uid: Some(current_uid),
            },
            DEFAULT_HTTP_PORT,
            current_uid,
            None,
        );
        append_port_owner(
            &mut owners,
            &RawPortOwner {
                pid: Some(101),
                command: "other-user".to_string(),
                uid: Some(current_uid.saturating_add(1)),
            },
            DEFAULT_HTTP_PORT,
            current_uid,
            None,
        );
        append_port_owner(
            &mut owners,
            &RawPortOwner {
                pid: Some(102),
                command: "managed-airlockd".to_string(),
                uid: Some(current_uid),
            },
            DEFAULT_HTTP_PORT,
            current_uid,
            Some(102),
        );
        assert_eq!(owners.len(), 1);
        assert_eq!(owners[0].pid, 100);
        assert_eq!(owners[0].command, "listenername");
    }
}
