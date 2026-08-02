# Install Airlock

[English](installation.md) | [简体中文](installation.zh-CN.md)

## Requirements

Airlock v0.1.6 supports:

- macOS 12 Monterey or newer on Apple Silicon and Intel Macs;
- Windows 10 or newer on x64, x86 (i686), and arm64;
- Linux x64 and arm64 desktop distributions (AppImage).

Linux ARMv7 (Raspberry Pi) remains a Core/CLI-only target in this release.

## Verify the Download

Download these files from the [v0.1.6 release](https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.6):

- `Airlock_0.1.6_aarch64.dmg` (Apple Silicon) or `Airlock_0.1.6_x64.dmg` (Intel)
- `Airlock_0.1.6_x64-setup.exe` / `Airlock_0.1.6_x86-setup.exe` / `Airlock_0.1.6_arm64-setup.exe` (Windows)
- `Airlock_0.1.6_x86_64.AppImage` / `Airlock_0.1.6_aarch64.AppImage` (Linux)
- `SHA256SUMS-v0.1.6.txt` for integrity verification

From the directory containing the downloads, verify the DMG before opening it:

```bash
shasum -a 256 -c SHA256SUMS-v0.1.6.txt
```

The command also checks the ZIP when all three files are in the same directory.

## Install

### Install with npm (recommended)

This command verifies the DMG bundled in the npm package, installs Airlock into
the current user's `~/Applications`, and opens it when finished. It neither
uploads local routes or credentials nor stops another Airlock instance.

```bash
npm install -g airlock-relay && airlock-installer install --open
```

A valid existing Airlock.app is replaced atomically. An incomplete bundle is
replaced only when you explicitly add `--force` after checking the path.

### Install manually

1. Open `Airlock_0.1.6_aarch64.dmg` (or `Airlock_0.1.6_x64.dmg`).
2. Drag **Airlock** into **Applications**.
3. Eject the Airlock disk image.
4. In Finder, open Applications, Control-click **Airlock**, and choose **Open**.
5. Confirm **Open** in the macOS dialog.

### Why macOS Shows a Warning

v0.1.6 is ad-hoc signed so its bundle integrity can be checked, but it is not
signed with an Apple Developer ID and has not been notarized by Apple. macOS
therefore cannot verify the developer identity. This is a release limitation,
not proof that a warning can be ignored: verify the SHA-256 checksum and ensure
the files came from the official GitHub release first.

If Control-click **Open** is unavailable, try opening Airlock once, then go to
**System Settings > Privacy & Security** and choose **Open Anyway** for Airlock.
As a last resort, after verifying the checksum, remove only this app's download
quarantine attribute:

```bash
xattr -dr com.apple.quarantine /Applications/Airlock.app
```

## Install on Windows

```powershell
npm install -g airlock-relay && airlock-installer install --open
```

`airlock-installer` downloads the pinned `Airlock_0.1.6_<arch>-setup.exe`,
verifies its SHA-256 against the release contract, and runs it silently. A
User Account Control prompt appears because the NSIS installer installs for the
machine. SmartScreen may warn because the preview installer is not code-signed.

## Install on Linux

```bash
npm install -g airlock-relay && airlock-installer install --open
```

The installer downloads the pinned AppImage for x64 or arm64, verifies its
SHA-256, and installs it to `~/.local/bin/Airlock.AppImage`. If FUSE is not
available on the distribution, launch it with:

```bash
~/.local/bin/Airlock.AppImage --appimage-extract-and-run
```

Linux artifacts are also GPG-signed. Import the release public key and verify
before running:

```bash
gpg --import Airlock-gpg-pubkey.asc
gpg --verify Airlock_0.1.6_amd64.AppImage.sig Airlock_0.1.6_amd64.AppImage
```

### Raspberry Pi

- **64-bit Raspberry Pi OS** (Pi 4/5): install the arm64 AppImage directly with
  `npm install -g airlock-relay && airlock-installer install`.
- **32-bit Raspberry Pi OS** (armv7/armhf): the desktop bundle is built on the
  Pi itself because no hosted armv7 runner exists. Run
  `bash scripts/build-armv7-desktop.sh` from the repository checkout (see the
  script header for dependencies), then publish the resulting `.deb` and
  `.AppImage` to the release so the installer contract can be updated.

## First Run

- HTTP and LLM routes listen on `127.0.0.1:4768` by default.
- SSH routes listen on `127.0.0.1:4770` by default.
- The desktop control channel is a current-user-only Unix socket; there is no
  Web management port.
- New installations use the **Standard** profile: loopback-only ingress and an
  unencrypted, current-user-only `0600` secret file. This avoids repeated login
  password prompts, but other processes running as the same user may read it.
- The opt-in **Strict** profile uses macOS Keychain. macOS may request the login
  password when Airlock accesses protected Keychain items.
- Private-LAN ingress is disabled by default. Enabling it exposes route entry
  points to other devices on the local network and must never be port-forwarded
  to the public Internet.

## Upgrade

Quit Airlock from its menu-bar item, replace `/Applications/Airlock.app` with
the newer application, and reopen it. Route metadata and credentials are kept
outside the application bundle and remain in place. Read the target version's
release notes before upgrading because pre-1.0 configuration formats may
change.

## Local Data

Airlock stores user-only local state under:

```text
~/Library/Application Support/io.airlock.relay/
```

It can contain:

- `routes.json`: HTTP and LLM route policy and capability digests
- `ssh-routes.json`: SSH route policy and capability digests
- `ssh-command-audit.json`: up to 100 recorded SSH command events
- `security-settings.json`: network and secret-store preferences
- `protected-targets.json`: plaintext secrets only when local-file mode is used
- `control.sock`: transient current-user-only desktop control socket

Keychain mode stores protected target material separately as generic-password
items with service `io.airlock.relay.targets`.

## Uninstall

Quit Airlock, then move `/Applications/Airlock.app` to Trash. This deliberately
does not delete routes, audit history, or credentials, so reinstalling does not
silently lose access configuration.

For a complete removal, also move the exact
`~/Library/Application Support/io.airlock.relay/` folder to Trash. Then open
**Keychain Access**, search for `io.airlock.relay.targets`, review the matches,
and delete the Airlock items you no longer need. Deleting this data cannot be
undone and invalidates the stored route configuration.

## Build from Source

Development requires Go 1.25 or newer, Node.js 20 or newer, Rust/Cargo, and the
Tauri 2 macOS prerequisites:

```bash
git clone https://github.com/LouisonH/airlock-relay.git
cd airlock-relay/apps/desktop
npm install
npm run tauri dev
```
