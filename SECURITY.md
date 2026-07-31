# Security Policy

[English](#english) | [简体中文](#简体中文)

## English

### Supported Versions

Airlock is currently a technical preview. Only the latest `0.1.x` release is
eligible for security fixes. There is no production-support or long-term-
support promise before 1.0.

### Reporting a Vulnerability

Do not publish credential-exposure, authentication-bypass, request-smuggling,
path-escape, or remote-command-execution reports in a public issue. Use the
repository's **Security > Report a vulnerability** workflow to open a private
GitHub Security Advisory. Include the affected version, platform, minimal
reproduction, impact, and whether real credentials were exposed. Do not attach
live secrets; use revocable test values.

The project will acknowledge a complete report when the maintainer reviews it,
then coordinate validation, remediation, and disclosure through the private
advisory. No fixed response-time SLA is offered for this technical preview.

### Security Model

Airlock reduces the exposure of upstream URLs and credentials to LLMs, agents,
scripts, and other callers. A caller receives a fixed route and a revocable
local capability. Airlock checks route policy before resolving the protected
target and injecting the upstream credential.

The following controls are part of v0.1.0:

- Desktop control uses a `0600` Unix socket and an ephemeral in-memory token.
- Route metadata stores capability digests rather than reusable local tokens.
- The standard default is a local-file store in a `0700` directory and `0600`
  file. It avoids repeated launch prompts, but secrets are not encrypted and
  can be read by other processes running as the same user.
- The stricter opt-in mode uses macOS Keychain. Its items are device-local and
  available after the first unlock, subject to macOS access control; macOS may
  request the login password when protected items are accessed.
- SSH uses separate client-facing and upstream sessions, pins the upstream host
  key, and denies interactive shells, PTY, SFTP, agent/X11 forwarding, and port
  forwarding in v0.1.0.
- LLM usage collection is opt-in, kept in memory, and does not store prompt or
  response bodies. Upstream usage fields are used for token totals.

### Important Boundaries

Airlock is not an operating-system sandbox, antivirus product, VPN, or open
proxy. It does not protect against root, a local administrator, malware running
as the same user, process debugging, compromised operating systems, or a
malicious upstream returning secrets in its own response.

- Local capability tokens and secondary API keys are credentials. Rotate them
  if disclosed.
- v0.1.0 capabilities have no TTL and are not one-time credentials.
- Unrestricted SSH `exec` gives the caller nearly all command execution power
  of the configured upstream account. Use a dedicated least-privilege account.
- SSH command audit records the complete command. Arguments may contain tokens,
  passwords, paths, or other sensitive data.
- The configured exact command is visible in the desktop policy editor. Do not
  embed passwords or tokens in it; reference protected files or environment
  configuration on the upstream host instead.
- Private-LAN mode makes data-plane listeners reachable from the local network.
  Do not expose ports `4768` or `4770` to the public Internet.
- Rate limits, concurrency counters, and LLM usage totals are in-memory state
  and reset when the sidecar restarts.
- v0.1.0 is not Developer ID signed or notarized and has not completed an
  independent production security audit.

## 简体中文

### 支持范围

Airlock 当前是技术预览版。只有最新 `0.1.x` 版本会考虑安全修复；1.0 之前不承诺
生产支持或长期支持周期。

### 报告漏洞

请勿在公开 Issue 中披露凭据泄露、认证绕过、请求走私、路径逃逸或远程命令执行问题。
请使用仓库 **Security > Report a vulnerability** 创建私有 GitHub Security Advisory，
并注明受影响版本、平台、最小复现、影响范围以及是否接触过真实凭据。不要附加仍有效的
Secret，请使用可撤销的测试值。

维护者查看完整报告后会进行确认，并通过私有 Advisory 协调验证、修复与披露。技术预览
阶段暂不提供固定响应时间 SLA。

### 安全模型

Airlock 用于减少上游 URL 和凭据暴露给 LLM、Agent、脚本及其他调用者的机会。调用者
只获得固定路由和可撤销的本地 Capability；请求通过策略检查后，Airlock 才解析受保护
目标并注入上游凭据。

v0.1.0 包含以下控制：

- 桌面控制面使用 `0600` Unix Socket 和只存在于内存的临时 Token。
- 路由元数据只保存 Capability 摘要，不保存可直接复用的本地 Token。
- “标准”默认方案使用 `0700` 目录和 `0600` 本地文件，可避免反复出现启动密码弹窗；
  但 Secret 不加密，同一用户身份运行的其他进程仍可能读取。
- 更严格的可选方案使用 macOS Keychain；项目仅限本设备，并在首次解锁后依据 macOS
  访问控制提供。读取受保护项目时，macOS 可能要求输入登录密码。
- SSH 分离调用端与上游会话、固定上游 Host Key，并在 v0.1.0 中拒绝交互 Shell、PTY、
  SFTP、Agent/X11 Forwarding 和端口转发。
- SSH 创建向导在 Airlock 窗口中收集上游凭据，经本机 Tauri IPC 单次传给 Rust 与
  `airlockd`，成功后立即清空前端状态；后续控制状态、活动记录与路由元数据均不返回这些值。
- LLM 统计需要主动开启，只保存在内存，不保存提示词或响应正文；Token 总量来自上游
  返回的 Usage 字段。

### 重要边界

Airlock 不是操作系统沙箱、杀毒软件、VPN 或开放代理。它无法防御 root、本机管理员、
同用户恶意进程、进程调试、已失陷的操作系统，也无法阻止恶意上游主动在响应中返回秘密。

- 本地 Capability Token 与二次 API Key 仍是凭据，泄露后应立即轮换。
- v0.1.0 的 Capability 没有 TTL，也不是一次性凭据。
- 不受限 SSH `exec` 几乎等同于上游账户的完整命令执行能力，应使用专用低权限账户。
- SSH 命令审计会记录完整命令，参数可能包含 Token、密码、路径或其他敏感内容。
- 配置的精确命令会显示在桌面策略编辑器中，请勿把密码或 Token 直接写进命令；应改用
  上游主机中的受保护文件或环境配置。
- SSH 内嵌录入改善了流程，但无法防御 WebView 被注入、进程调试或同用户恶意进程；需要更强
  边界时应使用 Keychain 严格模式、正式签名构建和专用低权限上游账户。
- 局域网模式会使数据入口对本地网络可达，不得把 `4768` 或 `4770` 映射到公网。
- 速率限制、并发计数与 LLM Usage 总量是内存状态，sidecar 重启后会清零。
- v0.1.0 没有 Developer ID 签名与 Apple 公证，也尚未完成独立的生产安全审计。
