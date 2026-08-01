#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const desktopDir = join(repoRoot, "apps", "desktop", "src-tauri");
const targets = {
  "darwin-arm64": { goos: "darwin", goarch: "arm64", extension: "", rustTriple: "aarch64-apple-darwin" },
  "darwin-amd64": { goos: "darwin", goarch: "amd64", extension: "", rustTriple: "x86_64-apple-darwin" },
  "windows-amd64": { goos: "windows", goarch: "amd64", extension: ".exe", rustTriple: "x86_64-pc-windows-msvc" },
  "windows-arm64": { goos: "windows", goarch: "arm64", extension: ".exe", rustTriple: "aarch64-pc-windows-msvc" },
  "linux-amd64": { goos: "linux", goarch: "amd64", extension: "", rustTriple: "x86_64-unknown-linux-gnu" },
  "linux-arm64": { goos: "linux", goarch: "arm64", extension: "", rustTriple: "aarch64-unknown-linux-gnu" },
  "linux-armv7": { goos: "linux", goarch: "arm", goarm: "7", extension: "", rustTriple: "armv7-unknown-linux-gnueabihf" },
};

function detectTarget() {
  const platform = process.platform;
  const arch = process.arch;
  if (platform === "darwin" && arch === "arm64") return "darwin-arm64";
  if (platform === "darwin" && arch === "x64") return "darwin-amd64";
  if (platform === "win32" && arch === "x64") return "windows-amd64";
  if (platform === "win32" && arch === "arm64") return "windows-arm64";
  if (platform === "linux" && arch === "x64") return "linux-amd64";
  if (platform === "linux" && arch === "arm64") return "linux-arm64";
  if (platform === "linux" && arch === "arm") return "linux-armv7";
  throw new Error(`Unsupported host ${platform}/${arch}; set AIRLOCK_TARGET or pass a target argument.`);
}

const requested = process.argv[2] || process.env.AIRLOCK_TARGET || "";
const targetName = requested || detectTarget();
const target = targets[targetName];
if (!target) {
  throw new Error(`Unsupported Airlock target: ${targetName}`);
}

const explicit = Boolean(requested);
const outputDir = process.env.AIRLOCK_OUTPUT_DIR
  ? resolve(process.env.AIRLOCK_OUTPUT_DIR)
  : explicit
    ? join(repoRoot, "bin", targetName)
    : join(desktopDir, "binaries");
mkdirSync(outputDir, { recursive: true });

const version = JSON.parse(
  readFileSync(join(desktopDir, "..", "package.json"), "utf8"),
).version;

const env = {
  ...process.env,
  GOOS: target.goos,
  GOARCH: target.goarch,
  CGO_ENABLED: target.goos === "darwin" ? process.env.CGO_ENABLED ?? "1" : "0",
};
if (target.goarm) env.GOARM = target.goarm;
if (target.goos === "darwin" && !env.MACOSX_DEPLOYMENT_TARGET) {
  env.MACOSX_DEPLOYMENT_TARGET = "12.0";
}

for (const binary of ["airlockd", "airlock"]) {
  const bundleName = outputDir === join(desktopDir, "binaries")
    ? `${binary}-${target.rustTriple}${target.extension}`
    : `${binary}${target.extension}`;
  const result = spawnSync(
    "go",
    [
      "build",
      "-buildvcs=false",
      "-trimpath",
      `-ldflags=-s -w -X main.version=${version}`,
      "-o",
      join(outputDir, bundleName),
      `./cmd/${binary}`,
    ],
    { cwd: repoRoot, env, stdio: "inherit" },
  );
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

console.log(`Built ${targetName} sidecars into ${outputDir}`);
