# Airlock 项目规划 v2

> 状态：已确认，P0 开发中
> 日期：2026-07-29
> 取代：`airlock.md`
> 一句话定位：向不受信任的 LLM、Agent 和脚本提供受限的本地转发能力，同时隐藏真实目标地址和上游凭据。

## 1. 本轮修正

Airlock 是一个纯粹的本地安全转发器，不是 AI 工具生态管理器，也不复刻 CC Switch。

CC Switch 只提供两点产品灵感：原生桌面控制台和系统托盘快速控制。Airlock 不管理 MCP、Prompt、Skill、会话、工作区、供应商市场或云同步。

项目的唯一核心是：

```text
不受信任调用者
  -> 本地别名 + 受限能力凭据
  -> Airlock 策略检查与秘密注入
  -> 直连或本地代理
  -> 隐藏的真实上游
```

## 2. 安全目标

LLM/Agent 只获得完成某个任务所需的最小能力，而不获得：

- 真实上游 URL、域名或 SSH 地址；
- 上游用户名、密码、私钥和私钥口令；
- LLM 服务商的真实 API Key；
- Clash 等代理的认证信息；
- 其他路由的存在、配置或访问权。

Airlock 发给调用者的本地 Token、SSH 本地账号或临时密钥不是上游秘密，而是可撤销的 Capability（能力凭据）。即使它被 LLM 输出或记录，影响也被限定在对应路由、方法、模型、速率和有效期内。

能力 Token 使用至少 256 位随机不透明值，服务端只保存哈希，不使用难以撤销的 JWT。短期 Agent 任务优先签发短 TTL、单路由、可选单次使用的 Token。

### 明确边界

- Airlock 不能防御本机管理员/root、可调试 Airlock 进程或已控制操作系统的攻击者。
- 如果上游响应正文、重定向内容、SSH shell 提示符或命令输出主动包含真实地址，通用转发器无法保证完全隐藏；Airlock 只能清理协议 Header、错误信息和已知格式字段。
- 凭据隔离不等于操作授权。SSH 上游账号本身仍需遵守最小权限原则，否则 LLM 获得 shell 后仍可能执行高风险操作。
- Airlock 不声称能识别所有 Prompt Injection；它通过固定路由、参数约束和最小权限限制注入后的破坏范围。

## 3. 路由与能力模型

每条 Route 由四部分组成：

1. **Ingress**：本地监听地址、别名和能力凭据。
2. **Target**：真实 URL、SSH 地址、上游用户与认证 Secret，全部按秘密处理。
3. **Policy**：允许的方法、路径、模型、参数、速率、并发、有效期和响应限制。
4. **Egress**：`direct`、`proxy` 或 `auto`，代理兼容 SOCKS5 与 HTTP CONNECT。

能力凭据至少支持：

- 单路由绑定，不可访问其他路由；
- 随时撤销与轮换；
- 可选到期时间；
- 请求速率和并发上限；
- 最近使用时间与脱敏审计；
- 仅保存哈希，无法从配置数据库还原明文。

## 4. 三种核心转发

### HTTP/Wget 路由

调用者看到：

```bash
wget --netrc http://127.0.0.1:4768/r/manual/releases/latest.pdf
```

Airlock 内部完成：

- 将 `/r/manual` 映射到隐藏的固定上游基址；
- 注入上游 Authorization、Cookie 或自定义 Header；
- 校验路径、查询参数、方法、Body 大小及重定向域名；
- 支持 GET、HEAD、Range/206 和流式下载；
- 清理会泄露上游的 hop-by-hop Header、错误细节和受控重定向；
- 按策略选择直连或 Clash 等代理出口。

首版不允许调用者提交完整目标 URL，防止 Airlock 退化成 SSRF 工具或开放代理。

### SSH 路由

调用者看到：

```bash
ssh build@127.0.0.1 -p 4769
```

`build` 是本地路由别名，不是真实上游用户。Airlock 必须终止本地 SSH 会话，再使用隐藏的地址、账号与凭据建立上游 SSH 会话。普通 TCP 转发无法替换 SSH 凭据。

首版策略：

