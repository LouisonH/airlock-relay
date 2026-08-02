const docsLanguageKey = "airlock.docs2.language";
const docsThemeKey = "airlock.site.theme";
const docsRoot = document.documentElement;
const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

const chapters = [
  { id: "getting-started", file: "getting-started.html", zh: "从零开始", en: "Getting started", ja: "はじめに" },
  { id: "install", file: "install.html", zh: "安装与校验", en: "Install and verify", ja: "インストールと検証" },
  { id: "desktop", file: "desktop.html", zh: "桌面控制台", en: "Desktop console", ja: "デスクトップコンソール" },
  { id: "security", file: "security.html", zh: "安全模型", en: "Security model", ja: "セキュリティモデル" },
  { id: "routes", file: "routes.html", zh: "路由与策略", en: "Routes and policies", ja: "ルートとポリシー" },
  { id: "server", file: "server.html", zh: "Server Core", en: "Server Core", ja: "Server Core" },
  { id: "operations", file: "operations.html", zh: "运维与排障", en: "Operations", ja: "運用とトラブルシューティング" },
  { id: "platforms", file: "platforms.html", zh: "跨平台与树莓派", en: "Platforms and Pi", ja: "プラットフォームとRaspberry Pi" },
  { id: "faq", file: "faq.html", zh: "常见问题 Q&A", en: "FAQ", ja: "よくある質問 Q&A" },
  { id: "cli", file: "cli.html", zh: "CLI 参考", en: "CLI reference", ja: "CLI リファレンス" },
];

function storageGet(key) {
  try { return localStorage.getItem(key); } catch { return null; }
}
function storageSet(key, value) {
  try { localStorage.setItem(key, value); } catch { return; }
}

function currentLanguage() {
  const queryLanguage = new URLSearchParams(window.location.search).get("lang");
  if (queryLanguage === "zh" || queryLanguage === "en" || queryLanguage === "ja") return queryLanguage;
  const stored = storageGet(docsLanguageKey);
  if (stored === "zh" || stored === "en" || stored === "ja") return stored;
  const language = navigator.language.toLowerCase();
  if (language.startsWith("zh")) return "zh";
  if (language.startsWith("ja")) return "ja";
  return "en";
}

function currentTheme() {
  const stored = storageGet(docsThemeKey);
  if (stored === "light" || stored === "dark") return stored;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function selectLanguage(language) {
  docsRoot.lang = language === "zh" ? "zh-CN" : language;
  docsRoot.dataset.lang = language;
  document.querySelectorAll("[data-language]").forEach((article) => {
    article.hidden = article.dataset.language !== language;
  });
  document.querySelectorAll("[data-language-button]").forEach((button) => {
    button.classList.toggle("active", button.dataset.languageButton === language);
  });
  document.querySelectorAll("[data-i18n]").forEach((element) => {
    const value = element.dataset[language];
    if (value !== undefined) element.textContent = value;
  });
  storageSet(docsLanguageKey, language);
  const searchInput = document.getElementById("docs-search-input");
  if (searchInput) {
    searchInput.placeholder = language === "zh" ? "搜索文档…" : language === "ja" ? "ドキュメントを検索…" : "Search documentation…";
  }
  buildPager(language);
  renderSearch();
}

function selectTheme(theme) {
  docsRoot.dataset.theme = theme;
  const button = document.getElementById("docs-theme");
  if (button) button.textContent = theme === "dark" ? "◑" : "◐";
  storageSet(docsThemeKey, theme);
}

function buildPager(language) {
  const pager = document.getElementById("docs-pager");
  if (!pager) return;
  const current = document.body.dataset.chapter;
  const index = chapters.findIndex((chapter) => chapter.id === current);
  const previous = index > 0 ? chapters[index - 1] : null;
  const next = index >= 0 && index < chapters.length - 1 ? chapters[index + 1] : null;
  const label = (chapter) => chapter[language] || chapter.en;
  pager.innerHTML = previous
    ? `<a class="docs-pager-link prev" href="${previous.file}"><span>←</span><b>${label(previous)}</b></a>`
    : "<span></span>";
  pager.innerHTML += next
    ? `<a class="docs-pager-link next" href="${next.file}"><b>${label(next)}</b><span>→</span></a>`
    : "";
}

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
    const toast = document.getElementById("copy-toast");
    if (!code || !toast) return;
    try {
      await copyText(code.textContent);
      toast.classList.add("visible");
      clearTimeout(toastTimer);
      toastTimer = setTimeout(() => toast.classList.remove("visible"), 1800);
    } catch {
      toast.textContent = "复制失败，请手动选择";
      toast.classList.add("visible");
      clearTimeout(toastTimer);
      toastTimer = setTimeout(() => toast.classList.remove("visible"), 2200);
    }
  });
});

