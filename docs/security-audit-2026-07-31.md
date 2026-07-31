# Production-Readiness Security Audit - 2026-07-31

**Status:** completed by the Airlock maintainers for v0.1.4.

This is a source and release-readiness audit, not an independent third-party
certification, penetration test, Apple notarization, or a promise that Airlock
is an operating-system sandbox.

## Scope

- Fixed HTTP/Wget and LLM route authorization, request validation, redirect
  handling, egress proxying, and response streaming.
- SSH listener, local authentication, pinned upstream Host Keys, command
  policy, session isolation, audit retention, and LAN exposure.
- Desktop Tauri capabilities, local control transport, persistent secret and
  metadata files, Web UI, CLI, systemd example, npm installer, CI, and Pages.
- Go, npm, and locked Rust dependency metadata.

## Completed Checks

- Upgraded `golang.org/x/crypto` to `v0.52.0`, which fixes the reachable SSH
  vulnerabilities reported against the former `v0.41.0` dependency.
- Added bounded SSH connection and session capacity, plus a global bounded
  in-flight HTTP request capacity. LLM routes retain their narrower per-route
  request-rate and concurrency controls.
- Ran `go vet ./...`, `go test -race ./...`, `govulncheck ./...`, and npm
  production dependency audits for the desktop and installer packages.
- Confirmed control and state files reject symlinks, require protected modes,
  use bounded parsing, and perform atomic writes. Confirmed the Web UI is
  loopback-only with a separate bearer token and that relay ingress remains
  fixed-route rather than open-proxy behavior.

## Required Operating Boundaries

- Use the **Strict / Keychain** profile when Airlock must resist an untrusted
  process running under the same macOS account. The Standard `local_file`
  profile is deliberately prompt-free but stores secrets unencrypted in a
  `0600` file and does not establish a same-user process boundary.
- Keep LAN mode behind a private firewall, VPN, or SSH tunnel. Never expose
  the HTTP or SSH listener to the public Internet.
- Disabling a route blocks new work. It does not terminate an already-running
  HTTP stream or SSH command; use least-privilege upstream accounts and stop
  the core during incident containment when immediate termination is required.
- The macOS package is ad-hoc signed and is not Developer ID signed or
  notarized. Verify the published checksum and install only from the official
  release.

## Follow-up Controls

- Pin GitHub Actions to immutable commit SHAs and add supply-chain provenance
  before a 1.0 release.
- Add an independently commissioned penetration test and Apple Developer ID
  signing/notarization before representing the project as third-party audited.
- Run `cargo audit` in a release environment where the tool is installed; this
  audit validated the locked Cargo metadata but did not install additional
  global tooling solely for the scan.