- 支持本地密码或本地公钥认证；Agent 场景优先使用短期本地公钥或高熵密码；
- 支持上游密码或私钥认证；
- 默认只允许受限 `exec`/forced-command；交互 shell、PTY、窗口变化需路由显式开启并显示高风险警告；
- 严格验证 known_hosts；
- 默认关闭 Agent Forwarding、X11、SFTP/SCP 和端口转发；
- 强约束场景依赖远端 forced-command wrapper 或专用低权限 SSH 账号，不把字符串黑名单当成沙箱；
- 可对路由配置命令 allowlist/denylist，但不能把文本过滤当成完整沙箱；
- 为高风险 SSH 路由提供“每次连接需在 GUI 确认”的可选模式。

### LLM API 路由

LLM 路由仍是 HTTP 转发的安全预设，不发展成供应商管理平台。

调用者只得到本地 Base URL 和受限 Capability Token：

```text
OPENAI_BASE_URL=http://127.0.0.1:4768/llm/coding
OPENAI_API_KEY=<airlock capability token>
```

Airlock 用真实上游 Base URL 和 API Key 转发请求，并可限制：

- OpenAI-compatible `/v1/responses`、`/v1/chat/completions`；
- Anthropic-compatible `/v1/messages`；
- 允许的模型列表；
- 单次最大输入/输出 Token；
- 每分钟请求数、并发数和可选额度；
- 禁止透传调用者提供的 Authorization 等敏感 Header；
- 上游失败时是否允许在预先配置的目标之间故障转移。

LLM 能看到自己的提示词和模型输出，但看不到 Airlock 注入的上游 API Key。

## 5. 原生 GUI 边界

Airlock 必须提供桌面 GUI，但不提供浏览器访问的 Web 管理页面。

推荐形态：

- 常驻系统托盘；
- 主窗口是安静、紧凑的运维控制台；
- GUI 只通过 Unix domain socket / Windows named pipe 与 `airlockd` 通信；
- 控制接口不监听普通 TCP 端口；
- 关闭主窗口后转发服务继续运行，退出守护进程需要明确操作。

### 页面结构

```text
概览
路由
  - HTTP / Wget
  - SSH
  - LLM API
活动
设置
```

路由页使用可扫描的表格/列表，显示：名称、类型、状态、本地入口、权限摘要、出口策略、最近使用和健康状态。上游目标始终显示为“已保护”，不在列表中展示真实值。

新增/编辑路由采用步骤式表单：

1. 类型和本地别名；
2. 上游目标与 Secret；
3. 权限和限额；
4. 直连/代理策略；
5. 连通性测试与启用。

敏感字段采用 replace-only：保存后只显示“已设置”，不能直接复制或回显。删除路由时同时询问是否删除对应 Secret。

活动页默认只记录时间、路由、调用者、动作类型、结果、延迟和出口，不记录请求/响应正文、Authorization、Cookie、真实地址或 Secret。SSH 路由可显式开启完整命令审计；开启时必须提示命令参数可能含敏感信息，并将审计文件限制为当前用户可读写。

系统托盘只提供服务启停、总体状态、最近错误、路由快速启停和打开主窗口，不承担复杂配置。

## 6. 技术架构

推荐采用分离式桌面架构：

```text
Airlock Desktop
  Tauri 2 + React + TypeScript
          |
          | 本地受保护控制通道
          v
airlockd
  Go 网络与策略核心
          |
          +-- HTTP/Wget gateway
          +-- SSH gateway
          +-- LLM HTTP gateway
          +-- direct / SOCKS5 / HTTP CONNECT
          +-- SecretStore / Policy / Audit
```

选择理由：

- Tauri 2 适合跨平台原生窗口、托盘、自动更新与安装包；
- Go 的 HTTP、SSH 服务端/客户端和代理拨号生态更成熟；
- 守护进程与 GUI 分离后，关闭窗口不会中断转发，CLI 也能复用同一核心；
- UI 进程不需要持有上游 Secret，只能调用受限控制命令。

代价是需要同时维护 Rust/Tauri 外壳、TypeScript 前端和 Go sidecar。P0 必须先验证 sidecar 打包、启动、升级和代码签名；若复杂度不可接受，再评估 Wails + Go 的单体桌面方案。

建议目录：

