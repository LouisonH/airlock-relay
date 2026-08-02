#!/usr/bin/env node

import { createHash } from "node:crypto";
import { constants, createReadStream, createWriteStream, realpathSync } from "node:fs";
import {
  access,
  chmod,
  mkdtemp,
  mkdir,
  rename,
  rm,
} from "node:fs/promises";
import { homedir, tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawn, spawnSync } from "node:child_process";
import { Readable } from "node:stream";
import { pipeline } from "node:stream/promises";
import {
  AIRLOCK_VERSION,
  RELEASE_URL,
  getPlatformContract,
  listPlatformContracts,
  releaseAssetURL,
  resolveReleasedArtifact,
} from "../lib/platform.mjs";

export { RELEASE_URL };

export const PACKAGE_NAME = "airlock-relay";
export const VERSION = AIRLOCK_VERSION;
const bundledMacTarget = resolveReleasedArtifact("darwin", "arm64");
export const ASSET_NAME = bundledMacTarget.artifactName;
export const ASSET_SHA256 = bundledMacTarget.sha256;

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
export const bundledAssetPath = resolve(scriptDirectory, "../dist", ASSET_NAME);

const help = `Airlock ${VERSION} npm installer

Usage:
  airlock install [--output <directory>] [--force] [--open]
  airlock verify
  airlock path
  airlock status [--json]
  airlock platform [--json]
  airlock doctor
  airlock release
  airlock version
  airlock help

Commands:
  install   Install the verified Airlock app for this platform.
  verify    Verify the SHA-256 digest of this platform's release asset.
  path      Print the bundled DMG path (Apple Silicon macOS only).
  status    Print the current platform and artifact status.
  platform  Print all platform release contracts.
  doctor    Verify the bundled installer without opening it.
  release   Print the official release page.
  version   Print the package version.

Options:
  --output  Destination directory for Airlock.app (macOS) or Airlock.AppImage (Linux).
  --force   Replace an incomplete Airlock.app at the destination.
  --open    Launch Airlock after it is installed.
  --json    Emit machine-readable status or platform data.
  --help    Show this help.

The verified npm installer supports Apple Silicon and Intel Macs (macOS 12 or newer),
Windows x64/x86/arm64, and Linux x64/arm64. macOS uses the bundled or release DMG,
Windows downloads the pinned NSIS installer, and Linux installs the pinned AppImage.
Every asset is SHA-256 verified against the release contract before installation and
fails closed on mismatch. Linux ARMv7 remains a Core/CLI-only target.
The macOS app is ad-hoc signed and is not Apple-notarized. Read the release notes:
${RELEASE_URL}
`;

export function parseArguments(argv) {
  const [first = "help", ...rest] = argv;
  if (first === "--help" || first === "-h") {
    return { command: "help", output: null, force: false, open: false, json: false };
  }
  if (first === "--version" || first === "-v") {
    return { command: "version", output: null, force: false, open: false, json: false };
  }

  const options = {
    command: first,
    output: null,
    force: false,
    open: false,
    json: false,
  };

  for (let index = 0; index < rest.length; index += 1) {
    const argument = rest[index];
    if (argument === "--force") {
      options.force = true;
      continue;
    }
    if (argument === "--open") {
      options.open = true;
      continue;
    }
    if (argument === "--json") {
      options.json = true;
      continue;
    }
    if (argument === "--output") {
      const value = rest[index + 1];
      if (!value || value.startsWith("--")) {
        throw new Error("--output requires a directory path");
      }
      options.output = value;
      index += 1;
      continue;
    }
    if (argument === "--help" || argument === "-h") {
      options.command = "help";
      continue;
    }
    throw new Error(`Unknown option: ${argument}`);
  }

  return options;
}

export async function sha256(filePath) {
  await access(filePath);
  return new Promise((resolveDigest, rejectDigest) => {
    const hash = createHash("sha256");
    const stream = createReadStream(filePath);
    stream.on("error", rejectDigest);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("end", () => resolveDigest(hash.digest("hex")));
  });
}

async function verifyFile(filePath, expectedDigest = ASSET_SHA256) {
  const digest = await sha256(filePath);
  if (digest !== expectedDigest) {
    throw new Error(
      `Integrity check failed for ${filePath}\nExpected ${expectedDigest}\nReceived ${digest}`,
    );
  }
  return digest;
}

function assertSupportedPlatform() {
  resolveReleasedArtifact(process.platform, process.arch);
}

async function fileExists(filePath) {
  try {
    await access(filePath);
    return true;
  } catch {
    return false;
  }
}

function runMacOSCommand(binary, args, label) {
  const result = spawnSync(binary, args, { encoding: "utf8" });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    const detail = result.stderr?.trim();
    throw new Error(`${label} failed${detail ? `: ${detail}` : ""}`);
  }
}

