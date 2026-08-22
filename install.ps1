# ForgeGuardian installer for Windows
# Usage: irm https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.ps1 | iex
#
# Environment variables (optional):
#   $env:FORGEGUARDIAN_VERSION  — version to install (default: latest)
#   $env:INSTALL_DIR            — install directory (default: ~\.local\bin)

$ErrorActionPreference = 'Stop'

$repo = 'Mah3Sec/ForgeGuardian'
$version = if ($env:FORGEGUARDIAN_VERSION) { $env:FORGEGUARDIAN_VERSION } else { 'latest' }
$installDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $HOME '.local\bin' }
$binaries = @('fgctl.exe', 'fg-agent.exe', 'intel-agent.exe')

function Write-Step { param($msg) Write-Host "  > $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "  ! $msg" -ForegroundColor Yellow }

Write-Host ''
Write-Host '  ForgeGuardian Installer (Windows)' -ForegroundColor Cyan
Write-Host '  ──────────────────────────────────' -ForegroundColor DarkGray
Write-Host ''

# Resolve latest version
if ($version -eq 'latest') {
    Write-Step 'Resolving latest version...'
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing
    $version = $release.tag_name
}
$version = $version -replace '^v', ''
Write-Step "Version: $version"

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) { 'amd64' } else { '386' }
$asset = "forgeguardian_${version}_windows_${arch}.zip"
$url = "https://github.com/$repo/releases/download/v${version}/$asset"

# Download
$tmpDir = Join-Path ([IO.Path]::GetTempPath()) "forgeguardian-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
$zipPath = Join-Path $tmpDir $asset

Write-Step "Downloading $asset..."
Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

# Extract
Write-Step 'Extracting...'
$extractDir = Join-Path $tmpDir 'extracted'
Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

# Install
New-Item -ItemType Directory -Path $installDir -Force | Out-Null
foreach ($bin in $binaries) {
    $src = Join-Path $extractDir $bin
    if (Test-Path $src) {
        Copy-Item $src (Join-Path $installDir $bin) -Force
        Write-Step "Installed $bin -> $installDir"
    }
}

# Clean up
Remove-Item -Recurse -Force $tmpDir

# Check PATH
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$installDir*") {
    Write-Warn "$installDir is not in your PATH."
    $add = Read-Host "  Add it now? [Y/n]"
    if ($add -ne 'n') {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
        $env:Path = "$env:Path;$installDir"
        Write-Step 'Added to PATH (restart your terminal for it to take effect).'
    } else {
        Write-Warn "Add this to your PATH manually:"
        Write-Host "  `$env:Path += `";$installDir`"" -ForegroundColor Gray
    }
}

Write-Host ''
Write-Host "  ForgeGuardian v$version installed." -ForegroundColor Green
Write-Host ''
Write-Host '  Quick start:' -ForegroundColor Cyan
Write-Host "    fgctl scan .                          # scan your project" -ForegroundColor Gray
Write-Host "    fgctl scan npm/lodash@4.17.21         # scan a package" -ForegroundColor Gray
Write-Host "    fgctl doctor                          # check your setup" -ForegroundColor Gray
Write-Host ''
