use serde_json::{json, Value};
use std::{
    io::Write,
    process::{Command, Stdio},
};

const POWERSHELL: &str = r"C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe";

const INPUT_SCRIPT: &str = r#"
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$form = New-Object System.Windows.Forms.Form
$form.Text = $payload.title
$form.ClientSize = New-Object System.Drawing.Size(560, 250)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.MinimizeBox = $false
$label = New-Object System.Windows.Forms.Label
$label.Location = New-Object System.Drawing.Point(18, 16)
$label.Size = New-Object System.Drawing.Size(524, 74)
$label.Text = $payload.message
$text = New-Object System.Windows.Forms.TextBox
$text.Location = New-Object System.Drawing.Point(18, 100)
$text.Size = New-Object System.Drawing.Size(524, 30)
if ($payload.hidden) { $text.UseSystemPasswordChar = $true }
$text.Text = [string]$payload.defaultValue
$accept = New-Object System.Windows.Forms.Button
$accept.Text = $payload.accept
$accept.DialogResult = 'OK'
$accept.Location = New-Object System.Drawing.Point(330, 165)
$accept.Size = New-Object System.Drawing.Size(100, 32)
$cancel = New-Object System.Windows.Forms.Button
$cancel.Text = $payload.cancel
$cancel.DialogResult = 'Cancel'
$cancel.Location = New-Object System.Drawing.Point(442, 165)
$cancel.Size = New-Object System.Drawing.Size(100, 32)
$form.Controls.AddRange(@($label, $text, $accept, $cancel))
$form.AcceptButton = $accept
$form.CancelButton = $cancel
$result = $form.ShowDialog()
if ($result -eq [System.Windows.Forms.DialogResult]::OK) {
  [Console]::Out.Write($text.Text)
  exit 0
}
if ($payload.optional) { exit 0 }
exit 1
"#;

const CHOOSE_MODE_SCRIPT: &str = r#"
Add-Type -AssemblyName System.Windows.Forms
$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$result = [System.Windows.Forms.MessageBox]::Show(
  $payload.message,
  $payload.title,
  [System.Windows.Forms.MessageBoxButtons]::YesNoCancel,
  [System.Windows.Forms.MessageBoxIcon]::Question
)
if ($result -eq [System.Windows.Forms.DialogResult]::Yes) {
  [Console]::Out.Write('custom')
  exit 0
}
if ($result -eq [System.Windows.Forms.DialogResult]::No) {
  [Console]::Out.Write('random')
  exit 0
}
exit 1
"#;

const CONFIRM_SCRIPT: &str = r#"
Add-Type -AssemblyName System.Windows.Forms
$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$result = [System.Windows.Forms.MessageBox]::Show(
  $payload.message,
  $payload.title,
  [System.Windows.Forms.MessageBoxButtons]::YesNo,
  [System.Windows.Forms.MessageBoxIcon]::Warning
)
if ($result -eq [System.Windows.Forms.DialogResult]::Yes) { exit 0 }
exit 1
"#;

