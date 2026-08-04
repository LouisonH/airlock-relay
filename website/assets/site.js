const translations = {
  zh: {
    pageTitle: "Airlock - 把能力交给 Agent，把凭据留在本机",
    pageDescription: "Airlock 是本地凭据隔离转发器：给 AI Agent、脚本和自动化可控访问 HTTP、SSH、LLM API 的能力，真实 URL、密码和 API Key 始终留在本机。",
    skipLink: "跳到主要内容",
    navLabel: "主导航",
    navHome: "首页",
    navBoundary: "安全边界",
    navRoutes: "路由类型",
    navQuickstart: "快速使用",
    navDownload: "下载",
    navSecurity: "安全模型",
    navDesktop: "桌面端",
    navRelease: "发布状态",
    navPlatform: "跨平台",
    navArchitecture: "架构",
    navFaq: "常见问题",
    navDocs: "文档",
    sourceCode: "源代码",
    menuLabel: "打开导航",
    heroEyebrow: "SECRETS STAY LOCAL",
    heroLead: "把能力交给 Agent，把凭据留在本机。",
    heroDetail: "Agent 要调用 API、下载文件、执行 SSH 命令，不必交出真实 URL、密码或 API Key。Airlock 在本地提供固定路由：调用者只拿到可撤销的本地凭据，秘密通过策略检查后才注入。",
    seeHow: "了解工作方式",
    releaseCta: "下载安装 v0.1.7",
    quickStartCta: "查看快速用法",
    releaseNotesLink: "查看发布说明",
    installLabel: "NPM 安装",
    downloadEyebrow: "DOWNLOAD & INSTALL",
    downloadPageTitle: "下载与安装",
    downloadTitle: "选择平台，直接下载或通过 npm 安装",
    downloadIntro: "所有安装包都附带固定 SHA-256 校验和；Linux 产物另有 GPG 签名。校验失败时安装器会拒绝执行。",
    downloadTeaserTitle: "macOS · Windows · Linux · Raspberry Pi",
    downloadTeaserDetail: "一个 npm 命令即可在全部支持平台安装；也可以到下载页获取各平台的安装包、SHA-256 清单与 GPG 公钥。",
    downloadTeaserCta: "进入下载页",
    dlMacTitle: "Apple Silicon / Intel",
    dlMacDetail: "DMG 安装到 ~/Applications；Apple Silicon 版本随 npm 包内置，Intel 版本从 Release 下载并校验。",
    dlMacNote: "ad-hoc 签名，未经 Apple 公证；首次打开需 Gatekeeper 确认，请先核对 SHA-256。",
    dlWinTitle: "x64 / x86 / arm64",
    dlWinDetail: "NSIS 安装包静默安装；npm 安装器会先下载固定校验和的安装程序并核验。",
    dlWinNote: "预览安装包未代码签名，会出现 UAC 提权提示，SmartScreen 可能警告。",
    dlLinuxTitle: "AppImage + GPG 签名",
    dlLinuxDetail: "AppImage 安装到 ~/.local/bin；64 位树莓派可直接使用 arm64 版本，32 位 armv7 见文档构建脚本。",
    dlLinuxNote: "GPG 公钥与 .sig 随 Release 发布；无 FUSE 时用 --appimage-extract-and-run。",
    verifyShaTitle: "固定校验和",
    verifyShaLink: "下载校验清单 ↗",
    verifyGpgTitle: "Linux 发布签名",
    verifyGpgLink: "导入公钥 ↗",
    verifyReleaseTitle: "全部平台产物",
    verifyReleaseLink: "GitHub Release ↗",
    principleTarget: "固定目标",
    principleTargetDetail: "不接受任意上游 URL",
    principleSecret: "凭据隔离",
    principleSecretDetail: "秘密不进入 Agent 上下文",
    principlePolicy: "最小权限",
    principlePolicyDetail: "每条路由独立授权",
    principleLocal: "本地优先",
    principleLocalDetail: "控制面仅当前用户可达",
    boundaryEyebrow: "THE BOUNDARY",
    securityPageTitle: "安全模型",
    boundaryTitle: "不可信的调用者只获得一项能力",
    boundaryIntro: "Agent 可以执行被允许的任务，但无法读取或还原真实上游凭据。",
    boundaryStatement: "Airlock 不是通用开放代理。每条路由都是预先配置的、有界的本地能力。",
    revocable: "可撤销",
    factOneTitle: "真实目标受保护",
    factOneDetail: "URL、域名、IP 和 SSH 账户均由 SecretStore 保存。",
    factTwoTitle: "请求在上游前校验",
    factTwoDetail: "方法、路径、模型、速率、并发与命令范围都有显式边界。",
    factThreeTitle: "错误也不泄密",
    factThreeDetail: "失败响应与活动记录只包含脱敏的本地上下文。",
    routesEyebrow: "ROUTE TYPES",
    routesTitle: "三种入口，同一条保护边界",
    routesIntro: "调用者看到的只是稳定的本地入口和属于该路由的本地凭据。",
    httpTitle: "固定 URL 转发",
    httpDetail: "隐藏上游基址与 Authorization，限制方法、Query 和重定向，支持 Range 下载。",
    httpFeatureOne: "GET / HEAD 白名单",
    httpFeatureTwo: "Range / 206 流式下载",
    httpFeatureThree: "Direct / Proxy / Auto 出口",
    sshTitle: "双会话 SSH 终止",
    sshDetail: "Airlock 分别终止本地与上游 SSH 会话，用完全独立的账号和凭据建立连接。",
    sshFeatureOne: "自定义本地密码 / 公钥",
    sshFeatureTwo: "自定义精确命令或高风险完整 exec",
    sshFeatureThree: "固定 Host Key 与可选审计",
    llmTitle: "本地二次 API Key",
    llmDetail: "兼容 OpenAI 与 Anthropic 协议，只注入受保护的上游 Key，并限制模型与资源用量。",
    llmFeatureOne: "模型、输出、速率和并发限额",
    llmFeatureTwo: "随机或自定义二次 Key",
    llmFeatureThree: "可选调用与 Token 统计",
    desktopEyebrow: "NATIVE DESKTOP",
    desktopPageTitle: "桌面控制台",
    desktopTitle: "一个安静的本地运维界面",
    desktopDetail: "配置、轮换和观测路由，但不在 WebView 中显示真实目标或 Secret。",
    desktopPreviewLabel: "Airlock 桌面路由列表预览",
    previewOverview: "概览",
    previewRoutes: "路由",
    previewActivity: "活动",
    previewSettings: "设置",
    previewConnected: "本地核心已连接",
    previewOpen: "3 条路由已开放",
    previewStop: "停止全部",
    previewRouteCount: "3 条 · 所有目标均受保护",
    previewNew: "新增路由",
    previewSearch: "搜索名称或别名",
    previewStatus: "状态",
    previewName: "名称",
    previewType: "类型",
    previewEntry: "本地入口",
    previewPolicy: "访问边界",
    previewEgress: "出口",
    enabled: "已启用",
    previewExactCommand: "1 条精确命令 · 已记录",
    quickstartEyebrow: "QUICK START",
    quickstartTitle: "调用者只需要本地入口",
    quickstartIntro: "先在 Airlock Desktop 中完成原生安全配置，然后把本地能力凭据交给需要执行任务的工具。",
    protocolExamples: "协议示例",
    httpExampleTitle: "下载受保护的固定资源",
    httpExampleDetail: "本地 Token 只能访问 release 路由",
    sshExampleTitle: "使用隔离的本地 SSH 身份",
    sshExampleDetail: "调用者不会得到上游用户名或密码",
    llmExampleTitle: "把 OpenAI-compatible 客户端指向本地路由",
    llmExampleDetail: "二次 Key 可轮换，与上游 Key 完全隔离",
    copyCommand: "复制命令",
    copied: "已复制命令",
    p0Label: "当前版本",
    p0Notice: "v0.1.7 已于 2026-08-03 发布；维护者生产就绪安全审计已完成。",
    releaseEyebrow: "RELEASE & ASSURANCE",
    releaseTitle: "v0.1.7 已发布，生产就绪审计已完成",
    releaseIntro: "2026 年 7 月 31 日发布。审计涵盖核心转发边界、依赖、发布工件、安装器和公开站点。",
    releasePackageLabel: "公开分发",
    releasePackageTitle: "GitHub Release + npm",
    releasePackageDetail: "已发布 Apple Silicon macOS 工件、SHA-256 清单与 airlock-relay@0.1.9。",
    releaseAuditLabel: "审计结论",
    releaseAuditTitle: "维护者生产就绪审计完成",
    releaseAuditDetail: "已复核 SSH / HTTP / LLM 边界、依赖漏洞、竞态测试、安装器和 CI 发布路径。",
    releaseBoundaryLabel: "仍需注意",
    releaseBoundaryTitle: "不是第三方认证",
    releaseBoundaryDetail: "当前为 Apple Silicon 技术预览；未进行 Developer ID 签名、Apple 公证或独立渗透测试。",
    releaseReadAudit: "查看完整审计记录",
    releaseReadNotes: "查看 v0.1.7 发布说明",
    platformEyebrow: "PLATFORM CONTRACT",
    platformTitle: "先识别系统，再选择受验证的安装路径",
    platformIntro: "同一个 npm 包会识别当前操作系统与 CPU；安装器只会在公开工件及 SHA-256 已固定时执行。",
    platformMacLabel: "已发布",
    platformMacTitle: "macOS / Apple Silicon · Intel",
    platformMacDetail: "arm64 与 x64 DMG 均已发布；npm 安装器直接安装到 ~/Applications。",
    platformWindowsLabel: "已发布预览",
    platformWindowsTitle: "Windows / x64 · x86 · ARM64",
    platformWindowsDetail: "NSIS 安装包已随 Release 发布并固定校验和；npm 安装器核验后静默安装。",
    platformLinuxLabel: "已发布预览",
    platformLinuxTitle: "Linux / x64 · arm64 · 树莓派",
    platformLinuxDetail: "AppImage 已发布并带 GPG 签名；64 位树莓派直接可用，32 位 armv7 提供设备端构建脚本。",
    platformReadDocs: "阅读跨平台测试与发布边界",
    platformReadContract: "查看平台契约",
    architectureEyebrow: "ARCHITECTURE",
    architectureTitle: "桌面控制面与网络核心分离",
    architectureIntro: "GUI 关闭后，本地转发服务仍可继续运行。控制命令不经过普通 TCP 端口。",
    architectureDesktop: "路由、策略、主题与脱敏状态。",
    architectureSocket: "受保护的本地控制通道。",
    architectureCore: "验证能力、应用策略、注入 Secret 并选择出口。",
    architectureSecret: "保存完整目标描述与真实凭据。",
    controlsEyebrow: "CONTROL WITHOUT EXPOSURE",
    controlsTitle: "可调整的便捷性，不可绕过的控制面",
    controlNetwork: "网络范围",
    controlNetworkValue: "仅本机 / 私有局域网",
    controlNetworkDetail: "局域网模式需要原生风险确认",
    controlSecrets: "Secret 保护",
    controlSecretsDetail: "切换前进行双向验证与失败回滚",
    controlProxy: "代理出口",
    controlProxyDetail: "兼容 Clash HTTP CONNECT 与 SOCKS5",
    controlAppearance: "桌面偏好",
    controlAppearanceValue: "深浅主题 · 密度 · 动效",
    controlAppearanceDetail: "只保存在本地 UI，不与路由 Secret 混合",
    faqEyebrow: "FAQ",
    faqTitle: "安全边界说明",
    faqOneQuestion: "Airlock 是通用代理或 VPN 吗？",
    faqOneAnswer: "不是。Airlock 不接受调用者提供的任意目标，只转发到用户预先配置并授权的固定路由。",
    faqTwoQuestion: "LLM 能看到上游 API Key 吗？",
    faqTwoAnswer: "不能。客户端使用的是可撤销的本地二次 Key，Airlock 在向固定上游发起请求时才注入真实 Key。",
    faqThreeQuestion: "Token 统计会保存提示词吗？",
    faqThreeAnswer: "不会。该功能默认关闭；开启后只提取上游 usage 数字，计数仅保存在 airlockd 内存中。",
    faqFourQuestion: "Airlock 能防御本机管理员吗？",
    faqFourAnswer: "不能。本机管理员、root、可调试 Airlock 进程或已控制操作系统的攻击者不在保护范围内。",
    faqFiveQuestion: "支持哪些平台？",
    faqFiveAnswer: "macOS 12+（Apple Silicon 与 Intel）、Windows 10+（x64/x86/arm64）与 Linux x64/arm64；64 位树莓派可直接安装，32 位 armv7 提供设备端构建脚本。",
    faqSixQuestion: "如何校验下载的安装包？",
    faqSixAnswer: "下载 SHA256SUMS-v0.1.7.txt 后用 shasum -a 256 -c 核对；Linux 产物还可导入 Airlock-gpg-pubkey.asc 后用 gpg --verify 校验签名。安装器本身也会在安装前强制校验。",
    closingEyebrow: "AIRLOCK v0.1.7",
    closingTitle: "让自动化拥有能力，不必拥有秘密。",
    viewGithub: "查看 GitHub 项目",
    footerTagline: "本地凭据隔离转发器",
    footerAffiliation: "华南理工大学（SCUT）相关开发者 · 独立项目",
    footerSecurity: "v0.1.7 · 维护者生产就绪审计完成 · 未经第三方认证或 Apple 公证"
  },
  en: {
    pageTitle: "Airlock - Give agents access. Keep secrets local.",
    pageDescription: "Airlock is a local relay that gives AI agents, scripts, and automation controlled access to HTTP, SSH, and LLM APIs — without exposing real URLs, passwords, or API keys.",
    skipLink: "Skip to main content",
    navLabel: "Primary navigation",
    navHome: "Home",
    navBoundary: "Security boundary",
    navRoutes: "Route types",
    navQuickstart: "Quick start",
    navDownload: "Download",
    navSecurity: "Security model",
    navDesktop: "Desktop",
    navRelease: "Release status",
    navPlatform: "Platforms",
    navArchitecture: "Architecture",
    navFaq: "FAQ",
    navDocs: "Documentation",
    sourceCode: "Source",
    menuLabel: "Open navigation",
    heroEyebrow: "SECRETS STAY LOCAL",
    heroLead: "Give agents capabilities. Keep credentials local.",
    heroDetail: "Agents, scripts, and automation get controlled access to HTTP, SSH, and LLM APIs — while real URLs, passwords, and API keys stay on your machine.",
    seeHow: "See how it works",
    releaseCta: "Download v0.1.7",
    quickStartCta: "View quick start",
    releaseNotesLink: "View release notes",
    installLabel: "NPM INSTALL",
    downloadEyebrow: "DOWNLOAD & INSTALL",
    downloadPageTitle: "Download and install",
    downloadTitle: "Pick a platform, then download or install via npm",
    downloadIntro: "Every installer ships with a pinned SHA-256 checksum; Linux artifacts are also GPG-signed. The installer fails closed on any mismatch.",
    downloadTeaserTitle: "macOS · Windows · Linux · Raspberry Pi",
    downloadTeaserDetail: "One npm command installs on every supported platform; visit the downloads page for per-platform installers, the SHA-256 manifest, and the GPG public key.",
    downloadTeaserCta: "Open downloads",
    dlMacTitle: "Apple Silicon / Intel",
    dlMacDetail: "DMG installs to ~/Applications. The Apple Silicon build is bundled with the npm package; the Intel build is downloaded and verified from the release.",
    dlMacNote: "Ad-hoc signed and not Apple-notarized; Gatekeeper confirmation and a SHA-256 check are expected on first open.",
    dlWinTitle: "x64 / x86 / arm64",
    dlWinDetail: "The NSIS installer runs silently; the npm installer first downloads and verifies the pinned setup program.",
    dlWinNote: "Preview installers are not code-signed; a UAC prompt and possible SmartScreen warning are expected.",
    dlLinuxTitle: "AppImage + GPG signing",
    dlLinuxDetail: "AppImage installs to ~/.local/bin. 64-bit Raspberry Pi OS uses the arm64 build directly; 32-bit armv7 uses the on-device build script.",
    dlLinuxNote: "The GPG public key and .sig files ship with the release; use --appimage-extract-and-run when FUSE is unavailable.",
    verifyShaTitle: "Pinned checksums",
    verifyShaLink: "Download checksum manifest ↗",
    verifyGpgTitle: "Linux release signing",
    verifyGpgLink: "Import public key ↗",
    verifyReleaseTitle: "All platform artifacts",
    verifyReleaseLink: "GitHub Release ↗",
    principleTarget: "Fixed targets",
    principleTargetDetail: "Never accepts arbitrary upstream URLs",
    principleSecret: "Credential isolation",
    principleSecretDetail: "Secrets stay outside agent context",
    principlePolicy: "Least privilege",
    principlePolicyDetail: "Every route is authorized separately",
    principleLocal: "Local first",
    principleLocalDetail: "Control plane is current-user only",
    boundaryEyebrow: "THE BOUNDARY",
    securityPageTitle: "Security model",
    boundaryTitle: "An untrusted caller receives one bounded capability",
    boundaryIntro: "An agent can perform an approved task without reading or reconstructing real upstream credentials.",
    boundaryStatement: "Airlock is not an open general-purpose proxy. Every route is a preconfigured, bounded local capability.",
    revocable: "revocable",
    factOneTitle: "Real targets stay protected",
    factOneDetail: "URLs, domains, IPs, and SSH accounts are stored in the SecretStore.",
    factTwoTitle: "Requests are checked before upstream",
    factTwoDetail: "Methods, paths, models, rate, concurrency, and command scope all have explicit boundaries.",
    factThreeTitle: "Errors do not disclose secrets",
    factThreeDetail: "Failure responses and activity records contain sanitized local context only.",
    routesEyebrow: "ROUTE TYPES",
    routesTitle: "Three ingress types, one protection boundary",
    routesIntro: "Callers see only a stable local endpoint and a local credential scoped to that route.",
    httpTitle: "Fixed URL relay",
    httpDetail: "Hide the upstream base URL and Authorization, constrain methods, query parameters, and redirects, and preserve Range downloads.",
    httpFeatureOne: "GET / HEAD allowlist",
    httpFeatureTwo: "Range / 206 streaming downloads",
    httpFeatureThree: "Direct / Proxy / Auto egress",
    sshTitle: "Dual-session SSH termination",
    sshDetail: "Airlock terminates the local and upstream SSH sessions separately, connecting with completely independent identities and credentials.",
    sshFeatureOne: "Custom local password / public key",
    sshFeatureTwo: "Custom exact command or high-risk unrestricted exec",
    sshFeatureThree: "Pinned host key and optional audit",
    llmTitle: "Secondary local API key",
    llmDetail: "OpenAI- and Anthropic-compatible routes inject the protected upstream key only after enforcing model and resource limits.",
    llmFeatureOne: "Model, output, rate, and concurrency limits",
    llmFeatureTwo: "Random or custom secondary key",
    llmFeatureThree: "Optional call and token statistics",
    desktopEyebrow: "NATIVE DESKTOP",
    desktopPageTitle: "Desktop console",
    desktopTitle: "A quiet local operations console",
    desktopDetail: "Configure, rotate, and observe routes without displaying real targets or secrets inside the WebView.",
    desktopPreviewLabel: "Preview of the Airlock desktop route list",
    previewOverview: "Overview",
    previewRoutes: "Routes",
    previewActivity: "Activity",
    previewSettings: "Settings",
    previewConnected: "Local core connected",
    previewOpen: "3 routes open",
    previewStop: "Stop all",
    previewRouteCount: "3 routes · all targets protected",
    previewNew: "New route",
    previewSearch: "Search name or alias",
    previewStatus: "Status",
    previewName: "Name",
    previewType: "Type",
    previewEntry: "Local endpoint",
    previewPolicy: "Access boundary",
    previewEgress: "Egress",
    enabled: "Enabled",
    previewExactCommand: "1 exact command · recorded",
    quickstartEyebrow: "QUICK START",
    quickstartTitle: "Callers need only the local endpoint",
    quickstartIntro: "Complete protected setup in Airlock Desktop, then give the local capability credential to the tool that needs to perform the task.",
    protocolExamples: "Protocol examples",
    httpExampleTitle: "Download a protected fixed resource",
    httpExampleDetail: "The local token can access only the release route",
    sshExampleTitle: "Use an isolated local SSH identity",
    sshExampleDetail: "The caller never receives the upstream username or password",
    llmExampleTitle: "Point an OpenAI-compatible client at a local route",
    llmExampleDetail: "The secondary key can rotate independently of the upstream key",
    copyCommand: "Copy command",
    copied: "Command copied",
    p0Label: "Current release",
    p0Notice: "v0.1.7 released on 2026-08-03; maintainer production-readiness audit completed.",
    releaseEyebrow: "RELEASE & ASSURANCE",
    releaseTitle: "v0.1.7 is released and the production-readiness audit is complete",
    releaseIntro: "Released on July 31, 2026. The review covered relay boundaries, dependencies, release artifacts, the installer, and the public site.",
    releasePackageLabel: "PUBLIC DISTRIBUTION",
    releasePackageTitle: "GitHub Release + npm",
    releasePackageDetail: "Apple Silicon macOS artifacts, their SHA-256 manifest, and airlock-relay@0.1.9 are published.",
    releaseAuditLabel: "AUDIT RESULT",
    releaseAuditTitle: "Maintainer production-readiness audit complete",
    releaseAuditDetail: "Reviewed SSH / HTTP / LLM boundaries, dependency findings, race tests, the installer, and the CI release path.",
    releaseBoundaryLabel: "REMAINING BOUNDARIES",
    releaseBoundaryTitle: "Not a third-party certification",
    releaseBoundaryDetail: "This remains an Apple Silicon technical preview without Developer ID signing, Apple notarization, or an independent penetration test.",
    releaseReadAudit: "Read the full audit record",
    releaseReadNotes: "Read v0.1.7 release notes",
    platformEyebrow: "PLATFORM CONTRACT",
    platformTitle: "Recognize the system before selecting a verified install path",
    platformIntro: "One npm package recognizes the current operating system and CPU. Installation runs only when a public artifact and pinned SHA-256 are available.",
    platformMacLabel: "RELEASED",
    platformMacTitle: "macOS / Apple Silicon · Intel",
    platformMacDetail: "arm64 and x64 DMGs are published; the npm installer installs directly to ~/Applications.",
    platformWindowsLabel: "RELEASED PREVIEW",
    platformWindowsTitle: "Windows / x64 · x86 · ARM64",
    platformWindowsDetail: "NSIS installers are published with pinned checksums; the npm installer verifies and runs them silently.",
    platformLinuxLabel: "RELEASED PREVIEW",
    platformLinuxTitle: "Linux / x64 · arm64 · Raspberry Pi",
    platformLinuxDetail: "GPG-signed AppImages are published; 64-bit Raspberry Pi works directly, and armv7 uses the on-device build script.",
    platformReadDocs: "Read testing and release boundaries",
    platformReadContract: "View platform contract",
    architectureEyebrow: "ARCHITECTURE",
    architectureTitle: "The desktop control plane is separate from the network core",
    architectureIntro: "The local relay can keep running after the GUI closes. Control commands never use an ordinary TCP port.",
    architectureDesktop: "Routes, policies, themes, and sanitized state.",
    architectureSocket: "Protected local control channel.",
    architectureCore: "Verify capabilities, apply policies, inject secrets, and select egress.",
    architectureSecret: "Store complete target descriptors and real credentials.",
    controlsEyebrow: "CONTROL WITHOUT EXPOSURE",
    controlsTitle: "Adjustable convenience, non-bypassable control",
    controlNetwork: "Network scope",
    controlNetworkValue: "Loopback / private LAN",
    controlNetworkDetail: "LAN mode requires a native risk confirmation",
    controlSecrets: "Secret protection",
    controlSecretsDetail: "Switches use two-way verification and failure rollback",
    controlProxy: "Proxy egress",
    controlProxyDetail: "Compatible with Clash HTTP CONNECT and SOCKS5",
    controlAppearance: "Desktop preferences",
    controlAppearanceValue: "Theme · density · motion",
    controlAppearanceDetail: "Stored only as local UI state, never mixed with route secrets",
    faqEyebrow: "FAQ",
    faqTitle: "Security boundary notes",
    faqOneQuestion: "Is Airlock a general proxy or VPN?",
    faqOneAnswer: "No. Airlock never accepts an arbitrary target from the caller. It relays only to fixed routes that the user has configured and authorized in advance.",
    faqTwoQuestion: "Can an LLM see the upstream API key?",
    faqTwoAnswer: "No. The client uses a revocable secondary local key. Airlock injects the real key only when sending a request to the fixed upstream.",
    faqThreeQuestion: "Do token statistics save prompts?",
    faqThreeAnswer: "No. Statistics are disabled by default. When enabled, Airlock extracts upstream usage numbers only, and counters remain in airlockd memory.",
    faqFourQuestion: "Does Airlock protect against a local administrator?",
    faqFourAnswer: "No. Local administrators, root, processes able to debug Airlock, and attackers that control the operating system are outside the threat model.",
    faqFiveQuestion: "Which platforms are supported?",
    faqFiveAnswer: "macOS 12+ (Apple Silicon and Intel), Windows 10+ (x64/x86/arm64), and Linux x64/arm64. 64-bit Raspberry Pi OS installs directly; 32-bit armv7 uses the on-device build script.",
    faqSixQuestion: "How do I verify a downloaded installer?",
    faqSixAnswer: "Download SHA256SUMS-v0.1.7.txt and run shasum -a 256 -c; for Linux, import Airlock-gpg-pubkey.asc and verify with gpg --verify. The installer also enforces verification before installing.",
    closingEyebrow: "AIRLOCK v0.1.7",
    closingTitle: "Let automation hold capabilities, not secrets.",
    viewGithub: "View on GitHub",
    footerTagline: "Local credential-isolation relay",
    footerAffiliation: "SCUT-affiliated developer · independent project",
    footerSecurity: "v0.1.7 · maintainer production-readiness audit complete · not third-party certified or notarized"
  }
};

