<p align="center">
  <img src="assets/airlock-logo.svg" width="88" height="88" alt="Airlock Logo" />
</p>

# airlock-relay

Airlock 的官方 npm 命令入口。它会识别当前操作系统和 CPU，并包含经过 SHA-256 校验的
Airlock v0.1.7 安装器（macOS DMG、Windows NSIS、Linux AppImage）；`npm install` 本身不会静默下载、启动应用或修改系统设置。

> 已公开校验安装器的目标只有 macOS 12+ Apple Silicon。Windows x64/x86/ARM64 与
> Linux x64/ARM64 当前可被 npm CLI 识别为 CI 预览，但没有公开校验和安装包；执行安装会
> 安全失败，不会下载 CI 工件或修改系统。

## 使用

```bash
# 在任意支持的操作系统上识别当前平台；无下载、启动或系统改动
npm install -g airlock-relay
airlock-installer status --json

# Apple Silicon macOS：安装已校验的 Airlock.app 到 ~/Applications，并启动它
airlock-installer install --open
```

查看当前安装包与平台发布状态：

```bash
npx airlock-relay status --json
npx airlock-relay platform --json
npx airlock-relay install
```

macOS 安装器也可指定目标目录：

```bash
npx airlock-relay@latest install --output /path/to/Applications
```

macOS 的安装命令会校验 npm 包内的 DMG，将 **Airlock.app** 原子安装到目标目录（默认
`~/Applications`），并仅在明确给出 `--open` 时启动。已验证的旧版本会更新；若已有
不完整应用，须明确传入 `--force` 才会替换。首次启动前请阅读完整的
[安装说明](https://github.com/LouisonH/airlock-relay/blob/v0.1.7/docs/installation.zh-CN.md)。

Airlock 是本地凭据隔离转发器，为 HTTP/Wget、SSH 与 LLM API 请求提供固定路由、
可撤销的本地凭据和最小权限策略。真实上游 URL、SSH 密码和 API Key 保留在本机。

项目主页：[github.com/LouisonH/airlock-relay](https://github.com/LouisonH/airlock-relay)
开发者：[LouisonH](https://0o0.site)

---

The official npm command entry point for Airlock. It detects the current
operating system and CPU, and contains a SHA-256-verified Airlock v0.1.7 macOS
disk image. `npm install` itself does not download or launch software, or
change system settings.

> The only target with a published verified installer is macOS 12+ on Apple
> Silicon. Windows x64/x86/ARM64 and Linux x64/ARM64 are recognized by the npm
> CLI as CI previews, but have no public checksummed installer. Installation
> fails closed: it never downloads CI artifacts or changes the system.

## Usage

```bash
# Detect the current platform on any supported operating system; no download,
# launch, or system change happens here
npm install -g airlock-relay
airlock-installer status --json

# On Apple Silicon macOS, install the verified Airlock.app and launch it
airlock-installer install --open
```

Inspect the local package and release contract:

```bash
npx airlock-relay status --json
npx airlock-relay platform --json
npx airlock-relay install
```

Choose a destination directory for the macOS installer:

```bash
npx airlock-relay@latest install --output /path/to/Applications
```

On macOS, the install command verifies the DMG bundled in the npm package, atomically
installs **Airlock.app** into the selected directory (default `~/Applications`),
and launches it only when `--open` is explicit. A verified existing app is
updated; replacing an incomplete bundle requires `--force`. Read the complete
[installation guide](https://github.com/LouisonH/airlock-relay/blob/v0.1.7/docs/installation.md)
before first launch.

Airlock is a local credential-isolation relay for fixed HTTP/Wget, SSH, and LLM
API routes. It gives callers revocable local capabilities while keeping the real
upstream URL, SSH password, and API key on the local machine.

Project: [github.com/LouisonH/airlock-relay](https://github.com/LouisonH/airlock-relay)
Developer: [LouisonH](https://0o0.site), affiliated with South China University
of Technology (SCUT). Airlock is an independent personal project and is not an
official SCUT project or endorsement.

The package also exports `airlock-relay/platform`, a fail-closed platform
contract for build tooling. macOS arm64/x64, Windows x64/x86/arm64, and Linux
x64/arm64 are released previews in v0.1.7; Linux ARMv7 remains Core/CLI-only.
Windows and Linux CI previews never resolve to an installer until a public
artifact and checksum are published.

## Integrity

Bundled asset: `Airlock_0.1.7_aarch64.dmg`

```text
SHA-256 628dbd59cba6b3cb6a68c8866dbd53543540c02cc77565e69540e7701211f832
```

## License

Apache License 2.0. Copyright 2026 LouisonH. See [LICENSE](LICENSE).