async function downloadFile(url, destination) {
  const response = await fetch(url, { redirect: "follow" });
  if (!response.ok || !response.body) {
    throw new Error(`Download failed (HTTP ${response.status}) for ${url}`);
  }
  await pipeline(Readable.fromWeb(response.body), createWriteStream(destination));
}

async function fetchVerifiedAsset(target) {
  const directory = await mkdtemp(join(tmpdir(), "airlock-asset-"));
  const destination = join(directory, target.artifactName);
  try {
    const url = releaseAssetURL(target);
    console.log(`Downloading ${url}`);
    await downloadFile(url, destination);
    await verifyFile(destination, target.sha256);
    return { directory, destination };
  } catch (error) {
    await rm(directory, { recursive: true, force: true });
    throw error;
  }
}

async function verifyApplicationBundle(applicationPath) {
  const desktopBinary = join(applicationPath, "Contents", "MacOS", "airlock-desktop");
  const sidecar = join(applicationPath, "Contents", "Resources", "airlockd");
  try {
    await access(desktopBinary, constants.X_OK);
    await access(sidecar, constants.X_OK);
  } catch {
    throw new Error("The Airlock.app bundle is incomplete or its local core is not executable.");
  }
}

async function mountDiskImage(filePath) {
  const mountDirectory = await mkdtemp(join(tmpdir(), "airlock-mount-"));
  try {
    runMacOSCommand(
      "/usr/bin/hdiutil",
      ["attach", "-nobrowse", "-readonly", "-mountpoint", mountDirectory, filePath],
      "macOS could not mount the verified installer",
    );
    return mountDirectory;
  } catch (error) {
    await rm(mountDirectory, { recursive: true, force: true });
    throw error;
  }
}

async function detachDiskImage(mountDirectory) {
  try {
    runMacOSCommand("/usr/bin/hdiutil", ["detach", mountDirectory], "macOS could not detach the installer");
  } catch (error) {
    console.warn(`Warning: ${error.message}`);
  } finally {
    await rm(mountDirectory, { recursive: true, force: true });
  }
}

function openApplication(applicationPath) {
  runMacOSCommand("/usr/bin/open", [applicationPath], "macOS could not launch Airlock");
}

function launchDetached(executable, args = []) {
  const child = spawn(executable, args, { stdio: "ignore", detached: true });
  child.on("error", () => {});
  child.unref();
}

async function installMacOSDMG(assetPath, target, options) {
  const applicationsDirectory = resolve(options.output ?? join(homedir(), "Applications"));
  const destination = join(applicationsDirectory, "Airlock.app");
  await mkdir(applicationsDirectory, { recursive: true });

  if (await fileExists(destination)) {
    try {
      await verifyApplicationBundle(destination);
      console.log(`Updating installed Airlock: ${destination}`);
    } catch {
      if (!options.force) {
        throw new Error(
          `An incomplete Airlock.app already exists at ${destination}. Use --force to replace it.`,
        );
      }
    }
  }

  const mountDirectory = await mountDiskImage(assetPath);
  const source = join(mountDirectory, "Airlock.app");
  const temporaryPath = join(applicationsDirectory, `.Airlock-${process.pid}.app`);
  try {
    await verifyApplicationBundle(source);
    await rm(temporaryPath, { recursive: true, force: true });
    runMacOSCommand("/usr/bin/ditto", [source, temporaryPath], "macOS could not copy Airlock.app");
    await verifyApplicationBundle(temporaryPath);
    if (await fileExists(destination)) {
      await rm(destination, { recursive: true, force: true });
    }
    await rename(temporaryPath, destination);
  } catch (error) {
    await rm(temporaryPath, { recursive: true, force: true });
    throw error;
  } finally {
    await detachDiskImage(mountDirectory);
  }

  console.log(`Installed Airlock: ${destination}`);
  console.log(`SHA-256: ${target.sha256}`);

  if (options.open) {
    openApplication(destination);
  }
}

async function installWindowsNSIS(installerPath, target, options) {
  const result = spawnSync(installerPath, ["/S"], { stdio: "inherit" });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`Airlock Windows installer exited with status ${result.status}`);
  }
  console.log("Installed Airlock (Windows).");
  console.log(`SHA-256: ${target.sha256}`);
  if (options.open) {
    const programFiles = process.env.ProgramFiles ?? "C:\\Program Files";
    launchDetached(join(programFiles, "Airlock", "Airlock.exe"));
  }
}

async function installLinuxAppImage(assetPath, target, options) {
  const binDirectory = resolve(options.output ?? join(homedir(), ".local", "bin"));
  await mkdir(binDirectory, { recursive: true });
  const destination = join(binDirectory, "Airlock.AppImage");
  await rm(destination, { force: true });
  await rename(assetPath, destination);
  await chmod(destination, 0o755);
  console.log(`Installed Airlock: ${destination}`);
  console.log(`SHA-256: ${target.sha256}`);
  console.log("Launch with ~/.local/bin/Airlock.AppImage (use --appimage-extract-and-run if FUSE is unavailable).");
  if (options.open) {
    launchDetached(destination, ["--appimage-extract-and-run"]);
  }
}

