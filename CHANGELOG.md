# Changelog

All notable changes to Airlock are documented here. Airlock follows
[Semantic Versioning](https://semver.org/), with pre-1.0 releases treated as
technical previews whose interfaces may still change.

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
