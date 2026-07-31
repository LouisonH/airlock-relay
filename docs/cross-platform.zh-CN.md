# 跨平台核心移植基线

Airlock v0.1.4 目前仅发布 Apple Silicon macOS 桌面测试版。本分支新增的是 Windows 和
Linux 的**核心/CLI 编译基线**，并不表示已经发布 Windows/Linux 桌面应用、安装器、自动更新
或签名产物。

| 目标 | Core / CLI 构建 | 本地控制通道 | 平台凭据后端 | 桌面安装包 | 状态 |
| --- | --- | --- | --- | --- | --- |
| macOS arm64 | 原生 | 当前用户 Unix Socket | Keychain / 受保护文件 | DMG / `.app` | 已发布测试版 |
| macOS x64 | 目标构建 | 当前用户 Unix Socket | Keychain / 受保护文件 | DMG / `.app` | 安装包计划中 |
| Windows x64 | 交叉编译 | 当前所有者 ACL 命名管道 | Credential Manager / 受保护文件 | NSIS / MSI | 桌面端计划中 |
| Windows arm64 | 交叉编译 | 当前所有者 ACL 命名管道 | Credential Manager / 受保护文件 | NSIS / MSI | 桌面端计划中 |
| Linux x64 | 交叉编译 | 当前用户 Unix Socket | Secret Service / 受保护文件 | AppImage / deb | 桌面端计划中 |
| Linux arm64 | 交叉编译 | 当前用户 Unix Socket | Secret Service / 受保护文件 | AppImage / deb | 桌面端计划中 |
| Linux ARMv7 | 交叉编译 | 当前用户 Unix Socket | Secret Service / 受保护文件 | AppImage / deb | 树莓派基线 |

“交叉编译”仅表示 CI 和目标构建脚本可用 `CGO_ENABLED=0` 编译 `airlockd` 与 `airlock`。
它不等同于真实硬件上的运行验收；下文的运行时和安装器检查完成前，对应平台不会发布。

## 已实现的核心边界

- Go Core 与运维 CLI 使用统一的本地控制抽象。macOS/Linux 使用 `0600` Unix Socket；
  Windows 为每个用户目录生成确定性的命名管道，并通过受保护的 owner ACL 创建。两者都不
  会开启控制面 TCP 端口。
- 桌面模式可使用 macOS Keychain、Linux Secret Service 或 Windows Credential Manager。
  Windows 后端会把较大的加密记录分块，并以原子切换的索引保存，避免超过通用凭据的载荷限制。
- Server Core 继续采取保守默认值：`local_file`、显式受保护的数据目录和独立控制令牌文件。
  `keychain` 模式仅在对应平台的凭据后端可用且已正确配置时使用。
- 构建目标显式隔离。每次目标构建都会产出 `airlockd` 与 `airlock`，不会生成 Tauri 包，也
  不会改变 npm 安装器的已发布平台范围。

## 构建 Core 与 CLI

需要 Go 1.25 或更新版本。在仓库根目录执行以下命令可交叉编译，并将结果放在
`bin/<target>`：

```bash
./scripts/build-sidecar.sh windows-amd64
./scripts/build-sidecar.sh windows-arm64
./scripts/build-sidecar.sh linux-amd64
./scripts/build-sidecar.sh linux-arm64
./scripts/build-sidecar.sh linux-armv7
```

不带参数时，脚本会保留桌面开发所需的当前主机输出位置：`bin/airlockd` 与 `bin/airlock`。
仅支持表中的目标名；未知目标会在写入任何二进制文件前失败。

对于运行 32 位 Raspberry Pi OS 或 Debian `armhf` 的树莓派 3/4，将
`bin/linux-armv7/airlockd` 与 `bin/linux-armv7/airlock` 复制到设备，再按
[Server Core 部署指南](server-deployment.zh-CN.md)使用非登录服务账户运行。64 位
Raspberry Pi OS 则使用 `linux-arm64`。本阶段尚未提供任一树莓派桌面安装包。

## 发布前仍需完成的运行时验收

Windows/Linux 成为正式发布目标前，维护者必须在每一种受支持架构和发行版完成：

1. 在真实 Windows Credential Manager 或 freedesktop.org Secret Service 会话中测试创建、
   读取、轮换、删除，以及凭据库锁定或不可用时的失败关闭。
2. 用另一台本地账户验证 Windows 命名管道和受保护状态/令牌路径不可访问；在 Linux 验证
   `0600` Unix Socket 与状态目录的所有者权限。
3. 移植 Rust/Tauri 控制客户端，移除 Unix stream、Unix 文件权限和 macOS 专属确认流程，
   并为高风险系统操作提供等价的原生确认。
4. 在目标硬件测试服务安装、干净卸载、升级、残留进程恢复、`Direct`/`Proxy`/`Auto` 出口、
   SSH Host Key 固定与失败关闭。
5. 构建架构专用安装器、独立签名和固定校验和，并分别测试安装、更新与卸载。

在这些检查通过前，`airlock-installer` 会继续把 Windows 和 Linux 显示为 `planned`，并拒绝
安装不存在的产物，避免把“能够编译”误表述为“已支持的桌面产品”。

## 不变的安全语义

SSH 用户名映射、固定上游路由、本地 capability、LLM 二次 API Key、审计脱敏和代理出口策略
均在共享 Go Core 中实现，不随平台改变。调用者始终只得到本地入口和本地凭据；上游 URL、
密码、私钥、Host Key 与 API Key 仍保留在选择的受保护存储中。

服务器部署命令请阅读 [Server Core 部署指南](server-deployment.zh-CN.md)，在开发环境之外
运行 Core 构建前请阅读[安全策略](../SECURITY.md)。
