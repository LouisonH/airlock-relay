export const AIRLOCK_VERSION = "0.1.0";

const targets = [
  {
    id: "macos-arm64",
    platform: "darwin",
    arch: "arm64",
    label: "macOS / Apple Silicon",
    bundles: ["dmg", "app"],
    secureEntry: "airlock-ssh-wizard+native-os-confirmation",
    status: "released",
    artifactName: "Airlock_0.1.0_aarch64.dmg",
    sha256: "4c49368349bd82a33dc0917facbb3b2b50a08aa20f029bada7bf48053368e941",
  },
  {
    id: "macos-x64",
    platform: "darwin",
    arch: "x64",
    label: "macOS / Intel",
    bundles: ["dmg", "app"],
    secureEntry: "airlock-ssh-wizard+native-os-confirmation",
    status: "planned",
  },
  {
    id: "windows-x64",
    platform: "win32",
    arch: "x64",
    label: "Windows / x64",
    bundles: ["nsis", "msi"],
    secureEntry: "airlock-ssh-wizard+windows-confirmation",
    status: "planned",
  },
  {
    id: "linux-x64",
    platform: "linux",
    arch: "x64",
    label: "Linux / x64",
    bundles: ["appimage", "deb"],
    secureEntry: "airlock-ssh-wizard+secret-service",
    status: "planned",
  },
  {
    id: "linux-arm64",
    platform: "linux",
    arch: "arm64",
    label: "Linux / arm64",
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
    super(`Airlock ${AIRLOCK_VERSION} does not publish an installer for ${target.label} yet.`);
    this.name = "UnreleasedPlatformError";
    this.target = target;
  }
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
  }));
}