document.querySelectorAll("[data-language-button]").forEach((button) => {
  button.addEventListener("click", () => selectLanguage(button.dataset.languageButton));
});
document.getElementById("docs-theme")?.addEventListener("click", () => {
  selectTheme(docsRoot.dataset.theme === "dark" ? "light" : "dark");
});

function searchIndex() {
  return window.AirlockDocsSearch ?? [];
}

function currentPageChapter() {
  return document.body.dataset.chapter;
}

function renderSearch() {
  const input = document.getElementById("docs-search-input");
  const results = document.getElementById("docs-search-results");
  if (!input || !results) return;
  const query = input.value.trim().toLowerCase();
  const language = docsRoot.dataset.lang || "zh";
  if (query.length < 1) {
    results.hidden = true;
    return;
  }
  const matches = [];
  for (const entry of searchIndex()) {
    if (entry.page === currentPageChapter()) continue;
    const title = entry[language]?.title || entry.en?.title || "";
    const summary = entry[language]?.summary || entry.en?.summary || "";
    const haystack = `${title} ${summary} ${(entry[language]?.keywords || []).join(" ")} ${(entry.en?.keywords || []).join(" ")}`.toLowerCase();
    if (!haystack.includes(query)) continue;
    matches.push({ file: entry.file, anchor: entry.anchor || "", title, summary });
    if (matches.length >= 8) break;
  }
  if (!matches.length) {
    results.innerHTML = `<div class="docs-search-empty">${language === "zh" ? "没有找到匹配内容" : language === "ja" ? "一致する項目がありません" : "No matching results"}</div>`;
  } else {
    results.innerHTML = matches.map((match) => {
      const href = match.anchor ? `${match.file}#${match.anchor}` : match.file;
      return `<a class="docs-search-result" href="${href}"><strong>${match.title}</strong><span>${match.summary}</span></a>`;
    }).join("");
  }
  results.hidden = false;
}

document.querySelectorAll(".docs-search input").forEach((input) => {
  const results = document.getElementById("docs-search-results");
  input.addEventListener("input", renderSearch);
  input.addEventListener("focus", () => {
    if (input.value.trim().length > 0) results.hidden = false;
  });
  document.addEventListener("click", (event) => {
    if (!event.target.closest(".docs-search")) results.hidden = true;
  });
});

const backToTop = document.createElement("button");
backToTop.type = "button";
backToTop.className = "docs-back-top";
backToTop.textContent = "↑";
backToTop.setAttribute("aria-label", "Back to top");
backToTop.addEventListener("click", () => window.scrollTo({ top: 0, behavior: "smooth" }));
document.body.appendChild(backToTop);
window.addEventListener("scroll", () => {
  backToTop.classList.toggle("visible", window.scrollY > 480);
}, { passive: true });

const revealObserver = "IntersectionObserver" in window
  ? new IntersectionObserver((entries, observer) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add("is-visible");
          observer.unobserve(entry.target);
        }
      });
    }, { threshold: 0.06 })
  : null;

document.querySelectorAll("[data-reveal]").forEach((element) => {
  if (reduceMotion.matches) {
    element.classList.add("is-visible");
  } else if (revealObserver) {
    revealObserver.observe(element);
  } else {
    element.classList.add("is-visible");
  }
});

const year = document.getElementById("current-year");
if (year) year.textContent = String(new Date().getFullYear());

selectTheme(currentTheme());
selectLanguage(currentLanguage());
