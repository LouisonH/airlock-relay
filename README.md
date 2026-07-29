# Airlock

Airlock 是一个运行在本机的凭据隔离型安全转发器，面向不受信任的 LLM、Agent、脚本和自动化任务。

调用者只接触本地路由、受限 Capability Token 或本地 SSH 身份。真实的上游 URL、SSH 地址、API Key、密码、私钥和代理凭据由 Airlock 保存并注入，默认不暴露给调用者。

当前项目处于规划阶段，尚未提供可运行版本。

## 计划中的能力

- HTTP/Wget 固定 URL 路由，支持受控路径、Range 下载和代理出口。
- SSH 双会话网关，用本地身份映射到隐藏的上游 SSH 凭据。
- OpenAI-compatible 与 Anthropic-compatible LLM API 路由。
- `direct`、SOCKS5、HTTP CONNECT 和安全的 `auto` 出口策略。
- 原生桌面 GUI 和系统托盘；GUI 不是 Web 管理页面。

## 安全边界

Airlock 的目标是减少不受信任调用者获得高权限秘密的机会，不是操作系统级沙箱。拥有本机管理员权限、能调试 Airlock 进程或已控制操作系统的攻击者不在保护范围内。上游账号仍必须配置最小权限。

详细设计见 [.claude/plan/airlock-1.md](.claude/plan/airlock-1.md)。
