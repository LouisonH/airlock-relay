# 我不想再把 API Key 塞进 Agent 了，所以做了 Airlock

最近用 Agent 写代码、跑自动化越来越多，但有件事一直让我不太舒服：为了让它下载一个文件、请求一个 API，或者上服务器执行一条命令，往往得把 API Key、密码、私钥甚至真实服务器地址交出去。

即使 Agent 很可靠，这些内容还是可能出现在提示词、日志、工具输出里。权限一旦给出去，想收回来也不够直观。

所以我做了 **Airlock**：一个开源的、本地运行的凭据隔离转发器。

它的思路很简单：Agent 不接触真实凭据，只拿到一个本地地址和一枚可撤销的“通行证”。真实目标、密码和 Key 留在本机；只有请求符合你预先设定的规则时，Airlock 才会代为转发并注入凭据。

换句话说，给 Agent 的不是万能钥匙，而是一张只能开指定一扇门、随时可以作废的门卡。

## 我希望它解决什么问题？

比如你希望 Agent：

- 只能从指定地址下载发布包，不能随便访问整个网络；
- 只能调用某个 LLM 服务的几个模型，不能拿着上游 Key 任意消费；
- 只能以一个低权限账号在服务器上执行一条固定命令，不能获得完整 SSH Shell；
- 使用完后可以直接轮换或撤销本地访问凭据，而不用立刻改动上游账户。

Airlock 面向的正是这些“想授权，但不想裸奔”的场景。

## 目前已经支持

- **HTTP / Wget**：固定上游地址，可限制方法、路径和 Query；支持 Range 下载、受控重定向，以及 Direct/Proxy/Auto 出口。
- **SSH**：本地与上游 SSH 会话分离；可固定 Host Key、限制精确命令，并默认拒绝 Shell、PTY、SFTP 和端口转发。
- **LLM API**：兼容 OpenAI 的 `/v1/responses`、`/v1/chat/completions` 与 Anthropic 的 `/v1/messages`；可配置模型白名单、输出 Token、速率和并发限制。
- **桌面控制台**：管理路由、凭据、Host Key、代理和监听端口。关闭窗口后，转发服务仍可以继续运行。
- **服务端模式**：不依赖桌面环境，可用 `airlockd --mode server` 与本地 Unix Socket CLI 管理固定路由。

## 为什么不是直接把 Key 放进 Agent？

Airlock 会把调用者能看到的内容和需要保护的内容分开：

| Agent 可以拿到 | Airlock 留在本机保护 |
| --- | --- |
| 本地路由别名 | 真实 URL、域名、IP 和 SSH 地址 |
| 路由专属、可撤销 Token | 上游密码、私钥、Cookie 和 API Key |
| 明确允许的操作 | 其他路由与无限制网络访问 |
| 脱敏后的本地错误 | 上游身份与凭据细节 |

它适合用在个人开发环境、代码 Agent、CI 辅助任务、内部自动化和需要把“访问能力”委托出去但不想暴露长期 Secret 的场景。

## 一个最小示例

给 Agent 的只是本地地址和本地 Token：

```bash
wget --header="Authorization: Bearer <local-token>" \
  http://127.0.0.1:4768/r/release/file.zip

export OPENAI_BASE_URL=http://127.0.0.1:4768/r/coding
export OPENAI_API_KEY=<local-api-key>

ssh build@127.0.0.1 -p 4770
```

这里的 Token/API Key 是路由能力凭据，不是上游 Secret。泄露时可以单独撤销或轮换，不必把所有上游凭据一起推倒重来。

## 想试试的话

当前为 **v0.1.4 技术预览版**，已完成维护者执行的生产就绪安全审计，下载包支持
**macOS 12+ Apple Silicon**：

```bash
npm install -g airlock-relay && airlock-installer install --open
```

项目地址：<https://github.com/LouisonH/airlock-relay>

也可以直接前往 [v0.1.4 Releases](https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.4) 下载 DMG 和校验文件。源码开发需要 Go 1.25+、Node.js 20+、Rust/Cargo 与 Tauri 2 平台依赖。

## 也把边界说清楚

Airlock 是固定路由转发器，不是开放代理、VPN，也不是操作系统级沙箱。当前版本已完成维护者执行的生产就绪安全审计，但尚未完成独立第三方审计、Developer ID 签名或 Apple 公证；上游响应或 SSH 输出仍可能主动泄露其自身环境。生产使用时请采用专用、低权限的上游账号，并先阅读项目的[安全策略](https://github.com/LouisonH/airlock-relay/blob/main/SECURITY.md)。

Airlock 还在很早期的阶段，但核心的 HTTP、SSH 和 LLM 路由已经可用。如果你正在用 Claude Code、Codex、OpenClaw 或自建 Agent，欢迎试用、提 Issue，也很想了解大家最想隔离的凭据和路由场景。

**关键词：** Agent 安全、LLM API、凭据隔离、Secret 管理、SSH 网关、HTTP 代理、最小权限、开源工具
