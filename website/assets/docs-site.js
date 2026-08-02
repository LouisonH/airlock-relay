const docsLanguageKey = "airlock.docs2.language";
const docsThemeKey = "airlock.site.theme";
const docsRoot = document.documentElement;
const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

const chapters = [
  { id: "getting-started", file: "getting-started.html", zh: "从零开始", en: "Getting started" },
  { id: "install", file: "install.html", zh: "安装与校验", en: "Install and verify" },
  { id: "desktop", file: "desktop.html", zh: "桌面控制台", en: "Desktop console" },
  { id: "security", file: "security.html", zh: "安全模型", en: "Security model" },
  { id: "routes", file: "routes.html", zh: "路由与策略", en: "Routes and policies" },
  { id: "server", file: "server.html", zh: "Server Core", en: "Server Core" },
  { id: "operations", file: "operations.html", zh: "运维与排障", en: "Operations" },
  { id: "platforms", file: "platforms.html", zh: "跨平台与树莓派", en: "Platforms and Pi" },
  { id: "cli", file: "cli.html", zh: "CLI 参考", en: "CLI reference" },
];

function storageGet(key) {
  try { return localStorage.getItem(key); } catch { return null; }
}
function storageSet(key, value) {
  try { localStorage.setItem(key, value); } catch { return; }
}

function currentLanguage() {
  const stored = storageGet(docsLanguageKey);
  if (stored === "zh" || stored === "en") return stored;
  return navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en";
}

function currentTheme() {
  const stored = storageGet(docsThemeKey);
  if (stored === "light" || stored === "dark") return stored;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function selectLanguage(language) {
  docsRoot.lang = language === "zh" ? "zh-CN" : "en";
  docsRoot.dataset.lang = language;
  document.querySelectorAll("[data-language]").forEach((article) => {
    article.hidden = article.dataset.language !== language;
  });
  document.querySelectorAll("[data-language-button]").forEach((button) => {
    button.classList.toggle("active", button.dataset.languageButton === language);
  });
  document.querySelectorAll("[data-i18n]").forEach((element) => {
    const key = element.dataset.i18n;
    const value = language === "zh"
      ? element.dataset.zh
      : element.dataset.en;
    if (value !== undefined) element.textContent = value;
  });
  storageSet(docsLanguageKey, language);
  buildPager(language);
}

function selectTheme(theme) {
  docsRoot.dataset.theme = theme;
  const button = document.getElementById("docs-theme");
  if (button) {
    button.textContent = theme === "dark" ? "◑" : "◐";
  }
  storageSet(docsThemeKey, theme);
}

function buildPager(language) {
  const pager = document.getElementById("docs-pager");
  if (!pager) return;
  const current = document.body.dataset.chapter;
  const index = chapters.findIndex((chapter) => chapter.id === current);
  const previous = index > 0 ? chapters[index - 1] : null;
  const next = index >= 0 && index < chapters.length - 1 ? chapters[index + 1] : null;
  const label = (chapter) => language === "zh" ? chapter.zh : chapter.en;
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

const revealObserver = "IntersectionObserver" in window
  ? new IntersectionObserver((entries, observer) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add("is-visible");
          observer.unobserve(entry.target);
        }
      });
    }, { threshold: 0.08 })
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