```text
apps/desktop            Tauri + React 桌面端
cmd/airlock             可选 CLI
cmd/airlockd            Go 守护进程
internal/control        本地控制通道
internal/routes         路由与策略
internal/secrets        SecretStore
internal/egress         direct / SOCKS5 / CONNECT / auto
internal/httpgw         HTTP/Wget/LLM 转发
internal/sshgw          SSH 双会话网关
internal/audit          脱敏审计
```

## 7. 数据与秘密存储

路由元数据与审计可存 SQLite，但以下内容不得明文进入 SQLite、YAML、日志、命令行参数或环境变量：

- 上游 URL、域名、IP、SSH 用户名；
- 密码、私钥、私钥口令；
- 上游 API Key、Cookie 和认证 Header；
- 代理认证信息。

配置数据库只保存 Secret 引用。SecretStore 首版优先 macOS Keychain，接口同时为 Windows Credential Manager 与 Linux Secret Service 留出实现。

如果目标 URL 本身属于秘密，应把完整 Target Descriptor 作为一项 Secret 保存，而不是只保护密码。

本地 Capability Token 仅在创建时显示一次，数据库保存 Argon2id/快速 Token 哈希及权限。调用者侧可以把它放在权限为 `0600` 的 `.netrc` 或专用配置中；泄露后只能使用对应的受限能力，不能还原上游秘密。

## 8. 防止不受信任 LLM 滥用

- 固定目标，不接受任意上游 URL。
- 对 URL path 做规范化后再匹配，拒绝 `..`、双重编码和绝对 URL 逃逸。
- 对 Query、Header 和 JSON 字段使用 allowlist/schema，不只做字符串黑名单。
- 重定向、DNS 重解析和代理出口后再次执行目标策略。
- 默认拒绝云元数据、回环和链路本地地址；内网地址逐路由显式授权。
- 限制 Body、响应大小、连接数、请求速率、空闲时间和总会话时间。
- 不允许调用者覆盖 Airlock 注入的认证 Header。
- 非幂等 HTTP 请求默认不做 direct -> proxy 自动重试。
- SSH host key 变化、TLS 失败、认证失败不触发代理回退，必须失败关闭。
- 高风险路由支持到期、一次性凭据、人工确认和紧急总开关。
- 日志使用结构化字段和集中脱敏，测试中扫描 Secret 哨兵值。

## 9. MVP 范围

### 必须具备

- macOS 原生桌面窗口与系统托盘；
- 独立 `airlockd`，默认仅监听 loopback；
- 路由增删改、启停、测试和健康状态；
- HTTP/Wget 固定映射与 Secret/Header 注入；
- SSH 双会话转发、凭据隔离与 host key 校验；
- OpenAI-compatible 与 Anthropic-compatible LLM 路由预设；
- 目标地址与凭据全部进入 SecretStore；
- 每路由 Capability Token、本地 SSH 身份、权限和限额；
- direct、SOCKS5 proxy、HTTP CONNECT proxy 与安全的 auto；
- 脱敏活动记录与紧急停止全部路由；
- GUI 保存后不回显 Secret。

### 明确不做

- MCP、Prompt、Skill、会话或工作区管理；
- 修改 Claude Code、Codex 等工具的配置文件；
- LLM 供应商市场、套餐、计费看板或云同步；
- 任意目标正向代理、VPN、透明抓包或 HTTPS MITM；
- 公网控制台、多人协作与远程管理；
- 把命令过滤包装成可靠的 SSH 沙箱。

## 10. 实施顺序

### P0：高风险技术验证

- 验证 Go SSH 双会话桥接 shell/exec/PTY 和退出码。
- 验证 HTTP/LLM Header 注入、流式响应和断点续传。
- 验证 SOCKS5/HTTP CONNECT 出口和安全回退。
- 验证 Tauri 启停 Go sidecar、托盘常驻和本地控制通道。
- 验证 Keychain 访问权限、卸载残留和升级兼容。

### P1：最小安全核心

- Route、Capability、Policy、Target Descriptor 和 SecretStore。
- 本地控制协议、loopback 监听与进程生命周期。
- 集中日志脱敏、known_hosts 和错误分类。

### P2：HTTP/Wget 与 LLM

