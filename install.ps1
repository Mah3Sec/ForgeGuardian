# ForgeGuardian — Complete installer for Windows
#
# One command installs everything:
#   irm https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.ps1 | iex
#
# What it does:
#   1. Installs fgctl + companion binaries
#   2. Installs the pre-built web dashboard
#   3. Installs scanner engines (grype, trivy, semgrep)
#   4. Downloads community threat signatures
#   5. Creates default configuration
#   6. Ready to run: fgctl serve

$ErrorActionPreference = 'Stop'

$repo = 'Mah3Sec/ForgeGuardian'
$version = if ($env:FORGEGUARDIAN_VERSION) { $env:FORGEGUARDIAN_VERSION } else { 'latest' }
$installDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $HOME '.local\bin' }
$dataDir = Join-Path $HOME '.forgeguardian'
$binaries = @('fgctl.exe', 'fg-agent.exe', 'intel-agent.exe')
$skipEngines = $env:SKIP_ENGINES -eq '1'

function Write-Step { param($n, $msg) Write-Host "`n  [$n/6] $msg" -ForegroundColor Cyan }
function Write-Ok { param($msg) Write-Host "  > $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "  ! $msg" -ForegroundColor Yellow }
function Has-Command { param($cmd) return [bool](Get-Command $cmd -ErrorAction SilentlyContinue) }

Write-Host ''
Write-Host '  +==========================================+' -ForegroundColor White
Write-Host '  |    ForgeGuardian - Full Installer         |' -ForegroundColor White
Write-Host '  +==========================================+' -ForegroundColor White
Write-Host "  Platform: windows/$([Environment]::Is64BitOperatingSystem ? 'amd64' : 'x86')" -ForegroundColor DarkGray
Write-Host ''

# ── Step 1: Download + install ForgeGuardian binaries ────────────────────────
Write-Step 1 'Installing ForgeGuardian'

if ($version -eq 'latest') {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing
    $version = $release.tag_name
}
$versionBare = $version -replace '^v', ''
$baseUrl = "https://github.com/$repo/releases/download/$version"
Write-Ok "Version: $version"

$arch = if ([Environment]::Is64BitOperatingSystem) { 'amd64' } else { '386' }
$asset = "forgeguardian_${versionBare}_windows_${arch}.zip"

$tmpDir = Join-Path ([IO.Path]::GetTempPath()) "forgeguardian-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

Write-Ok "Downloading $asset..."
Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile (Join-Path $tmpDir $asset) -UseBasicParsing

$extractDir = Join-Path $tmpDir 'extracted'
Expand-Archive -Path (Join-Path $tmpDir $asset) -DestinationPath $extractDir -Force

New-Item -ItemType Directory -Path $installDir -Force | Out-Null
foreach ($bin in $binaries) {
    $src = Join-Path $extractDir $bin
    if (Test-Path $src) {
        Copy-Item $src (Join-Path $installDir $bin) -Force
    }
}
Write-Ok "Installed fgctl -> $installDir"

# ── Step 2: Install dashboard ────────────────────────────────────────────────
Write-Step 2 'Installing dashboard'

$dashDir = Join-Path $dataDir 'dashboard'
$dashTar = Join-Path $tmpDir 'dashboard-dist.tar.gz'

try {
    Invoke-WebRequest -Uri "$baseUrl/dashboard-dist.tar.gz" -OutFile $dashTar -UseBasicParsing
    if (Test-Path $dashDir) { Remove-Item -Recurse -Force $dashDir }
    New-Item -ItemType Directory -Path $dashDir -Force | Out-Null
    tar -xzf $dashTar -C $dashDir 2>$null
    Write-Ok "Dashboard -> $dashDir"
} catch {
    Write-Warn 'Dashboard not found in release - will be available in next tagged release'
}

# ── Step 3: Install scanner engines ──────────────────────────────────────────
Write-Step 3 'Installing scanner engines'

$enginesInstalled = 0

if (-not $skipEngines) {
    # grype
    if (Has-Command 'grype') {
        Write-Ok 'grype: already installed'
        $enginesInstalled++
    } else {
        Write-Host '    Installing grype...' -ForegroundColor DarkGray
        try {
            $grypeUrl = "https://raw.githubusercontent.com/anchore/grype/main/install.sh"
            $grypeScript = Join-Path $tmpDir 'install-grype.sh'
            Invoke-WebRequest -Uri $grypeUrl -OutFile $grypeScript -UseBasicParsing
            bash $grypeScript -b $installDir 2>$null
            if ($?) { Write-Ok "grype: installed -> $installDir"; $enginesInstalled++ }
            else { throw 'bash install failed' }
        } catch {
            try {
                if (Has-Command 'scoop') { scoop install grype 2>$null; Write-Ok 'grype: installed via scoop'; $enginesInstalled++ }
                elseif (Has-Command 'choco') { choco install grype -y 2>$null; Write-Ok 'grype: installed via choco'; $enginesInstalled++ }
                else { Write-Warn 'grype: install failed - install manually: https://github.com/anchore/grype' }
            } catch {
                Write-Warn 'grype: install failed - install manually: https://github.com/anchore/grype'
            }
        }
    }

    # trivy
    if (Has-Command 'trivy') {
        Write-Ok 'trivy: already installed'
        $enginesInstalled++
    } else {
        Write-Host '    Installing trivy...' -ForegroundColor DarkGray
        try {
            if (Has-Command 'scoop') { scoop install trivy 2>$null; Write-Ok 'trivy: installed via scoop'; $enginesInstalled++ }
            elseif (Has-Command 'choco') { choco install trivy -y 2>$null; Write-Ok 'trivy: installed via choco'; $enginesInstalled++ }
            else { Write-Warn 'trivy: install failed - install manually: https://github.com/aquasecurity/trivy' }
        } catch {
            Write-Warn 'trivy: install failed - install manually: https://github.com/aquasecurity/trivy'
        }
    }

    # semgrep
    if (Has-Command 'semgrep') {
        Write-Ok 'semgrep: already installed'
        $enginesInstalled++
    } else {
        Write-Host '    Installing semgrep...' -ForegroundColor DarkGray
        try {
            if (Has-Command 'pip3') { pip3 install semgrep --quiet 2>$null; if ($?) { Write-Ok 'semgrep: installed via pip3'; $enginesInstalled++ } else { throw 'pip3 failed' } }
            elseif (Has-Command 'pip') { pip install semgrep --quiet 2>$null; if ($?) { Write-Ok 'semgrep: installed via pip'; $enginesInstalled++ } else { throw 'pip failed' } }
            elseif (Has-Command 'pipx') { pipx install semgrep 2>$null; if ($?) { Write-Ok 'semgrep: installed via pipx'; $enginesInstalled++ } else { throw 'pipx failed' } }
            else { Write-Warn 'semgrep: install failed - install manually: pip install semgrep' }
        } catch {
            Write-Warn 'semgrep: install failed - install manually: pip install semgrep'
        }
    }

    Write-Ok "$enginesInstalled/3 scanner engines ready"
} else {
    Write-Warn 'Skipping scanner engines (SKIP_ENGINES=1)'
}

# ── Step 4: PATH check + signatures ─────────────────────────────────────────
Write-Step 4 'Downloading threat signatures'

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
    $env:Path = "$env:Path;$installDir"
    Write-Ok 'Added install directory to PATH'
}

