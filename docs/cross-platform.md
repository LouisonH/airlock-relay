# Cross-Platform Adaptation Plan

Airlock v0.1.1 currently ships only for Apple Silicon Macs. The entries below
are implementation contracts, not claims that installers already exist.

| Target | Bundle | Local control transport | Protected store | Status |
| --- | --- | --- | --- | --- |
| macOS arm64 | DMG / `.app` | user-only Unix socket | 0600 file / Keychain | Released preview |
| macOS x64 | DMG / `.app` | user-only Unix socket | 0600 file / Keychain | Contract only |
| Windows x64 | NSIS / MSI | user-ACL named pipe | protected file / Credential Manager | Contract only |
| Linux x64 | AppImage / deb | 0600 Unix socket | protected file / Secret Service | Contract only |
| Linux arm64 | AppImage / deb | 0600 Unix socket | protected file / Secret Service | Contract only |

The reusable resolver in `packages/airlock/lib/platform.mjs` exposes this
matrix to packaging tools. It fails closed unless the selected target has both
a published artifact name and a fixed SHA-256 digest.

## Adaptation Boundaries

1. Extract the desktop control transport behind a platform interface. Keep Unix
   sockets on macOS/Linux and use a current-user ACL named pipe on Windows.
2. Keep `airlockd` as the shared Go core and build a sidecar per target. CI must
   run race tests natively and verify architecture before bundling.
3. Keep SSH entry in the cross-platform Airlock wizard and send it through a
   one-shot local IPC command. Platform-native prompts remain available for
   other secrets and security-sensitive OS changes. No secret may enter the
   command line, environment, process list, logs, or persistent control state.
4. Add Windows Credential Manager and Linux Secret Service backends with the
   existing copy-verify-switch-cleanup migration contract.
5. Sign and verify each platform artifact independently. A target remains
   `planned` until installation, removal, update, and fail-closed tests pass.

## Version and Update Contract

The desktop version check is explicit and read-only. After a user requests it,
the desktop WebView reads only the official public GitHub Releases metadata.
It never sends local route state, protected targets, credentials, or activity
data; it never downloads, installs, restarts, or opens a release automatically.
Each platform must independently verify a user-selected installer before an
update flow can be described as released.

SSH username mappings remain platform-independent: the local username chooses
one route on a shared listener, while each route retains its own capability
digest and protected upstream target.
