import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { access, mkdtemp, readFile, rm, symlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import {
  ASSET_NAME,
  ASSET_SHA256,
  PACKAGE_NAME,
  RELEASE_URL,
  VERSION,
  parseArguments,
  sha256,
} from "../bin/airlock.js";
import {
  PLATFORM_TARGETS,
  UnreleasedPlatformError,
  UnsupportedPlatformError,
  getPlatformContract,
  releaseAssetURL,
  resolveReleasedArtifact,
} from "../lib/platform.mjs";

const packageDirectory = resolve(fileURLToPath(new URL("..", import.meta.url)));
const cliPath = resolve(packageDirectory, "bin/airlock.js");
const assetPath = resolve(packageDirectory, "dist", ASSET_NAME);
const logoPath = resolve(packageDirectory, "assets", "airlock-logo.svg");
const sourceOnly = process.env.AIRLOCK_SOURCE_TEST === "1";
const runInstallerIntegration = process.env.AIRLOCK_INSTALLER_INTEGRATION_TEST === "1";

test("parses install options", () => {
  assert.deepEqual(parseArguments(["install", "--output", "/tmp/airlock", "--open"]), {
    command: "install",
    output: "/tmp/airlock",
    force: false,
    open: true,
    json: false,
  });
});

test("rejects unknown options", () => {
  assert.throws(() => parseArguments(["install", "--unknown"]), /Unknown option/);
});

test("prints the package version", () => {
  const output = execFileSync(process.execPath, [cliPath, "version"], {
    encoding: "utf8",
  });
  assert.equal(output.trim(), `${PACKAGE_NAME} v${VERSION}`);
});

test("prints the official release URL", () => {
  const output = execFileSync(process.execPath, [cliPath, "release"], {
    encoding: "utf8",
  });
  assert.equal(output.trim(), RELEASE_URL);
  assert.equal(RELEASE_URL, `https://github.com/LouisonH/airlock-relay/releases/tag/v${VERSION}`);
});

test("runs through the symlink shape created for npm bins", async () => {
  const temporaryDirectory = await mkdtemp(
    resolve(tmpdir(), "airlock-npm-bin-test-"),
  );
  const linkedCli = resolve(temporaryDirectory, "airlock");
  try {
    await symlink(cliPath, linkedCli);
    const output = execFileSync(linkedCli, ["version"], { encoding: "utf8" });
    assert.equal(output.trim(), `${PACKAGE_NAME} v${VERSION}`);
  } finally {
    await rm(temporaryDirectory, { recursive: true, force: true });
  }
});

test("ships the verified release artifact", { skip: sourceOnly }, async () => {
  assert.equal(await sha256(assetPath), ASSET_SHA256);
});

test("ships the Airlock package icon", async () => {
  await access(logoPath);
  assert.match(await readFile(logoPath, "utf8"), /<title id="title">Airlock logo<\/title>/);
});

test("platform resolver releases only verified targets", () => {
  assert.equal(resolveReleasedArtifact("darwin", "arm64").artifactName, ASSET_NAME);
  assert.equal(PLATFORM_TARGETS.filter((target) => target.status === "released").length, 7);
  assert.equal(resolveReleasedArtifact("win32", "x64").artifactName, "Airlock_0.1.6_x64-setup.exe");
  assert.equal(resolveReleasedArtifact("linux", "x64").installType, "linux-appimage");
  assert.equal(getPlatformContract("win32", "arm64").status, "released");
  assert.equal(getPlatformContract("win32", "ia32").status, "released");
  assert.equal(getPlatformContract("linux", "x64").status, "released");
  assert.equal(getPlatformContract("linux", "arm64").status, "released");
  assert.equal(getPlatformContract("darwin", "x64").status, "released");
  assert.equal(getPlatformContract("linux", "ia32").status, "planned");
  assert.equal(getPlatformContract("win32", "arm64").installerAvailable, true);
  assert.equal(getPlatformContract("linux", "arm").status, "planned");
  assert.throws(
    () => resolveReleasedArtifact("freebsd", "x64"),
    UnsupportedPlatformError,
  );
});

test("builds pinned release asset URLs for every released target", () => {
  const target = resolveReleasedArtifact("linux", "arm64");
  assert.equal(
    releaseAssetURL(target),
    `https://github.com/LouisonH/airlock-relay/releases/download/v${VERSION}/${target.artifactName}`,
  );
  assert.throws(() => releaseAssetURL(getPlatformContract("linux", "arm")), UnreleasedPlatformError);
});

test("reports platform contracts for released targets", () => {
  const output = execFileSync(process.execPath, [cliPath, "platform", "--json"], { encoding: "utf8" });
  const report = JSON.parse(output);
  assert.equal(report.version, VERSION);
  assert.equal(report.targets.find((target) => target.platform === "darwin" && target.arch === "arm64").status, "released");
  assert.equal(report.targets.find((target) => target.platform === "win32" && target.arch === "x64").status, "released");
  assert.equal(report.targets.find((target) => target.platform === "win32" && target.arch === "arm64").status, "released");
  assert.equal(report.targets.find((target) => target.platform === "win32" && target.arch === "ia32").status, "released");
  assert.equal(report.targets.find((target) => target.platform === "linux" && target.arch === "x64").status, "released");
  assert.equal(report.targets.find((target) => target.platform === "linux" && target.arch === "ia32").status, "planned");
  assert.equal(report.targets.find((target) => target.platform === "linux" && target.arch === "arm").label, "Linux / ARMv7 (Raspberry Pi)");
});

test("reports the current platform contract without opening the application", () => {
  const output = execFileSync(process.execPath, [cliPath, "status", "--json"], { encoding: "utf8" });
  const report = JSON.parse(output);
  assert.equal(report.package, PACKAGE_NAME);
  assert.equal(report.currentTarget.platform, process.platform);
  assert.equal(report.currentTarget.arch, process.arch);
  assert.equal(report.installerAvailable, getPlatformContract().status === "released");
  assert.equal(report.currentTarget.installerAvailable, report.installerAvailable);
});

test("package metadata allows npm to install the diagnostic CLI on every supported OS", async () => {
  const manifest = JSON.parse(await readFile(resolve(packageDirectory, "package.json"), "utf8"));
  assert.equal(manifest.os, undefined);
  assert.equal(manifest.cpu, undefined);
});

test("installs the verified app bundle without opening it", { skip: sourceOnly || !runInstallerIntegration }, async () => {
  const outputDirectory = await mkdtemp(resolve(tmpdir(), "airlock-npm-test-"));
  try {
    const output = execFileSync(
      process.execPath,
      [cliPath, "install", "--output", outputDirectory],
      { encoding: "utf8" },
    );
    assert.match(output, /Installed Airlock/);
    await access(resolve(outputDirectory, "Airlock.app", "Contents", "MacOS", "airlock-desktop"));
    await access(resolve(outputDirectory, "Airlock.app", "Contents", "Resources", "airlockd"));
    const update = execFileSync(
      process.execPath,
      [cliPath, "install", "--output", outputDirectory],
      { encoding: "utf8" },
    );
    assert.match(update, /Updating installed Airlock/);
    assert.match(update, /Installed Airlock/);
  } finally {
    await rm(outputDirectory, { recursive: true, force: true });
  }
});
