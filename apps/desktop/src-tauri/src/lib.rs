use serde::{Deserialize, Serialize};
use std::{
    io::{BufRead, BufReader, Read, Write},
    os::unix::net::UnixStream,
    path::PathBuf,
    process::{Child, Command, Stdio},
    sync::{Arc, Mutex},
    time::Duration,
};
use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager,
};

const CONTROL_PROTOCOL_VERSION: u8 = 1;
const MAX_CONTROL_RESPONSE: u64 = 64 << 10;

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

struct DaemonProcess(Mutex<Option<Child>>);

impl DaemonProcess {
    fn stop(&self) {
        if let Ok(mut guard) = self.0.lock() {
            if let Some(mut child) = guard.take() {
                let _ = child.kill();
                let _ = child.wait();
            }
        }
    }
}

impl Drop for DaemonProcess {
    fn drop(&mut self) {
        self.stop();
    }
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
}

#[derive(Serialize)]
struct ControlRequest<'a> {
    version: u8,
    token: &'a str,
    action: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    create: Option<CreateHTTPRoute<'a>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    alias: Option<&'a str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enabled: Option<bool>,
}

#[derive(Serialize)]
struct CreateHTTPRoute<'a> {
    name: &'a str,
    alias: &'a str,
    base_url: &'a str,
    #[serde(skip_serializing_if = "str::is_empty")]
    authorization: &'a str,
}

#[derive(Deserialize)]
struct ControlResponse {
    ok: bool,
    #[serde(default)]
    error: String,
    #[serde(default)]
    running: bool,
    #[serde(default)]
    routes: Vec<RouteSummary>,
    created: Option<CreatedRoute>,
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
            }),
            alias: request.alias,
            enabled: request.enabled,
        };
        let mut payload =
            serde_json::to_vec(&authenticated).map_err(|_| "无法编码控制请求".to_string())?;
        payload.push(b'\n');
        let result = self.exchange(&payload);
        payload.fill(0);
        drop(authenticated);
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
fn get_control_state(client: tauri::State<'_, ControlClient>) -> ControlState {
    let request = ControlRequest {
        version: CONTROL_PROTOCOL_VERSION,
        token: "",
        action: "status",
        create: None,
        alias: None,
        enabled: None,
    };
    match client.request(&request) {
        Ok(response) => ControlState {
            connected: true,
            running: response.running,
            routes: response.routes,
            message: None,
        },
        Err(message) => ControlState {
            connected: false,
            running: false,
            routes: Vec::new(),
            message: Some(message),
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
        alias: Some(&alias),
        enabled: Some(enabled),
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
        alias: None,
        enabled: None,
    };
    client.request(&request).map(|response| response.routes)
}

#[tauri::command]
async fn create_http_route(
    client: tauri::State<'_, ControlClient>,
    name: String,
    alias: String,
) -> Result<RouteSummary, String> {
    let client = client.inner().clone();
    tauri::async_runtime::spawn_blocking(move || create_http_route_blocking(client, name, alias))
        .await
        .map_err(|_| "原生安全录入意外终止".to_string())?
}

fn create_http_route_blocking(
    client: ControlClient,
    name: String,
    alias: String,
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
        }),
        alias: None,
        enabled: None,
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
        alias: Some(&alias),
        enabled: Some(true),
    };
    client
        .request(&enable)?
        .routes
        .into_iter()
        .find(|route| route.alias == alias)
        .ok_or_else(|| "新路由未出现在控制状态中".to_string())
}

#[cfg(target_os = "macos")]
fn prompt_protected_value(message: &str, optional: bool) -> Result<String, String> {
    const REQUIRED_SCRIPT: &str = r#"
const app = Application.currentApplication();
app.includeStandardAdditions = true;
const result = app.displayDialog('输入内容', {
  withTitle: 'Airlock 安全录入',
  defaultAnswer: '', hiddenAnswer: true,
  buttons: ['取消', '继续'], defaultButton: '继续', cancelButton: '取消'
});
result.textReturned;
"#;
    const OPTIONAL_SCRIPT: &str = r#"
const app = Application.currentApplication();
app.includeStandardAdditions = true;
const result = app.displayDialog('输入内容', {
  withTitle: 'Airlock 安全录入',
  defaultAnswer: '', hiddenAnswer: true,
  buttons: ['不设置', '保存'], defaultButton: '保存'
});
result.buttonReturned === '保存' ? result.textReturned : '';
"#;
    let script = if optional {
        OPTIONAL_SCRIPT
    } else {
        REQUIRED_SCRIPT
    };
    let script = script.replace("输入内容", message);
    let output = Command::new("/usr/bin/osascript")
        .args(["-l", "JavaScript", "-e", &script])
        .output()
        .map_err(|_| "无法打开 macOS 安全录入窗口".to_string())?;
    if !output.status.success() {
        return Err("已取消安全录入".to_string());
    }
    let mut value =
        String::from_utf8(output.stdout).map_err(|_| "安全录入返回了无效内容".to_string())?;
    while value.ends_with(['\r', '\n']) {
        value.pop();
    }
    if !optional && value.is_empty() {
        return Err("目标 URL 不能为空".to_string());
    }
    Ok(value)
}

#[cfg(not(target_os = "macos"))]
fn prompt_protected_value(_message: &str, _optional: bool) -> Result<String, String> {
    Err("当前版本仅支持 macOS 原生安全录入".to_string())
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

#[cfg(not(target_os = "macos"))]
fn present_capability(_endpoint: &str, _capability: &str) -> Result<(), String> {
    Err("当前版本仅支持 macOS 原生 Capability 窗口".to_string())
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

fn spawn_sidecar(app: &tauri::AppHandle, token: &SecretToken) -> Result<Child, String> {
    let binary = locate_sidecar(app)?;
    let mut child = Command::new(binary)
        .arg("--control-token-stdin")
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
            let token = generate_control_token().map_err(std::io::Error::other)?;
            let child = spawn_sidecar(app.handle(), &token).map_err(std::io::Error::other)?;
            app.manage(ControlClient {
                socket_path: config_directory.join("control.sock"),
                token,
            });
            app.manage(DaemonProcess(Mutex::new(Some(child))));

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
            create_http_route
        ])
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running Airlock Desktop");
}