const languageStorageKey = "airlock.site.language";
const themeStorageKey = "airlock.site.theme";
const root = document.documentElement;
const languageButton = document.getElementById("language-toggle");
const themeButton = document.getElementById("theme-toggle");
const menuButton = document.getElementById("menu-toggle");
const navigation = document.getElementById("site-nav");
const copyToast = document.getElementById("copy-toast");
const preferredTheme = window.matchMedia("(prefers-color-scheme: dark)");

function storageGet(key) {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function storageSet(key, value) {
  try {
    localStorage.setItem(key, value);
  } catch {
    return;
  }
}

function initialLanguage() {
  const queryLanguage = new URLSearchParams(window.location.search).get("lang");
  if (queryLanguage === "en" || queryLanguage === "zh") return queryLanguage;
  const stored = storageGet(languageStorageKey);
  if (stored === "en" || stored === "zh") return stored;
  return navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en";
}

function updateThemeControl(theme, language) {
  const switchToLight = theme === "dark";
  const label = language === "zh"
    ? (switchToLight ? "切换到浅色主题" : "切换到深色主题")
    : (switchToLight ? "Switch to light theme" : "Switch to dark theme");
  themeButton.setAttribute("aria-label", label);
  themeButton.title = label;
}

function applyLanguage(language) {
  const dictionary = translations[language];
  root.lang = language === "zh" ? "zh-CN" : "en";
  root.dataset.lang = language;
  const pageTitleKey = document.body.dataset.pageTitle;
  document.title = dictionary[pageTitleKey] || dictionary.pageTitle;
  document.querySelector('meta[name="description"]').setAttribute("content", dictionary.pageDescription);
  document.querySelector('meta[property="og:title"]').setAttribute("content", dictionary.pageTitle);
  document.querySelector('meta[property="og:description"]').setAttribute("content", dictionary.pageDescription);
  document.querySelectorAll("[data-i18n]").forEach((element) => {
    const key = element.dataset.i18n;
    if (dictionary[key]) element.textContent = dictionary[key];
  });
  document.querySelectorAll("[data-i18n-aria]").forEach((element) => {
    const key = element.dataset.i18nAria;
    if (dictionary[key]) element.setAttribute("aria-label", dictionary[key]);
  });
  languageButton.textContent = language === "zh" ? "EN" : "中";
  languageButton.setAttribute("aria-label", language === "zh" ? "Switch to English" : "切换到中文");
  updateThemeControl(root.dataset.theme, language);
  document.querySelectorAll("[data-i18n-aria][title]").forEach((element) => {
    const key = element.dataset.i18nAria;
    if (dictionary[key]) element.title = dictionary[key];
  });
  refreshCharacterReveals();
  storageSet(languageStorageKey, language);
}

function refreshCharacterReveals() {
  document.querySelectorAll("[data-character-reveal]").forEach((element) => {
    const value = element.textContent.trim();
    element.replaceChildren();
    element.setAttribute("aria-label", value);
    Array.from(value).forEach((character, index) => {
      if (/\s/.test(character)) {
        element.append(document.createTextNode(character));
        return;
      }
      const letter = document.createElement("span");
      letter.className = "character-reveal";
      letter.style.setProperty("--character-delay", `${Math.min(index * 24, 720)}ms`);
      letter.setAttribute("aria-hidden", "true");
      letter.textContent = character;
      element.append(letter);
    });
  });
}

function currentTheme() {
  const stored = storageGet(themeStorageKey);
  if (stored === "light" || stored === "dark") return stored;
  return preferredTheme.matches ? "dark" : "light";
}

function applyTheme(theme) {
  root.dataset.theme = theme;
  themeButton.querySelector("span").textContent = theme === "dark" ? "◑" : "◐";
  updateThemeControl(theme, root.dataset.lang);
  storageSet(themeStorageKey, theme);
}

languageButton.addEventListener("click", () => {
  applyLanguage(root.dataset.lang === "zh" ? "en" : "zh");
});

themeButton.addEventListener("click", () => {
  applyTheme(root.dataset.theme === "dark" ? "light" : "dark");
});

menuButton.addEventListener("click", () => {
  const open = navigation.classList.toggle("open");
  menuButton.setAttribute("aria-expanded", String(open));
});

navigation.querySelectorAll("a").forEach((link) => {
  link.addEventListener("click", () => {
    navigation.classList.remove("open");
    menuButton.setAttribute("aria-expanded", "false");
  });
});

document.querySelectorAll("[data-tab]").forEach((tab) => {
  tab.addEventListener("click", () => {
    const selected = tab.dataset.tab;
    document.querySelectorAll("[data-tab]").forEach((candidate) => candidate.setAttribute("aria-selected", String(candidate === tab)));
    document.querySelectorAll("[data-panel]").forEach((panel) => {
      panel.hidden = panel.dataset.panel !== selected;
      panel.classList.toggle("active", panel.dataset.panel === selected);
    });
  });
});

async function copyText(value) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand("copy");
  textarea.remove();
}

