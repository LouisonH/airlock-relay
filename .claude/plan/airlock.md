# Airlock 项目规划

> 状态：已被 `airlock-1.md` 取代
> 日期：2026-07-29
> 一句话定位：运行在本机的 SSH 与 HTTP 安全中转器，隔离上游地址和凭据，并按规则选择直连或代理出口。

## 1. 名称与定位

推荐名称：**Airlock**，中文名可用“隔舱”。

- 守护进程：`airlockd`
- 命令行工具：`airlock`
- 配置概念：Route（路由）、Secret（凭据）、Egress（出口）
- 项目描述：Local credential-isolating relay for SSH and HTTP

备选名称：RelayVault、GateTunnel、渡口。当前目录已经叫 Airlock，且名称与“客户端和真实目标之间有一层隔离舱”的产品语义最吻合。

## 2. 目标定义

Airlock 解决三类问题：

1. **固定 URL 中转**：`wget`、脚本或浏览器请求一个本地自定义 URL，由 Airlock 转发到配置好的上游 URL。
2. **SSH 凭据隔离**：用户用本地账号、密码或公钥连接 Airlock；Airlock 再使用保存在系统钥匙串中的真实上游凭据连接 SSH 主机。
3. **代理出口切换**：直连失败或规则要求时，通过 Clash 等程序提供的 HTTP CONNECT 或 SOCKS5 代理访问网页或 SSH 主机。

### 威胁模型

本项目保护上游地址、密码、私钥和令牌不被普通调用脚本或使用者直接获取，并避免它们出现在命令行、配置文件及日志中。

它不承诺防御拥有本机管理员/root 权限、可调试 Airlock 进程或已控制操作系统的攻击者。该边界必须在文档和 UI 中明确说明。

## 3. 协议边界

### SSH

SSH 客户端不能向普通 `http://` URL 建立 SSH 会话。Airlock 必须提供本地 SSH 地址，例如：

```bash
ssh prod@127.0.0.1 -p 4769
```

`prod` 是本地路由账号。Airlock 验证本地账号后终止这条 SSH 会话，再使用该路由绑定的上游账号和凭据建立第二条 SSH 会话。单纯 TCP 转发无法替换 SSH 密码，也就无法实现真正的密码隔离。

可在 `~/.ssh/config` 中提供更友好的别名：

```sshconfig
Host airlock-prod
    HostName 127.0.0.1
    Port 4769
    User prod
```

### HTTP/Wget

产品最终可提供两种互补模式：

1. **固定映射模式**：本地 URL 的路由别名映射到固定上游，只允许配置范围内的路径和方法。
2. **标准代理模式（MVP 后）**：Airlock 作为本地 HTTP/HTTPS 代理，供 `wget` 的 `http_proxy`/`https_proxy` 使用。HTTPS 通过 CONNECT 隧道，不做 TLS 中间人解密。

固定映射示例：

```bash
wget http://127.0.0.1:4768/r/manual/releases/latest.pdf
```

后续标准代理示例：

```bash
https_proxy=http://127.0.0.1:4770 wget https://example.com/file.zip
```

## 4. MVP 范围

### 必须具备

- 仅监听 `127.0.0.1`/`::1`，默认不允许局域网访问。
- 使用一份可校验的配置定义 SSH、HTTP 路由和出口策略。
- HTTP 固定 URL 映射，流式转发响应，支持 GET/HEAD、Range 断点续传、查询参数和受控重定向。
- HTTP 路由使用本地 Token 或 Basic Auth，自动化下载可通过权限为 `0600` 的 `.netrc` 提供本地凭据。
- SSH 本地密码或公钥认证，并映射至上游密码或私钥认证。
- SSH 交互 shell 与非交互 `exec`；SFTP/SCP 是否进入首版由协议验证结果决定。
- 出口策略：`direct`、`proxy`、`auto`。
- 代理类型：SOCKS5 和 HTTP CONNECT，可直接指向 Clash 的 mixed/socks/http 端口。
- 上游密码、私钥口令、HTTP 令牌存入 macOS Keychain / Windows Credential Manager / Linux Secret Service。
- 上游 SSH host key 严格校验；首次信任需显式确认并写入 Airlock 的 known_hosts。
- 日志自动脱敏，不记录密码、私钥、Authorization、Cookie 或完整敏感查询参数。
- CLI 提供配置、启动、停止、状态检查和连通性测试。

### 暂不进入 MVP

- 公网部署、多机同步、团队账号与云端控制台。
- HTTPS 内容解密、证书注入或 MITM。
- 任意目标 URL 的匿名开放代理。
- 标准 HTTP/HTTPS 正向代理及 PAC；固定路由稳定后再增加。
- RDP、数据库代理等更多协议。
- 自动修改系统代理或 Clash 配置。
- 完整桌面 GUI；核心稳定后再做托盘控制和本地管理页。

## 5. 推荐技术方案

首选 **Go**：适合网络并发、跨平台单文件分发，并有成熟的 SSH、HTTP、SOCKS5 和系统钥匙串库。

建议模块：

