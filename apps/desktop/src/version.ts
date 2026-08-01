export const APP_VERSION = "0.1.5-328";
export const RELEASES_URL = "https://github.com/LouisonH/airlock-relay/releases";

const latestReleaseEndpoint = "https://api.github.com/repos/LouisonH/airlock-relay/releases/latest";

export type UpdateCheckStatus = "current" | "available" | "unavailable";

export interface UpdateCheckResult {
  current: string;
  status: UpdateCheckStatus;
  latest?: string;
  releaseUrl: string;
}

type ReleaseResponse = { tag_name?: unknown; draft?: unknown; prerelease?: unknown };

function parseVersion(value: string): number[] | undefined {
  const match = value.trim().match(/^v?(\d+)\.(\d+)\.(\d+)$/);
  if (!match) return undefined;
  return match.slice(1).map((part) => Number(part));
}

export function compareVersions(left: string, right: string): number | undefined {
  const parsedLeft = parseVersion(left);
  const parsedRight = parseVersion(right);
  if (!parsedLeft || !parsedRight) return undefined;
  for (let index = 0; index < parsedLeft.length; index += 1) {
    if (parsedLeft[index] !== parsedRight[index]) return parsedLeft[index] > parsedRight[index] ? 1 : -1;
  }
  return 0;
}

export async function checkForUpdates(fetcher: typeof fetch = fetch): Promise<UpdateCheckResult> {
  try {
    const response = await fetcher(latestReleaseEndpoint, {
      method: "GET",
      cache: "no-store",
      headers: {
        Accept: "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
      },
    });
    if (!response.ok) throw new Error("release metadata unavailable");
    const release = await response.json() as ReleaseResponse;
    if (release.draft === true || release.prerelease === true || typeof release.tag_name !== "string") throw new Error("invalid release metadata");
    const comparison = compareVersions(release.tag_name, APP_VERSION);
    if (comparison === undefined) throw new Error("invalid release version");
    const latest = release.tag_name.replace(/^v/, "");
    return {
      current: APP_VERSION,
      latest,
      status: comparison > 0 ? "available" : "current",
      releaseUrl: RELEASES_URL,
    };
  } catch {
    return { current: APP_VERSION, status: "unavailable", releaseUrl: RELEASES_URL };
  }
}
