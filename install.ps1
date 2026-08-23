# ForgeGuardian Windows Installer
# Bootstrapper — downloads the universal install.sh and runs the embedded PowerShell section.
# All logic lives in install.sh (single source of truth for all platforms).
#
# Usage:   irm https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.ps1 | iex

$ErrorActionPreference = 'Stop'
$raw = Invoke-RestMethod 'https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh'
if ($raw -match "(?s): <<'PS_BLOCK_END'\r?\n(.+?)\r?\nPS_BLOCK_END") {
    Invoke-Expression $Matches[1]
} else {
    Write-Host 'Failed to extract installer. Use Git Bash: curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash' -ForegroundColor Red
}
