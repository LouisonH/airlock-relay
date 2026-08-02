const docsRoot = document.documentElement;
const docsLanguageKey = "airlock.docs.language";
const docsThemeKey = "airlock.site.theme";
const docsThemeButton = document.getElementById("theme-toggle");
const docsSearch = document.getElementById("docs-search-input");
const docsSearchClear = document.getElementById("docs-search-clear");
const docsSearchResults = document.getElementById("docs-search-results");
const docsSearchLabel = document.getElementById("docs-search-label");
const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
let revealObserver;
let chapterObserver;

const copy = {
  zh: { title: "Airlock 文档", home: "主页", backHome: "返回项目主页", chapters: "章节", chapterCore: "核心概念", chapterDesktop: "桌面控制台", chapterOperate: "发布与运维", searchLabel: "搜索文档", searchPlaceholder: "搜索章节、命令或主题", searchClear: "清除搜索", searchEmpty: "没有找到匹配章节", sections: { boundary: "安全边界", desktop: "路由与活动", version: "版本与更新", install: "安装与下载", cli: "安装 CLI", server: "Server Core", platform: "多平台状态" } },
  en: { title: "Airlock Documentation", home: "Home", backHome: "Back to project home", chapters: "Chapters", chapterCore: "Core concepts", chapterDesktop: "Desktop console", chapterOperate: "Release and operations", searchLabel: "Search documentation", searchPlaceholder: "Search chapters, commands, or topics", searchClear: "Clear search", searchEmpty: "No matching chapters", sections: { boundary: "Security boundary", desktop: "Routes and activity", version: "Versions and updates", install: "Install and download", cli: "Installer CLI", server: "Server Core", platform: "Platform status" } },
  ja: { title: "Airlock ドキュメント", home: "ホーム", backHome: "プロジェクトホームへ戻る", chapters: "チャプター", chapterCore: "基本概念", chapterDesktop: "デスクトップコンソール", chapterOperate: "リリースと運用", searchLabel: "ドキュメントを検索", searchPlaceholder: "章、コマンド、トピックを検索", searchClear: "検索をクリア", searchEmpty: "一致する章はありません", sections: { boundary: "セキュリティ境界", desktop: "ルートとアクティビティ", version: "バージョンと更新", install: "インストールとダウンロード", cli: "インストーラー CLI", server: "Server Core", platform: "プラットフォーム状況" } },
};

const guideSections = window.AirlockDocsGuides ?? {};

function ensureGuideSections(language) {
  const article = document.querySelector(`.docs-language[data-language="${language}"]`);
  if (!article || article.dataset.guidesReady === "true") return;
  (guideSections[language] ?? []).forEach((guide) => {
    const section = document.createElement("section");
    section.id = `${language}-${guide.id}`;
    section.className = "docs-guide-section";
    section.innerHTML = `<span>${guide.number}</span><div><p class="docs-section-eyebrow">${guide.eyebrow}</p><h2>${guide.title}</h2>${guide.body}</div>`;
    article.append(section);
  });
  article.dataset.guidesReady = "true";
}

function updateGuideLinks(language) {
  document.querySelectorAll(".docs-dynamic-link, .docs-dynamic-group").forEach((element) => element.remove());
  const guides = guideSections[language] ?? [];
  if (!guides.length) return;
  const nav = document.querySelector(".docs-chapter-nav");
  if (!nav) return;
  const group = document.createElement("div");
  group.className = "docs-chapter-group docs-dynamic-group";
  const title = document.createElement("span");
  title.textContent = language === "zh" ? "实用指南" : language === "ja" ? "実用ガイド" : "Practical guides";
  group.append(title);
  guides.forEach((guide) => {
    const link = document.createElement("a");
    link.className = "docs-chapter-link docs-dynamic-link";
    link.dataset.docSection = guide.id;
    link.href = `#${language}-${guide.id}`;
    link.textContent = guide.title;
    group.append(link);
  });
  nav.append(group);
}

function readPreference(key) {
  try { return localStorage.getItem(key); } catch { return null; }
}

function savePreference(key, value) {
  try { localStorage.setItem(key, value); } catch { /* Storage is optional. */ }
}

function selectedArticle() {
  return document.querySelector(".docs-language:not([hidden])");
}

function updateCopy(language) {
  const labels = copy[language];
  document.querySelectorAll("[data-doc-copy]").forEach((element) => {
    const value = labels[element.dataset.docCopy];
    if (value) element.textContent = value;
  });
  document.querySelectorAll(".docs-chapter-link").forEach((link) => {
    const section = link.dataset.docSection;
    link.href = `#${language}-${section}`;
    link.textContent = labels.sections[section];
  });
  const homeLanguage = language === "ja" ? "en" : language;
  ["docs-home-link", "docs-sidebar-home", "docs-footer-home"].forEach((id) => {
    const link = document.getElementById(id);
    if (link) link.href = `./index.html?lang=${homeLanguage}`;
  });
  const home = document.getElementById("docs-home-link");
  if (home) home.textContent = labels.home;
  docsSearchLabel.textContent = labels.searchLabel;
  docsSearch.placeholder = labels.searchPlaceholder;
  docsSearchClear.setAttribute("aria-label", labels.searchClear);
}

function selectLanguage(language) {
  const selected = Object.prototype.hasOwnProperty.call(copy, language) ? language : "zh";
  ensureGuideSections(selected);
  docsRoot.dataset.lang = selected;
  docsRoot.lang = selected === "zh" ? "zh-CN" : selected;
  document.querySelectorAll(".docs-language").forEach((article) => { article.hidden = article.dataset.language !== selected; });
  document.querySelectorAll("[data-language]").forEach((button) => { button.classList.toggle("active", button.dataset.language === selected); });
  document.title = copy[selected].title;
  updateCopy(selected);
  updateGuideLinks(selected);
  closeSearch();
  savePreference(docsLanguageKey, selected);
  revealActiveLanguage();
  observeChapters();
}

