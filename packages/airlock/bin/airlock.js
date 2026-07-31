#!/usr/bin/env node

import { createHash } from "node:crypto";
import { createReadStream, realpathSync } from "node:fs";
import {
  access,
  copyFile,
  mkdir,
  rename,
  rm,
} from "node:fs/promises";
import { homedir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";
import { AIRLOCK_VERSION, getPlatformContract, listPlatformContracts, resolveReleasedArtifact } from "../lib/platform.mjs";

export const PACKAGE_NAME = "airlock-relay";
export const VERSION = AIRLOCK_VERSION;
const releasedMacArtifact = resolveReleasedArtifact("darwin", "arm64");
export const ASSET_NAME = releasedMacArtifact.artifactName;
export const ASSET_SHA256 = releasedMacArtifact.sha256;
export const RELEASE_URL =
  "https://github.com/LouisonH/airlock-relay/releases/tag/v0.1.1";

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
  install   Copy the verified Airlock DMG to ~/Downloads.
  verify    Verify the SHA-256 digest of the bundled DMG.
  path      Print the bundled DMG path.
  status    Print the current platform and artifact status.
  platform  Print all platform release contracts.
  doctor    Verify the bundled installer without opening it.
  release   Print the official release page.
  version   Print the package version.

Options:
  --output  Destination directory for the DMG.
  --force   Replace a different file at the destination.
  --open    Open the verified DMG after copying it.
  --json    Emit machine-readable status or platform data.
  --help    Show this help.

Airlock v0.1.1 supports Apple Silicon Macs running macOS 12 or newer.
The app is ad-hoc signed and is not Apple-notarized. Read the release notes:
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

async function verifyFile(filePath) {
  const digest = await sha256(filePath);
  if (digest !== ASSET_SHA256) {
    throw new Error(
      `Integrity check failed for ${filePath}\nExpected ${ASSET_SHA256}\nReceived ${digest}`,
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

function openDiskImage(filePath) {
  const result = spawnSync("/usr/bin/open", [filePath], { stdio: "inherit" });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`macOS could not open ${filePath}`);
  }
}

async function install(options) {
  assertSupportedPlatform();
  await verifyFile(bundledAssetPath);

  const outputDirectory = resolve(options.output ?? join(homedir(), "Downloads"));
  const destination = join(outputDirectory, ASSET_NAME);
  await mkdir(outputDirectory, { recursive: true });

  if (await fileExists(destination)) {
    const currentDigest = await sha256(destination);
    if (currentDigest === ASSET_SHA256) {
      console.log(`Already installed and verified: ${destination}`);
      if (options.open) {
        openDiskImage(destination);
      }
      return;
    }
    if (!options.force) {
      throw new Error(
        `A different file already exists at ${destination}. Use --force to replace it.`,
      );
    }
  }

  const temporaryPath = `${destination}.airlock-${process.pid}.part`;
  try {
    await copyFile(bundledAssetPath, temporaryPath);
    await verifyFile(temporaryPath);
    await rename(temporaryPath, destination);
  } catch (error) {
    await rm(temporaryPath, { force: true });
    throw error;
  }

  console.log(`Installed and verified: ${destination}`);
  console.log(`SHA-256: ${ASSET_SHA256}`);
  console.log("Next: open the DMG, then drag Airlock into Applications.");

  if (options.open) {
    openDiskImage(destination);
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
    installerReleased: target.status === "released",
  };
  if (options.json) {
    writeJson(report);
    return;
  }
  console.log(`${PACKAGE_NAME} v${VERSION}`);
  console.log(`Platform: ${target.label} (${target.platform}/${target.arch})`);
  console.log(`Release status: ${target.status}`);
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
  assertSupportedPlatform();
  const digest = await verifyFile(bundledAssetPath);
  console.log(`Installer integrity: verified`);
  console.log(`SHA-256: ${digest}`);
  console.log("No application was opened or installed.");
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
      const digest = await verifyFile(bundledAssetPath);
      console.log(`Verified ${ASSET_NAME}`);
      console.log(`SHA-256: ${digest}`);
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
