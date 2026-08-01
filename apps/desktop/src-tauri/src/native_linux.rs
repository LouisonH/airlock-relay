use std::{
    io::Write,
    process::{Command, Stdio},
    sync::OnceLock,
};

#[derive(Clone, Copy, PartialEq)]
enum DialogBackend {
    Zenity,
    Kdialog,
}

fn backend() -> DialogBackend {
    static BACKEND: OnceLock<DialogBackend> = OnceLock::new();
    *BACKEND.get_or_init(|| {
        let zenity = Command::new("zenity")
            .arg("--version")
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .map(|status| status.success())
            .unwrap_or(false);
        if zenity {
            DialogBackend::Zenity
        } else {
            DialogBackend::Kdialog
        }
    })
}

fn cancelled(status: std::process::ExitStatus) -> bool {
    status.code() == Some(1)
}

fn run_with_stdin(command: &mut Command, payload: &str) -> Result<String, String> {
    let mut child = command
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|_| "无法打开 Linux 原生窗口".to_string())?;
    let write_result = child
        .stdin
        .take()
        .ok_or_else(|| "Linux 原生窗口管道不可用".to_string())?
        .write_all(payload.as_bytes());
    if let Err(error) = write_result {
        let _ = child.kill();
        let _ = child.wait();
        return Err(format!("无法展示 Linux 原生窗口: {error}"));
    }
    let output = child
        .wait_with_output()
        .map_err(|_| "Linux 原生窗口意外终止".to_string())?;
    if cancelled(output.status) {
        return Err("已取消操作".to_string());
    }
    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!(
            "Linux 原生窗口失败: {}",
            stderr.trim().chars().take(160).collect::<String>()
        ));
    }
    Ok(String::from_utf8_lossy(&output.stdout).into_owned())
}

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
    let result = match backend() {
        DialogBackend::Zenity => {
            let mut command = Command::new("zenity");
            command
                .arg("--entry")
                .arg("--title")
                .arg(super::native_text(title).as_ref())
                .arg("--text")
                .arg(super::native_text(message).as_ref());
            if hidden {
                command.arg("--hide-text");
            }
            if !default_value.is_empty() {
                command.arg("--entry-text").arg(default_value);
            }
            command
                .arg("--ok-label")
                .arg(super::native_text(if optional { "保存" } else { "继续" }).as_ref())
                .arg("--cancel-label")
                .arg(super::native_text(if optional { "不设置" } else { "取消" }).as_ref());
            run_with_stdin(&mut command, "")
        }
        DialogBackend::Kdialog => {
            let mut command = Command::new("kdialog");
            command
                .arg("--title")
                .arg(super::native_text(title).as_ref());
            if hidden {
                command
                    .arg("--password")
                    .arg(super::native_text(message).as_ref());
            } else {
                command
                    .arg("--inputbox")
                    .arg(super::native_text(message).as_ref());
                if !default_value.is_empty() {
                    command.arg(default_value);
                }
            }
            run_with_stdin(&mut command, "")
        }
    }?;
    let mut value = result;
    while value.ends_with(['\r', '\n']) {
        value.pop();
    }
    if !optional && value.is_empty() {
        return Err("安全录入内容不能为空".to_string());
    }
    Ok(value)
}

pub fn choose_llm_local_api_key_mode() -> Result<bool, String> {
    let result = match backend() {
        DialogBackend::Zenity => {
            let mut command = Command::new("zenity");
            command
                .arg("--question")
                .arg("--title")
                .arg(super::native_text("LLM 设置 3/3 · 二次 API Key").as_ref())
                .arg("--text")
                .arg(super::native_text(
                    "为调用者创建一把独立的二次 API Key。它只用于访问 Airlock，真实上游 Key 不会暴露。\n\n随机生成提供 256-bit 强度并仅显示一次；自定义 Key 会要求隐藏输入两次。",
                ).as_ref())
                .arg("--ok-label")
                .arg(super::native_text("自定义").as_ref())
                .arg("--cancel-label")
                .arg(super::native_text("随机生成").as_ref());
            run_with_stdin(&mut command, "")
        }
        DialogBackend::Kdialog => {
            let mut command = Command::new("kdialog");
            command
                .arg("--title")
                .arg(super::native_text("LLM 设置 3/3 · 二次 API Key").as_ref())
                .arg("--yesno")
                .arg(super::native_text(
                    "为调用者创建一把独立的二次 API Key。\n\n随机生成提供 256-bit 强度并仅显示一次；自定义 Key 会要求隐藏输入两次。\n\n选择“是”使用自定义 Key，选择“否”随机生成。",
                ).as_ref());
            run_with_stdin(&mut command, "")
        }
    };
    match result {
        Ok(_) => Ok(true),
        Err(error) if error == "已取消操作" => Ok(false),
        Err(error) => Err(error),
    }
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
    let result = match backend() {
        DialogBackend::Zenity => {
            let mut command = Command::new("zenity");
            command
                .arg("--question")
                .arg("--title")
                .arg(super::native_text(title).as_ref())
                .arg("--text")
                .arg(super::native_text(message).as_ref())
                .arg("--width")
                .arg("520");
            run_with_stdin(&mut command, "")
        }
        DialogBackend::Kdialog => {
            let mut command = Command::new("kdialog");
            command
                .arg("--title")
                .arg(super::native_text(title).as_ref())
                .arg("--yesno")
                .arg(super::native_text(message).as_ref());
            run_with_stdin(&mut command, "")
        }
    };
    match result {
        Ok(_) => Ok(()),
        Err(error) if error == "已取消操作" => Err("已取消".to_string()),
        Err(error) => Err(error),
    }
}

pub fn present_text(title: &str, message: &str, text: &str) -> Result<(), String> {
    let payload = format!("{message}\n\n{text}");
    match backend() {
        DialogBackend::Zenity => {
            let mut command = Command::new("zenity");
            command
                .arg("--text-info")
                .arg("--title")
                .arg(super::native_text(title).as_ref())
                .arg("--width")
                .arg("680")
                .arg("--height")
                .arg("420")
                .arg("--ok-label")
                .arg(super::native_text("完成").as_ref());
            run_with_stdin(&mut command, &payload).map(|_| ())
        }
        DialogBackend::Kdialog => {
            let mut command = Command::new("kdialog");
            command
                .arg("--title")
                .arg(super::native_text(title).as_ref())
                .arg("--msgbox")
                .arg(payload);
            run_with_stdin(&mut command, "").map(|_| ())
        }
    }
}