let toastTimer;
document.querySelectorAll("[data-copy]").forEach((button) => {
  button.addEventListener("click", async () => {
    const code = document.getElementById(`code-${button.dataset.copy}`);
    if (!code) return;
    try {
      await copyText(code.textContent);
      copyToast.classList.add("visible");
      clearTimeout(toastTimer);
      toastTimer = setTimeout(() => copyToast.classList.remove("visible"), 1800);
    } catch {
      copyToast.textContent = root.dataset.lang === "zh" ? "复制失败，请手动选择" : "Copy failed; select the command manually";
      copyToast.classList.add("visible");
      clearTimeout(toastTimer);
      toastTimer = setTimeout(() => {
        copyToast.classList.remove("visible");
        copyToast.textContent = translations[root.dataset.lang].copied;
      }, 2200);
    }
  });
});

const revealObserver = "IntersectionObserver" in window
  ? new IntersectionObserver((entries, observer) => {
      entries.forEach((entry) => {
        if (!entry.isIntersecting) return;
        entry.target.classList.add("visible");
        observer.unobserve(entry.target);
      });
    }, { threshold: 0.12, rootMargin: "0px 0px -35px" })
  : null;

document.querySelectorAll(".reveal").forEach((element) => {
  if (revealObserver) revealObserver.observe(element);
  else element.classList.add("visible");
});

document.getElementById("current-year").textContent = String(new Date().getFullYear());
applyTheme(currentTheme());
applyLanguage(initialLanguage());
