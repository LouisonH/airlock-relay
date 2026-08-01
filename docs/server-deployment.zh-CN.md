# Airlock Server Core 部署与命令行

本指南部署无桌面端的 `airlockd`。它适合服务器、NAS、跳板机和运维主机：核心作为长期服务运行，`airlock` CLI 通过当前服务账户专属 Unix Socket 管理固定 HTTP/Wget、SSH 和 LLM 路由；可选 Web UI 只提供脱敏状态、启停与健康检查。

Airlock 仍然不是开放代理或 VPN。每条路由只会转发到创建时写入受保护存储的一个上游。调用方不能用 Airlock 指定任意目标。

## 组件与边界

```text
调用方 / wget / ssh / LLM client
       | 路由别名 + 本地 capability
       v
airlockd 核心 (HTTP :4768, SSH :4770) --> 固定上游
       ^                  |
       | Unix Socket      | 受保护的本地 SecretStore
airlock CLI               v
                 /var/lib/airlock (0700)

可选 Web UI (:4769, 仅 loopback + 独立 token)
```

- `airlockd`：不依赖 Tauri 或桌面 GUI 的 Go 核心；服务模式默认使用本地 `0600` SecretStore。
- `airlock`：运维 CLI。它只连接同一服务账户可读的 Unix Socket，不会把上游 URL、密码或 API Key 放入命令行参数。
- Web UI：独立于数据入口；默认关闭。不能创建路由、录入或显示上游凭据，也不能删除路由。
- Desktop：可选的本地 GUI 控制面，和服务器核心没有运行时依赖。不要把桌面控制 Socket 暴露到网络。

## 构建与准备

需要 Go 1.25 或更新版本。以下示例使用专用的非登录账户 `airlock`；不要用 root 运行守护进程。

```bash
go build -trimpath -o /usr/local/bin/airlockd ./cmd/airlockd
go build -trimpath -o /usr/local/bin/airlock ./cmd/airlock
sudo useradd --system --create-home --shell /usr/sbin/nologin airlock
sudo install -d -o airlock -g airlock -m 0700 /var/lib/airlock
sudo install -d -o airlock -g airlock -m 0700 /etc/airlock
```

在 macOS 或没有 `useradd` 的系统上，创建等价的非管理员服务账户，并保证数据目录、令牌和控制 Socket 只由该账户读取。

生成两枚不同令牌。令牌内容不会输出到终端，只写入新建的 `0600` 文件：

```bash
sudo -u airlock /usr/local/bin/airlock token generate --output /etc/airlock/control.token
sudo -u airlock /usr/local/bin/airlock token generate --output /etc/airlock/web.token
```

`control.token` 用于 CLI 到核心的控制通道。`web.token` 仅用于浏览器登录 Web UI；绝不能复用，也不要交给普通路由调用方。

## 启动服务

服务模式必须同时提供绝对数据目录与 `0600` 控制令牌文件：

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

省略 `--web-listen` 和 `--web-token-file` 即可关闭 Web UI。Web UI 只允许 `127.0.0.1` 或 `::1`；要从管理电脑访问，请使用 SSH 本地端口转发：

```bash
ssh -L 4769:127.0.0.1:4769 operator@example-server
```

然后在管理电脑打开 `http://127.0.0.1:4769`，从服务主机的受保护文件读取并粘贴 **Web UI token**。令牌只保存在当前浏览器标签的 `sessionStorage`，关闭标签即失效。

数据入口默认仅回环。需要向私有局域网提供固定路由时，可额外设置 `--network-scope lan`，并显式监听私有地址或 `0.0.0.0`。不要将 HTTP、SSH 或 Web UI 直接暴露到公网；使用防火墙、VPN 或 SSH 隧道。

可复制 [systemd 示例](../deploy/systemd/airlock.service.example) 为 `/etc/systemd/system/airlock.service`，再执行：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now airlock
sudo systemctl status airlock
```

## 运维 CLI

所有操作都应以 `airlock` 服务账户执行，以匹配 `control.sock` 的 `0600` 权限：

```bash
sudo -u airlock /usr/local/bin/airlock \
  --data-dir /var/lib/airlock \
  --token-file /etc/airlock/control.token \
  status
```

常用命令：

```bash
# 查看脱敏路由摘要
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes list

# 启用、停用、探测指定路由
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable releases
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes health releases
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes disable releases

