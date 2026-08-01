use std::{
    fs::{File, OpenOptions},
    io,
    path::Path,
    time::Duration,
};

#[cfg(unix)]
pub fn local_control_endpoint(directory: &Path) -> String {
    directory
        .join("control.sock")
        .to_string_lossy()
        .into_owned()
}

#[cfg(windows)]
pub fn local_control_endpoint(directory: &Path) -> String {
    use sha2::{Digest, Sha256};

    let normalized = directory.to_string_lossy().to_lowercase();
    let digest = Sha256::digest(normalized.as_bytes());
    let mut suffix = String::with_capacity(24);
    for byte in &digest[..12] {
        suffix.push_str(&format!("{byte:02x}"));
    }
    format!(r"\\.\pipe\airlock-relay-{suffix}")
}

#[cfg(unix)]
pub fn exchange_control(endpoint: &str, payload: &[u8], timeout: Duration) -> io::Result<String> {
    use std::{
        io::{BufRead, BufReader, Write},
        os::unix::net::UnixStream,
    };

    let mut stream = UnixStream::connect(endpoint)?;
    stream.set_read_timeout(Some(timeout))?;
    stream.set_write_timeout(Some(timeout))?;
    stream.write_all(payload)?;
    let mut raw = String::new();
    BufReader::new(stream).read_line(&mut raw)?;
    Ok(raw)
}

#[cfg(windows)]
pub fn exchange_control(endpoint: &str, payload: &[u8], timeout: Duration) -> io::Result<String> {
    windows_control::exchange(endpoint, payload, timeout)
}

#[cfg(windows)]
mod windows_control {
    use super::*;
    use std::os::windows::ffi::OsStrExt;
    use windows_sys::Win32::{
        Foundation::{
            CloseHandle, GetLastError, ERROR_IO_PENDING, ERROR_PIPE_BUSY, GENERIC_READ,
            GENERIC_WRITE, HANDLE, INVALID_HANDLE_VALUE, WAIT_OBJECT_0, WAIT_TIMEOUT,
        },
        Storage::FileSystem::{
            CreateFileW, ReadFile, WriteFile, FILE_ATTRIBUTE_NORMAL, FILE_FLAG_OVERLAPPED,
            OPEN_EXISTING,
        },
        System::Threading::{CreateEventW, WaitForSingleObject},
        System::IO::{GetOverlappedResult, OVERLAPPED},
    };

    pub(super) fn exchange(
        endpoint: &str,
        payload: &[u8],
        timeout: Duration,
    ) -> io::Result<String> {
        let pipe = connect(endpoint)?;
        let event = EventHandle::create()?;
        let timeout_ms = timeout.as_millis().min(u32::MAX as u128) as u32;

        let mut overlapped: OVERLAPPED = unsafe { std::mem::zeroed() };
        overlapped.hEvent = event.0;
        let written = unsafe {
            wait_for_io(
                pipe.0,
                event.0,
                &mut overlapped,
                timeout_ms,
                |transferred| {
                    WriteFile(
                        pipe.0,
                        payload.as_ptr() as *const _,
                        payload.len() as u32,
                        transferred,
                        &mut overlapped,
                    )
                },
            )
        }?;
        if written as usize != payload.len() {
            return Err(io::Error::new(
                io::ErrorKind::WriteZero,
                "control request was not fully written",
            ));
        }

        let mut raw = Vec::new();
        let mut chunk = [0_u8; 4096];
        loop {
            let mut overlapped: OVERLAPPED = unsafe { std::mem::zeroed() };
            overlapped.hEvent = event.0;
            let read = unsafe {
                wait_for_io(
                    pipe.0,
                    event.0,
                    &mut overlapped,
                    timeout_ms,
                    |transferred| {
                        ReadFile(
                            pipe.0,
                            chunk.as_mut_ptr() as *mut _,
                            chunk.len() as u32,
                            transferred,
                            &mut overlapped,
                        )
                    },
                )
            }?;
            if read == 0 {
                break;
            }
            raw.extend_from_slice(&chunk[..read as usize]);
            if raw.len() > 64 << 10 {
                return Err(io::Error::new(
                    io::ErrorKind::InvalidData,
                    "control response is too large",
                ));
            }
            if raw.contains(&b'\n') {
                break;
            }
        }
        String::from_utf8(raw).map_err(|_| {
            io::Error::new(io::ErrorKind::InvalidData, "control response is not UTF-8")
        })
    }

    fn connect(endpoint: &str) -> io::Result<PipeHandle> {
        let wide: Vec<u16> = Path::new(endpoint)
            .as_os_str()
            .encode_wide()
            .chain(Some(0))
            .collect();
        let mut last_error = 0_u32;
        for _ in 0..50 {
            let handle = unsafe {
                CreateFileW(
                    wide.as_ptr(),
                    GENERIC_READ | GENERIC_WRITE,
                    0,
                    std::ptr::null(),
                    OPEN_EXISTING,
                    FILE_FLAG_OVERLAPPED | FILE_ATTRIBUTE_NORMAL,
                    std::ptr::null_mut(),
                )
            };
            if handle != INVALID_HANDLE_VALUE {
                return Ok(PipeHandle(handle));
            }
            let error = unsafe { GetLastError() };
            last_error = error;
            if error != ERROR_PIPE_BUSY {
                break;
            }
            std::thread::sleep(Duration::from_millis(100));
        }
        Err(io::Error::from_raw_os_error(last_error as i32))
    }