```text
cmd/airlock        CLI
cmd/airlockd       后台服务
internal/config    配置解析、校验、热加载
internal/route     路由模型与匹配
internal/secret    系统钥匙串抽象
internal/egress    direct / HTTP CONNECT / SOCKS5 / auto
internal/httpgw    固定映射与正向代理
internal/sshgw     SSH 服务端、上游客户端与通道桥接
internal/audit     脱敏日志与审计事件
internal/control   CLI 与守护进程的本地控制通道
```

技术基线：

- HTTP 服务与反向代理优先使用 Go 标准库。
- SSH 使用 `golang.org/x/crypto/ssh`，SOCKS5 使用 `golang.org/x/net/proxy`。
- 配置使用 YAML；密码等秘密只存钥匙串引用，不写明文。
- 本地账号密码只保存 Argon2id 哈希。
- CLI 与守护进程优先使用 Unix domain socket；Windows 使用 named pipe。
- 首版不引入数据库，避免额外迁移和运维复杂度。

## 6. 核心数据模型

```yaml
version: 1

listeners:
  http_relay: 127.0.0.1:4768
  ssh: 127.0.0.1:4769

egresses:
  clash:
    type: socks5
    address: 127.0.0.1:7890

routes:
  - id: manual
    type: http
    local_path: /r/manual
    upstream_base_url: https://downloads.example.com/docs/
    methods: [GET, HEAD]
    local_auth_ref: keyring://airlock/inbound/manual
    allow_redirect_hosts: [downloads.example.com]
    egress: auto:clash

  - id: prod
    type: ssh
    local_user: prod
    local_auth_ref: keyring://airlock/inbound/prod
    upstream_host: server.example.com
    upstream_port: 22
    upstream_user: deploy
    upstream_auth_ref: keyring://airlock/upstream/prod
    host_key_ref: known-hosts://server.example.com:22
    egress: auto:clash
```

真实配置格式可在实现阶段调整，但必须保持“公开配置只引用 Secret，Secret 不落盘”的原则。

## 7. 请求流程

### HTTP 固定映射

1. 客户端访问 `/r/{route}/{suffix}`。
2. Airlock 验证本地访问令牌、路由、方法、路径和请求体限制。
3. 使用结构化 URL API 将 `suffix` 拼接到固定上游，禁止 `..`、绝对 URL 等逃逸，并检查解析后的 IP/域名策略。
4. 按出口策略拨号：直连、代理，或在安全条件下直连失败后转代理。
5. 流式返回响应，并按规则重写仍属于该上游的 `Location`。

### HTTP 正向代理（MVP 后）

1. `wget` 将普通 HTTP 请求或 HTTPS CONNECT 发给 Airlock。
2. Airlock 校验本地代理认证和目标 ACL。
3. 根据域名/端口匹配出口规则，再直连或经 Clash 拨号。
4. HTTPS 只转发加密字节流，Airlock 不读取内容。

### SSH

1. SSH 客户端连接本地端口，以 `local_user` 和本地密码/公钥认证。
2. Airlock 用该本地账号确定唯一 SSH 路由。
3. 从系统钥匙串读取上游凭据，并按出口策略连接上游 SSH。
4. 严格验证上游 host key。
5. 桥接允许的 SSH channel/request；首版默认禁用 agent forwarding 和远程端口转发。
6. 会话结束后释放上游连接，只写脱敏审计事件。

## 8. `auto` 出口策略

`auto` 不是无限重试：

- SSH/TCP：直连在指定连接超时或明确网络错误后，尝试一次配置的代理。
- HTTP GET/HEAD：仅在尚未收到响应、且请求体可安全重放时切换代理。
- POST/PUT/PATCH：默认不自动重试，避免重复提交；需路由显式开启幂等策略。
- TLS、认证、host key、HTTP 4xx/5xx 错误不视为“网络不通”，不得静默改走代理掩盖问题。
- 代理不可用时返回包含阶段信息但不泄漏秘密的错误。

Clash 端口不应依赖猜测。初始化向导可探测常见的本地端口，但最终必须由用户确认并持久化明确地址。

## 9. 安全基线

- 回环监听不是完整认证；HTTP relay/proxy 和控制通道仍需本地令牌或 OS 身份校验。
- 禁止默认绑定 `0.0.0.0`；若未来开放远程访问，必须另行设计 TLS/mTLS、ACL 和速率限制。
- 固定 URL 路由不可接受完整目标 URL，防止变成 SSRF 或开放代理。
- 结构化处理 URL、Header 和 SSH 消息，移除 hop-by-hop headers。
- 限制请求体、Header、并发连接、空闲时间和总会话时间。
- 上游跳转默认只允许配置域名；跨域跳转需显式 allowlist。
- SSH host key 不使用 `InsecureIgnoreHostKey`。
- 默认关闭 SSH agent forwarding、X11、远程端口转发；本地端口转发按路由授权。
- 密码不得作为 CLI 参数传入，使用交互式无回显输入或系统钥匙串。
- 日志中的用户名、目标、路径按配置决定是否脱敏，永不记录 Secret。
- 配置文件采用用户专属权限；启动时拒绝权限过宽的敏感文件。
- 默认拒绝云元数据、回环和链路本地目标；内网目标必须在具体路由显式授权，DNS 重解析和每次重定向都重新检查。
- 本地 SSH/HTTP 认证有失败限速、并发上限和空闲超时，避免本机其他进程暴力尝试。