- 固定 URL 路由、认证注入、路径/参数约束、Range 和重定向。
- OpenAI/Anthropic 预设、模型 allowlist、速率与并发限制。
- direct/proxy/auto 出口及端到端测试。

### P3：SSH

- 本地身份、上游 Secret、host key 与双会话桥接。
- 默认受限 exec/forced-command、超时、限流、审计与可选人工确认；交互 shell 作为显式高风险能力。

### P4：桌面 GUI

- 路由列表、步骤式编辑器、测试、启停和紧急总开关。
- 活动记录、设置、系统托盘、开机启动和错误恢复。
- 完成 macOS 安装包、签名/公证前准备和升级策略。

### P5：跨平台与扩展

- Windows/Linux SecretStore、服务管理与安装包。
- 根据真实需求评估 SFTP/SCP、端口转发和更多 LLM 协议。

## 11. 验收标准

- 使用哨兵 Secret 运行全部 E2E 后，进程参数、环境变量、配置、SQLite 和日志扫描不到哨兵值。
- LLM/脚本只能访问其 Capability 绑定的路由，不能枚举或调用其他路由。
- HTTP 调用者无法覆盖 Authorization、逃逸固定路径或借重定向访问未授权目标。
- LLM 路由只允许配置的模型、端点、速率和并发，并正确注入上游 API Key。
- SSH 客户端只使用本地身份即可执行允许的上游操作，且得不到上游密码或私钥。
- SSH host key 变化、TLS 失败或策略不匹配时失败关闭，不泄露真实目标。
- 直连故障时，GET/HEAD、LLM 流式请求和 SSH 按各自安全规则选择代理；非幂等请求不被重复提交。
- 关闭 GUI 窗口后服务继续运行；托盘与紧急停止可可靠控制全部入口。
- 删除 Route 并选择删除 Secret 后，旧 Capability 立即失效且系统钥匙串条目被清理。
- GUI、错误通知和活动页不显示真实 Target 或 Secret。

## 12. Git 与 GitHub 状态

仓库已以 Private 可见性创建并推送到 `LouisonH/airlock-relay`，默认分支为 `main`。

已完成：

1. 设置仓库级 `user.name` 与 `user.email`，不修改用户未确认的全局身份。
2. 增加 `.gitignore`，排除 Secret、私有配置、数据库、日志、构建产物和签名材料。
3. 创建首次 Conventional Commit。
4. 安装/使用 GitHub CLI 完成浏览器登录，创建 Private 远程仓库并推送 `main`。

发布前仍需补充 SECURITY、许可证、贡献说明，以及包含 Go、前端、Rust 和 Secret 扫描的 CI；在安全边界审查完成前保持 Private。

## 13. 当前推荐决策

- 产品名：Airlock；仓库名为 `airlock-relay`。
- 产品类型：纯本地凭据隔离转发器。
- GUI：Tauri 2 + React，网络核心为 Go sidecar。
- 首发：macOS，结构保持跨平台。
- 首版路由：HTTP/Wget、SSH、OpenAI-compatible、Anthropic-compatible。
- GitHub：先 Private，安全审查后再公开。
- 路由目标：地址和凭据同等级保护，默认不在 GUI 中回显。

## 14. P0 实施记录（2026-07-29）

已完成：

