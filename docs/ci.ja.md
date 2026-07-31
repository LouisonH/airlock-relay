# CI 検証

`.github/workflows/verify.yml` は Pull Request と保護 branch で動作します。これは deploy ではなく verification workflow です。

## 確認する内容

- Go format と full Go test。
- Desktop の TypeScript / Vite production build。
- npm installer test と package manifest。
- static project site と documentation script の JavaScript syntax。

## 渡してはいけないもの

通常の検証 job と Pull Request は、npm publish token、GitHub Pages / release deploy token、Airlock control token、Web UI token、route JSON spec、上流 URL、password、API key、生成済み local capability を受け取ってはいけません。

## Publishing

人が release artifact と checksum を確認した後、protected branch からだけ publish します。least-privilege npm token を使ってください。package の `prepack` は DMG SHA-256 が release definition と一致した時だけ artifact を stage し、不足や不一致では fail closed します。

CI で production Airlock route を作成しないでください。route spec と一回だけ出力される local credential は sensitive data です。
