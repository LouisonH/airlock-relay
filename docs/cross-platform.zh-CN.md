# 跨平台核心移植基线

Airlock v0.1.4 目前只发布经过校验的 Apple Silicon macOS 桌面测试版。npm 包可在已识别的
Windows/Linux 目标上无副作用安装，用于输出平台契约。Windows x64/x86/arm64 与 Linux
x64/arm64 属于 CI 预览目标：其桌面构建产物不是公开、带固定校验和的安装器，因此
`airlock-installer install` 会失败关闭，绝不会下载 CI 工件。桌面 GUI、本地控制通道与原生
提示流程已在代码层面完成移植，但仍需真实设备运行验收后才会发布。

| 目标 | Core / CLI 构建 | 本地控制通道 | 平台凭据后端 | 桌面安装包 | 状态 |
| --- | --- | --- | --- | --- | --- |
| macOS arm64 | 原生 | 当前用户 Unix Socket | Keychain / 受保护文件 | DMG / `.app` | 已发布测试版 |
| macOS x64 | 目标构建 | 当前用户 Unix Socket | Keychain / 受保护文件 | DMG / `.app` | 安装包计划中 |
| Windows x64 | 交叉编译 | 当前所有者 ACL 命名管道 | Credential Manager / 受保护文件 | NSIS / MSI | CI 预览 · 无公开校验安装器 |
| Windows x86（i686） | 交叉编译 | 当前所有者 ACL 命名管道 | Credential Manager / 受保护文件 | NSIS / MSI | CI 预览 · 无公开校验安装器 |
| Windows arm64 | 交叉编译 | 当前所有者 ACL 命名管道 | Credential Manager / 受保护文件 | NSIS / MSI | CI 预览 · 无公开校验安装器 |
| Linux x64 | 交叉编译 | 当前用户 Unix Socket | Secret Service / 受保护文件 | AppImage / deb | CI 预览 · 无公开校验安装器 |
| Linux arm64 | 交叉编译 | 当前用户 Unix Socket | Secret Service / 受保护文件 | AppImage / deb | CI 预览 · 无公开校验安装器 |
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
- Rust/Tauri 桌面客户端已平台化：macOS/Linux 使用仅当前用户可读的 Unix Socket 交换控制
  消息，Windows 使用由 SHA-256 派生的命名管道与重叠 I/O；受保护文件在 Unix 上使用
  `0600`/`0700` 权限，在 Windows 上使用仅当前用户的 `icacls` ACL 与原子替换。
- 敏感桌面流程在各平台均使用原生弹窗：macOS 使用 `osascript`，Windows 使用
  PowerShell/Windows Forms，覆盖安全录入、LLM Key 选择、高风险 SSH 确认、一次性
  Capability 交接与安全设置确认。Windows 端口管理通过 `netstat -ano` +
  `Win32_Process` 列出占用者、过滤当前账户，并只对确认的进程使用 `taskkill`。
- 前端提供平台感知文案与中/英/日翻译，覆盖控制通道（Unix Socket / 命名管道）、凭据存储
  （Keychain / Credential Manager / Secret Service）、安全等级说明与原生风险提示。
- Linux 原生弹窗使用 `zenity`（GNOME）或 `kdialog`（KDE），按会话探测后端，覆盖安全录入、
  LLM Key 选择、高风险 SSH 确认、Capability 交接与安全设置确认；桌面包运行时依赖二者之一，
  Linux 端口占用管理直接读取 `/proc`，不需要额外的 `lsof`。无桌面环境的服务器请使用 CLI。
- CI 会在每次推送时对 Windows x64、Windows x86、Windows arm64、Linux x64 与 Linux arm64
  桌面目标运行 Rust `cargo check`，在真实目标工具链上持续复核已移植的控制客户端，直到进入
  运行验收阶段。
- 独立的 `desktop-windows` 工作流会在 GitHub 官方 Windows runner 上构建 Windows x64、
  x86（i686）与 arm64 的 NSIS/MSI 安装包，并以可下载产物形式发布。正式签名配置完成前产物不签名，
  SmartScreen 首次运行时可能提示。x64 冒烟测试会无控制台启动 `airlockd`，再使用编译后的
  `airlock` CLI 验证已认证的命名管道 `status` 交换。
- `desktop-linux` 工作流在 x64 runner 上构建 Linux x64 的 `deb`/AppImage 安装包，并在
  原生 ARM runner 上构建 Linux arm64 安装包；冒烟测试会启动 `airlockd`，再用编译后的
  `airlock` CLI 验证已认证的 Unix Socket `status` 交换。包内不附带 `zenity`/`kdialog`，
  见上方运行时说明。
- 构建目标显式隔离。每次目标构建都会产出 `airlockd` 与 `airlock`，不会生成 Tauri 包，也
  不会改变 npm 安装器的已发布平台范围。

## 构建 Core 与 CLI

需要 Go 1.25 或更新版本。在仓库根目录执行以下命令可交叉编译，并将结果放在
`bin/<target>`：

```bash
node scripts/build-sidecar.mjs windows-amd64
node scripts/build-sidecar.mjs windows-arm64
node scripts/build-sidecar.mjs windows-386
node scripts/build-sidecar.mjs linux-amd64
node scripts/build-sidecar.mjs linux-arm64
node scripts/build-sidecar.mjs linux-armv7
```

`scripts/build-sidecar.sh` 保留为同一 Node 驱动的兼容入口。不带参数时，脚本会把桌面开发
sidecar 按 Tauri 的目标三元组命名写入 `apps/desktop/src-tauri/binaries/`（例如
`airlockd-aarch64-apple-darwin`）；显式指定目标时，则以普通 `airlockd`/`airlock` 名称写入
`bin/<target>`。仅支持表中的目标名；未知目标会在写入任何二进制文件前失败。运行
`node scripts/build-sidecar.mjs --help` 可查看全部目标与输出规则。

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
3. 在真实 Windows/Linux 工具链上编译 Rust/Tauri 控制客户端，并在物理设备上核验已移植的
   命名管道/Unix 控制交换、受保护文件 ACL、Windows Forms 原生弹窗、端口管理与前端平台文案。
4. 在目标硬件测试服务安装、干净卸载、升级、残留进程恢复、`Direct`/`Proxy`/`Auto` 出口、
   SSH Host Key 固定与失败关闭。
5. 构建架构专用安装器、独立签名和固定校验和，并分别测试安装、更新与卸载。

在这些检查通过前，`airlock-installer status --json` 会把已识别的 Windows/Linux 目标显示为
`preview` 和 `installerAvailable: false`，同时拒绝 `install`；Linux ARMv7 等计划目标显示为
`planned`。这避免把 CI 候选构建误表述为已支持的桌面产品。

## 不变的安全语义

SSH 用户名映射、固定上游路由、本地 capability、LLM 二次 API Key、审计脱敏和代理出口策略
均在共享 Go Core 中实现，不随平台改变。调用者始终只得到本地入口和本地凭据；上游 URL、
密码、私钥、Host Key 与 API Key 仍保留在选择的受保护存储中。

服务器部署命令请阅读 [Server Core 部署指南](server-deployment.zh-CN.md)，在开发环境之外
运行 Core 构建前请阅读[安全策略](../SECURITY.md)。