- Capability 生成、摘要存储与常数时间验证。
- 线程安全的 HTTP 路由注册表与内存 SecretStore 原型。
- HTTP/Wget GET/HEAD 固定路由、Query allowlist、Range、上游认证注入、多层编码路径防护、同源重定向重写和响应 Header 白名单。
- `airlockd` loopback-only 监听和健康检查。
- Tauri 2 + React 桌面控制台、系统托盘、紧急停止确认和全平台图标资产。
- 跟随系统/浅色/深色模式、青峦/海岸/暖阳配色与可持久化的 UI-only 主题偏好。
- 可写、读取和删除的 macOS Keychain SecretStore；哨兵 Secret 测试使用内存后端，不触碰真实钥匙串。
- 权限为 `0600` 的 Unix Socket 控制通道；控制令牌由 Tauri 生成、通过 sidecar stdin 传递且不落盘、不进入环境变量或进程参数。
- macOS 原生隐藏输入窗口和一次性 Capability 窗口；真实目标与认证不经过 WebView/IPC 参数。
- Tauri sidecar 启停、健康检查和退出清理，以及更简洁的三步 HTTP 路由流程与 reduced-motion 动画降级。
- 权限为 `0600` 的版本化路由元数据，仅持久化别名、策略、Keychain 引用和 Capability SHA-256 摘要。
- 路由删除、Capability 持久化撤销与 Keychain 清理；创建/启停/删除在元数据写入失败时回滚。
- Clash 兼容的 HTTP/HTTPS CONNECT、SOCKS5/SOCKS5H 出口，代理 URL 与认证信息作为 Secret 进入 Keychain。
- 每路由 `Direct` / `Proxy` / `Auto` 出口；`Auto` 仅在无 Body 的 GET/HEAD 遇到拨号或 DNS 错误时回退，不对 TLS、已收到响应或非幂等请求重试。
- 桌面设置页的代理安全录入、出口状态、开发者 LouisonH 与本地头像资源。
- SSH 原始 TCP 出口支持 HTTP/HTTPS CONNECT、SOCKS5/SOCKS5H 与安全 `Auto`；`Auto` 只在 TCP 拨号阶段回退，Host Key 和认证失败不会二次拨号。
- SSH Target 将真实地址、上游用户名、密码或加密私钥、私钥口令与固定 Host Key 整体存入 SecretStore；普通路由对象仅保存 `ssh/<alias>` 引用。
- 线程安全的 SSH 路由注册表、本地 Capability/公钥指纹认证、严格 Host Key 字节固定、现代算法集合与双会话 `exec` 桥接。
- 默认仅允许完整命令 allowlist，并拒绝 stdin、shell、PTY、window change、SFTP/subsystem、agent/X11 forwarding、local/remote port forwarding 和非回环监听。
- SSH listener 已固定在 `127.0.0.1:4770`；本地 Host Key 通过 Keychain 持久化，SSH 路由策略与 Capability 摘要通过 `0600` 元数据持久化。
- SSH 创建流程已接入原生分步录入、认证前 Host Key 探测与显式确认、真实本地 listener 自测和一次性 Capability 展示。
- 路由可在不更换 Capability 或上游 Secret 的情况下切换“指定命令 / 所有非交互 exec”策略；所有命令模式必须经过原生高风险确认，仍拒绝交互 Shell、PTY、SFTP 和端口转发。
- SSH 命令审计按路由启用，完整命令写入 `0600` 滚动文件，控制接口和活动页只返回最近记录与有界预览。
- 端到端测试覆盖上游密码/加密私钥、本地 Capability/公钥、退出码、拒绝请求不触达上游、错误脱敏、Host Key/认证失败关闭、CONNECT 代理与 `-race` 并发检查。

当前仍是技术验证版，不是已完成发布安全评审的 MVP。HTTP 与 SSH 创建入口均已接入桌面端和持久安全核心；SSH 仍缺少速率限制、并发限制、TTL、Capability 轮换和每次连接人工确认。下一里程碑是补齐这些约束并实现 LLM 安全预设。

## 15. 可配置入口与凭据保护（2026-07-30）

### 目标

- SSH 路由可选用户自定义的本地登录密码；上游密码仍与本地密码隔离。
- HTTP/SSH 入口可选暴露给同一局域网的设备，但默认仍仅监听 loopback。
- 设置页允许在 macOS Keychain 与本机 `0600` 权限文件之间选择，并如实说明安全差异。
- 用安全预设快速选择“严格 / 标准 / 便捷”，同时保留独立调整网络范围和凭据保护方式的能力。

### 安全决策

- 自定义本地 SSH 密码只经 macOS 原生隐藏输入传给本机核心，元数据只保存 SHA-256 Capability 摘要；默认仍是 256 位随机 Capability。
- 局域网模式只放开 HTTP/SSH 数据入口；控制协议始终使用当前用户专属的 `0600` Unix Socket。
- LAN 监听只允许 loopback、未指定地址或私有网段，不接受任意公网 IP 绑定。
- 本机权限文件是明文 Secret 容器，安全边界仅是当前 OS 账户与 `0700/0600` 权限；界面不将其描述为加密保护。
- 切换 SecretStore 时先复制、反向解析验证所有 HTTP/SSH 目标、代理和 SSH Host Key，全部成功后再移除旧副本并重启 sidecar。
- 切换 LAN 或降级到本机文件前显示原生风险确认；任意失败保留上一个可运行设置。

