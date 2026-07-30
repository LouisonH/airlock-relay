use serde::{Deserialize, Serialize};
use std::{
    fs::OpenOptions,
    io::{BufRead, BufReader, Read, Write},
    os::unix::fs::{OpenOptionsExt, PermissionsExt},
    os::unix::net::UnixStream,
    path::PathBuf,
    process::{Child, Command, Stdio},
    sync::{Arc, Mutex},
    time::{Duration, SystemTime, UNIX_EPOCH},
};
use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager,
};

const CONTROL_PROTOCOL_VERSION: u8 = 1;
const MAX_CONTROL_RESPONSE: u64 = 64 << 10;
const SECURITY_SETTINGS_VERSION: u8 = 1;

#[derive(Clone)]
struct ControlClient {
    socket_path: PathBuf,
    token: Arc<SecretToken>,
}

struct SecretToken(String);

impl Drop for SecretToken {
    fn drop(&mut self) {
        clear_string(&mut self.0);
    }
}

#[derive(Clone)]
struct DaemonProcess(Arc<Mutex<Option<Child>>>);

impl DaemonProcess {
    fn stop(&self) {
        if let Ok(mut guard) = self.0.lock() {
            if let Some(mut child) = guard.take() {
                let _ = child.kill();
                let _ = child.wait();
            }
        }
    }

    fn replace(&self, child: Child) {
        if let Ok(mut guard) = self.0.lock() {
            *guard = Some(child);
        }
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
}

impl Default for SecuritySettings {
    fn default() -> Self {
        Self {
            version: SECURITY_SETTINGS_VERSION,
            network_scope: "loopback".to_string(),
            secret_store: "local_file".to_string(),
        }
    }
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
    address: &'a str,
    username: &'a str,
    password: &'a str,
    #[serde(skip_serializing_if = "str::is_empty")]
    local_password: &'a str,
    expected_host_key: &'a str,
    allowed_command: &'a str,
    allow_all_commands: bool,
    record_commands: bool,
    egress: &'a str,
}

#[derive(Serialize)]
struct SSHPolicyUpdate<'a> {
    allowed_command: &'a str,
    allow_all_commands: bool,
    record_commands: bool,
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
}