## 10. CLI 草案

```bash
airlock init
airlock egress add clash --type socks5 --address 127.0.0.1:7890
airlock route add http manual --base-url https://downloads.example.com/docs/
airlock route add ssh prod --host server.example.com --user deploy
airlock secret set upstream/prod
airlock trust ssh prod
airlock serve
airlock status
airlock test manual
airlock test prod
```

`route add` 与 `secret set` 分开，确保 Secret 不出现在 shell history。后续本地管理界面也应调用相同的控制 API，而不是直接改配置文件。

## 11. 实施阶段

### P0：协议验证

- 建立最小 Go 工程和测试框架。
- 验证经 SOCKS5/HTTP CONNECT 建立 TCP 与 SSH 连接。
- 验证 SSH 服务端终止会话后桥接 shell、exec、窗口尺寸和退出码。
- 验证 HTTP relay 的流式下载、重定向和取消传播。
- 产出明确的 SCP/SFTP 支持结论。

### P1：安全基础与配置

- 完成配置 schema、校验、权限检查、路由模型。
- 完成跨平台 SecretStore 和 known_hosts 管理。
- 完成控制通道、结构化日志和脱敏测试。

### P2：HTTP/Wget 能力

- 实现固定映射、认证、ACL、流式转发和重定向处理。
- 支持 Range/206、查询参数、内容处置和客户端取消传播。
- 接入 direct/proxy/auto 出口并覆盖失败分类测试。

### P3：SSH 能力

- 实现本地认证、路由选择、上游认证和 host key 验证。
- 支持 shell、exec、PTY、窗口变更、信号与退出码。
- 按 P0 结论增加 SFTP/SCP；端口转发使用显式能力开关。

### P4：可用性与发布

- 完成初始化向导、route/secret/trust/test/status 命令。
- 增加崩溃恢复、优雅停机、配置重载和诊断包。
- 完成 macOS、Windows、Linux 的打包与端到端测试。

### P5：本地界面

- 托盘状态、路由启停、连通性检查和审计记录。
- 所有敏感录入走系统安全输入，界面不回显已保存 Secret。

### P6：协议扩展

- 在独立监听端口增加带认证和目标 ACL 的 HTTP 正向代理、HTTPS CONNECT 与可选 PAC。
- 根据实际需求扩展 SFTP/SCP、本地端口转发及更多协议；各能力默认关闭并按路由授权。

## 12. 验收标准

- `wget` 访问本地映射能下载和断点续传上游文件，内容哈希一致，且大文件不会被完整缓存在内存。
- 配置 `auto:clash` 后，模拟直连不可达时 GET/HEAD 和 SSH 能经代理成功；非幂等 HTTP 请求不被自动重复。
- SSH 使用本地自定义账号认证后能进入配置的上游 shell，执行命令并得到正确退出码。
- 客户端、命令行参数、配置文件和日志中均不出现上游密码/私钥内容。
- 上游 SSH host key 变化时连接失败并产生清晰审计事件。
- 错误本地密码、无效 HTTP token、越权路径和未授权端口转发均被拒绝。
- 服务默认只在 loopback 监听；重启后路由可用，Secret 仍由系统钥匙串提供。
- 代理进程未启动时快速失败，并明确区分直连失败与代理失败。
- 单元测试覆盖 URL 拼接、header 清理、重试判定、配置校验、日志脱敏和 SSH 权限规则。
- 端到端测试至少覆盖 direct、SOCKS5、HTTP CONNECT 三种出口。

## 13. 需要确认的产品决策

1. 第一版是否只做 CLI，还是必须同时提供托盘/本地 Web 管理页？推荐先 CLI，P5 再补界面。
2. SSH 第一版是否必须支持 SFTP/SCP 和端口转发？推荐 shell + exec 必做，SFTP/SCP 尽量纳入，端口转发默认关闭。
3. HTTP 固定映射是否需要 POST/上传？推荐首版只开放 GET/HEAD，随后按路由增加允许的方法。
4. 首发平台优先级是什么？推荐 macOS 优先，接口保持跨平台，随后验证 Windows/Linux。
5. 一个 SSH 路由是否允许多名本地用户共享同一上游身份？推荐数据模型支持，但首版 UI 保持一对一配置。

## 14. 建议的首个里程碑

先完成一个不带 GUI 的技术样机：

- 一个固定 HTTP 路由可被 `wget` 下载；
- 一个本地 SSH 路由可进入上游 shell；
- 二者都能在 direct 与 Clash SOCKS5 之间切换；
- 上游 Secret 只存在系统钥匙串；
- 全流程有 host key 校验、访问认证和脱敏日志。

这个里程碑能最早验证项目中风险最高的两点：SSH 双会话桥接是否完整，以及直连失败后的代理切换是否可靠。
