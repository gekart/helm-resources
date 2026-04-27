#!/usr/bin/env sh
set -eu

case "$(uname -s)" in
    Linux*)  os=linux ;;
    Darwin*) os=darwin ;;
    *) echo "helm-resources: unsupported OS '$(uname -s)'" >&2; exit 1 ;;
esac

case "$(uname -m)" in
    x86_64|amd64)   arch=amd64 ;;
    arm64|aarch64)  arch=arm64 ;;
    *) echo "helm-resources: unsupported arch '$(uname -m)'" >&2; exit 1 ;;
esac

plugin_dir="${HELM_PLUGIN_DIR:-$(cd "$(dirname "$0")/.." && pwd)}"
version=$(awk '$1 == "version:" { print $2; exit }' "$plugin_dir/plugin.yaml" | tr -d '"')
asset="helm-resources-${os}-${arch}"
out="$plugin_dir/bin/helm-resources"

mkdir -p "$plugin_dir/bin"

# Tarball installs already ship per-arch binaries; reuse one if present.
if [ -f "$plugin_dir/bin/$asset" ]; then
    mv "$plugin_dir/bin/$asset" "$out"
    chmod +x "$out"
    rm -f "$plugin_dir/bin/helm-resources-"*
    exit 0
fi

url="https://github.com/gekart/helm-resources/releases/download/v${version}/${asset}"
echo "helm-resources: downloading ${url}" >&2

if command -v curl >/dev/null 2>&1; then
    curl --fail --location --silent --show-error -o "$out" "$url"
elif command -v wget >/dev/null 2>&1; then
    wget --quiet -O "$out" "$url"
else
    echo "helm-resources: need curl or wget on PATH to download the binary" >&2
    exit 1
fi

chmod +x "$out"
