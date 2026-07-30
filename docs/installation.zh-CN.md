# 在 macOS 上安装 Airlock

[English](installation.md) | [简体中文](installation.zh-CN.md)

## 系统要求

Airlock v0.1.0 支持运行 macOS 12 Monterey 或更高版本的 Apple Silicon Mac。
首版暂不提供 Intel、Windows 或 Linux 安装包。

## 核验下载

从 [v0.1.0 Release](https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.0) 下载：

- `Airlock_0.1.0_aarch64.dmg`：常规安装包
- `Airlock_0.1.0_aarch64.app.zip`：便携压缩包
- `SHA256SUMS.txt`：完整性校验值

在下载目录执行：

```bash
shasum -a 256 -c SHA256SUMS.txt
```

三个文件位于同一目录时，该命令会同时核验 DMG 和 ZIP。

## 安装

1. 打开 `Airlock_0.1.0_aarch64.dmg`。
2. 将 **Airlock** 拖入 **Applications（应用程序）**。
3. 推出 Airlock 磁盘映像。
4. 在 Finder 的应用程序目录中按住 Control 点击 **Airlock**，选择**打开**。
5. 在 macOS 弹窗中再次确认**打开**。

### 为什么 macOS 会警告

v0.1.0 已进行 ad-hoc 签名，可检查应用包结构完整性，但没有 Apple Developer
ID 签名，也没有经过 Apple 公证。因此 macOS 无法验证开发者身份。这个限制并不
意味着可以直接忽略安全警告；请先确认文件来自官方 GitHub Release，并核对
SHA-256。

如果右键打开仍不可用，可先尝试启动一次，再前往**系统设置 > 隐私与安全性**，
为 Airlock 选择**仍要打开**。只有在校验值正确时，才建议用下面的兜底命令仅移除
该应用的下载隔离标记：

```bash
xattr -dr com.apple.quarantine /Applications/Airlock.app
```

## 首次运行

- HTTP 与 LLM 路由默认监听 `127.0.0.1:4768`。
- SSH 路由默认监听 `127.0.0.1:4770`。
- 桌面控制面使用当前用户专属 Unix Socket，不存在 Web 管理端口。
- 新安装默认使用**标准**方案：入口仅限本机，Secret 不加密地写入当前用户专属
  `0600` 文件。这样不会在每次启动时反复要求登录密码，但同一用户运行的其他进程可能读取。
- 可主动切换到**严格**方案，将 Secret 保存到 macOS Keychain；读取受保护项目时，
  macOS 可能要求输入登录密码。
- 局域网入口默认关闭。启用后，同一局域网设备可连接路由入口，绝不能再将端口映射到公网。

## 升级

从菜单栏退出 Airlock，用新版本替换 `/Applications/Airlock.app`，再重新打开。
路由元数据和凭据存放在应用包以外，会继续保留。1.0 之前配置格式仍可能变化，升级前
请先阅读目标版本发布说明。

## 本地数据

Airlock 的当前用户状态位于：

```text
~/Library/Application Support/io.airlock.relay/
```

其中可能包括：

- `routes.json`：HTTP/LLM 路由策略与 Capability 摘要
- `ssh-routes.json`：SSH 路由策略与 Capability 摘要
- `ssh-command-audit.json`：最多 100 条 SSH 命令事件
- `security-settings.json`：网络范围和 SecretStore 设置
- `protected-targets.json`：仅本地文件模式存在，其中含未加密 Secret
- `control.sock`：当前用户专属的临时桌面控制 Socket

Keychain 模式会把受保护目标另存为通用密码项目，服务名为
`io.airlock.relay.targets`。

## 卸载

退出 Airlock，再将 `/Applications/Airlock.app` 移到废纸篓。此操作不会自动删除
路由、审计记录或凭据，避免一次普通卸载静默丢失访问配置。

如需彻底删除，再把准确路径
`~/Library/Application Support/io.airlock.relay/` 移到废纸篓；随后打开**钥匙串访问**，
搜索 `io.airlock.relay.targets`，核对后删除不再需要的 Airlock 项目。删除无法撤销，
并会使相应路由配置失效。

## 从源码运行

开发环境需要 Go 1.24+、Node.js 20+、Rust/Cargo 及 Tauri 2 的 macOS 依赖：

```bash
git clone https://github.com/LouisonH/airlock-relay.git
cd airlock-relay/apps/desktop
npm install
npm run tauri dev
```