$fgctl = Join-Path $installDir 'fgctl.exe'
if (Test-Path $fgctl) {
    try {
        & $fgctl update 2>$null
        Write-Ok 'Community signatures downloaded'
    } catch {
        Write-Warn 'Signature download failed - run "fgctl update" later'
    }
}

# ── Step 5: Create default configuration ─────────────────────────────────────
Write-Step 5 'Setting up configuration'

New-Item -ItemType Directory -Path $dataDir -Force | Out-Null

$cfgFile = Join-Path $dataDir 'config.yaml'
if (-not (Test-Path $cfgFile)) {
    @"
default_ecosystem: auto
output_format: text
verbose: false
engines:
  osv: true
  grype: true
  trivy: true
  semgrep: true
  behavioral: true
  malware_pattern: true
  typosquat: true
  ai_model: true
  mcp_injection: true
"@ | Set-Content $cfgFile -Encoding utf8
    Write-Ok "Config created -> $cfgFile"
} else {
    Write-Ok "Config exists -> $cfgFile"
}

# ── Step 6: Verify installation ──────────────────────────────────────────────
Write-Step 6 'Verifying installation'

Write-Host ''
$checkPass = 0; $checkTotal = 0

foreach ($item in @('fgctl', 'grype', 'trivy', 'semgrep')) {
    $checkTotal++
    if (Has-Command $item) {
        Write-Ok "${item}: $((Get-Command $item).Source)"
        $checkPass++
    } else {
        Write-Warn "${item}: not found"
    }
}

$checkTotal++
$dashIndex = Join-Path $dashDir 'index.html'
if (Test-Path $dashIndex) {
    Write-Ok 'dashboard: installed'
    $checkPass++
} else {
    Write-Warn 'dashboard: not installed'
}

$checkTotal++
$sigFile = Join-Path $dataDir 'signatures.json'
if (Test-Path $sigFile) {
    Write-Ok 'signatures: loaded'
    $checkPass++
} else {
    Write-Warn 'signatures: not found'
}

# ── Clean up ─────────────────────────────────────────────────────────────────
Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue

# ── Done ─────────────────────────────────────────────────────────────────────
Write-Host ''
Write-Host '  ==========================================' -ForegroundColor White
if ($checkPass -eq $checkTotal) {
    Write-Host '    Installation complete - all checks passed' -ForegroundColor Green
} else {
    Write-Host "    Installation complete ($checkPass/$checkTotal components)" -ForegroundColor Green
}
Write-Host '  ==========================================' -ForegroundColor White
Write-Host ''
Write-Host '  Start the platform:' -ForegroundColor Cyan
Write-Host '    fgctl serve' -ForegroundColor Green
Write-Host '    -> API + Dashboard at http://localhost:8080' -ForegroundColor DarkGray
Write-Host ''
Write-Host '  Scan a project:' -ForegroundColor Cyan
Write-Host '    fgctl scan .                        # scan current directory' -ForegroundColor DarkGray
Write-Host '    fgctl scan npm/lodash@4.17.21       # scan a specific package' -ForegroundColor DarkGray
Write-Host '    fgctl audit system                  # audit all installed packages' -ForegroundColor DarkGray
Write-Host ''
Write-Host '  Check setup:' -ForegroundColor Cyan
Write-Host '    fgctl doctor                        # verify environment' -ForegroundColor DarkGray
Write-Host ''
