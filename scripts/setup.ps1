# =============================================================================
# setup.ps1 — Bootstrap dns-cli (requirements + bin/ + build)
# =============================================================================
# Repo-root helper for any dns-cli use. Does not scaffold demo/ (that is handled
# by `dns-cli demo run` via EnsureDemoLayout).
# From repo root: .\scripts\setup.ps1
# =============================================================================

[CmdletBinding()]
param(
    [switch] $Yes,
    [switch] $SkipBuild
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ScriptDir = $PSScriptRoot
$Root = Split-Path -Parent $ScriptDir
$BinDir = Join-Path $Root 'bin'
$BinExe = Join-Path $BinDir 'dns-cli.exe'
$MinGo = [version]'1.25.10'

function Test-AssumeYes {
    if ($Yes) { return $true }
    if ($env:ASSUME_YES -match '^(1|true|yes|on)$') { return $true }
    if ($env:DEMO_ASSUME_YES -match '^(1|true|yes|on)$') { return $true }
    return $false
}

function Get-GoVersion {
    $cmd = Get-Command go -ErrorAction SilentlyContinue
    if ($null -eq $cmd) {
        return $null
    }
    $out = & go version 2>&1
    if ($out -match 'go(\d+\.\d+(?:\.\d+)?)') {
        return [version]$Matches[1]
    }
    return $null
}

Write-Host "dns-cli setup (root: $Root)"

$goVer = Get-GoVersion
if ($null -eq $goVer) {
    Write-Host @"
Go is not on PATH (need >= $MinGo).

Install: https://go.dev/dl/
Then re-run: .\scripts\setup.ps1
"@
    exit 1
}
if ($goVer -lt $MinGo) {
    Write-Host "Go $goVer is below required $MinGo. Upgrade: https://go.dev/dl/"
    exit 1
}
Write-Host "Go $goVer OK"

$aiken = Get-Command aiken -ErrorAction SilentlyContinue
if ($null -eq $aiken) {
    Write-Host "Note: aiken not on PATH (needed for system prepare / demo fresh). See https://aiken-lang.org/installation-instructions"
} else {
    Write-Host "aiken found: $($aiken.Source)"
}

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
Write-Host "bin/ ready: $BinDir"

if ($SkipBuild) {
    Write-Host "SkipBuild set; not compiling."
    exit 0
}

Push-Location $Root
try {
    $commit = 'unknown'
    try { $commit = (git rev-parse --short HEAD 2>$null).Trim() } catch { }
    if ([string]::IsNullOrWhiteSpace($commit)) { $commit = 'unknown' }
    $built = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    $pkg = 'github.com/blinklabs-io/dns-cli/internal/cli'
    $ldflags = "-X $pkg.GitCommit=$commit -X $pkg.BuildDate=$built"
    Write-Host "Building -> $BinExe"
    Write-Host "ldflags: commit=$commit built=$built"
    go build -ldflags $ldflags -o $BinExe ./cmd/dns-cli
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed"
    }
}
finally {
    Pop-Location
}

& $BinExe version | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "built binary failed: version"
}

Write-Host @"

Next:
  $BinExe version
  $BinExe dashboard --config dns-cli.json
  $BinExe demo run
"@
