# クロスプラットフォーム Core 移植ベースライン

Airlock v0.1.4 で公開済みなのは Apple Silicon macOS 向け Desktop preview だけです。
この branch で追加するのは Windows と Linux の **Core/CLI コンパイルベースライン**であり、
Windows/Linux Desktop、インストーラー、auto-update、署名済み成果物の公開を意味しません。
Desktop GUI、local control transport、native prompt フローはコードレベルで Windows/Linux
へ移植済みですが、実機での runtime acceptance を完了するまで公開しません。

| 対象 | Core / CLI build | local control transport | platform secret backend | Desktop bundle | 状態 |
| --- | --- | --- | --- | --- | --- |
| macOS arm64 | native | user-only Unix Socket | Keychain / protected file | DMG / `.app` | preview 公開済み |
| macOS x64 | target build | user-only Unix Socket | Keychain / protected file | DMG / `.app` | installer は予定 |
| Windows x64 | cross-compiled | current-owner ACL Named Pipe | Credential Manager / protected file | NSIS / MSI | Desktop 移植済み（コード）· 未公開 |
| Windows arm64 | cross-compiled | current-owner ACL Named Pipe | Credential Manager / protected file | NSIS / MSI | Desktop 移植済み（コード）· 未公開 |
| Linux x64 | cross-compiled | user-only Unix Socket | Secret Service / protected file | AppImage / deb | Desktop 移植済み（コード）· 未公開 |
| Linux arm64 | cross-compiled | user-only Unix Socket | Secret Service / protected file | AppImage / deb | Desktop 移植済み（コード）· 未公開 |
| Linux ARMv7 | cross-compiled | user-only Unix Socket | Secret Service / protected file | AppImage / deb | Raspberry Pi baseline |

`cross-compiled` は CI と target-aware build script が `CGO_ENABLED=0` で
`airlockd` と `airlock` をコンパイルできることだけを示します。実機 runtime acceptance
ではありません。下記の確認が完了するまで対象は unreleased のままです。

## この段階で実装した Core 境界

- Go Core と operations CLI は local control の platform abstraction を共有します。
  macOS/Linux は `0600` Unix domain socket、Windows は user directory から決定される
  Named Pipe を protected owner ACL で作成します。どちらも control TCP port を開きません。
- Desktop mode は macOS Keychain、Linux Secret Service、Windows Credential Manager を
  利用できます。Windows backend は大型の protected record を chunk に分割し、atomically
  switched index で管理して generic credential の payload limit を超えないようにします。
- Server Core の conservative default は引き続き `local_file`、explicit protected data
  directory、separate control token file です。`keychain` mode は対応 platform store が
  利用可能で正しく設定された場合だけ使用してください。
- Rust/Tauri desktop client は platform-aware です。macOS/Linux では user-only Unix
  Socket、Windows では SHA-256 から導出した Named Pipe と overlapped I/O で control
  を交換します。protected file は Unix では `0600`/`0700`、Windows では user-only
  `icacls` ACL と atomic replace を使用します。
- 高リスク操作の native prompt は全 platform で用意しています。macOS は `osascript`、
  Windows は PowerShell/Windows Forms で、protected input、LLM Key 選択、高リスク SSH
  確認、Capability の受け渡し、security setting 確認に対応します。Windows の port 管理は
  `netstat -ano` + `Win32_Process` で所有者を列挙し、現在の account に絞り込み、確認した
  process だけを `taskkill` で終了します。
- Frontend は platform に応じたラベルと zh/en/ja 翻訳を提供します。control transport
  （Unix Socket / Named Pipe）、credential store（Keychain / Credential Manager /
  Secret Service）、security profile、native risk 表記を切り替えます。
- CI は push のたびに Windows x64、Windows arm64、Linux x64 の Desktop ターゲットで
  Rust `cargo check` を実行し、移植済みの control client を実機 acceptance 前に
  継続的に検証します。
- Target build は明示的に分離され、`airlockd` と `airlock` の両方を生成します。Tauri bundle
  を作らず、npm installer が公開済みとする対象範囲も変更しません。

## Core と CLI のビルド

Go 1.25 以上が必要です。repository root で次を実行すると、toolchain を追加せずに
cross-compile し、`bin/<target>` に配置します。

```bash
node scripts/build-sidecar.mjs windows-amd64
node scripts/build-sidecar.mjs windows-arm64
node scripts/build-sidecar.mjs linux-amd64
node scripts/build-sidecar.mjs linux-arm64
node scripts/build-sidecar.mjs linux-armv7
```

`scripts/build-sidecar.sh` は同じ Node ドライバへの互換ラッパーとして残します。argument
なしの command は Desktop development 用の sidecar を Tauri の target triple 名で
`apps/desktop/src-tauri/binaries/` に書き出します（例: `airlockd-aarch64-apple-darwin`）。
target を明示した場合は `bin/<target>` に `airlockd`/`airlock` の通常名で書き出します。
table にない target name は binary を作成する前に fail します。

32-bit Raspberry Pi OS または Debian `armhf` の Raspberry Pi 3/4 では、
`bin/linux-armv7/airlockd` と `bin/linux-armv7/airlock` をコピーし、non-login service
account で [Server Core guide](server-deployment.ja.md) に従って実行します。64-bit
Raspberry Pi OS は `linux-arm64` を使用します。この段階では Pi 向け Desktop package は
ありません。

## Release 前に必要な Runtime Acceptance

Windows/Linux を release target にする前に、対応 architecture と distribution ごとに次を
完了する必要があります。

1. 実際の Windows Credential Manager または freedesktop.org Secret Service session で、
   create/read/rotation/delete、locked/unavailable store の fail-closed を確認する。
2. 別 local account から Windows Named Pipe と protected state/token path にアクセス
   できないこと、Linux の `0600` Unix Socket と state protection を確認する。
3. 実機の Windows/Linux toolchain で Rust/Tauri control client をコンパイルし、移植済みの
   Named Pipe/Unix control 交換、protected file ACL、Windows Forms native prompt、
   port 管理、frontend の platform ラベルを物理デバイスで検証する。
4. target hardware で service install、clean removal、upgrade、stale process recovery、
   `Direct`/`Proxy`/`Auto` egress、SSH Host Key pinning、failure closure を試験する。
5. architecture 別 installer を作成、sign、fixed checksum を公開し、install/update/uninstall
   を個別に試験する。

それまでは `airlock-installer` は Windows/Linux を `planned` と表示し、存在しない artifact
の install を拒否します。コンパイル可能な Core を supported Desktop product と誤認させない
ためです。

## 変わらない Security Semantics

SSH username mapping、fixed upstream route、local capability、LLM secondary API key、audit
redaction、proxy egress policy はすべて shared Go Core にあります。呼び出し元が受け取るのは
local endpoint と local credential だけで、upstream URL、password、private key、Host Key、
API Key は選択された protected store に残ります。

service command は [Server Core guide](server-deployment.ja.md)、development environment
以外で Core build を運用する前には [security policy](../SECURITY.md) を確認してください。
