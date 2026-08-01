# Changelog

All notable changes to Airlock are documented here. Airlock follows
[Semantic Versioning](https://semver.org/), with pre-1.0 releases treated as
technical previews whose interfaces may still change.

## [0.1.5-beta] - 2026-08-01

### Added

- Optional per-route interactive SSH shells, disabled by default. Setting
  `allow_interactive_shell: true` (which requires `allow_all_commands: true`)
  lets PuTTY and `ssh` clients enter the upstream shell while Airlock still
  injects the stored upstream credentials; PTY metadata is forwarded only
  while the switch is enabled.
- Plain shell requests on non-interactive routes now receive actionable
  guidance instead of a refused shell, and single-command routes run their
  exact command when a shell is requested.
- Protected keyword egress rewrites for SSH commands.
- Explicit per-route SFTP permission, disabled by default, for modern
  `scp`/SFTP clients.
- Windows x64/x86/arm64 and Linux x64/arm64/ARMv7 Core and CLI
  cross-compilation with owner-only Windows named-pipe control, Credential
  Manager / Secret Service backends, Linux native prompts, and CI preview
  installer contracts that fail closed.

## [0.1.4] - 2026-07-31

### Security

- Upgrade `golang.org/x/crypto` to `v0.52.0` to remediate reachable SSH
  vulnerabilities identified by the production-readiness audit.
- Add bounded SSH connection/session and gateway request capacities to reduce
  listener and stream resource-exhaustion risk.
- Publish the maintainer-run production-readiness security audit and its
  remaining operating boundaries.

## [0.1.3] - 2026-07-31

### Added

- Native desktop listener-port management for HTTP and SSH, including
  current-user process discovery and confirmed graceful termination.
- Custom unprivileged listener ports with offline recovery, automatic restart,
  and rollback on startup failure.

### Changed

- Legacy security settings now receive the default `4768` and `4770` listener
  ports when those fields are absent.

## [0.1.2] - 2026-07-31

### Fixed

- Verify desktop-sidecar readiness without blocking the interface, expose a
  retry action, and show sanitized diagnostics when the local core cannot bind
  its ports or establish its protected control channel.
- Change the npm installer from opening a verified DMG to atomically installing
  the verified app bundle in the current user's `~/Applications` directory.

## [0.1.1] - 2026-07-31

### Added

- Standalone `airlockd` server mode, protected Unix-socket operations CLI, and
  loopback-only token-authenticated Web UI for delegated server operations.
- SSH username-to-host mappings, edit and credential-rotation flows, upstream
  port selection, manual health checks, and sanitized disabled-route attempts.
- Multilingual documentation website, server deployment guide, npm installer
  package, platform release contract, and GitHub Pages deployment workflow.

### Changed

- Default new installations to the prompt-free Standard secret-store profile;
  retain Keychain storage as an explicit stricter option.
- Publish the v0.1.1 Apple Silicon macOS preview artifacts with fixed checksums.

## [0.1.0] - 2026-07-30

### Added

- Native Tauri 2 desktop application with system, light, and dark appearance,
  three accent themes, density controls, reduced motion, and configurable
  refresh cadence.
- Fixed-route HTTP/Wget relay with protected targets, route capability tokens,
  method and path policy, Range downloads, redirect checks, and sanitized
  errors.
- Dual-session SSH gateway with a separate local password or capability,
  upstream host-key pinning, editable exact-command and unrestricted
  non-interactive exec modes, and optional rolling command audit.
- OpenAI-compatible and Anthropic-compatible LLM routes with secondary local
  API keys, model allowlists, output limits, rate and concurrency controls,
  streaming, and optional in-memory token statistics.
- Direct, proxy, and automatic egress through HTTP CONNECT or SOCKS5/SOCKS5H
  proxies, including Clash-compatible local proxy endpoints.
- A prompt-free local `0600` file store as the standard default, an opt-in
  stricter macOS Keychain mode, and guided migration between them.
- Loopback-only ingress by default and an explicitly confirmed private-LAN
  mode for access from other local devices.
- Bilingual README, installation guides, security policy, release notes, and
  static product documentation.

### Release Scope

- The downloadable package supports Apple Silicon Macs running macOS 12 or
  newer.
- The application is ad-hoc signed but is not Developer ID signed or notarized.
- This release has automated test coverage but has not received an independent
  production security audit.

[0.1.0]: https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.0
[0.1.4]: https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.4
[0.1.1]: https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.1
[0.1.2]: https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.2
[0.1.3]: https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.3