### 验收标准

- 生成的 Capability 和自定义本地密码都能登录对应 SSH 路由，错误密码被拒绝，且明文不进入路由元数据、命令行、环境变量或日志。
- 仅本机模式拒绝非 loopback 监听；局域网模式可绑定 `0.0.0.0`，SSH 自测仍回连 `127.0.0.1`。
- Keychain 与本机文件往返迁移后路由可继续使用，文件和目录权限分别是 `0600` 和 `0700`。
- 设置更改会重启数据面但不暴露控制令牌；启动失败自动回滚。
- 设置页清楚展示当前安全等级、凭据保护、网络暴露范围、重启影响和降级风险。

## 16. LLM 安全路由（2026-07-30）

### 已实现

- OpenAI-compatible 预设仅允许 `/v1/responses` 与 `/v1/chat/completions`；Anthropic-compatible 预设仅允许 `/v1/messages`。
- 模型白名单、`2 MiB` 请求体上限、最大输出 Token、`60 次/分钟`与 `4` 并发的默认边界在触达上游前执行。
- 请求体使用结构化 JSON 解析和重新编码，防止重复字段在策略校验与上游解析之间产生歧义。
- 上游 API Key 只进入 SecretStore；macOS 原生流程会明确选择随机生成 256-bit 本地 Key（推荐），或隐藏输入并二次确认自定义 Key，元数据只保存摘要。
- OpenAI 的 Bearer 与 Anthropic 的 `x-api-key` 本地认证都会在转发前替换为受保护的上游认证，调用者不能覆盖。
- SSE 响应边读边写并显式 flush；`Auto` 仅对已校验、可重放的 LLM POST 在直连拨号/DNS 失败时回退代理。
- `routes.json` 元数据已升级到 v2，可读取 v1 HTTP 路由并在下次保存时完成迁移。
- 桌面三步向导已接入供应商预设、模型白名单、输出上限、二次 API Key 模式选择和原生一次性环境变量窗口。
- LLM 限流状态绑定路由与本地 API Key 摘要，后续轮换或重建凭据时不会继承旧 Key 的窗口配额。
- GUI 可按路由编辑模型白名单、最大输出 Token、每分钟请求数和并发上限，并可轮换随机或自定义二次 API Key。
- 每路由可显式开启调用次数、输入 Token 与输出 Token 统计；只解析上游 JSON/SSE `usage` 数字，不持久化计数或请求/响应正文，可手动清零。
- 设置页新增本机自动刷新频率、舒适/紧凑密度与系统/标准/精简动效偏好；路由表在中等桌面宽度下保留可达的策略与操作控件。

### 仍待实现

- TTL、一次性 Capability、可持久额度/费用统计和脱敏 LLM 活动记录。

## 17. 品牌与多语言说明站（2026-07-30）

### 已实现

- 以“左右气闸 + 中央锁”为核心识别的小尺寸 SVG Logo，同时用于 favicon、说明站导航与 GitHub README。
- `website/` 下的无后端静态说明站，可直接打开，不接入 Airlock 控制面。
- 简体中文与英文动态切换，并支持独立英文入口、深浅主题、协议示例、复制命令和窄屏导航。
- 说明站覆盖产品定位、安全边界、HTTP/SSH/LLM 路由、桌面端预览、架构、快速用法与 FAQ。
- `README.md` 作为英文 GitHub 首页，`README.zh-CN.md` 提供等量简体中文版，两者共享 Logo、语言导航、功能状态、安全警告与构建命令。

### 验收标准

- 中英文切换不遗留上一语言的界面文案，语言与主题偏好只保存在浏览器本地。
- 桌面与窄屏宽度下没有水平裁剪、文字重叠或不可达控件。
- Logo 在 favicon 和 README 小尺寸下仍可识别，不依赖外部字体或网络资源。
- 网页只提供说明与本地示例，不提供 Web 管理能力或暴露任何 Secret。
