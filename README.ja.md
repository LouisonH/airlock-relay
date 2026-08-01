<div align="center">
  <img src="website/assets/airlock-logo.svg" width="92" height="92" alt="Airlock ロゴ" />
  <h1>Airlock</h1>
  <p><strong>Agent に機能を渡し、認証情報はローカルに保つ。</strong></p>
  <p>HTTP/Wget、SSH、LLM API 向けのネイティブ認証情報分離リレーです。</p>
  <p>
    <a href="README.md">English</a> |
    <a href="README.zh-CN.md">简体中文</a> |
    <a href="README.ja.md">日本語</a> |
    <a href="docs/README.md">Documentation</a> |
    <a href="website/en/index.html">スタティックサイト</a>
  </p>
  <p>
    <a href="https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.5"><img src="https://img.shields.io/badge/release-v0.1.5%20technical%20preview-b26b25" alt="v0.1.5 テクニカルプレビュー" /></a>
    <img src="https://img.shields.io/badge/desktop-Tauri%202-397b9b" alt="Tauri 2 デスクトップ" />
    <img src="https://img.shields.io/badge/core-Go%201.25%2B-267d5f" alt="Go 1.25 以上" />
    <img src="https://img.shields.io/badge/platform-macOS-343b38" alt="macOS" />
  </p>
</div>

> [!WARNING]
> Airlock v0.1.5 はテクニカルプレビューです。保守者による本番運用準備セキュリティ監査は完了しましたが、独立した第三者監査、Developer ID 署名、Apple 公証は未完了です。運用前に[監査記録](docs/security-audit-2026-07-31.md)を確認してください。

## Airlock が必要な理由

信頼できない LLM、Agent、スクリプト、自動化ツール、サードパーティ API 中継サービスが、API 呼び出し、ファイル取得、SSH コマンド実行を必要とすることがあります。実 URL、上流アカウント、パスワード、API Key をそのまま渡すと、プロンプト、ログ、ツール出力、中継事業者に漏れる可能性があります。

Airlock は呼び出し元に固定ローカルエンドポイントと取り消し可能なルート専用認証情報だけを渡します。実対象と上流認証情報はローカル SecretStore に保ち、ポリシー検査を通過したリクエストにだけ注入します。

| 呼び出し元が受け取るもの | Airlock が保護するもの |
| --- | --- |
| ローカルルート別名 | 実 URL、ドメイン、IP、SSH アドレス |
| 取り消し可能な Capability | 上流パスワード、秘密鍵、Cookie、API Key |
| 明示的に許可された操作 | 他のルートと無制限のネットワークアクセス |
| 脱敏済みローカルエラー | 上流アイデンティティと認証情報の詳細 |

Airlock は固定ルートリレーであり、オープンプロキシ、VPN、汎用プロバイダー管理プラットフォームではありません。

## 主な機能

### HTTP / Wget

- 保護された Authorization またはカスタム Header を持つ固定上流 Base URL。
- GET/HEAD、Query 許可リスト、パス遍歴防止、同一オリジンリダイレクト制御。
- Range/206 ストリーミングとレスポンス Header のサニタイズ。
- ルートごとの `Direct`、`Proxy`、接続失敗時の安全な `Auto` 出口。

### SSH

- ローカル SSH セッションと上流 SSH セッションを分離し、身元と認証情報を分離。
- ローカルのランダム Capability、カスタムパスワード、または公開鍵認証。
- 保護された上流パスワードまたは暗号化秘密鍵と厳密な Host Key 固定。
- デフォルトはユーザー定義の 1 つの完全一致コマンド。無制限の非対話 `exec` には Airlock 内で明示的な高リスク確認が必要。
- 複数のルートで同じ上流アドレスを使用可能。異なるローカルユーザー名で、独立した上流アカウントと保護認証情報を選択。
- 対話 Shell は default で無効で、route ごとに有効化できます（`allow_interactive_shell: true`。`allow_all_commands: true` が必須）。有効にすると、PuTTY や `ssh` client は上流 Shell へ直接入り、Airlock は保存済みの上流認証情報を注入します。`su` などの対話ワークフローを扱えます。Agent/X11 Forwarding とポート転送は引き続き拒否され、PTY metadata は対話 Shell が有効なときだけ上流へ転送されます。SFTP は default で無効で、modern `scp`/SFTP client 向けに route ごとに明示的に有効化できる別の high-risk file access permission です。
- ルートごとのオプションコマンド監査は、現在のユーザー専用の `0600` ローリングファイルに保存。

### LLM API

