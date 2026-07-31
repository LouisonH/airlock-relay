# クロスプラットフォーム対応計画

Airlock v0.1.3 で配布しているのは Apple Silicon Mac 版だけです。その他の項目は
実装契約であり、インストーラーが公開済みであることを意味しません。

| 対象 | バンドル | ローカル制御経路 | 保護ストア | 状態 |
| --- | --- | --- | --- | --- |
| macOS arm64 | DMG / `.app` | ユーザー専用 Unix Socket | 0600 ファイル / Keychain | プレビュー公開済み |
| macOS x64 | DMG / `.app` | ユーザー専用 Unix Socket | 0600 ファイル / Keychain | 契約のみ |
| Windows x64 | NSIS / MSI | ユーザー ACL 付き Named Pipe | 保護ファイル / Credential Manager | 契約のみ |
| Linux x64 | AppImage / deb | 0600 Unix Socket | 保護ファイル / Secret Service | 契約のみ |
| Linux arm64 | AppImage / deb | 0600 Unix Socket | 保護ファイル / Secret Service | 契約のみ |

`packages/airlock/lib/platform.mjs` はパッケージング用の共通リゾルバーです。公開済みの
成果物名と固定 SHA-256 の両方がない対象は必ず拒否し、計画中の対象を配布済みとして扱いません。

## 対応境界

1. デスクトップ制御経路を抽象化し、macOS/Linux では Unix Socket、Windows では
   現在のユーザーだけがアクセスできる Named Pipe を使用します。
2. Go 製 `airlockd` コアを共有し、対象ごとの sidecar をビルドします。ネイティブ CI で
   race test とアーキテクチャ検証を実行します。
3. SSH の機密入力はクロスプラットフォーム Airlock ウィザードで行い、一度限りのローカル IPC
   コマンドで送信します。他の Secret と OS セキュリティ変更には各 OS のネイティブ画面を使用します。
   Secret をコマンドライン、環境変数、プロセス一覧、ログ、永続制御状態へ渡しません。
4. Windows Credential Manager と Linux Secret Service を追加し、既存の
   コピー、検証、切り替え、消去という移行手順を維持します。
5. 各成果物を個別に署名・検証し、インストール、削除、更新、fail-closed テストが完了してから
   `released` に変更します。

## バージョンと更新の契約

デスクトップのバージョン確認はユーザーが明示的に実行した場合だけ行う読み取り専用操作です。
WebView は公式 GitHub Releases の公開メタデータだけを読み取り、ローカルルート状態、
保護対象、認証情報、アクティビティデータを送信しません。自動ダウンロード、インストール、
再起動、リリースページ表示も行いません。各プラットフォームでユーザーが選んだインストーラーを
独立して検証できるまで、更新フローを公開済みとして説明してはいけません。

SSH ユーザー名マッピングは全 OS で共通です。同じリスナー上のローカルユーザー名が一つの
ルートを選択し、各ルートは独立した Capability ダイジェストと保護された上流対象を保持します。
