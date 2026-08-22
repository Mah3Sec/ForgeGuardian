# ForgeGuardian — One-line installer for Windows
#
# Usage:
#   irm https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.ps1 | iex
#
# What it does:
#   1. Downloads fgctl + companion binaries
#   2. Installs the pre-built dashboard
#   3. Downloads community threat signatures
#   4. Ready to run: fgctl serve
#
# Environment variables (all optional):
#   $env:FORGEGUARDIAN_VERSION  — version to install (default: latest)
#   $env:INSTALL_DIR            — install directory (default: ~\.local\bin)

$ErrorActionPreference = 'Stop'

$repo = 'Mah3Sec/ForgeGuardian'
$version = if ($env:FORGEGUARDIAN_VERSION) { $env:FORGEGUARDIAN_VERSION } else { 'latest' }
$installDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $HOME '.local\bin' }
$dataDir = Join-Path $HOME '.forgeguardian'
$binaries = @('fgctl.exe', 'fg-agent.exe', 'intel-agent.exe')

function Write-Step { param($msg) Write-Host "  > $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "  ! $msg" -ForegroundColor Yellow }

Write-Host ''
Write-Host '  ForgeGuardian Installer (Windows)' -ForegroundColor Cyan
Write-Host '  ──────────────────────────────────' -ForegroundColor DarkGray
Write-Host ''

# ── Step 1: Resolve version ──────────────────────────────────────────────────
if ($version -eq 'latest') {
    Write-Step 'Resolving latest version...'
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing
    $version = $release.tag_name
}
$versionBare = $version -replace '^v', ''
$baseUrl = "https://github.com/$repo/releases/download/$version"
Write-Step "Version: $version"

# ── Step 2: Download + install binaries ──────────────────────────────────────
$arch = if ([Environment]::Is64BitOperatingSystem) { 'amd64' } else { '386' }
$asset = "forgeguardian_${versionBare}_windows_${arch}.zip"
$url = "$baseUrl/$asset"

$tmpDir = Join-Path ([IO.Path]::GetTempPath()) "forgeguardian-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
$zipPath = Join-Path $tmpDir $asset

Write-Step "Downloading $asset..."
Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

Write-Step 'Extracting...'
$extractDir = Join-Path $tmpDir 'extracted'
Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

New-Item -ItemType Directory -Path $installDir -Force | Out-Null
foreach ($bin in $binaries) {
    $src = Join-Path $extractDir $bin
    if (Test-Path $src) {
        Copy-Item $src (Join-Path $installDir $bin) -Force
    }
}
Write-Step "Binaries installed -> $installDir"

# ── Step 3: Download + install dashboard ─────────────────────────────────────
$dashDir = Join-Path $dataDir 'dashboard'
$dashUrl = "$baseUrl/dashboard-dist.tar.gz"
$dashTar = Join-Path $tmpDir 'dashboard-dist.tar.gz'

Write-Step 'Downloading dashboard...'
try {
    Invoke-WebRequest -Uri $dashUrl -OutFile $dashTar -UseBasicParsing
    if (Test-Path $dashDir) { Remove-Item -Recurse -Force $dashDir }
    New-Item -ItemType Directory -Path $dashDir -Force | Out-Null
    tar -xzf $dashTar -C $dashDir 2>$null
    Write-Step "Dashboard installed -> $dashDir"
} catch {
    Write-Warn 'Dashboard not available in this release - fgctl serve will run API-only'
}

# ── Step 4: Clean up temp files ──────────────────────────────────────────────
Remove-Item -Recurse -Force $tmpDir

# ── Step 5: PATH check ──────────────────────────────────────────────────────
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$installDir*") {
    Write-Warn "$installDir is not in your PATH."
    $add = Read-Host "  Add it now? [Y/n]"
    if ($add -ne 'n') {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
        $env:Path = "$env:Path;$installDir"
        Write-Step 'Added to PATH (restart your terminal for full effect).'
    }
}

# ── Step 6: Download threat signatures ───────────────────────────────────────
$fgctl = Join-Path $installDir 'fgctl.exe'
if (Test-Path $fgctl) {
    Write-Step 'Downloading community threat signatures...'
    try {
        & $fgctl update 2>$null
        Write-Step 'Signatures updated'
    } catch {
        Write-Warn 'Signature download failed - run "fgctl update" manually'
    }
}

# ── Done ─────────────────────────────────────────────────────────────────────
Write-Host ''
Write-Host '  Installation complete!' -ForegroundColor Green
Write-Host ''
Write-Host '  Start ForgeGuardian:' -ForegroundColor Cyan
Write-Host '    fgctl serve' -ForegroundColor Green
Write-Host '    -> Opens API + dashboard on http://localhost:8080' -ForegroundColor DarkGray
Write-Host ''
Write-Host '  Or scan right away:' -ForegroundColor Cyan
Write-Host '    fgctl scan .                        # scan current project' -ForegroundColor DarkGray
Write-Host '    fgctl scan npm/lodash@4.17.21       # scan a package' -ForegroundColor DarkGray
Write-Host '    fgctl audit system                  # audit all installed packages' -ForegroundColor DarkGray
Write-Host ''
