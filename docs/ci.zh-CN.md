# CI 核验

仓库中的 `.github/workflows/verify.yml` 会在 Pull Request 与受保护分支上运行。它刻意只做核验，不做部署。

## 核验内容

- Go 格式与完整 Go 测试。
- Desktop 的 TypeScript 与 Vite 生产构建。
- npm 安装器测试与包清单。
- 项目主页和文档脚本的 JavaScript 语法。

## 不应获得的凭据

普通核验任务和 Pull Request 不需要，也绝不能拿到：

- npm 发布 token；
- GitHub Pages 或 Release 部署 token；
- Airlock control token 或 Web UI token；
- 路由 JSON 规格、上游 URL、密码或 API Key；
- 创建路由时生成的本地 capability 或二次 API Key。

## 发布原则

只在人工核验发布产物与校验和后，从受保护分支发布；使用独立、最小权限的 npm token。包的 `prepack` 会在 DMG SHA-256 与发布定义一致后才暂存产物；缺少或不匹配时会失败关闭。

不要在 CI 中创建生产 Airlock 路由。路由规格和创建时一次性输出的本地凭据都属于敏感数据。
