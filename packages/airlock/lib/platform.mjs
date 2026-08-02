export const AIRLOCK_VERSION = "0.1.6";

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
    artifactName: "Airlock_0.1.6_aarch64.dmg",
    sha256: "4226c214c17e58082e6cbbc25336b575d4bf879e07c8c5364b0165131e237c9e",
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
    artifactName: "Airlock_0.1.6_x64.dmg",
    sha256: "ea8c07252d94fed56dd74cc12f76ca389323ff76f4391dc18ea33d34dafc82c9",
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
    artifactName: "Airlock_0.1.6_x64-setup.exe",
    sha256: "209da80a8d145a34f3821a5b7c9e8af557e1b12cce7f02e4095db7751f10125a",
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
    artifactName: "Airlock_0.1.6_arm64-setup.exe",
    sha256: "57f6344f009238b574d8f95f688cf0e8e3da662cc99f4ef8c1a2bd10d9a5a212",
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
    artifactName: "Airlock_0.1.6_x86-setup.exe",
    sha256: "3a0e82a324f640b3ce69c30da4d8f8ae0e2af0a4df40700e6d3e9dd7f73eea41",
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
    artifactName: "Airlock_0.1.6_amd64.AppImage",
    sha256: "6111aa3bede819f037dfbd9b6bab0aa81d63f9f3ef6e4d7c1c1c1fe69915535f",
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
    artifactName: "Airlock_0.1.6_aarch64.AppImage",
    sha256: "08eb0bd08d32ed90b9a4a966ea5fbf0e18a1a08897dbd6479b466658ac4bf521",
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
