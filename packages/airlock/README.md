<p align="center">
  <img src="assets/airlock-logo.svg" width="88" height="88" alt="Airlock Logo" />
</p>

# airlock-relay

Airlock 的官方 npm 安装入口。它包含经过 SHA-256 校验的 Airlock v0.1.1
macOS 安装镜像，不会在 `npm install` 阶段静默下载、启动应用或修改系统设置。

> v0.1.1 仅支持 macOS 12+ 与 Apple Silicon。应用采用 ad-hoc 签名，尚未经过
> Apple 公证，也尚未完成独立生产安全审计。

## 使用

```bash
# 将已校验的 DMG 放入 ~/Downloads，并打开它
npx airlock-relay install --open

# 只检查 npm 包内安装镜像的完整性
npx airlock-relay verify
```

查看当前安装包与平台发布状态：

```bash
npx airlock-relay status --json
npx airlock-relay platform --json
npx airlock-relay doctor
```

指定下载目录：

```bash
npx airlock-relay install --output /path/to/directory
```

安装命令只复制并校验 DMG。随后请在磁盘镜像中将 **Airlock** 拖入
**Applications（应用程序）**。首次启动前请阅读完整的
[安装与 Gatekeeper 说明](https://github.com/LouisonH/airlock-relay/blob/v0.1.1/docs/installation.zh-CN.md)。

Airlock 是本地凭据隔离转发器，为 HTTP/Wget、SSH 与 LLM API 请求提供固定路由、
可撤销的本地凭据和最小权限策略。真实上游 URL、SSH 密码和 API Key 保留在本机。

项目主页：[github.com/LouisonH/airlock-relay](https://github.com/LouisonH/airlock-relay)
开发者：[LouisonH](https://0o0.site)

---

The official npm installation entry point for Airlock. The package contains a
SHA-256-verified Airlock v0.1.1 macOS disk image. It does not download or launch
software, or change system settings, during `npm install`.

> v0.1.1 supports Apple Silicon Macs running macOS 12 or newer. The application
> is ad-hoc signed, is not Apple-notarized, and has not completed an independent
> production security audit.

## Usage

```bash
# Copy the verified DMG to ~/Downloads and open it
npx airlock-relay install --open

# Verify the installer bundled in the npm package
npx airlock-relay verify
```

Inspect the local package and release contract:

```bash
npx airlock-relay status --json
npx airlock-relay platform --json
npx airlock-relay doctor
```

Choose a destination directory:

```bash
npx airlock-relay install --output /path/to/directory
```

The install command only copies and verifies the DMG. Drag **Airlock** into
**Applications** from the disk image, then read the complete
[installation and Gatekeeper guide](https://github.com/LouisonH/airlock-relay/blob/v0.1.1/docs/installation.md)
before first launch.

Airlock is a local credential-isolation relay for fixed HTTP/Wget, SSH, and LLM
API routes. It gives callers revocable local capabilities while keeping the real
upstream URL, SSH password, and API key on the local machine.

Project: [github.com/LouisonH/airlock-relay](https://github.com/LouisonH/airlock-relay)
Developer: [LouisonH](https://0o0.site)

The package also exports `airlock-relay/platform`, a fail-closed platform
contract for build tooling. Only macOS arm64 is marked as released in v0.1.1;
planned targets never resolve to an installer until an artifact and checksum are
published.

## Integrity

Bundled asset: `Airlock_0.1.1_aarch64.dmg`

```text
SHA-256 644b7310e51ebbd2668a40ee95699194903f3d73347a6c071f2b4140bffefc9b
```

License: `UNLICENSED`. Installing the npm package does not grant permission to
redistribute or modify Airlock.
