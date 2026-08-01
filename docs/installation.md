# Install Airlock on macOS

[English](installation.md) | [简体中文](installation.zh-CN.md)

## Requirements

Airlock v0.1.5 supports Apple Silicon Macs running macOS 12 Monterey or newer.
This release does not include an Intel, Windows, or Linux installer.

## Verify the Download

Download these files from the [v0.1.5 release](https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.5):

- `Airlock_0.1.5_aarch64.dmg` for normal installation
- `Airlock_0.1.5_aarch64.app.zip` as a portable archive
- `SHA256SUMS-v0.1.5.txt` for integrity verification

From the directory containing the downloads, verify the DMG before opening it:

```bash
shasum -a 256 -c SHA256SUMS-v0.1.5.txt
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

1. Open `Airlock_0.1.5_aarch64.dmg`.
2. Drag **Airlock** into **Applications**.
3. Eject the Airlock disk image.
4. In Finder, open Applications, Control-click **Airlock**, and choose **Open**.
5. Confirm **Open** in the macOS dialog.

### Why macOS Shows a Warning

v0.1.5 is ad-hoc signed so its bundle integrity can be checked, but it is not
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
