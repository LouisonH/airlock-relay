const translations = {
  zh: {
    pageTitle: "Airlock - 本地凭据隔离转发器",
    pageDescription: "Airlock 是一个面向 HTTP、SSH 和 LLM 流量的本地凭据隔离转发器。",
    skipLink: "跳到主要内容",
    navLabel: "主导航",
    navBoundary: "安全边界",
    navRoutes: "路由类型",
    navQuickstart: "快速使用",
    navArchitecture: "架构",
    navFaq: "常见问题",
    navDocs: "文档",
    sourceCode: "源代码",
    menuLabel: "打开导航",
    heroEyebrow: "LOCAL CREDENTIAL BOUNDARY",
    heroLead: "把能力交给 Agent，把凭据留在本机。",
    heroDetail: "Airlock 使用固定本地路由、可撤销凭据与最小权限策略，为 HTTP、SSH 和 LLM 请求隔离真实上游地址、密码与 API Key。",
    seeHow: "了解工作方式",
    releaseCta: "下载 v0.1.0",
    quickStartCta: "查看快速用法",
    installLabel: "一行安装",
    principleTarget: "固定目标",
    principleTargetDetail: "不接受任意上游 URL",
    principleSecret: "凭据隔离",
    principleSecretDetail: "秘密不进入 Agent 上下文",
    principlePolicy: "最小权限",
    principlePolicyDetail: "每条路由独立授权",
    principleLocal: "本地优先",
    principleLocalDetail: "控制面仅当前用户可达",
    boundaryEyebrow: "THE BOUNDARY",
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
    p0Notice: "v0.1.0 技术预览版：尚未完成独立生产安全审计。",
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
    closingEyebrow: "AIRLOCK v0.1.0",
    closingTitle: "让自动化拥有能力，不必拥有秘密。",
    viewGithub: "查看 GitHub 项目",
    footerTagline: "本地凭据隔离转发器",
    footerSecurity: "v0.1.0 技术预览版 · 未公证 · 尚未完成独立生产安全审计"
  },
  en: {
    pageTitle: "Airlock - Local credential-isolation relay",
    pageDescription: "Airlock is a local credential-isolation relay for HTTP, SSH, and LLM traffic.",
    skipLink: "Skip to main content",
    navLabel: "Primary navigation",
    navBoundary: "Security boundary",
    navRoutes: "Route types",
    navQuickstart: "Quick start",
    navArchitecture: "Architecture",
    navFaq: "FAQ",
    navDocs: "Documentation",
    sourceCode: "Source",
    menuLabel: "Open navigation",
    heroEyebrow: "LOCAL CREDENTIAL BOUNDARY",
    heroLead: "Give agents capabilities. Keep credentials local.",
    heroDetail: "Airlock uses fixed local routes, revocable credentials, and least-privilege policies to isolate real upstream addresses, passwords, and API keys from HTTP, SSH, and LLM callers.",
    seeHow: "See how it works",
    releaseCta: "Download v0.1.0",
    quickStartCta: "View quick start",
    installLabel: "ONE-LINE INSTALL",
    principleTarget: "Fixed targets",
    principleTargetDetail: "Never accepts arbitrary upstream URLs",
    principleSecret: "Credential isolation",
    principleSecretDetail: "Secrets stay outside agent context",
    principlePolicy: "Least privilege",
    principlePolicyDetail: "Every route is authorized separately",
    principleLocal: "Local first",
    principleLocalDetail: "Control plane is current-user only",
    boundaryEyebrow: "THE BOUNDARY",
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
    p0Notice: "v0.1.0 technical preview: no independent production security audit has been completed.",
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
    closingEyebrow: "AIRLOCK v0.1.0",
    closingTitle: "Let automation hold capabilities, not secrets.",
    viewGithub: "View on GitHub",
    footerTagline: "Local credential-isolation relay",
    footerSecurity: "v0.1.0 technical preview · not notarized · no independent production security audit"
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
  document.title = dictionary.pageTitle;
  document.querySelector('meta[name="description"]').setAttribute("content", dictionary.pageDescription);
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
