export const AIRLOCK_VERSION = "0.1.7";

const releaseTag = `v${AIRLOCK_VERSION}`;
export const RELEASE_URL = `https://github.com/LouisonH/airlock-relay/releases/tag/${releaseTag}`;
export const RELEASE_DOWNLOAD_BASE = `https://github.com/LouisonH/airlock-relay/releases/download/${releaseTag}`;

const targets = [
  {
    id: "macos-arm64",
    platform: "darwin",
    arch: "arm64",
    label: "macOS / Apple Silicon",
    bundles: ["dmg", "app"],
    secureEntry: "airlock-ssh-wizard+native-os-confirmation",
    status: "released",
    installType: "macos-dmg",
    artifactName: "Airlock_0.1.7_aarch64.dmg",
    sha256: "59d3bff839a96afbb8ac47bdbb57096d8a3b621a4cc91628de6c9f18f02d3c4c",
  },
  {
    id: "macos-x64",
    platform: "darwin",
    arch: "x64",
    label: "macOS / Intel",
    bundles: ["dmg", "app"],
    secureEntry: "airlock-ssh-wizard+native-os-confirmation",
    status: "released",
    installType: "macos-dmg",
    artifactName: "Airlock_0.1.7_x64.dmg",
    sha256: "PENDING_MACOS_X64",
  },
  {
    id: "windows-x64",
    platform: "win32",
    arch: "x64",
    label: "Windows / x64",
    bundles: ["nsis", "msi"],
    secureEntry: "airlock-ssh-wizard+windows-confirmation",
    status: "released",
    installType: "windows-nsis",
    artifactName: "Airlock_0.1.7_x64-setup.exe",
    sha256: "PENDING_WINDOWS_X64",
  },
  {
    id: "windows-arm64",
    platform: "win32",
    arch: "arm64",
    label: "Windows / arm64",
    bundles: ["nsis", "msi"],
    secureEntry: "airlock-ssh-wizard+windows-confirmation",
    status: "released",
    installType: "windows-nsis",
    artifactName: "Airlock_0.1.7_arm64-setup.exe",
    sha256: "PENDING_WINDOWS_ARM64",
  },
  {
    id: "windows-x86",
    platform: "win32",
    arch: "ia32",
    label: "Windows / x86 (i686)",
    bundles: ["nsis", "msi"],
    secureEntry: "airlock-ssh-wizard+windows-confirmation",
    status: "released",
    installType: "windows-nsis",
    artifactName: "Airlock_0.1.7_x86-setup.exe",
    sha256: "PENDING_WINDOWS_X86",
  },
  {
    id: "linux-x64",
    platform: "linux",
    arch: "x64",
    label: "Linux / x64",
    bundles: ["appimage", "deb"],
    secureEntry: "airlock-ssh-wizard+secret-service",
    status: "released",
    installType: "linux-appimage",
    artifactName: "Airlock_0.1.7_amd64.AppImage",
    sha256: "PENDING_LINUX_X64",
  },
  {
    id: "linux-arm64",
    platform: "linux",
    arch: "arm64",
    label: "Linux / arm64",
    bundles: ["appimage", "deb"],
    secureEntry: "airlock-ssh-wizard+secret-service",
    status: "released",
    installType: "linux-appimage",
    artifactName: "Airlock_0.1.7_aarch64.AppImage",
    sha256: "PENDING_LINUX_ARM64",
  },
  {
    id: "linux-x86",
    platform: "linux",
    arch: "ia32",
    label: "Linux / x86 (i686)",
    bundles: ["appimage", "deb"],
    secureEntry: "airlock-ssh-wizard+secret-service",
    status: "planned",
  },
  {
    id: "linux-armv7",
    platform: "linux",
    arch: "arm",
    label: "Linux / ARMv7 (Raspberry Pi)",
    bundles: ["appimage", "deb"],
    secureEntry: "airlock-ssh-wizard+secret-service",
    status: "planned",
  },
];

export const PLATFORM_TARGETS = Object.freeze(
  targets.map((target) => Object.freeze({ ...target, bundles: Object.freeze([...target.bundles]) })),
);

export class UnsupportedPlatformError extends Error {
  constructor(platform, arch) {
    super(`Airlock ${AIRLOCK_VERSION} has no platform contract for ${platform}/${arch}.`);
    this.name = "UnsupportedPlatformError";
    this.platform = platform;
    this.arch = arch;
  }
}

export class UnreleasedPlatformError extends Error {
  constructor(target) {
    const detail = target.status === "preview"
      ? "a CI preview exists, but no verified public installer is published"
      : "no installer is published";
    super(`Airlock ${AIRLOCK_VERSION} recognizes ${target.label}, but ${detail}.`);
    this.name = "UnreleasedPlatformError";
    this.target = target;
  }
}

export function releaseAssetURL(target = getPlatformTarget()) {
  if (target.status !== "released" || !target.artifactName || !target.sha256) {
    throw new UnreleasedPlatformError(target);
  }
  return `${RELEASE_DOWNLOAD_BASE}/${encodeURIComponent(target.artifactName)}`;
}

export function getPlatformTarget(platform = process.platform, arch = process.arch) {
  const target = PLATFORM_TARGETS.find(
    (candidate) => candidate.platform === platform && candidate.arch === arch,
  );
  if (!target) throw new UnsupportedPlatformError(platform, arch);
  return target;
}

export function resolveReleasedArtifact(platform = process.platform, arch = process.arch) {
  const target = getPlatformTarget(platform, arch);
  if (target.status !== "released" || !target.artifactName || !target.sha256) {
    throw new UnreleasedPlatformError(target);
  }
  return target;
}

export function getPlatformContract(platform = process.platform, arch = process.arch) {
  const target = getPlatformTarget(platform, arch);
  const installerAvailable = target.status === "released" && Boolean(target.artifactName && target.sha256);
  return Object.freeze({
    version: AIRLOCK_VERSION,
    platform: target.platform,
    arch: target.arch,
    label: target.label,
    bundles: [...target.bundles],
    secureEntry: target.secureEntry,
    status: target.status,
    artifactName: target.artifactName ?? null,
    sha256: target.sha256 ?? null,
    installerAvailable,
  });
}

export function listPlatformContracts() {
  return PLATFORM_TARGETS.map((target) => Object.freeze({
    version: AIRLOCK_VERSION,
    platform: target.platform,
    arch: target.arch,
    label: target.label,
    bundles: [...target.bundles],
    secureEntry: target.secureEntry,
    status: target.status,
    artifactName: target.artifactName ?? null,
    sha256: target.sha256 ?? null,
    installerAvailable: target.status === "released" && Boolean(target.artifactName && target.sha256),
  }));
}