function selectTheme(theme) {
  docsRoot.dataset.theme = theme;
  docsThemeButton.textContent = theme === "dark" ? "◑" : "◐";
  docsThemeButton.title = theme === "dark" ? "Switch to light theme" : "Switch to dark theme";
  savePreference(docsThemeKey, theme);
}

function revealActiveLanguage() {
  const article = selectedArticle();
  if (!article) return;
  article.querySelectorAll("[data-reveal], section").forEach((element) => {
    if (element.tagName === "SECTION") element.classList.add("docs-reveal");
    if (reduceMotion.matches) element.classList.add("is-visible");
    else revealObserver.observe(element);
  });
}

function setActiveChapter(id) {
  document.querySelectorAll(".docs-chapter-link").forEach((link) => link.classList.toggle("active", link.getAttribute("href") === `#${id}`));
}

function observeChapters() {
  if (chapterObserver) chapterObserver.disconnect();
  const article = selectedArticle();
  if (!article) return;
  const first = article.querySelector("section");
  if (first) setActiveChapter(first.id);
  article.querySelectorAll("section").forEach((section) => chapterObserver.observe(section));
}

function closeSearch() {
  docsSearch.value = "";
  docsSearchClear.hidden = true;
  docsSearchResults.hidden = true;
  docsSearchResults.replaceChildren();
  docsSearch.setAttribute("aria-expanded", "false");
}

function searchSnippet(text, query) {
  const compact = text.replace(/\s+/g, " ").trim();
  const found = compact.toLocaleLowerCase().indexOf(query.toLocaleLowerCase());
  if (found < 0) return compact.slice(0, 138);
  const start = Math.max(0, found - 38);
  const end = Math.min(compact.length, found + query.length + 108);
  return `${start > 0 ? "..." : ""}${compact.slice(start, end)}${end < compact.length ? "..." : ""}`;
}

function renderSearch() {
  const query = docsSearch.value.trim();
  docsSearchClear.hidden = !query;
  docsSearchResults.replaceChildren();
  if (!query) {
    docsSearchResults.hidden = true;
    docsSearch.setAttribute("aria-expanded", "false");
    return;
  }
  const article = selectedArticle();
  const matches = article ? [...article.querySelectorAll("section")].filter((section) => section.textContent.toLocaleLowerCase().includes(query.toLocaleLowerCase())).slice(0, 8) : [];
  const language = docsRoot.dataset.lang;
  if (!matches.length) {
    const empty = document.createElement("div");
    empty.className = "docs-search-empty";
    empty.textContent = copy[language].searchEmpty;
    docsSearchResults.append(empty);
  }
  matches.forEach((section) => {
    const button = document.createElement("button");
    const title = section.querySelector("h2")?.textContent ?? section.id;
    const detail = document.createElement("span");
    const heading = document.createElement("strong");
    button.type = "button";
    button.className = "docs-search-result";
    button.setAttribute("role", "option");
    heading.textContent = title;
    detail.textContent = searchSnippet(section.textContent, query);
    button.append(heading, detail);
    button.addEventListener("click", () => {
      closeSearch();
      section.tabIndex = -1;
      section.scrollIntoView({ behavior: reduceMotion.matches ? "auto" : "smooth", block: "start" });
      section.focus({ preventScroll: true });
      history.replaceState(null, "", `#${section.id}`);
      setActiveChapter(section.id);
    });
    docsSearchResults.append(button);
  });
  docsSearchResults.hidden = false;
  docsSearch.setAttribute("aria-expanded", "true");
}

if (!reduceMotion.matches && "IntersectionObserver" in window) {
  revealObserver = new IntersectionObserver((entries) => entries.forEach((entry) => {
    if (!entry.isIntersecting) return;
    entry.target.classList.add("is-visible");
    revealObserver.unobserve(entry.target);
  }), { threshold: 0.1, rootMargin: "0px 0px -28px" });
} else {
  revealObserver = { observe: (element) => element.classList.add("is-visible") };
}

chapterObserver = new IntersectionObserver((entries) => entries.forEach((entry) => {
  if (entry.isIntersecting) setActiveChapter(entry.target.id);
}), { rootMargin: "-20% 0px -68%", threshold: 0.01 });

document.querySelectorAll(".language-switch button").forEach((button) => button.addEventListener("click", () => selectLanguage(button.dataset.language)));
docsThemeButton.addEventListener("click", () => selectTheme(docsRoot.dataset.theme === "dark" ? "light" : "dark"));
docsSearch.addEventListener("input", renderSearch);
docsSearchClear.addEventListener("click", () => { closeSearch(); docsSearch.focus(); });
docsSearch.addEventListener("keydown", (event) => { if (event.key === "Escape") { closeSearch(); docsSearch.blur(); } });
document.addEventListener("click", (event) => { if (!event.target.closest(".docs-search")) closeSearch(); });

const hashLanguage = location.hash.match(/^#(zh|en|ja)-/)?.[1];
const initialLanguage = hashLanguage ?? new URLSearchParams(window.location.search).get("lang") ?? readPreference(docsLanguageKey) ?? (navigator.language.toLowerCase().startsWith("ja") ? "ja" : navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en");
selectLanguage(initialLanguage);
selectTheme(readPreference(docsThemeKey) ?? (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"));
window.addEventListener("hashchange", () => {
  const language = location.hash.match(/^#(zh|en|ja)-/)?.[1];
  if (language && language !== docsRoot.dataset.lang) selectLanguage(language);
});