async function install(options) {
  const target = resolveReleasedArtifact();
  let downloaded = null;
  let assetPath = bundledAssetPath;
  if (target.platform !== "darwin" || target.arch !== "arm64") {
    downloaded = await fetchVerifiedAsset(target);
    assetPath = downloaded.destination;
  } else {
    await verifyFile(bundledAssetPath, target.sha256);
  }
  try {
    switch (target.installType) {
      case "macos-dmg":
        await installMacOSDMG(assetPath, target, options);
        break;
      case "windows-nsis":
        await installWindowsNSIS(assetPath, target, options);
        break;
      case "linux-appimage":
        await installLinuxAppImage(assetPath, target, options);
        break;
      default:
        throw new Error(`Unsupported installer type: ${target.installType}`);
    }
  } finally {
    if (downloaded) {
      await rm(downloaded.directory, { recursive: true, force: true });
    }
  }
}

function writeJson(value) {
  console.log(JSON.stringify(value, null, 2));
}

function status(options) {
  const target = getPlatformContract();
  const report = {
    package: PACKAGE_NAME,
    version: VERSION,
    releaseUrl: RELEASE_URL,
    currentTarget: target,
    installerAvailable: target.installerAvailable,
    installAction: target.installerAvailable
      ? target.platform === "darwin" && target.arch === "arm64"
        ? "bundled-verified-installer"
        : "release-download-verified-installer"
      : "no-verified-installer-published",
  };
  if (options.json) {
    writeJson(report);
    return;
  }
  console.log(`${PACKAGE_NAME} v${VERSION}`);
  console.log(`Platform: ${target.label} (${target.platform}/${target.arch})`);
  console.log(`Release status: ${target.status}`);
  console.log(`Verified installer: ${target.installerAvailable ? "available" : "not published"}`);
  console.log(`Artifact: ${target.artifactName ?? "not published"}`);
  console.log(`Release page: ${RELEASE_URL}`);
}

function platform(options) {
  const contracts = listPlatformContracts();
  if (options.json) {
    writeJson({ version: VERSION, targets: contracts });
    return;
  }
  for (const target of contracts) {
    const artifact = target.artifactName ?? "not published";
    console.log(`${target.label}: ${target.status} (${artifact})`);
  }
}

async function doctor() {
  const target = resolveReleasedArtifact();
  let downloaded = null;
  let filePath = bundledAssetPath;
  if (target.platform !== "darwin" || target.arch !== "arm64") {
    downloaded = await fetchVerifiedAsset(target);
    filePath = downloaded.destination;
  }
  try {
    const digest = await verifyFile(filePath, target.sha256);
    console.log("Installer integrity: verified");
    console.log(`SHA-256: ${digest}`);
    console.log("No application was opened or installed.");
  } finally {
    if (downloaded) {
      await rm(downloaded.directory, { recursive: true, force: true });
    }
  }
}

export async function main(argv = process.argv.slice(2)) {
  const options = parseArguments(argv);
  if (options.json && options.command !== "status" && options.command !== "platform") {
    throw new Error("--json is available with status and platform only");
  }

  switch (options.command) {
    case "help":
      console.log(help);
      return;
    case "version":
      console.log(`${PACKAGE_NAME} v${VERSION}`);
      return;
    case "path":
      if (process.platform !== "darwin" || process.arch !== "arm64") {
        throw new Error(
          "path is only available on Apple Silicon macOS. Use `airlock doctor` to verify this platform's release asset.",
        );
      }
      await verifyFile(bundledAssetPath);
      console.log(bundledAssetPath);
      return;
    case "status":
      status(options);
      return;
    case "platform":
      platform(options);
      return;
    case "doctor":
      await doctor();
      return;
    case "release":
      console.log(RELEASE_URL);
      return;
    case "verify": {
      const target = resolveReleasedArtifact();
      if (target.platform === "darwin" && target.arch === "arm64") {
        const digest = await verifyFile(bundledAssetPath, target.sha256);
        console.log(`Verified ${ASSET_NAME}`);
        console.log(`SHA-256: ${digest}`);
      } else {
        const downloaded = await fetchVerifiedAsset(target);
        try {
          console.log(`Verified ${target.artifactName}`);
          console.log(`SHA-256: ${target.sha256}`);
        } finally {
          await rm(downloaded.directory, { recursive: true, force: true });
        }
      }
      return;
    }
    case "install":
      await install(options);
      return;
    default:
      throw new Error(`Unknown command: ${options.command}\n\n${help}`);
  }
}

const invokedPath = process.argv[1]
  ? pathToFileURL(realpathSync(resolve(process.argv[1]))).href
  : "";
if (import.meta.url === invokedPath) {
  main().catch((error) => {
    console.error(`airlock: ${error.message}`);
    process.exitCode = 1;
  });
}
