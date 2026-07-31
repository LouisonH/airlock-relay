# クロスプラットフォーム Core 移植ベースライン

Airlock v0.1.4 で公開済みなのは Apple Silicon macOS 向け Desktop preview だけです。
この branch で追加するのは Windows と Linux の **Core/CLI コンパイルベースライン**であり、
Windows/Linux Desktop、インストーラー、auto-update、署名済み成果物の公開を意味しません。

| 対象 | Core / CLI build | local control transport | platform secret backend | Desktop bundle | 状態 |
| --- | --- | --- | --- | --- | --- |
| macOS arm64 | native | user-only Unix Socket | Keychain / protected file | DMG / `.app` | preview 公開済み |
| macOS x64 | target build | user-only Unix Socket | Keychain / protected file | DMG / `.app` | installer は予定 |
| Windows x64 | cross-compiled | current-owner ACL Named Pipe | Credential Manager / protected file | NSIS / MSI | Desktop は予定 |
| Windows arm64 | cross-compiled | current-owner ACL Named Pipe | Credential Manager / protected file | NSIS / MSI | Desktop は予定 |
| Linux x64 | cross-compiled | user-only Unix Socket | Secret Service / protected file | AppImage / deb | Desktop は予定 |
| Linux arm64 | cross-compiled | user-only Unix Socket | Secret Service / protected file | AppImage / deb | Desktop は予定 |
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
- Target build は明示的に分離され、`airlockd` と `airlock` の両方を生成します。Tauri bundle
  を作らず、npm installer が公開済みとする対象範囲も変更しません。

## Core と CLI のビルド

Go 1.25 以上が必要です。repository root で次を実行すると、toolchain を追加せずに
cross-compile し、`bin/<target>` に配置します。

```bash
./scripts/build-sidecar.sh windows-amd64
./scripts/build-sidecar.sh windows-arm64
./scripts/build-sidecar.sh linux-amd64
./scripts/build-sidecar.sh linux-arm64
./scripts/build-sidecar.sh linux-armv7
```

argument を省略した command は、Desktop development 用に current host の
`bin/airlockd` と `bin/airlock` を維持します。table にない target name は binary を作成する
前に fail します。

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
3. Rust/Tauri control client を移植し、Unix-only import、Unix stream、filesystem permission、
   macOS-only confirmation を置き換え、高リスク操作に同等の native confirmation を用意する。
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
