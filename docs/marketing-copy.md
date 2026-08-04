# Airlock Marketing Copy

One place for every public-facing intro. Keep the same story everywhere:
**problem → how it feels → what Airlock does → what it covers → why it is not a sandbox.**

---

## 1. Taglines

English:

- Give agents capabilities. Keep credentials local. *(default)*
- Let your agents work. Keep your secrets home.
- The agent gets the job. You keep the keys.
- Access without exposure.

中文：

- 把能力交给 Agent，把凭据留在本机。 *(default)*
- 让 Agent 干活，不让 Agent 拿钥匙。
- 给 Agent 放行，不给 Agent 秘密。

日本語：

- Agent に機能を渡し、認証情報はローカルに保つ。 *(default)*
- 仕事は Agent に、鍵は手元に。

---

## 2. One-liner (GitHub subtitle, website hero detail)

English:

> Your AI agents, scripts, and automation get the access they need — without
> ever seeing your real URLs, passwords, or API keys.

中文：

> 你的 AI Agent、脚本和自动化任务可以拿到完成任务所需的访问权——但永远看不到
> 真实 URL、密码或 API Key。

---

## 3. Elevator pitch (30 seconds)

English:

> Agents and automation are powerful, but they ask for your keys: an API key
> to call a model, a URL to download a file, a password or private key to run
> an SSH command. Once a secret enters an agent, it can end up in prompts,
> logs, tool output, or a breach.
>
> Airlock is a local relay that sits between the caller and your real
> resources. The agent only gets a fixed local endpoint and a revocable
> route credential. The real URL, password, and API key stay in a local
> secret store and are injected only after the request passes your policy.
>
> It covers HTTP/Wget downloads, SSH command execution, and OpenAI/Anthropic
> LLM APIs — one boundary for the tools you actually delegate to.

中文：

> Agent 和自动化越来越强，但它们总向你要钥匙：调用模型要 API Key，下载文件要
> 真实 URL，上服务器执行命令要密码或私钥。Secret 一旦交出去，就可能出现在提示词、
> 日志、工具输出里，甚至随一次泄露一起曝光。
>
> Airlock 是一个运行在本机的转发器，挡在调用者和真实资源之间。Agent 只拿到一个
> 固定本地入口和一枚可撤销的路由凭据；真实 URL、密码和 API Key 留在本地
> SecretStore 里，只有请求通过你的策略检查后才会被注入。
>
> 覆盖 HTTP/Wget 下载、SSH 命令执行和 OpenAI/Anthropic LLM API——你真正会
> 委托出去的那几类访问，用同一条边界管起来。

---

## 4. Launch post (short, HN / Reddit / 即刻 style)

English:

> I got tired of handing my API keys and SSH passwords to AI agents, so I
> built a local "airlock" between them and my real credentials.
>
> The idea: the agent never sees the real secret. It talks to
> `127.0.0.1:4768`, presents a revocable route token, and Airlock injects
> the real upstream credential only after policy checks pass.
>
> What you can lock down today:
> - HTTP/Wget: fixed URL, method/query allowlists, Range downloads
> - SSH: dual-session termination, pinned host keys, exact-command-only by default
> - LLM APIs: OpenAI/Anthropic compatible, model/rate/token limits, local
>   secondary API key that rotates independently
>
> It runs as a desktop app (Tauri) or headless server, is Apache-2.0, and
> v0.1.7 ships with a maintainer-run production-readiness security audit.
> `npm install -g airlock-relay` to try it.

中文：

> 我不再把 API Key 和 SSH 密码直接塞给 AI Agent 了，所以写了一个本地的
> "Airlock"（气闸），挡在 Agent 和真实凭据之间。
>
> 思路很简单：Agent 永远接触不到真实 Secret。它只连本机
> `127.0.0.1:4768`，出示一枚可撤销的路由令牌；Airlock 检查策略通过后，
> 才向上游注入真实凭据。
>
> 现在就能锁住的场景：
> - HTTP/Wget：固定地址、方法/Query 白名单、Range 下载
> - SSH：双会话隔离、Host Key 固定、默认只允许精确命令
> - LLM API：兼容 OpenAI/Anthropic，模型/速率/Token 限额，本地二次
>   API Key 可独立轮换
>
> 有桌面版（Tauri），也能无头跑服务器模式；Apache-2.0 开源，v0.1.7 已完成
> 维护者执行的生产就绪安全审计。`npm install -g airlock-relay` 即可试用。

---

## 5. Positioning guardrails

Always say what Airlock is **not**, before someone else does:

- Not an open proxy or VPN — only pre-configured fixed routes.
- Not an OS sandbox — a credential and policy boundary.
- Not a password manager — a delegation layer for HTTP, SSH, and LLM APIs.

Always lead with the user, not the architecture:

- 用户：AI Agent、脚本、自动化、CI、需要委托访问的人。
- 痛点：Secret 进上下文、日志和工具输出；权限给出去收不回来。
- 方案：本地固定路由 + 可撤销能力凭据 + 策略注入。
- 边界：本机管理员/root 不在威胁模型内；上游输出仍可能主动泄露自身环境。
