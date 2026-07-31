# Airlock Server Core Deployment and CLI

This guide runs `airlockd` without the desktop application. It is intended for servers, NAS devices, bastions, and operations hosts: the core runs persistently, the `airlock` CLI administers fixed HTTP/Wget, SSH, and LLM routes over a user-only Unix socket, and an optional Web UI exposes sanitized operational state.

Airlock is still a fixed-route relay, not an open proxy or VPN. A caller cannot submit an arbitrary target.

## Components and Security Boundary

```text
caller / wget / ssh / LLM client
        | route alias + local capability
        v
airlockd core (HTTP :4768, SSH :4770) --> fixed upstream
        ^                 |
        | Unix socket     | protected local SecretStore
airlock CLI               v
                  /var/lib/airlock (0700)

Optional Web UI (:4769, loopback only + separate bearer token)
```

- `airlockd`: the Go core. Server mode defaults to the local `0600` SecretStore and has no Tauri/Desktop dependency.
- `airlock`: the operations CLI. It uses the service account's Unix socket and never accepts upstream credentials as command arguments.
- Web UI: disabled by default; separate from relay ingress. It can show sanitized status and run health/enable/disable/stop-all operations. It cannot create routes, receive credentials, show upstream targets, or delete routes.
- Desktop: an optional local GUI control plane with no runtime dependency from the server core. Never expose a desktop control socket to the network.

## Build and Prepare

Go 1.25 or newer is required. Run the daemon as a dedicated non-login user rather than root.

```bash
go build -trimpath -o /usr/local/bin/airlockd ./cmd/airlockd
go build -trimpath -o /usr/local/bin/airlock ./cmd/airlock
sudo useradd --system --create-home --shell /usr/sbin/nologin airlock
sudo install -d -o airlock -g airlock -m 0700 /var/lib/airlock
sudo install -d -o airlock -g airlock -m 0700 /etc/airlock
```

Create two different tokens as the service user. The command writes a new `0600` file and does not echo token content:

```bash
sudo -u airlock /usr/local/bin/airlock token generate --output /etc/airlock/control.token
sudo -u airlock /usr/local/bin/airlock token generate --output /etc/airlock/web.token
```

`control.token` authenticates the CLI-to-core channel. `web.token` is only for the Web UI browser login. Do not reuse either token as a route capability.

## Start the Core

Server mode requires an absolute data directory and protected control-token file:

```bash
sudo -u airlock /usr/local/bin/airlockd \
  --mode server \
  --data-dir /var/lib/airlock \
  --control-token-file /etc/airlock/control.token \
  --listen 127.0.0.1:4768 \
  --ssh-listen 127.0.0.1:4770 \
  --web-listen 127.0.0.1:4769 \
  --web-token-file /etc/airlock/web.token
```

Omit both Web UI flags to disable it. The Web UI accepts loopback addresses only. Access it remotely through an SSH local forward:

```bash
ssh -L 4769:127.0.0.1:4769 operator@example-server
```

Then open `http://127.0.0.1:4769` locally and paste the Web UI token from its protected file. The browser keeps it in tab-scoped `sessionStorage` only.

HTTP and SSH ingress are loopback-only by default. For private-LAN access, explicitly use `--network-scope lan` and a private address or `0.0.0.0`, then enforce access with a firewall, VPN, or SSH tunnel. Do not directly expose relay or Web UI ports to the public internet.

Install the provided [systemd service example](../deploy/systemd/airlock.service.example) as `/etc/systemd/system/airlock.service`, then run:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now airlock
sudo systemctl status airlock
```

## Operations CLI

Run the CLI as the `airlock` service user so it can access the `0600` control socket:

```bash
sudo -u airlock /usr/local/bin/airlock \
  --data-dir /var/lib/airlock \
  --token-file /etc/airlock/control.token \
  status
```

```bash
# Sanitized route summaries
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes list

