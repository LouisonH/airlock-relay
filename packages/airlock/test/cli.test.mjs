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
  VERSION,
  parseArguments,
  sha256,
} from "../bin/airlock.js";
import {
  PLATFORM_TARGETS,
  UnreleasedPlatformError,
  UnsupportedPlatformError,
  resolveReleasedArtifact,
} from "../lib/platform.mjs";

const packageDirectory = resolve(fileURLToPath(new URL("..", import.meta.url)));
const cliPath = resolve(packageDirectory, "bin/airlock.js");
const assetPath = resolve(packageDirectory, "dist", ASSET_NAME);
const logoPath = resolve(packageDirectory, "assets", "airlock-logo.svg");

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

test("ships the verified release artifact", async () => {
  assert.equal(await sha256(assetPath), ASSET_SHA256);
});

test("ships the Airlock package icon", async () => {
  await access(logoPath);
  assert.match(await readFile(logoPath, "utf8"), /<title id="title">Airlock logo<\/title>/);
});

test("platform resolver releases only verified targets", () => {
  assert.equal(resolveReleasedArtifact("darwin", "arm64").artifactName, ASSET_NAME);
  assert.equal(PLATFORM_TARGETS.filter((target) => target.status === "released").length, 1);
  assert.throws(
    () => resolveReleasedArtifact("win32", "x64"),
    UnreleasedPlatformError,
  );
  assert.throws(
    () => resolveReleasedArtifact("freebsd", "x64"),
    UnsupportedPlatformError,
  );
});

test("reports platform contracts without claiming planned targets are released", () => {
  const output = execFileSync(process.execPath, [cliPath, "platform", "--json"], { encoding: "utf8" });
  const report = JSON.parse(output);
  assert.equal(report.version, VERSION);
  assert.equal(report.targets.find((target) => target.platform === "darwin" && target.arch === "arm64").status, "released");
  assert.equal(report.targets.find((target) => target.platform === "win32" && target.arch === "x64").status, "planned");
});

test("reports a local installer status without opening the application", () => {
  const output = execFileSync(process.execPath, [cliPath, "status", "--json"], { encoding: "utf8" });
  const report = JSON.parse(output);
  assert.equal(report.package, PACKAGE_NAME);
  assert.equal(report.currentTarget.artifactName, ASSET_NAME);
});

test("installs and verifies the DMG without opening it", async () => {
  const outputDirectory = await mkdtemp(resolve(tmpdir(), "airlock-npm-test-"));
  try {
    const output = execFileSync(
      process.execPath,
      [cliPath, "install", "--output", outputDirectory],
      { encoding: "utf8" },
    );
    assert.match(output, /Installed and verified/);
    assert.equal(
      await sha256(resolve(outputDirectory, ASSET_NAME)),
      ASSET_SHA256,
    );
  } finally {
    await rm(outputDirectory, { recursive: true, force: true });
  }
});
