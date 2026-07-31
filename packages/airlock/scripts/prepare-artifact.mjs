import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { access, copyFile, mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { resolveReleasedArtifact } from "../lib/platform.mjs";

const { artifactName: assetName, sha256: expectedSha256 } = resolveReleasedArtifact(
  "darwin",
  "arm64",
);
const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const packageDirectory = resolve(scriptDirectory, "..");
const repositoryDirectory = resolve(packageDirectory, "../..");
const source = resolve(repositoryDirectory, "release", assetName);
const destinationDirectory = resolve(packageDirectory, "dist");
const destination = resolve(destinationDirectory, assetName);

async function sha256(filePath) {
  await access(filePath);
  return new Promise((resolveDigest, rejectDigest) => {
    const hash = createHash("sha256");
    const stream = createReadStream(filePath);
    stream.on("error", rejectDigest);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("end", () => resolveDigest(hash.digest("hex")));
  });
}

async function verify(filePath) {
  const digest = await sha256(filePath);
  if (digest !== expectedSha256) {
    throw new Error(
      `Refusing to stage ${filePath}: expected ${expectedSha256}, received ${digest}`,
    );
  }
}

try {
  await verify(source);
} catch (error) {
  throw new Error(
    `The verified v0.1.1 DMG must exist at ${source} before packing. ${error.message}`,
  );
}

await mkdir(destinationDirectory, { recursive: true });
await copyFile(source, destination);
await verify(destination);
console.log(`Staged and verified ${assetName}`);
