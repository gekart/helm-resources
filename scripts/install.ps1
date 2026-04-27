$ErrorActionPreference = "Stop"

switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $arch = "amd64" }
    "ARM64" { $arch = "arm64" }
    default { throw "helm-resources: unsupported arch '$($env:PROCESSOR_ARCHITECTURE)'" }
}

$pluginDir = $env:HELM_PLUGIN_DIR
if (-not $pluginDir) { $pluginDir = Split-Path -Parent $PSScriptRoot }

$version = (Select-String -Path (Join-Path $pluginDir "plugin.yaml") -Pattern '^version:\s*(.+)$').Matches[0].Groups[1].Value.Trim('"', ' ')
$asset = "helm-resources-windows-$arch.exe"
$binDir = Join-Path $pluginDir "bin"
$out = Join-Path $binDir "helm-resources.exe"

New-Item -ItemType Directory -Force -Path $binDir | Out-Null

$staged = Join-Path $binDir $asset
if (Test-Path $staged) {
    Move-Item -Force $staged $out
    Get-ChildItem $binDir -Filter "helm-resources-*" | Remove-Item -Force
    exit 0
}

$url = "https://github.com/gekart/helm-resources/releases/download/v$version/$asset"
Write-Host "helm-resources: downloading $url"
Invoke-WebRequest -Uri $url -OutFile $out -UseBasicParsing
