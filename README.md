# Airlock

Airlock 是一个运行在本机的凭据隔离型安全转发器，面向不受信任的 LLM、Agent、脚本和自动化任务。

调用者只接触本地路由和受限 Capability Token。真实上游 URL、SSH 地址、API Key、密码、私钥和代理凭据由 Airlock 保存并注入，不进入桌面 WebView，也不暴露给调用者。

> 当前是 P0 技术验证版，尚不适合保存真实生产凭据。

## 已实现

- Go `airlockd` 原型，严格限制为 loopback 监听，并提供健康检查。
- 256-bit 不透明 Capability Token，服务端仅保存 SHA-256 摘要并使用常数时间校验。
- HTTP/Wget 固定目标转发：GET/HEAD、Query allowlist、Range、认证注入、路径逃逸防护、同源重定向重写和响应 Header 白名单。
- 可更新、删除的 SecretStore 接口与 macOS Keychain 后端；目标和 Secret 不出现在 GUI IPC 类型中。
- Tauri 2 + React 原生桌面壳，包含概览、路由、活动、设置、五步路由编辑器、紧急停止确认、系统托盘和跟随系统/浅色/深色主题。
- macOS、Windows、Linux 所需应用图标资产。

## 待实现

- 路由元数据持久化、原生安全录入窗口，以及桌面端到 `airlockd` 的受保护控制通道与 sidecar 生命周期。
- Clash HTTP/SOCKS5 代理出口和安全的 `auto` 回退策略。
- SSH 双会话网关与 OpenAI/Anthropic 路由预设。
- 速率、并发、TTL、一次性 Capability 与脱敏审计。

## 本地开发

需要 Go 1.24+、Node.js 20+、Rust/Cargo 和 Tauri 2 的平台依赖。

```bash
# Go 核心
go test ./...
go run ./cmd/airlockd

# 桌面前端
cd apps/desktop
npm install
npm run build

# 原生桌面窗口
npm run tauri dev
```

`airlockd` 默认仅监听 `127.0.0.1:4768`，并在 macOS 上使用 Keychain SecretStore。当前运行时注册表仍为空，原生安全录入窗口和受保护控制通道尚未完成，因此请勿用开发界面录入真实凭据。

## 安全边界

Airlock 用最小能力和固定目标减少凭据暴露，但不是操作系统级沙箱。本机管理员、可调试 Airlock 进程或已控制操作系统的攻击者不在保护范围内。上游账号仍必须保持最小权限。

详细威胁模型与实施计划见 [.claude/plan/airlock-1.md](.claude/plan/airlock-1.md)，桌面交互规范见 [docs/ui-spec.md](docs/ui-spec.md)。
