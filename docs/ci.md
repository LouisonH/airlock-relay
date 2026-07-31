# CI verification

The repository workflow at `.github/workflows/verify.yml` runs on pull requests and protected branches. It is intentionally a verification workflow, not a deployment workflow.

## What it checks

- Go formatting and the full Go test suite.
- The Desktop TypeScript and Vite production build.
- The npm installer tests and package manifest.
- JavaScript syntax for the static project site and documentation guide.

## What it must not receive

Pull requests and ordinary verification jobs do not need and must not receive:

- npm publishing tokens;
- GitHub Pages or release deployment tokens;
- Airlock control tokens or Web UI tokens;
- route JSON specifications, upstream URLs, passwords, or API keys;
- generated local capabilities or secondary API keys.

## Publishing

Publish from a protected branch only after a human has verified the release artifacts and their checksums. Use a dedicated least-privilege npm token. The package `prepack` hook stages a DMG only after its SHA-256 matches the released artifact definition; it will fail closed if that release input is absent or mismatched.

Do not create production Airlock routes in CI. A route specification and the one-time local credential generated when it is created are sensitive data.