#[derive(Deserialize)]
struct SSHHostKeyProbe {
    host_key: String,
    fingerprint: String,
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

impl ControlClient {
    fn request(&self, request: &ControlRequest<'_>) -> Result<ControlResponse, String> {
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
                address: create.address,
                username: create.username,
                password: create.password,
                local_password: create.local_password,
                expected_host_key: create.expected_host_key,
                allowed_command: create.allowed_command,
                allow_all_commands: create.allow_all_commands,
                record_commands: create.record_commands,
                egress: create.egress,
            }),
            probe_ssh: request.probe_ssh.as_ref().map(|probe| ProbeSSHHostKey {
                address: probe.address,
                egress: probe.egress,
            }),
            ssh_policy: request.ssh_policy.as_ref().map(|policy| SSHPolicyUpdate {
                allowed_command: policy.allowed_command,
                allow_all_commands: policy.allow_all_commands,
                record_commands: policy.record_commands,
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
        let result = self.exchange(&payload);
        payload.fill(0);
        result
    }

    fn exchange(&self, payload: &[u8]) -> Result<ControlResponse, String> {
        let mut stream = UnixStream::connect(&self.socket_path)
            .map_err(|_| "无法连接 airlockd 控制通道".to_string())?;
        stream
            .set_read_timeout(Some(Duration::from_secs(10)))
            .map_err(|_| "无法保护控制通道".to_string())?;
        stream
            .set_write_timeout(Some(Duration::from_secs(10)))
            .map_err(|_| "无法保护控制通道".to_string())?;
        stream
            .write_all(payload)
            .map_err(|_| "控制请求发送失败".to_string())?;

        let mut raw = String::new();
        BufReader::new(stream)
            .take(MAX_CONTROL_RESPONSE)
            .read_line(&mut raw)
            .map_err(|_| "控制响应读取失败".to_string())?;
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
fn get_control_state(
    client: tauri::State<'_, ControlClient>,
    security: tauri::State<'_, SecurityConfiguration>,
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
    match client.request(&request) {
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
            message: Some(message),
            proxy_configured: false,
            ssh_ready: false,
            activity: Vec::new(),
            security_settings,
        },
    }
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
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "delete_route",
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
    client.request(&request).map(|response| ControlUpdate {
        routes: response.routes,
        message: if response.warning.is_empty() {
            None
        } else {
            Some("路由已删除，但 Keychain 清理需要检查".to_string())
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
        "输入完整目标 URL。该内容仅发送到本机 airlockd 并保存进 Keychain。",
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

#[tauri::command]
async fn create_ssh_route(
    client: tauri::State<'_, ControlClient>,
    name: String,
    alias: String,
    egress: String,
    allowed_command: String,
    allow_all_commands: bool,
    record_commands: bool,
) -> Result<RouteSummary, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        create_ssh_route_blocking(
            client,
            name,
            alias,
            egress,
            allowed_command,
            allow_all_commands,
            record_commands,
        )
    })
    .await
    .map_err(|_| "SSH 原生安全录入意外终止".to_string())?
}

fn create_ssh_route_blocking(
    client: ControlClient,
    name: String,
    alias: String,
    egress: String,
    allowed_command: String,
    allow_all_commands: bool,
    record_commands: bool,
) -> Result<RouteSummary, String> {
    validate_route_identity(&name, &alias, &egress)?;
    validate_ssh_command(&allowed_command, allow_all_commands)?;
    if allow_all_commands {
        confirm_allow_all_commands(&alias)?;
    }
    let mut address = prompt_native_value_with_title(
        "SSH 设置 · 上游地址",
        "输入上游 SSH 地址，可使用 host、host:port 或 IP。地址只会发送到本机 airlockd。",
        false,
        false,
        "",
    )?;
    let mut username = prompt_native_value_with_title(
        "SSH 设置 · 上游账号",
        "输入上游 SSH 用户名。调用者只会看到本地路由别名。",
        false,
        false,
        "",
    )?;
    let mut password = prompt_native_value_with_title(
        "SSH 设置 · 上游密码",
        "输入上游 SSH 密码。密码只会交给本机核心，并按设置中的凭据保护方式保存。",
        false,
        true,
        "",
    )?;
    let mut local_password = prompt_native_value_with_title(
	    "SSH 设置 · 本地登录密码",
	    "可选：输入至少 12 个字节的本地 SSH 密码。它与上游密码完全隔离，Airlock 只保存摘要。选择“不设置”将自动生成高强度 Capability。",
	    true,
	    true,
	    "",
	)?;
    if !local_password.is_empty() {
        if local_password.len() < 12
            || local_password.len() > 1024
            || local_password.contains(['\0', '\r', '\n'])
        {
            clear_ssh_credentials(
                &mut address,
                &mut username,
                &mut password,
                &mut local_password,
            );
            return Err("本地 SSH 密码需要 12 到 1024 个字节，且不能包含换行".to_string());
        }
        let mut confirmation = prompt_native_value_with_title(
            "SSH 设置 · 确认本地密码",
            "再次输入本地 SSH 密码。",
            false,
            true,
            "",
        )?;
        let matches = local_password == confirmation;
        clear_string(&mut confirmation);
        if !matches {
            clear_ssh_credentials(
                &mut address,
                &mut username,
                &mut password,
                &mut local_password,
            );
            return Err("两次输入的本地 SSH 密码不一致".to_string());
        }
    }
    let mut command = if allow_all_commands {
        "printf airlock-ok".to_string()
    } else {
        allowed_command
    };

    let probe_request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "probe_ssh_host_key",
        create: None,
        create_ssh: None,
        probe_ssh: Some(ProbeSSHHostKey {
            address: &address,
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
    let probe_result = client.request(&probe_request);
    let mut probe = match probe_result {
        Ok(response) => response
            .ssh_host_key_probe
            .ok_or_else(|| "airlockd 未返回 SSH Host Key".to_string())?,
        Err(error) => {
            clear_ssh_inputs(
                &mut address,
                &mut username,
                &mut password,
                &mut local_password,
                &mut command,
            );
            return Err(error);
        }
    };
    let confirmation = confirm_ssh_host_key(&probe.fingerprint);
    clear_string(&mut probe.fingerprint);
    if let Err(error) = confirmation {
        clear_string(&mut probe.host_key);
        clear_ssh_inputs(
            &mut address,
            &mut username,
            &mut password,
            &mut local_password,
            &mut command,
        );
        return Err(error);
    }

    let create_request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "create_ssh_route",
        create: None,
        create_ssh: Some(CreateSSHRoute {
            name: name.trim(),
            alias: &alias,
            address: &address,
            username: &username,
            password: &password,
            local_password: &local_password,
            expected_host_key: &probe.host_key,
            allowed_command: &command,
            allow_all_commands,
            record_commands,
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
    clear_string(&mut probe.host_key);
    clear_string(&mut address);
    clear_string(&mut username);
    clear_string(&mut password);
    let custom_local_password = !local_password.is_empty();
    let mut created = match create_result {
        Ok(response) => response
            .created
            .ok_or_else(|| "airlockd 未返回新 SSH 路由".to_string())?,
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
    let mut local_authentication = if custom_local_password {
        std::mem::take(&mut local_password)
    } else {
        std::mem::take(&mut created.capability)
    };

    let enable = route_enabled_request(&alias, true);
    let enabled_response = match client.request(&enable) {
        Ok(response) => response,
        Err(error) => {
            clear_string(&mut local_authentication);
            clear_string(&mut command);
            return Err(error);
        }
    };
    let test = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "test_ssh_route",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(&alias),
        enabled: None,
        proxy_url: None,
        capability: Some(&local_authentication),
        command: Some(&command),
        secret_store_mode: None,
    };
    if let Err(error) = client.request(&test) {
        let _ = client.request(&route_enabled_request(&alias, false));
        clear_string(&mut local_authentication);
        clear_string(&mut command);
        return Err(format!("SSH 路由已保存但保持停用：{error}"));
    }
    clear_string(&mut command);

    let presentation = if custom_local_password {
        present_custom_ssh_access(&created.route.local_endpoint)
    } else {
        present_capability(&created.route.local_endpoint, &local_authentication)
    };
    if let Err(error) = presentation {
        let _ = client.request(&route_enabled_request(&alias, false));
        clear_string(&mut local_authentication);
        return Err(error);
    }
    clear_string(&mut local_authentication);
    enabled_response
        .routes
        .into_iter()
        .find(|route| route.alias == alias)
        .ok_or_else(|| "新 SSH 路由未出现在控制状态中".to_string())
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
    if egress != "Direct" && egress != "Proxy" && egress != "Auto" {
        return Err("出口策略无效".to_string());
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

fn route_enabled_request(alias: &str, enabled: bool) -> ControlRequest<'_> {
    ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "set_route_enabled",
        create: None,
        create_ssh: None,
        probe_ssh: None,
        ssh_policy: None,
        alias: Some(alias),
        enabled: Some(enabled),
        proxy_url: None,
        capability: None,
        command: None,
        secret_store_mode: None,
    }
}

fn clear_ssh_inputs(
    address: &mut String,
    username: &mut String,
    password: &mut String,
    local_password: &mut String,
    command: &mut String,
) {
    clear_ssh_credentials(address, username, password, local_password);
    clear_string(command);
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
async fn set_ssh_policy(
    client: tauri::State<'_, ControlClient>,
    alias: String,
    allowed_command: String,
    allow_all_commands: bool,
    record_commands: bool,
) -> Result<Vec<RouteSummary>, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        set_ssh_policy_blocking(
            client,
            alias,
            allowed_command,
            allow_all_commands,
            record_commands,
        )
    })
    .await
    .map_err(|_| "SSH 权限设置意外终止".to_string())?
}

fn set_ssh_policy_blocking(
    client: ControlClient,
    alias: String,
    allowed_command: String,
    allow_all_commands: bool,
    record_commands: bool,
) -> Result<Vec<RouteSummary>, String> {
    validate_ssh_command(&allowed_command, allow_all_commands)?;
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
            allowed_command: &command,
            allow_all_commands,
            record_commands,
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
async fn configure_proxy(client: tauri::State<'_, ControlClient>) -> Result<bool, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || configure_proxy_blocking(client))
        .await
        .map_err(|_| "原生代理录入意外终止".to_string())?
}

fn configure_proxy_blocking(client: ControlClient) -> Result<bool, String> {
    let mut proxy_url = prompt_protected_value(
        "输入 Clash 或其他本地代理 URL，例如 http://127.0.0.1:7890 或 socks5://127.0.0.1:7890。认证信息可写在 URL 中，内容仅进入 Keychain。",
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
  buttons: payload.optional ? ['不设置', '保存'] : ['取消', '继续'],
  defaultButton: payload.optional ? '保存' : '继续'
};
if (!payload.optional) options.cancelButton = '取消';
const result = app.displayDialog(payload.message, options);
payload.optional && result.buttonReturned !== '保存' ? '' : result.textReturned;
"#;
    let mut payload = serde_json::to_vec(&serde_json::json!({
        "title": title,
        "message": message,
        "optional": optional,
        "hidden": hidden,
        "defaultValue": default_value,
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
const app = Application.currentApplication();
app.includeStandardAdditions = true;
const result = app.displayDialog('为调用者创建一把独立的二次 API Key。它只用于访问 Airlock，真实上游 Key 不会暴露。\n\n随机生成提供 256-bit 强度并仅显示一次；自定义 Key 会要求隐藏输入两次。', {
  withTitle: 'LLM 设置 3/3 · 二次 API Key',
  buttons: ['取消', '自定义 Key', '随机生成（推荐）'],
  defaultButton: '随机生成（推荐）',
  cancelButton: '取消'
});
result.buttonReturned === '自定义 Key' ? 'custom' : 'random';
"#;
    let output = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", SCRIPT])
        .output()
        .map_err(|_| "无法打开二次 API Key 选择窗口".to_string())?;
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

#[cfg(not(target_os = "macos"))]
fn prompt_protected_value(_message: &str, _optional: bool) -> Result<String, String> {
    Err("当前版本仅支持 macOS 原生安全录入".to_string())
}

#[cfg(not(target_os = "macos"))]
fn prompt_native_value(
    _message: &str,
    _optional: bool,
    _hidden: bool,
    _default_value: &str,
) -> Result<String, String> {
    Err("当前版本仅支持 macOS 原生安全录入".to_string())
}

#[cfg(not(target_os = "macos"))]
fn prompt_native_value_with_title(
    _title: &str,
    _message: &str,
    _optional: bool,
    _hidden: bool,
    _default_value: &str,
) -> Result<String, String> {
    Err("当前版本仅支持 macOS 原生安全录入".to_string())
}

#[cfg(not(target_os = "macos"))]
fn choose_llm_local_api_key_mode() -> Result<bool, String> {
    Err("当前版本仅支持 macOS 原生二次 API Key 设置".to_string())
}

#[cfg(not(target_os = "macos"))]
fn prompt_llm_local_api_key() -> Result<(String, bool), String> {
    Err("当前版本仅支持 macOS 原生二次 API Key 设置".to_string())
}

#[cfg(target_os = "macos")]
fn confirm_allow_all_commands(alias: &str) -> Result<(), String> {
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const alias = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const app = Application.currentApplication();
app.includeStandardAdditions = true;
app.displayDialog('路由 “' + alias + '” 将允许调用者执行任意非交互 exec 命令。\n\nShell、PTY、SFTP 与端口转发仍会被拒绝，但上游账号能访问的数据和操作都可能被命令读取或修改。请仅配合低权限专用账号使用。', {
  withTitle: '高风险 SSH 权限',
  buttons: ['取消', '允许所有 exec'], defaultButton: '取消', cancelButton: '取消',
  withIcon: 'caution'
});
true;
"#;
    let mut payload = alias.as_bytes().to_vec();
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

#[cfg(not(target_os = "macos"))]
fn confirm_allow_all_commands(_alias: &str) -> Result<(), String> {
    Err("当前版本仅支持 macOS 原生 SSH 风险确认".to_string())
}

#[cfg(target_os = "macos")]
fn confirm_ssh_host_key(fingerprint: &str) -> Result<(), String> {
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const fingerprint = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const app = Application.currentApplication();
app.includeStandardAdditions = true;
app.displayDialog('请通过可信渠道核对上游 SSH Host Key 指纹。指纹不一致时请取消。\n\n' + fingerprint, {
  withTitle: '确认 SSH Host Key',
  buttons: ['取消', '指纹一致'], defaultButton: '指纹一致', cancelButton: '取消',
  withIcon: 'caution'
});
true;
"#;
    let mut payload = fingerprint.as_bytes().to_vec();
    let mut child = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", SCRIPT])
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .spawn()
        .map_err(|_| "无法打开 Host Key 确认窗口".to_string())?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(&payload)
            .map_err(|_| "无法展示 Host Key 指纹".to_string())?;
    }
    payload.fill(0);
    if !child
        .wait()
        .map_err(|_| "Host Key 确认窗口意外终止".to_string())?
        .success()
    {
        return Err("已取消 SSH Host Key 信任".to_string());
    }
    Ok(())
}

#[cfg(not(target_os = "macos"))]
fn confirm_ssh_host_key(_fingerprint: &str) -> Result<(), String> {
    Err("当前版本仅支持 macOS 原生 Host Key 确认".to_string())
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
app.displayDialog('路由已安全保存。Capability 仅显示这一次，请交给需要访问该路由的客户端。', {
  withTitle: 'Airlock 路由已创建',
  defaultAnswer: payload.endpoint + '\n' + payload.capability,
  buttons: ['完成'], defaultButton: '完成'
});
true;
"#;
    let mut payload = serde_json::to_vec(&serde_json::json!({
        "endpoint": endpoint,
        "capability": capability,
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
const apiKey = payload.customLocalKey ? '<使用刚才设置的本地 API Key>' : payload.capability;
const details = prefix + '_BASE_URL=' + baseURL + '\n' + prefix + '_API_KEY=' + apiKey;
const message = payload.customLocalKey
  ? 'LLM 路由已启用。Airlock 不会回显自定义的本地 API Key。'
  : 'LLM 路由已启用。随机生成的本地 API Key 仅显示这一次。';
app.displayDialog(message, {
  withTitle: 'Airlock LLM 路由已创建',
  defaultAnswer: details,
  buttons: ['完成'], defaultButton: '完成'
});
true;
"#;
    let mut payload = serde_json::to_vec(&serde_json::json!({
        "provider": provider,
        "endpoint": endpoint,
        "capability": capability,
        "customLocalKey": custom_local_key,
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

#[cfg(target_os = "macos")]
fn present_custom_ssh_access(endpoint: &str) -> Result<(), String> {
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const endpoint = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const app = Application.currentApplication();
app.includeStandardAdditions = true;
app.displayDialog('路由已安全保存。请使用刚才设置的本地密码登录；Airlock 不会回显该密码。', {
  withTitle: 'Airlock SSH 路由已创建',
  defaultAnswer: endpoint,
  buttons: ['完成'], defaultButton: '完成'
});
true;
"#;
    let mut payload = endpoint.as_bytes().to_vec();
    let mut child = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", SCRIPT])
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .spawn()
        .map_err(|_| "无法打开 SSH 访问信息窗口".to_string())?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(&payload)
            .map_err(|_| "无法展示 SSH 访问信息".to_string())?;
    }
    payload.fill(0);
    if !child
        .wait()
        .map_err(|_| "SSH 访问信息窗口意外终止".to_string())?
        .success()
    {
        return Err("SSH 访问信息未确认，路由已保存但保持停用".to_string());
    }
    Ok(())
}

#[cfg(not(target_os = "macos"))]
fn present_capability(_endpoint: &str, _capability: &str) -> Result<(), String> {
    Err("当前版本仅支持 macOS 原生 Capability 窗口".to_string())
}

#[cfg(not(target_os = "macos"))]
fn present_llm_access(
    _provider: &str,
    _endpoint: &str,
    _capability: &str,
    _custom_local_key: bool,
) -> Result<(), String> {
    Err("当前版本仅支持 macOS 原生 LLM 连接信息窗口".to_string())
}

#[cfg(not(target_os = "macos"))]
fn present_custom_ssh_access(_endpoint: &str) -> Result<(), String> {
    Err("当前版本仅支持 macOS 原生 SSH 访问信息窗口".to_string())
}

fn clear_string(value: &mut String) {
    unsafe { value.as_bytes_mut().fill(0) };
    value.clear();
}

fn generate_control_token() -> Result<Arc<SecretToken>, String> {
    let mut random = [0_u8; 32];
    std::fs::File::open("/dev/urandom")
        .and_then(|mut source| source.read_exact(&mut random))
        .map_err(|_| "无法生成本地控制令牌".to_string())?;
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
    {
        return Err("安全设置无效".to_string());
    }
    Ok(())
}

fn load_security_settings(path: &PathBuf) -> Result<SecuritySettings, String> {
    let parent = path
        .parent()
        .ok_or_else(|| "安全设置路径无效".to_string())?;
    std::fs::create_dir_all(parent).map_err(|_| "无法创建安全设置目录".to_string())?;
    std::fs::set_permissions(parent, std::fs::Permissions::from_mode(0o700))
        .map_err(|_| "无法保护安全设置目录".to_string())?;
    let metadata = match std::fs::symlink_metadata(path) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            let settings = SecuritySettings::default();
            save_security_settings(path, &settings)?;
            return Ok(settings);
        }
        Err(_) => return Err("无法读取安全设置".to_string()),
    };
    if !metadata.is_file()
        || metadata.file_type().is_symlink()
        || metadata.permissions().mode() & 0o077 != 0
        || metadata.len() > 4096
    {
        return Err("安全设置文件权限或类型无效".to_string());
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
    std::fs::create_dir_all(parent).map_err(|_| "无法创建安全设置目录".to_string())?;
    std::fs::set_permissions(parent, std::fs::Permissions::from_mode(0o700))
        .map_err(|_| "无法保护安全设置目录".to_string())?;
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| "无法生成安全设置临时路径".to_string())?
        .as_nanos();
    let temporary = parent.join(format!(
        ".security-settings-{}-{nonce}.tmp",
        std::process::id()
    ));
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(&temporary)
        .map_err(|_| "无法创建安全设置临时文件".to_string())?;
    let result = (|| {
        let payload =
            serde_json::to_vec_pretty(settings).map_err(|_| "无法编码安全设置".to_string())?;
        file.write_all(&payload)
            .map_err(|_| "无法写入安全设置".to_string())?;
        file.sync_all()
            .map_err(|_| "无法同步安全设置".to_string())?;
        drop(file);
        std::fs::rename(&temporary, path).map_err(|_| "无法安装安全设置".to_string())?;
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
        if client.request(&status_request()).is_ok() {
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
    let mut risks = Vec::new();
    if current.network_scope != "lan" && next.network_scope == "lan" {
        risks.push("局域网设备将能连接 Airlock 的 HTTP/SSH 入口，仍需要每条路由的凭据。");
    }
    if current.secret_store != "local_file" && next.secret_store == "local_file" {
        risks.push(
            "上游地址和凭据将保存在仅当前用户可读的 0600 文件中，不再由 macOS Keychain 加密保护。",
        );
    }
    if risks.is_empty() {
        return Ok(());
    }
    const SCRIPT: &str = r#"
ObjC.import('Foundation');
const input = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
const message = $.NSString.alloc.initWithDataEncoding(input, $.NSUTF8StringEncoding).js;
const app = Application.currentApplication();
app.includeStandardAdditions = true;
app.displayDialog(message + '\n\n应用设置会短暂重启本地转发核心。', {
  withTitle: 'Airlock 安全设置',
  buttons: ['取消', '应用并重启'], defaultButton: '取消', cancelButton: '取消',
  withIcon: 'caution'
});
true;
"#;
    let mut payload = risks.join("\n\n").into_bytes();
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

#[cfg(not(target_os = "macos"))]
fn confirm_security_change(
    _current: &SecuritySettings,
    _next: &SecuritySettings,
) -> Result<(), String> {
    Err("当前版本仅支持 macOS 原生安全设置确认".to_string())
}

fn locate_sidecar(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    if let Some(path) = std::env::var_os("AIRLOCKD_BIN").map(PathBuf::from) {
        if path.is_file() {
            return Ok(path);
        }
    }
    let mut candidates = Vec::new();
    if let Ok(resource_dir) = app.path().resource_dir() {
        candidates.push(resource_dir.join("airlockd"));
    }
    if let Ok(executable) = std::env::current_exe() {
        if let Some(directory) = executable.parent() {
            candidates.push(directory.join("airlockd"));
        }
    }
    candidates.push(PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../bin/airlockd"));
    candidates
        .into_iter()
        .find(|path| path.is_file())
        .ok_or_else(|| "找不到 airlockd sidecar，请先运行 npm run sidecar:build".to_string())
}

fn spawn_sidecar(
    app: &tauri::AppHandle,
    token: &SecretToken,
    settings: &SecuritySettings,
) -> Result<Child, String> {
    let binary = locate_sidecar(app)?;
    let mut command = Command::new(binary);
    command
        .arg("--control-token-stdin")
        .args(["--network-scope", &settings.network_scope])
        .args(["--secret-store", &settings.secret_store]);
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
    Ok(child)
}

fn start_configured_sidecar(
    app: &tauri::AppHandle,
    client: &ControlClient,
    process: &DaemonProcess,
    settings: &SecuritySettings,
) -> Result<(), String> {
    let child = spawn_sidecar(app, client.token.as_ref(), settings)?;
    process.replace(child);
    if wait_for_control(client) {
        return Ok(());
    }
    process.stop();
    Err("airlockd 重启后未能建立受保护控制通道".to_string())
}

fn apply_security_settings_blocking(
    app: tauri::AppHandle,
    client: ControlClient,
    process: DaemonProcess,
    security: SecurityConfiguration,
    network_scope: String,
    secret_store: String,
) -> Result<SecurityUpdate, String> {
    let next = SecuritySettings {
        version: SECURITY_SETTINGS_VERSION,
        network_scope,
        secret_store,
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
    let restart_result = start_configured_sidecar(&app, &client, &process, &next);
    if let Err(error) = restart_result {
        let _ = save_security_settings(&security.path, &current);
        let rollback = start_configured_sidecar(&app, &client, &process, &current);
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
    network_scope: String,
    secret_store: String,
) -> Result<SecurityUpdate, String> {
    let client = client.inner().clone();
    let process = process.inner().clone();
    let security = security.inner().clone();
    tauri::async_runtime::spawn_blocking(move || {
        apply_security_settings_blocking(
            app,
            client,
            process,
            security,
            network_scope,
            secret_store,
        )
    })
    .await
    .map_err(|_| "安全设置更新意外终止".to_string())?
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
            let child = spawn_sidecar(app.handle(), &token, &security_settings)
                .map_err(std::io::Error::other)?;
            app.manage(ControlClient {
                socket_path: config_directory.join("control.sock"),
                token,
            });
            app.manage(DaemonProcess(Arc::new(Mutex::new(Some(child)))));
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
            create_ssh_route,
            set_llm_policy,
            rotate_llm_api_key,
            reset_llm_usage,
            set_ssh_policy,
            configure_proxy,
            clear_proxy,
            apply_security_settings
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
        ] {
            assert!(validate_security_settings(&invalid).is_err());
        }
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
    fn security_settings_persist_with_user_only_permissions() {
        let directory = TestDirectory::new();
        let path = directory.path().join("security-settings.json");
        let settings = SecuritySettings {
            version: SECURITY_SETTINGS_VERSION,
            network_scope: "lan".to_string(),
            secret_store: "local_file".to_string(),
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
}
