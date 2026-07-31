# Airlock Server Core の導入と CLI

`airlockd` は Desktop を必要とせず、サーバー、NAS、踏み台、運用ホストで常駐できます。`airlock` CLI はサービスアカウント専用 Unix Socket 経由で固定 HTTP/Wget、SSH、LLM ルートを管理し、任意の Web UI はサニタイズ済み状態と安全な運用操作だけを提供します。

Airlock はオープンプロキシや VPN ではありません。呼び出し元は任意の宛先を指定できず、作成時に保護ストアへ保存された固定上流だけへ転送されます。

## 構成

- `airlockd`: Tauri/Desktop 非依存の Go コア。server モードではデフォルトでローカル `0600` SecretStore を使用します。
- `airlock`: 運用 CLI。上流 URL、パスワード、API Key をコマンド引数で受け取りません。
- Web UI: デフォルト無効、loopback 専用、別 Bearer token。状態、ヘルス、ルートの有効/無効、全停止だけが可能で、ルート作成、上流認証情報の入力/表示、削除はできません。
- Desktop: 任意のローカル GUI であり、サーバーコアの実行時依存ではありません。制御 Socket をネットワークへ公開しないでください。

## ビルドと起動

Go 1.24+ が必要です。root ではなく専用の非ログインユーザーで動かします。

```bash
go build -trimpath -o /usr/local/bin/airlockd ./cmd/airlockd
go build -trimpath -o /usr/local/bin/airlock ./cmd/airlock
sudo useradd --system --create-home --shell /usr/sbin/nologin airlock
sudo install -d -o airlock -g airlock -m 0700 /var/lib/airlock
sudo install -d -o airlock -g airlock -m 0700 /etc/airlock
sudo -u airlock /usr/local/bin/airlock token generate --output /etc/airlock/control.token
sudo -u airlock /usr/local/bin/airlock token generate --output /etc/airlock/web.token
```

トークンは端末に表示されず、新しい `0600` ファイルにだけ書き込まれます。control と Web UI には異なるトークンを使用してください。

```bash
sudo -u airlock /usr/local/bin/airlockd \
  --mode server \
  --data-dir /var/lib/airlock \
  --control-token-file /etc/airlock/control.token \
  --listen 127.0.0.1:4768 \
  --ssh-listen 127.0.0.1:4770 \
  --web-listen 127.0.0.1:4769 \
  --web-token-file /etc/airlock/web.token
```

Web UI を使わない場合は両方の Web flag を省略します。Web UI は `127.0.0.1` または `::1` だけを受け付けます。リモート運用では SSH ローカル転送を使用します。

```bash
ssh -L 4769:127.0.0.1:4769 operator@example-server
```

`http://127.0.0.1:4769` を開き、保護された Web token を貼り付けます。token はタブ単位の `sessionStorage` にのみ保持されます。LAN 公開が必要な場合だけ `--network-scope lan` と private address を明示し、必ず firewall、VPN、または SSH tunnel で制限してください。公開インターネットへ直接公開しないでください。

[systemd サンプル](../deploy/systemd/airlock.service.example) を `/etc/systemd/system/airlock.service` に配置後、`systemctl daemon-reload`、`systemctl enable --now airlock` を実行できます。

## CLI

Socket は `0600` のため、サービスユーザーとして実行します。

```bash
sudo -u airlock /usr/local/bin/airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token status
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes list
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes health releases
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes stop-all --yes
```

`--socket /var/lib/airlock/control.sock` は `--data-dir` の代替です。出力は JSON です。作成時に生成される local capability または二次 API Key は一度だけ出力されるため、shell history、CI log、ticket に残さないでください。

## 保護された仕様ファイル

上流情報は、絶対パス、通常ファイル、symlink ではない、`0600` JSON からのみ読みます。URL や password を flag に入れないでください。

HTTP/Wget:

```json
{"name":"Release mirror","alias":"releases","base_url":"https://upstream.example.invalid/releases/","authorization":"Bearer upstream-secret","egress":"Auto"}
```

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes create http --file /etc/airlock/releases.json
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable releases
wget --header="Authorization: Bearer <local-capability>" http://127.0.0.1:4768/r/releases/file.tar.gz
```

LLM:

```json
{"name":"Coding model","alias":"coding","base_url":"https://api.example.invalid/v1","authorization":"upstream-api-key","provider":"openai","models":["example-coding"],"max_output_tokens":4096,"requests_per_minute":60,"max_concurrent":4,"track_usage":true,"egress":"Auto"}
```

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes create llm --file /etc/airlock/coding.json
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable coding
```

`anthropic` provider では `authorization` を上流 `X-Api-Key` として保存します。usage tracking は数値だけで、prompt/response 本文は保存しません。

## SSH と Proxy

最初に Host Key を取得して人手で確認します。port を省略すると SSH は `22` を使用します。

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token ssh probe --address ssh.example.invalid:22 --egress Auto
```

SSH 仕様には probe 出力の `host_key`、`local_username`、上流 `username`/`password`、正確な `allowed_command` を入れます。`local_username` により同じ上流 address の別アカウントを選択できます。作成時は route と保護 target を **disabled** で保存するだけで、上流へ接続したり失敗時に設定を削除したりしません。まず明示的に health check を実行し、成功後に enable してください。

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes health build-host
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token routes enable build-host
```

health check は route の認証 budget 内で固定 Host Key と上流 password を検証します。default は 20 秒、範囲は 3-120 秒です。`allow_all_commands: true` は `--allow-all-confirmed` も必須で、専用最小権限アカウント以外では使わないでください。shell、PTY、SFTP、port forwarding、Agent/X11 forwarding は拒否されます。

Clash 互換 HTTP CONNECT/HTTPS CONNECT/SOCKS5/SOCKS5H を使うには `0600` の proxy JSON に `{ "url": "socks5://127.0.0.1:7890" }` を書きます。

```bash
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token proxy set --file /etc/airlock/proxy.json
airlock --data-dir /var/lib/airlock --token-file /etc/airlock/control.token proxy clear --yes
```

ルートごとに `Direct`、`Proxy`、`Auto` を選択します。`Auto` はレスポンス開始前の再試行可能な直連失敗だけで proxy を試します。

本番導入前に [セキュリティポリシー](../SECURITY.md)、[English](server-deployment.md)、[简体中文](server-deployment.zh-CN.md) を確認してください。