    unsafe fn wait_for_io(
        pipe: HANDLE,
        event: HANDLE,
        overlapped: *mut OVERLAPPED,
        timeout_ms: u32,
        start: impl FnOnce(*mut u32) -> i32,
    ) -> io::Result<u32> {
        let mut transferred: u32 = 0;
        if start(&mut transferred) != 0 {
            return Ok(transferred);
        }
        let error = GetLastError();
        if error != ERROR_IO_PENDING {
            return Err(io::Error::from_raw_os_error(error as i32));
        }
        let wait = WaitForSingleObject(event, timeout_ms);
        if wait == WAIT_TIMEOUT {
            return Err(io::Error::new(
                io::ErrorKind::TimedOut,
                "control channel timed out",
            ));
        }
        if wait != WAIT_OBJECT_0 {
            return Err(io::Error::last_os_error());
        }
        if GetOverlappedResult(pipe, overlapped, &mut transferred, 0) == 0 {
            return Err(io::Error::last_os_error());
        }
        Ok(transferred)
    }

    struct PipeHandle(HANDLE);

    impl Drop for PipeHandle {
        fn drop(&mut self) {
            unsafe {
                let _ = CloseHandle(self.0);
            }
        }
    }

    struct EventHandle(HANDLE);

    impl EventHandle {
        fn create() -> io::Result<Self> {
            let event = unsafe { CreateEventW(std::ptr::null(), 0, 0, std::ptr::null()) };
            if event.is_null() {
                return Err(io::Error::last_os_error());
            }
            Ok(Self(event))
        }
    }

    impl Drop for EventHandle {
        fn drop(&mut self) {
            unsafe {
                let _ = CloseHandle(self.0);
            }
        }
    }
}

#[cfg(not(windows))]
pub fn sidecar_binary_name() -> &'static str {
    "airlockd"
}

#[cfg(windows)]
pub fn sidecar_binary_name() -> &'static str {
    "airlockd.exe"
}

#[cfg(not(windows))]
pub fn sidecar_bundle_name() -> Option<String> {
    None
}

#[cfg(windows)]
pub fn sidecar_bundle_name() -> Option<String> {
    let triple = match std::env::consts::ARCH {
        "x86_64" => "x86_64-pc-windows-msvc",
        "aarch64" => "aarch64-pc-windows-msvc",
        "x86" => "i686-pc-windows-msvc",
        _ => return None,
    };
    Some(format!("airlockd-{triple}.exe"))
}

#[cfg(unix)]
pub fn protect_directory(path: &Path) -> io::Result<()> {
    use std::os::unix::fs::PermissionsExt;

    std::fs::create_dir_all(path)?;
    std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o700))
}

#[cfg(unix)]
pub fn open_private_file(path: &Path, create_new: bool, truncate: bool) -> io::Result<File> {
    use std::os::unix::fs::OpenOptionsExt;

    let mut options = OpenOptions::new();
    options.write(true).mode(0o600);
    if create_new {
        options.create_new(true);
    } else {
        options.create(true).truncate(truncate);
    }
    options.open(path)
}

#[cfg(unix)]
pub fn replace_file(source: &Path, destination: &Path) -> io::Result<()> {
    std::fs::rename(source, destination)
}

#[cfg(windows)]
pub fn protect_directory(path: &Path) -> io::Result<()> {
    std::fs::create_dir_all(path)?;
    restrict_acl(path)
}

#[cfg(windows)]
pub fn open_private_file(path: &Path, create_new: bool, truncate: bool) -> io::Result<File> {
    let mut options = OpenOptions::new();
    options.write(true);
    if create_new {
        options.create_new(true);
    } else {
        options.create(true).truncate(truncate);
    }
    let file = options.open(path)?;
    restrict_acl(path)?;
    Ok(file)
}

#[cfg(windows)]
pub fn replace_file(source: &Path, destination: &Path) -> io::Result<()> {
    use std::os::windows::ffi::OsStrExt;
    use windows_sys::Win32::Storage::FileSystem::{
        MoveFileExW, MOVEFILE_REPLACE_EXISTING, MOVEFILE_WRITE_THROUGH,
    };

    let source: Vec<u16> = source.as_os_str().encode_wide().chain(Some(0)).collect();
    let destination: Vec<u16> = destination
        .as_os_str()
        .encode_wide()
        .chain(Some(0))
        .collect();
    let ok = unsafe {
        MoveFileExW(
            source.as_ptr(),
            destination.as_ptr(),
            MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH,
        )
    };
    if ok == 0 {
        return Err(io::Error::last_os_error());
    }
    Ok(())
}

#[cfg(windows)]
fn restrict_acl(path: &Path) -> io::Result<()> {
    use std::process::Command;

    let escaped = path.to_string_lossy().replace('\'', "''");
    let script = format!(
        "$p='{escaped}'; $u=[System.Security.Principal.WindowsIdentity]::GetCurrent().Name; \
         & icacls $p /inheritance:r /grant:r \"$($u):F\" | Out-Null; \
         if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}"
    );
    let status = Command::new("powershell.exe")
        .args(["-NoProfile", "-NonInteractive", "-Command", &script])
        .status()?;
    if !status.success() {
        return Err(io::Error::new(
            io::ErrorKind::PermissionDenied,
            "failed to restrict Windows ACL",
        ));
    }
    Ok(())
}