# 明确确认后停用全部路由；删除单一路由
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes stop-all --yes
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes delete releases
```

也可用 `--socket /var/lib/airlock/control.sock` 代替 `--data-dir`。命令输出为 JSON；创建路由时，随机生成的本地 capability 或二次 API Key 只在该次输出中出现一次。请将它交给被授权的调用方，不要写入 Shell 历史、CI 日志或工单。

## 受保护的路由规格

上游信息只能通过常规、非符号链接、权限为 `0600` 的 JSON 文件传给 CLI。文件路径必须为绝对路径；不要把 JSON 内容、密码或 URL 作为命令行 flag。

```bash
install -m 0600 /dev/null /etc/airlock/releases.json
editor /etc/airlock/releases.json
```

HTTP/Wget 路由示例：

```json
{
  "name": "Release mirror",
  "alias": "releases",
  "base_url": "https://upstream.example.invalid/releases/",
  "authorization": "Bearer upstream-secret",
  "egress": "Auto"
}
```

创建、启用并通过本地入口下载：

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes create http --file /etc/airlock/releases.json
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable releases
wget --header="Authorization: Bearer <local-capability>" http://127.0.0.1:4768/r/releases/file.tar.gz
```

`<local-capability>` 是创建命令返回的本地随机凭据，不是 `upstream-secret`。HTTP 路由只允许 GET/HEAD 和固定上游基址下的路径。

LLM 路由示例：

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

`provider: "anthropic"` 会把 `authorization` 保存为上游 `X-Api-Key`；OpenAI 兼容模式会添加上游 `Bearer` 前缀。`track_usage` 只保存数值统计，不保存提示词或响应正文。

## SSH 映射

先探测并人工核对上游 Host Key。未指定端口时 SSH 默认使用 `22`：

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token ssh probe --address ssh.example.invalid:22 --egress Auto
```

将输出中的 `host_key` 原样写入受保护规格文件。下面的 `local_username` 是调用者连接 Airlock 时使用的用户名，可以让多个本地用户名映射到同一上游地址的不同账号。

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
  "allow_sftp": false,
  "allow_interactive_shell": false,
  "egress": "Auto"
}
```

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes create ssh --file /etc/airlock/build-host.json
ssh build@127.0.0.1 -p 4770 uptime
```

SSH 创建只会把路由和受保护宿主保存为**禁用状态**，不会立刻连接上游，也不会因连接失败自动删除配置。请先手动检查健康度，成功后再启用：

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes health build-host
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable build-host
```

健康检查会在该路由的认证预算内验证固定 Host Key 与上游密码，默认 20 秒，可设为 3-120 秒。`allowed_command` 是精确匹配。若确实需要非交互任意 `exec`，规格中设置 `allow_all_commands: true`，并额外传入 `--allow-all-confirmed`；这接近对上游账户的远程代码执行，应使用最小权限专用账号。交互式 Shell 默认关闭；设置 `allow_interactive_shell: true`（要求同时开启 `allow_all_commands: true`）后，PuTTY 与 `ssh` 会直接进入上游交互式 Shell，可运行 `su` 等交互操作，上游凭据仍由 Airlock 注入。Agent/X11 与端口转发始终拒绝；只有开启交互式 Shell 时才会向上游转发 PTY 元数据。SFTP 默认关闭；在规格中显式设置 `allow_sftp: true` 后，现代 OpenSSH 的 `scp` 才能通过 SFTP 子系统工作。该开关允许上游账号可访问文件的列出、读取、写入、重命名和删除，请仅对专用低权限账号启用。

## 代理出口与故障处理

Airlock 可通过已有 Clash 或其他 HTTP CONNECT、HTTPS CONNECT、SOCKS5/SOCKS5H 代理访问固定上游。代理本身也从 `0600` JSON 文件读取：

```json
{ "url": "socks5://127.0.0.1:7890" }
```

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token proxy set --file /etc/airlock/proxy.json
```

为每条路由选择 `Direct`、`Proxy` 或 `Auto`。`Auto` 只会在直连尚未收到响应且发生可重试的连通性失败时重试代理，不会把已经开始的应用请求重放到另一路径。移除代理需要明确确认：

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token proxy clear --yes
```

## 备份、升级与限制

- 停止服务后备份整个 `--data-dir` 及两枚令牌文件；它们都包含必要状态。将备份视为机密。
- 升级时先停止服务、备份、替换 `airlockd` 和 `airlock`、再启动并运行 `status` 与各关键路由的 `health`。
- `local_file` 依赖操作系统文件权限，不是硬件加密或操作系统沙箱。服务账户、root 和能调试该进程的本机管理员在威胁模型之外。
- Web UI 的 `healthz` 只表示 Web 进程存活；路由连通性请使用每条路由的 Health 操作或 CLI。

参见 [安全模型](../SECURITY.md) 和 [英文版](server-deployment.md)。