# Enable, health-check, disable, stop all, or delete a route
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable releases
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes health releases
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes disable releases
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes stop-all --yes
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes delete releases
```

`--socket /var/lib/airlock/control.sock` can replace `--data-dir`. Output is JSON. A generated local capability or secondary API key appears only in the creation command's one output; do not send it to shell history, CI logs, or tickets.

## Protected Route Specifications

The CLI reads upstream data only from an absolute, regular, non-symlink `0600` JSON file. Do not pass JSON, passwords, keys, or upstream URLs in flags.

```bash
install -m 0600 /dev/null /etc/airlock/releases.json
editor /etc/airlock/releases.json
```

HTTP/Wget example:

```json
{
  "name": "Release mirror",
  "alias": "releases",
  "base_url": "https://upstream.example.invalid/releases/",
  "authorization": "Bearer upstream-secret",
  "egress": "Auto"
}
```

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes create http --file /etc/airlock/releases.json
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable releases
wget --header="Authorization: Bearer <local-capability>" http://127.0.0.1:4768/r/releases/file.tar.gz
```

The local capability is not the upstream Authorization value. HTTP routes are fixed to GET/HEAD under the configured upstream base URL.

LLM example:

```json
{
  "name": "Coding model",
  "alias": "coding",
  "base_url": "https://api.example.invalid/v1",
  "authorization": "upstream-api-key",
  "provider": "openai",
  "models": ["example-coding"],
  "max_output_tokens": 4096,
  "requests_per_minute": 60,
  "max_concurrent": 4,
  "track_usage": true,
  "egress": "Auto"
}
```

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes create llm --file /etc/airlock/coding.json
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable coding
export OPENAI_BASE_URL=http://127.0.0.1:4768/r/coding
export OPENAI_API_KEY='<local-secondary-api-key>'
```

For `provider: "anthropic"`, `authorization` becomes the protected upstream `X-Api-Key`; OpenAI-compatible mode adds the upstream Bearer prefix. Usage tracking retains numbers only, never prompts or response bodies.

## SSH Mapping

Probe and manually verify the upstream host key first. SSH defaults to port `22` when no port is supplied.

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token ssh probe --address ssh.example.invalid:22 --egress Auto
```

Copy the returned `host_key` exactly into a protected spec. `local_username` selects the route at the local SSH listener, so different local usernames can map to independent upstream accounts at one address.

```json
{
  "name": "Build host",
  "alias": "build-host",
  "local_username": "build",
  "address": "ssh.example.invalid:22",
  "username": "upstream-build",
  "password": "upstream-password",
  "local_password": "a-long-local-password",
  "expected_host_key": "BASE64_VALUE_FROM_PROBE",
  "allowed_command": "uptime",
  "record_commands": true,
  "egress": "Auto"
}
```

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes create ssh --file /etc/airlock/build-host.json
ssh build@127.0.0.1 -p 4770 uptime
```

SSH creation stores the route and protected target **disabled**. It does not connect to the upstream or delete configuration automatically. First run an explicit health check, then enable only after it succeeds:

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes health build-host
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable build-host
```

The health check verifies the pinned Host Key and upstream password within the route authentication budget (20 seconds by default; 3-120 seconds). `allowed_command` is an exact match. `allow_all_commands: true` additionally requires `--allow-all-confirmed`; unrestricted non-interactive exec is close to remote code execution, so use a dedicated least-privilege account. Shells, PTYs, SFTP, port forwarding, and Agent/X11 forwarding remain denied.

## Proxy Egress and Lifecycle

Use an existing Clash-compatible HTTP CONNECT, HTTPS CONNECT, SOCKS5, or SOCKS5H proxy for fixed upstream routes. The proxy URL also belongs in a protected file:

```json
{ "url": "socks5://127.0.0.1:7890" }
```

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token proxy set --file /etc/airlock/proxy.json
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token proxy clear --yes
```

Choose `Direct`, `Proxy`, or `Auto` per route. `Auto` retries through the configured proxy only after a retryable direct connectivity failure before a response starts; it does not replay a partially handled application request.

Stop the service before backing up the full data directory and both token files. Treat backups as secrets. After an upgrade, run `status` and health checks for critical routes. `local_file` relies on OS file permissions; local administrators, root, and processes able to debug the daemon are outside this threat model.

Read the [security policy](../SECURITY.md) and the [Chinese guide](server-deployment.zh-CN.md) before production use.
