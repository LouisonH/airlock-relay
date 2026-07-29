# Airlock

Airlock 是一个运行在本机的凭据隔离型安全转发器，面向不受信任的 LLM、Agent、脚本和自动化任务。

调用者只接触本地路由和受限 Capability Token。真实上游 URL、SSH 地址、API Key、密码、私钥和代理凭据由 Airlock 保存并注入，不进入桌面 WebView，也不暴露给调用者。

> 当前是 P0 技术验证版。路由元数据、凭据与代理出口已接入安全存储，但在完成完整审计和发布安全评审前，仍不建议保存高价值生产凭据。

## 已实现

- Go `airlockd` 原型，严格限制为 loopback 监听，并提供健康检查。
- 256-bit 不透明 Capability Token，服务端仅保存 SHA-256 摘要并使用常数时间校验。
- HTTP/Wget 固定目标转发：GET/HEAD、Query allowlist、Range、认证注入、路径逃逸防护、同源重定向重写和响应 Header 白名单。
- 可更新、删除的 SecretStore 接口与 macOS Keychain 后端；目标和 Secret 不出现在 GUI IPC 类型中。
- Tauri 管理的 `airlockd` sidecar 与权限为 `0600` 的 Unix Socket 控制通道；随机控制令牌只经 stdin 传递并保留在内存中。
- macOS 原生安全录入窗口；完整 URL、Authorization 和一次性 Capability 均不经过 WebView。
- 版本化的路由元数据持久化；`routes.json` 权限为 `0600`，仅保存别名、策略、Keychain 引用和 Capability 摘要。
- 路由删除会先持久化撤销 Capability，再清理 Keychain 目标；存储失败时会回滚内存状态。
- Clash 兼容的 HTTP/HTTPS CONNECT 与 SOCKS5/SOCKS5H 出口；代理 URL 和认证只进入 Keychain，不经过 WebView。
- 每路由 `Direct` / `Proxy` / `Auto` 策略；`Auto` 仅对无 Body 的 GET/HEAD 在拨号或 DNS 错误时回退，TLS 和非幂等请求失败关闭。
- 已通过端到端与 race 测试的 SSH 双会话安全核心：本地 Capability/公钥认证、上游密码/加密私钥注入、精确 Host Key 固定和完整命令 allowlist。
- SSH 默认拒绝 stdin、shell、PTY、SFTP/subsystem、agent/X11 forwarding 与端口转发；Host Key 或认证失败不会触发代理回退，也不会向客户端暴露受保护目标。
- 简洁的 Tauri 2 + React 桌面控制台，包含三步 HTTP 路由编辑器、安全删除、紧急停止、系统托盘、轻量动画、系统/浅色/深色模式与三套可持久化配色。
- macOS、Windows、Linux 所需应用图标资产。

## 待实现

- sidecar 异常恢复与元数据迁移工具。
- SSH 持久监听、本地主机密钥、控制协议和原生安全录入接入；在此之前桌面端 SSH 创建入口保持关闭。
- OpenAI/Anthropic 路由预设。
- 速率、并发、TTL、一次性 Capability 与脱敏审计。

## 本地开发

需要 Go 1.24+、Node.js 20+、Rust/Cargo 和 Tauri 2 的平台依赖。

```bash
# Go 核心测试
go test ./...

# 桌面前端
cd apps/desktop
npm install
npm run build

# 原生桌面窗口
# 此命令会先构建并由 Tauri 启动 airlockd sidecar
npm run tauri dev
```

`airlockd` 默认仅监听 `127.0.0.1:4768`，并在 macOS 上使用 Keychain SecretStore。桌面端可以安全创建、启停和删除 HTTP 路由；重启后从受保护的本地元数据恢复路由，真实 URL 和 Authorization 仍仅存在 Keychain。SSH 核心当前只作为经过隔离测试的后端组件存在，尚未由桌面端开放。

## 安全边界

Airlock 用最小能力和固定目标减少凭据暴露，但不是操作系统级沙箱。本机管理员、可调试 Airlock 进程或已控制操作系统的攻击者不在保护范围内。普通同用户进程无法从磁盘、环境变量或进程参数读取控制令牌；上游账号仍必须保持最小权限。SSH 的完整命令匹配是能力边界，不是远端 shell 沙箱；强隔离仍应使用专用低权限账号或远端 forced command。

详细威胁模型与实施计划见 [.claude/plan/airlock-1.md](.claude/plan/airlock-1.md)，桌面交互规范见 [docs/ui-spec.md](docs/ui-spec.md)。

## 开发者

Airlock 由 **LouisonH** 进行产品设计与核心开发，并使用 **GPT-5.6 Sol** 辅助工程实现与验证。桌面端开发者头像来源为 `https://cdn.osdn.xyz/louyee.png`，并以本地资源方式打包以支持离线使用。
