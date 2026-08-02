#!/usr/bin/env bash
set -euo pipefail

# Build the Airlock desktop bundle for 32-bit Raspberry Pi OS (armv7/armhf).
#
# Run this script ON the Raspberry Pi itself. GitHub-hosted runners do not
# provide armv7 machines, and cross-compiling Tauri/WebKitGTK to armv7 from
# another host is not a supported path. The Pi needs:
#   - 32-bit Raspberry Pi OS (Bullseye or newer)
#   - at least 4 GB of RAM (2 GB works with swap)
#   - Node.js 20+, Rust (rustup stable), and the Tauri Linux dependencies:
#       sudo apt-get install -y libwebkit2gtk-4.1-dev libgtk-3-dev \
#         libayatana-appindicator3-dev librsvg2-dev libdbus-1-dev pkg-config
#
# Usage:
#   bash scripts/build-armv7-desktop.sh [--upload]
#
# Without --upload the artifacts are left in apps/desktop/src-tauri/target/
# armv7-unknown-linux-gnueabihf/release/bundle/. With --upload they are
# published to the matching GitHub Release using `gh` (authenticated on the Pi).

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

case "$(uname -m)" in
  armv7l|armhf)
    ;;
  *)
    echo "This script must run on a 32-bit ARM (armv7l) system, got $(uname -m)." >&2
    exit 1
    ;;
esac

if ! command -v node >/dev/null || ! command -v cargo >/dev/null; then
  echo "Node.js and Rust are required. See the header comment in this script." >&2
  exit 1
fi

version="$(node -p "require('./apps/desktop/package.json').version")"
triple="armv7-unknown-linux-gnueabihf"

echo "Building Airlock v${version} desktop bundle for ${triple}"

if ! rustup target list --installed 2>/dev/null | grep -q "${triple}"; then
  rustup target add "${triple}"
fi

mkdir -p apps/desktop/src-tauri/binaries
AIRLOCK_OUTPUT_DIR="$PWD/apps/desktop/src-tauri/binaries" \
  node scripts/build-sidecar.mjs linux-armv7

cd apps/desktop
npm ci
npm run tauri build -- --target "${triple}" --bundles deb,appimage
cd "$repo_root"

bundle_dir="apps/desktop/src-tauri/target/${triple}/release/bundle"
echo "Built artifacts:"
find "${bundle_dir}" -maxdepth 2 -type f \( -name '*.deb' -o -name '*.AppImage' \) -print

if [[ "${1:-}" == "--upload" ]]; then
  tag="v${version}"
  for artifact in "${bundle_dir}"/deb/*.deb "${bundle_dir}"/appimage/*.AppImage; do
    [ -f "$artifact" ] || continue
    echo "Uploading ${artifact} to ${tag}"
    gh release upload "${tag}" "${artifact}" --clobber
  done
  echo "Artifacts uploaded. The maintainer can now pin SHA-256 checksums and"
  echo "mark the linux/armv7 target as released in the npm installer contract."
fi
