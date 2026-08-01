# Cross-Platform Core Bootstrap

Airlock v0.1.4 releases an Apple Silicon macOS desktop preview only. This
branch adds a **Core/CLI compilation baseline** for Windows and Linux. It does
not publish a Windows or Linux desktop application, installer, auto-updater,
or signed artifact. The desktop GUI, local control transport, and native
prompt flows are now ported at the code level for Windows and Linux; they still
require real-device runtime acceptance before any release is published.

| Target | Core / CLI build | Local control transport | Platform secret backend | Desktop bundle | Status |
| --- | --- | --- | --- | --- | --- |
| macOS arm64 | Native | owner-only Unix socket | Keychain / protected file | DMG / `.app` | Released preview |
| macOS x64 | Target build | owner-only Unix socket | Keychain / protected file | DMG / `.app` | Installer planned |
| Windows x64 | Cross-compiled | current-owner ACL named pipe | Credential Manager / protected file | NSIS / MSI | Desktop port coded · not released |
| Windows x86 (i686) | Cross-compiled | current-owner ACL named pipe | Credential Manager / protected file | NSIS / MSI | Desktop port coded · not released |
| Windows arm64 | Cross-compiled | current-owner ACL named pipe | Credential Manager / protected file | NSIS / MSI | Desktop port coded · not released |
| Linux x64 | Cross-compiled | owner-only Unix socket | Secret Service / protected file | AppImage / deb | Desktop port coded · not released |
| Linux arm64 | Cross-compiled | owner-only Unix socket | Secret Service / protected file | AppImage / deb | Desktop port coded · not released |
| Linux ARMv7 | Cross-compiled | owner-only Unix socket | Secret Service / protected file | AppImage / deb | Raspberry Pi baseline |

“Cross-compiled” means CI and the target-aware build script compile both
`airlockd` and `airlock` with `CGO_ENABLED=0`. It is not a claim of runtime
acceptance on physical hardware. A target remains unreleased until the runtime
and installer checklist below is completed.

## Core Boundary Implemented Here

- The Go Core and operations CLI share one platform abstraction for local
  control. macOS/Linux use a `0600` Unix domain socket; Windows derives a
  deterministic per-user named pipe and creates it with a protected owner ACL.
  Neither transport opens a control TCP port.
- Desktop-side defaults can use macOS Keychain, Linux Secret Service, or
  Windows Credential Manager. The Windows Credential Manager backend chunks
  encrypted records behind an atomically switched index so oversized protected
  entries do not exceed a generic credential's payload limit.
- The server Core retains its conservative default: `local_file`, an explicit
  protected data directory, and a separate control token file. The `keychain`
  mode is available only where its platform store is configured and working.
- The Rust/Tauri desktop client is platform-aware: the control exchange uses
  the owner-only Unix socket on macOS/Linux and a SHA-256 derived named pipe
  with overlapped I/O on Windows; protected files use `0600`/`0700` permissions
  on Unix and an owner-only `icacls` ACL with atomic replace on Windows.
- Security-sensitive desktop flows use native prompts on every platform:
  `osascript` dialogs on macOS and PowerShell/Windows Forms dialogs on Windows
  for protected input, LLM key choice, high-risk SSH confirmation, capability
  handoff, and security-setting confirmation. Port management on Windows lists
  owners through `netstat -ano` + `Win32_Process`, filters to the current
  account, and ends only confirmed processes with `taskkill`.
- The frontend renders platform-aware labels and zh/en/ja translations for the
  control transport (Unix socket vs named pipe), credential store (Keychain /
  Credential Manager / Secret Service), security profiles, and native risk
  wording.
- Linux native prompts use `zenity` (GNOME) or `kdialog` (KDE), selected at
  runtime with a per-session backend probe. They cover protected input, LLM
  key choice, high-risk SSH confirmation, capability handoff, and
  security-setting confirmation. Linux port-ownership management reads
  `/proc` directly and needs no external `lsof`; desktop bundles still expect
  one prompt backend. Headless servers use the CLI instead.