const TEXT_SCRIPT: &str = r#"
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$form = New-Object System.Windows.Forms.Form
$form.Text = $payload.title
$form.ClientSize = New-Object System.Drawing.Size(620, 330)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.MinimizeBox = $false
$label = New-Object System.Windows.Forms.Label
$label.Location = New-Object System.Drawing.Point(18, 16)
$label.Size = New-Object System.Drawing.Size(584, 56)
$label.Text = $payload.message
$text = New-Object System.Windows.Forms.TextBox
$text.Location = New-Object System.Drawing.Point(18, 84)
$text.Size = New-Object System.Drawing.Size(584, 170)
$text.Multiline = $true
$text.ReadOnly = $true
$text.ScrollBars = 'Vertical'
$text.Text = $payload.text
$copy = New-Object System.Windows.Forms.Button
$copy.Text = $payload.copy
$copy.Location = New-Object System.Drawing.Point(330, 268)
$copy.Size = New-Object System.Drawing.Size(110, 32)
$copy.Add_Click({ [System.Windows.Forms.Clipboard]::SetText($payload.text) })
$ok = New-Object System.Windows.Forms.Button
$ok.Text = $payload.done
$ok.DialogResult = 'OK'
$ok.Location = New-Object System.Drawing.Point(452, 268)
$ok.Size = New-Object System.Drawing.Size(150, 32)
$form.Controls.AddRange(@($label, $text, $copy, $ok))
$form.AcceptButton = $ok
if ($form.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { exit 0 }
exit 1
"#;

pub fn prompt_protected_value(message: &str, optional: bool) -> Result<String, String> {
    prompt_native_value(message, optional, true, "")
}

pub fn prompt_native_value(
    message: &str,
    optional: bool,
    hidden: bool,
    default_value: &str,
) -> Result<String, String> {
    prompt_native_value_with_title("Airlock 安全录入", message, optional, hidden, default_value)
}

pub fn prompt_native_value_with_title(
    title: &str,
    message: &str,
    optional: bool,
    hidden: bool,
    default_value: &str,
) -> Result<String, String> {
    let payload = json!({
        "title": super::native_text(title),
        "message": super::native_text(message),
        "optional": optional,
        "hidden": hidden,
        "defaultValue": default_value,
        "accept": super::native_text(if optional { "保存" } else { "继续" }),
        "cancel": super::native_text(if optional { "不设置" } else { "取消" }),
    });
    let mut value = run_script(INPUT_SCRIPT, &payload)?;
    while value.ends_with(['\r', '\n']) {
        value.pop();
    }
    if !optional && value.is_empty() {
        return Err("安全录入内容不能为空".to_string());
    }
    Ok(value)
}

pub fn choose_llm_local_api_key_mode() -> Result<bool, String> {
    let payload = json!({
        "title": super::native_text("LLM 设置 3/3 · 二次 API Key"),
        "message": super::native_text("为调用者创建一把独立的二次 API Key。它只用于访问 Airlock，真实上游 Key 不会暴露。\n\n随机生成提供 256-bit 强度并仅显示一次；自定义 Key 会要求隐藏输入两次。"),
    });
    let choice = run_script(CHOOSE_MODE_SCRIPT, &payload)?;
    Ok(choice.trim() == "custom")
}

pub fn prompt_llm_local_api_key() -> Result<(String, bool), String> {
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
        super::clear_string(&mut local_api_key);
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
            super::clear_string(&mut local_api_key);
            return Err(error);
        }
    };
    let matches = local_api_key == confirmation;
    super::clear_string(&mut confirmation);
    if !matches {
        super::clear_string(&mut local_api_key);
        return Err("两次输入的本地 API Key 不一致".to_string());
    }
    Ok((local_api_key, true))
}

pub fn confirm_yes_no(title: &str, message: &str) -> Result<(), String> {
    let payload = json!({
        "title": title,
        "message": message,
    });
    run_script(CONFIRM_SCRIPT, &payload).map(|_| ())
}

pub fn present_text(title: &str, message: &str, text: &str) -> Result<(), String> {
    let payload = json!({
        "title": title,
        "message": message,
        "text": text,
        "copy": super::native_text("复制"),
        "done": super::native_text("完成"),
    });
    run_script(TEXT_SCRIPT, &payload).map(|_| ())
}

fn run_script(script: &str, payload: &Value) -> Result<String, String> {
    let mut payload_bytes = payload.to_string().into_bytes();
    payload_bytes.push(b'\n');
    let mut child = Command::new(POWERSHELL)
        .args([
            "-NoProfile",
            "-Sta",
            "-NonInteractive",
            "-ExecutionPolicy",
            "Bypass",
            "-Command",
            script,
        ])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|_| "无法打开 Windows 原生确认窗口".to_string())?;
    let write_result = child
        .stdin
        .take()
        .ok_or_else(|| "Windows 原生确认管道不可用".to_string())?
        .write_all(&payload_bytes);
    payload_bytes.fill(0);
    if let Err(error) = write_result {
        let _ = child.kill();
        let _ = child.wait();
        return Err(format!("无法展示 Windows 原生窗口: {error}"));
    }
    let output = child
        .wait_with_output()
        .map_err(|_| "Windows 原生窗口意外终止".to_string())?;
    if !output.status.success() {
        if output.status.code() == Some(1) {
            return Err("已取消操作".to_string());
        }
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!(
            "Windows 原生窗口失败: {}",
            stderr.trim().chars().take(160).collect::<String>()
        ));
    }
    Ok(String::from_utf8_lossy(&output.stdout).into_owned())
}
