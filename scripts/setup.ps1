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
$MinGo = [version]'1.25.13'

function Test-AssumeYes {
    if ($Yes) { return $true }
    if ($env:ASSUME_YES -match '^(1|true|yes|on)$') { return $true }
    if ($env:DEMO_ASSUME_YES -match '^(1|true|yes|on)$') { return $true }
    return $false
}

function Get-GoExeVersion {
    param([Parameter(Mandatory)][string] $GoExe)
    # `go version` can emit toolchain-download progress to stderr. With
    # $ErrorActionPreference='Stop' that surfaces as NativeCommandError, so
    # relax it for this call only.
    $out = & {
        $ErrorActionPreference = 'Continue'
        & $GoExe version 2>&1
    } | Out-String
    if ($out -match 'go(\d+\.\d+(?:\.\d+)?)') {
        return [version]$Matches[1]
    }
    return $null
}

function Resolve-GoExe {
    # Prefer a local go-toolchains install over Program Files. On Windows the
    # System PATH entry (C:\Program Files\Go) wins over User PATH, so an older
    # system Go would otherwise keep downloading go.mod's toolchain pin and
    # fail with "toolchain not available" when the proxy/network blocks it.
    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        $candidates += (Join-Path $env:LOCALAPPDATA 'go-toolchains\go\bin\go.exe')
    }
    $repoTools = Join-Path (Split-Path $Root -Parent) '.tools\go\bin\go.exe'
    $candidates += $repoTools
    $pathGo = Get-Command go -ErrorAction SilentlyContinue
    if ($null -ne $pathGo) {
        $candidates += $pathGo.Source
    }

    foreach ($exe in $candidates) {
        if ([string]::IsNullOrWhiteSpace($exe)) { continue }
        if (-not (Test-Path -LiteralPath $exe)) { continue }
        $ver = Get-GoExeVersion -GoExe $exe
        if ($null -eq $ver) { continue }
        if ($ver -lt $MinGo) {
            Write-Host "Skipping $exe (Go $ver < $MinGo)"
            continue
        }
        return @{ Exe = $exe; Version = $ver }
    }
    return $null
}

Write-Host "dns-cli setup (root: $Root)"

$go = Resolve-GoExe
if ($null -eq $go) {
    Write-Host @"
Go >= $MinGo not found.

Install: https://go.dev/dl/
Or unpack to: $env:LOCALAPPDATA\go-toolchains\go
Then re-run: .\scripts\setup.ps1
"@
    exit 1
}

# Use the resolved binary for this session and pin GOTOOLCHAIN=local so go.mod's
# toolchain directive does not trigger a download of an already-installed version.
$env:PATH = "$(Split-Path -Parent $go.Exe);$env:PATH"
$env:GOTOOLCHAIN = 'local'
Write-Host "Go $($go.Version) OK ($($go.Exe))"

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

    $contracts = 'unknown'
    $sibling = Join-Path (Split-Path $Root -Parent) 'dns-contracts'
    if (Test-Path -LiteralPath $sibling) {
        try { $contracts = (git -C $sibling rev-parse --short HEAD 2>$null).Trim() } catch { }
    }
    if ([string]::IsNullOrWhiteSpace($contracts) -or $contracts -eq 'unknown') {
        try { $contracts = (git log -1 --format=%h -- demo/fixtures/contracts 2>$null).Trim() } catch { }
    }
    if ([string]::IsNullOrWhiteSpace($contracts)) { $contracts = 'unknown' }

    $built = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    $pkg = 'github.com/blinklabs-io/dns-cli/internal/cli'
    $ldflags = "-X $pkg.GitCommit=$commit -X $pkg.BuildDate=$built -X $pkg.ContractRevision=$contracts"
    Write-Host "Building -> $BinExe"
    Write-Host "ldflags: commit=$commit built=$built contracts=$contracts"
    # Windows: an existing/locked dns-cli.exe makes `go build -o` fail with
    # "already exists and is not an object file". Remove first when possible.
    if (Test-Path -LiteralPath $BinExe) {
        try {
            Remove-Item -LiteralPath $BinExe -Force -ErrorAction Stop
        } catch {
            throw "cannot overwrite $BinExe (is it running?). Stop it and re-run setup."
        }
    }
    & $go.Exe build -ldflags $ldflags -o $BinExe ./cmd/dns-cli
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
