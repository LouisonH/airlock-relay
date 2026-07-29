import { getCurrentWindow } from "@tauri-apps/api/window";

export type ThemePreference = "system" | "light" | "dark";
export type ResolvedTheme = Exclude<ThemePreference, "system">;
export type AccentTheme = "forest" | "ocean" | "amber";

const storageKey = "airlock.ui.theme";
const accentStorageKey = "airlock.ui.accent";
const darkModeQuery = "(prefers-color-scheme: dark)";

export function getThemePreference(): ThemePreference {
  const stored = localStorage.getItem(storageKey);
  return stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
}

export function resolveTheme(preference: ThemePreference): ResolvedTheme {
  if (preference !== "system") {
    return preference;
  }
  return window.matchMedia(darkModeQuery).matches ? "dark" : "light";
}

export function applyTheme(preference: ThemePreference): void {
  const resolved = resolveTheme(preference);
  document.documentElement.dataset.theme = resolved;
  document.documentElement.style.colorScheme = resolved;
  if ("__TAURI_INTERNALS__" in window) {
    void getCurrentWindow().setTheme(resolved).catch(() => undefined);
  }
}

export function saveThemePreference(preference: ThemePreference): void {
  localStorage.setItem(storageKey, preference);
  applyTheme(preference);
}

export function getAccentTheme(): AccentTheme {
  const stored = localStorage.getItem(accentStorageKey);
  return stored === "forest" || stored === "ocean" || stored === "amber" ? stored : "forest";
}

export function saveAccentTheme(accent: AccentTheme): void {
  localStorage.setItem(accentStorageKey, accent);
  document.documentElement.dataset.accent = accent;
}

export function watchSystemTheme(onChange: () => void): () => void {
  const media = window.matchMedia(darkModeQuery);
  media.addEventListener("change", onChange);
  return () => media.removeEventListener("change", onChange);
}

applyTheme(getThemePreference());
saveAccentTheme(getAccentTheme());
