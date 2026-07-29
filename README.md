# Airlock

Airlock 是一个运行在本机的凭据隔离型安全转发器，面向不受信任的 LLM、Agent、脚本和自动化任务。

调用者只接触本地路由和受限 Capability Token。真实上游 URL、SSH 地址、API Key、密码、私钥和代理凭据由 Airlock 保存并注入，不进入桌面 WebView，也不暴露给调用者。

> 当前是 P0 技术验证版，尚不适合保存真实生产凭据。

## 已实现

- Go `airlockd` 原型，严格限制为 loopback 监听，并提供健康检查。
- 256-bit 不透明 Capability Token，服务端仅保存 SHA-256 摘要并使用常数时间校验。
- HTTP/Wget 固定目标转发：GET/HEAD、Query allowlist、Range、认证注入、路径逃逸防护、同源重定向重写和响应 Header 白名单。
- 内存 SecretStore 接口原型；目标和 Secret 不出现在 GUI IPC 类型中。
- Tauri 2 + React 原生桌面壳，包含概览、路由、活动、设置、五步路由编辑器、紧急停止确认和系统托盘。
- macOS、Windows、Linux 所需应用图标资产。

## 待实现

- 桌面端与 `airlockd` 的本地受保护控制通道及 sidecar 生命周期。
- macOS Keychain SecretStore、路由持久化和真实的原生安全录入窗口。
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

`airlockd` 默认仅监听 `127.0.0.1:4768`。当前运行时注册表为空，可转发路由由测试代码构造；在 Keychain 和本地控制通道完成前，请勿向内存 Store 填入真实凭据。

## 安全边界

Airlock 用最小能力和固定目标减少凭据暴露，但不是操作系统级沙箱。本机管理员、可调试 Airlock 进程或已控制操作系统的攻击者不在保护范围内。上游账号仍必须保持最小权限。

详细威胁模型与实施计划见 [.claude/plan/airlock-1.md](.claude/plan/airlock-1.md)，桌面交互规范见 [docs/ui-spec.md](docs/ui-spec.md)。