- CI runs Rust `cargo check` for the Windows x64, Windows x86, Windows arm64, and Linux x64
  desktop targets on every push, so the ported control client is re-verified
  on real target toolchains before runtime acceptance.
- A separate `desktop-windows` workflow builds the NSIS/MSI installers for
  Windows x64, x86 (i686), and arm64 on GitHub-hosted Windows runners and publishes them as
  downloadable artifacts. The artifacts are unsigned until release signing is
  configured, so SmartScreen may warn on first run.
- A `desktop-linux` workflow builds `deb` and AppImage installers for Linux x64
  on an x64 runner and Linux arm64 on a native ARM runner. Packages do not
  bundle `zenity`/`kdialog`; see the runtime note above.
- Build targets are explicit and isolated. A target build creates both
  `airlockd` and `airlock`; it does not create a Tauri bundle or alter the
  released npm installer contract.

## Build the Core and CLI

Go 1.25 or newer is required. From the repository root, the following
cross-compile without a target toolchain and place output under `bin/<target>`:

```bash
node scripts/build-sidecar.mjs windows-amd64
node scripts/build-sidecar.mjs windows-arm64
node scripts/build-sidecar.mjs windows-386
node scripts/build-sidecar.mjs linux-amd64
node scripts/build-sidecar.mjs linux-arm64
node scripts/build-sidecar.mjs linux-armv7
```

`scripts/build-sidecar.sh` remains as a compatibility wrapper around the same
Node driver. The default command, without an argument, writes the desktop
developer sidecars into `apps/desktop/src-tauri/binaries/` using Tauri's
target-triple naming (`airlockd-aarch64-apple-darwin` and so on); an explicit
target writes plain `airlockd`/`airlock` names under `bin/<target>`. The target
names are deliberately limited to the contract table above; an unknown target
fails before a binary is written. Run `node scripts/build-sidecar.mjs --help`
to list targets and output rules.

For a Raspberry Pi 3/4 running a 32-bit Raspberry Pi OS or Debian `armhf`
system, copy `bin/linux-armv7/airlockd` and `bin/linux-armv7/airlock` to the
device. Run them as a non-login service account using the [Server Core
deployment guide](server-deployment.md). A 64-bit Raspberry Pi OS uses the
`linux-arm64` target instead. Desktop packaging for either Pi target is not
available in this stage.

## Runtime Acceptance Still Required

Before Windows or Linux becomes a release target, maintainers must complete
these checks on each supported architecture and distribution:

1. Exercise create, read, rotation, and deletion through Windows Credential
   Manager or a real freedesktop.org Secret Service session, including a
   locked or unavailable store.
2. Verify owner-only access to the Windows named pipe and protected state/token
   paths from a different local account; verify `0600` Unix socket and state
   protections on Linux.
3. Compile the Rust/Tauri control client on real Windows and Linux toolchains
   and verify the ported named-pipe/Unix control exchange, protected-file ACLs,
   Windows Forms native prompts, port management, and frontend platform labels
   on physical devices.
4. Test service installation, clean removal, upgrades, recovery after a stale
   process, `Direct`/`Proxy`/`Auto` egress, SSH host-key pinning, and failure
   closure on target hardware.
5. Produce architecture-specific installers, sign them, publish fixed
   checksums, and test install/update/uninstall independently.

Until those checks pass, `airlock-installer` intentionally reports Windows and
Linux as `planned` and refuses to install a nonexistent artifact. This avoids
mistaking a compilable core for a supported desktop product.

## Security Invariants

SSH username mapping, fixed upstream routing, local capability tokens,
secondary LLM API keys, audit redaction, and proxy egress policy are all in the
shared Go Core. Their behavior does not change by platform. The caller still
receives only a local endpoint and local credential; upstream URLs, passwords,
private keys, host keys, and API keys remain in the selected protected store.

Read the [server deployment guide](server-deployment.md) for service-mode
commands and the [security policy](../SECURITY.md) before deploying a Core
build outside a development environment.
