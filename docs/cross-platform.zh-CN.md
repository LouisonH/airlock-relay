# 跨平台适配方案

Airlock v0.1.4 目前只发布 Apple Silicon Mac 测试版。下表中的其余平台是实现契约，
并不表示对应安装包已经可用。

| 目标 | 安装形式 | 本地控制通道 | 凭据存储 | 当前状态 |
| --- | --- | --- | --- | --- |
| macOS arm64 | DMG / `.app` | 当前用户 Unix Socket | 0600 文件 / Keychain | 已发布测试版 |
| macOS x64 | DMG / `.app` | 当前用户 Unix Socket | 0600 文件 / Keychain | 仅契约 |
| Windows x64 | NSIS / MSI | 当前用户 ACL 命名管道 | 受保护文件 / Credential Manager | 仅契约 |
| Linux x64 | AppImage / deb | 0600 Unix Socket | 受保护文件 / Secret Service | 仅契约 |
| Linux arm64 | AppImage / deb | 0600 Unix Socket | 受保护文件 / Secret Service | 仅契约 |

`packages/airlock/lib/platform.mjs` 提供可复用的平台与产物解析。只有同时存在正式产物名
和固定 SHA-256 的目标才会解析成功，其余目标默认拒绝，避免把“计划支持”误报为“已经发布”。

## 适配边界

1. 抽象桌面控制通道：macOS/Linux 保留 Unix Socket，Windows 使用仅当前用户可访问的
   ACL 命名管道。
2. 继续复用 Go 编写的 `airlockd` 核心，为每个平台构建独立 sidecar，并在原生 CI
   上运行竞态测试和架构检查。
3. SSH 敏感录入保留在跨平台 Airlock 向导中，通过一次性本机 IPC 命令提交；其他 Secret
   与系统安全变更继续按平台提供原生窗口。Secret 不得进入命令行、环境变量、进程列表、
   日志或持久化控制状态。
4. 增加 Windows Credential Manager 与 Linux Secret Service 后端，并沿用
   “复制、验证、切换、清理”的迁移流程。
5. 各平台独立签名与校验；安装、卸载、升级及失败关闭测试全部通过后，才把状态改为已发布。

## 版本与更新契约

桌面端版本检测必须由用户主动触发，且只读。触发后，WebView 只读取官方 GitHub Releases
的公开元数据；不会发送本机路由状态、受保护目标、凭据或活动数据，也不会自动下载、安装、
重启或打开发布页。只有某个平台可独立验证用户选择的安装器后，才可以把该平台的更新流程
描述为已发布。

SSH 用户名映射不依赖平台：本地用户名在同一监听地址上选择唯一一条路由，每条路由仍保存
独立的 Capability 摘要与受保护上游目标。