- OpenAI-compatible `/v1/responses`、`/v1/chat/completions` ルート。
- Anthropic-compatible `/v1/messages` ルート。
- モデル許可リスト、最大出力 Token、1 分あたりリクエスト数、同時実行上限。
- 上流 Key と独立してローテーションできるランダムまたはカスタムの二次ローカル API Key。
- SSE ストリーミングとオプションのメモリ内呼び出し数、入力 Token、出力 Token 統計。
- 統計はデフォルトで無効。プロンプトやレスポンス本文は保存しません。

### ネイティブデスクトップ

- Tauri 2 + React デスクトップコンソールと Go `airlockd` sidecar。
- SSH 認証情報、Host Key 照合、一度だけ表示するローカルアクセス情報は Airlock ウィザードに統合し、
  ローカル Tauri IPC から `airlockd` へ一度だけ送信します。
- HTTP、LLM、プロキシの Secret は引き続き保護されたネイティブ入力を使用します。
- 系統/ライト/ダーク、3 つのアクセント、密度、更新間隔、モーション、中国語/英語/日本語の切り替え。
- デフォルトは loopback。プライベート LAN 公開前にネイティブ確認を実施。
- デフォルトは起動時にパスワードダイアログを表示しないローカル `0600` ファイル。macOS Keychain はより厳格なオプトイン保護モード。
- Clash 互換 HTTP CONNECT と SOCKS5/SOCKS5H プロキシ出口。

### Server Core と運用

- `airlockd --mode server` は Tauri や Desktop session なしで固定ルート core を実行します。
- `airlock` Unix Socket CLI は、上流 Secret を引数に置かずに route、SSH mapping、health、proxy egress を管理します。
- 任意の Web UI は別 token と loopback 専用 listener を使い、サニタイズ済み status と安全な route 操作だけを公開します。リモート運用では SSH tunnel を使ってください。
- service account、systemd、保護 JSON、Wget、SSH、LLM、Clash の例は [Server Core 導入と CLI](docs/server-deployment.ja.md) を参照してください。

## npm とプラットフォーム状態

Apple Silicon macOS では、検証済み Desktop preview を次で導入できます。

```bash
npm install -g airlock-relay && airlock-installer install --open
```

Windows x64/x86/ARM64 と Linux x64/ARM64 でも npm 診断 CLI は副作用なく導入できます。
`airlock-installer status --json` または `airlock-installer platform --json` で現在の契約を
確認してください。これらは CI preview であり public verified installer ではありません。
`install` は fail-closed し、未検証の CI artifact を download しません。Linux ARMv7 と macOS
x64 は planned です。

## 開発環境での起動

必要環境：Go 1.25+、Node.js 20+、Rust/Cargo、Tauri 2 プラットフォーム依存関係。

```bash
git clone https://github.com/LouisonH/airlock-relay.git
cd airlock-relay/apps/desktop
npm install
npm run build
npm run tauri dev
```

リポジトリルートで検査を実行します。

```bash
go test -race ./...
go vet ./...
```

デフォルト入口：

- HTTP / LLM: `127.0.0.1:4768`
- SSH: `127.0.0.1:4770`
- 制御面：現在のユーザー専用 Unix Socket

## セキュリティモデル

Airlock は固定対象、最小権限、認証情報の置き換え、脱敏済みエラーで Secret 露出を減らしますが、OS サンドボックスではありません。

- ローカル管理者、root、Airlock をデバッグできるプロセス、OS を制御する攻撃者は脅威モデル外です。
- 上流応答や SSH コマンド出力が自身の環境を開示する場合があります。汎用リレーはすべてのアプリケーション層情報を除去できません。
- 無制限 SSH `exec` は上流アカウントのリモートコード実行に近い権限です。専用の最小権限アカウントを使用してください。
- Capability は 1 つのルートにアクセスを制限しますが、漏えいした場合はローテーションしてください。
- コマンド監査が有効な場合、パスワードや Token をコマンド引数に入れないでください。

詳細は [セキュリティポリシー](SECURITY.md)、[実装と脅威モデル](.claude/plan/airlock-1.md)、[デスクトップ UI セキュリティ仕様](docs/ui-spec.md) を参照してください。

## ライセンス

Copyright 2026 LouisonH。Airlock は [Apache License 2.0](LICENSE) で公開されています。

## 開発者

今後の対象と安全境界については、[クロスプラットフォーム対応計画](docs/cross-platform.ja.md) を参照してください。

Airlock は華南理工大学（SCUT）に関わる開発者 [**LouisonH**](https://0o0.site) が製品設計とコア開発を行い、**GPT-5.6 Sol** を用いた AI 支援の実装と検証を行っています。独立した個人プロジェクトであり、華南理工大学の公式プロジェクト、立場、承認を表すものではありません。GitHub: [github.com/LouisonH](https://github.com/LouisonH)。
