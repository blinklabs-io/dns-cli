# =============================================================================
# run-demo.ps1 — Thin wrapper around `dns-cli demo run`
# =============================================================================
# Logic lives in Go (`dns-cli demo run`). This script resolves the binary,
# interactively fills unset flags, then forwards to the CLI.
# From demo/: .\scripts\run-demo.ps1 [flags]
#
# Binary resolution: $env:CLI → dns-cli/bin/ → tree root → PATH.
# Missing/outdated builds ask before compiling into dns-cli/bin/.
# =============================================================================

[CmdletBinding()]
param(
    [ValidateSet('fresh', 'existing')]
    [string] $Mode,

    [ValidateSet('blockfrost', 'utxorpc')]
    [string] $Provider,

    [string] $Tld,
    [string] $Sld,

    [ValidateSet('Quiet', 'Normal', 'Extensive')]
    [string] $LogLevel,

    [switch] $ExtensiveLogging,
    [switch] $Yes,
    [switch] $SkipInstall,
    [switch] $NoClipboard
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$DemoRoot = Split-Path -Parent $PSScriptRoot
$CliRoot = Split-Path $DemoRoot -Parent
$BinDir = Join-Path $CliRoot 'bin'
$BinExe = Join-Path $BinDir 'dns-cli.exe'
$BinUnix = Join-Path $BinDir 'dns-cli'

function Test-AssumeYes {
    if ($Yes) { return $true }
    if ($env:DEMO_ASSUME_YES -match '^(1|true|yes|on)$') { return $true }
    return $false
}

function Read-Choice {
    param(
        [Parameter(Mandatory)][string] $Prompt,
        [AllowEmptyString()]
        [string] $Default = '',
        [string[]] $Allowed
    )
    if (Test-AssumeYes) {
        Write-Host "$Prompt [$Default]: $Default (assume-yes)"
        return $Default
    }

    # Free-text when no fixed options.
    if (-not $Allowed -or $Allowed.Count -eq 0) {
        $ans = Read-Host "$Prompt [$Default]"
        if ([string]::IsNullOrWhiteSpace($ans)) {
            return $Default
        }
        return $ans.Trim()
    }

    $defaultIndex = 1
    for ($i = 0; $i -lt $Allowed.Count; $i++) {
        if ($Allowed[$i].Equals($Default, [System.StringComparison]::OrdinalIgnoreCase)) {
            $defaultIndex = $i + 1
            break
        }
    }

    Write-Host $Prompt
    for ($i = 0; $i -lt $Allowed.Count; $i++) {
        $mark = if (($i + 1) -eq $defaultIndex) { ' (default)' } else { '' }
        Write-Host ("  {0}) {1}{2}" -f ($i + 1), $Allowed[$i], $mark)
    }
    $ans = Read-Host "Enter number [$defaultIndex]"
    if ([string]::IsNullOrWhiteSpace($ans)) {
        return $Allowed[$defaultIndex - 1]
    }
    $ans = $ans.Trim()
    if ($ans -match '^\d+$') {
        $n = [int]$ans
        if ($n -ge 1 -and $n -le $Allowed.Count) {
            return $Allowed[$n - 1]
        }
    }
    # Also accept the option name if typed in full.
    $match = $Allowed | Where-Object { $_.Equals($ans, [System.StringComparison]::OrdinalIgnoreCase) }
    if ($match) {
        return $match
    }
    Write-Host "Invalid choice '$ans'; keeping default '$($Allowed[$defaultIndex - 1])'."
    return $Allowed[$defaultIndex - 1]
}

function Read-YesNo {
    param(
        [Parameter(Mandatory)][string] $Prompt,
        [bool] $DefaultYes = $false
    )
    if (Test-AssumeYes) {
        $pick = if ($DefaultYes) { 'Y' } else { 'N' }
        Write-Host "$Prompt : $pick (assume-yes)"
        return $DefaultYes
    }
    $hint = if ($DefaultYes) { 'Y/n' } else { 'y/N' }
    $ans = Read-Host "$Prompt [$hint]"
    if ([string]::IsNullOrWhiteSpace($ans)) {
        return $DefaultYes
    }
    return ($ans -match '^(y|yes)$')
}

function Test-DnsCliHasDemo {
    param([Parameter(Mandatory)][string] $Path)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & $Path demo --help 2>$null | Out-Null
        return ($LASTEXITCODE -eq 0)
    }
    catch {
        return $false
    }
    finally {
        $ErrorActionPreference = $prev
    }
}

function Show-DnsCliBuildGuide {
    Write-Host @"
Build dns-cli yourself, then re-run this script:

  cd $CliRoot
  mkdir bin -ErrorAction SilentlyContinue
  go build -o bin\dns-cli.exe ./cmd/dns-cli

Or set `$env:CLI` to an existing binary that includes ``demo``.
"@
}

function Confirm-BuildDnsCli {
    param([string] $Reason = 'missing or outdated')
    if ($SkipInstall) {
        Write-Host "dns-cli is $Reason and -SkipInstall was set; not building."
        Show-DnsCliBuildGuide
        return $false
    }
    if (Test-AssumeYes) {
        return $true
    }
    $ans = Read-Host "dns-cli is $Reason. Build into bin\ now? [y/N]"
    if ($ans -match '^(y|yes)$') {
        return $true
    }
    Show-DnsCliBuildGuide
    return $false
}

function Build-DnsCli {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "go is not on PATH; install Go 1.25.10+ or build dns-cli manually"
    }
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    Push-Location $CliRoot
    try {
        Write-Host "Building dns-cli → $BinExe"
        go build -o $BinExe ./cmd/dns-cli
        if ($LASTEXITCODE -ne 0) {
            throw "go build dns-cli failed"
        }
    }
    finally {
        Pop-Location
    }
    if (-not (Test-DnsCliHasDemo $BinExe)) {
        throw "built $BinExe but 'demo' is still missing"
    }
    Write-Host "Built $BinExe"
    return $BinExe
}

function Resolve-DnsCli {
    if ($env:CLI -and (Test-Path -LiteralPath $env:CLI)) {
        $path = (Resolve-Path -LiteralPath $env:CLI).Path
        if (-not (Test-DnsCliHasDemo $path)) {
            throw "CLI at `$env:CLI` lacks 'demo'. Rebuild dns-cli or unset CLI."
        }
        return $path
    }

    foreach ($cand in @($BinExe, $BinUnix,
            (Join-Path $CliRoot 'dns-cli.exe'),
            (Join-Path $CliRoot 'dns-cli'))) {
        if (-not (Test-Path -LiteralPath $cand)) {
            continue
        }
        $path = (Resolve-Path -LiteralPath $cand).Path
        if (Test-DnsCliHasDemo $path) {
            return $path
        }
        if (-not (Confirm-BuildDnsCli -Reason 'outdated (missing demo)')) {
            throw "dns-cli build declined"
        }
        return (Build-DnsCli)
    }

    $cmd = Get-Command dns-cli -ErrorAction SilentlyContinue
    if ($null -ne $cmd -and (Test-DnsCliHasDemo $cmd.Source)) {
        return $cmd.Source
    }

    if (-not (Confirm-BuildDnsCli -Reason 'not found')) {
        throw "dns-cli build declined"
    }
    return (Build-DnsCli)
}

function Resolve-DemoFlags {
    # Explicit CLI / env wins; otherwise ask (or assume-yes defaults).
    $resolvedMode = $Mode
    $resolvedProvider = $Provider
    $resolvedTld = $Tld
    $resolvedSld = $Sld

    if (-not $resolvedMode -and $env:DEMO_MODE) { $resolvedMode = $env:DEMO_MODE }
    if (-not $resolvedProvider -and $env:DEMO_PROVIDER) { $resolvedProvider = $env:DEMO_PROVIDER }

    $level = $LogLevel
    if ($ExtensiveLogging) { $level = 'Extensive' }
    if (-not $level -and $env:DEMO_EXTENSIVE_LOGGING -match '^(1|true|yes|on)$') { $level = 'Extensive' }
    if (-not $level -and $env:DEMO_LOG_LEVEL) { $level = $env:DEMO_LOG_LEVEL }

    $skipSet = $PSBoundParameters.ContainsKey('SkipInstall')
    $clipSet = $PSBoundParameters.ContainsKey('NoClipboard')
    $skip = [bool]$SkipInstall
    $noClip = [bool]$NoClipboard

    $needAsk = (-not $resolvedMode) -or (-not $level) -or (-not $skipSet) -or (-not $clipSet)
    if ($resolvedMode -ne 'existing') {
        if (-not $resolvedProvider) { $needAsk = $true }
        if (-not $resolvedTld -or -not $resolvedSld) { $needAsk = $true }
    }

    if ($needAsk -and -not (Test-AssumeYes)) {
        Write-Host ''
        if ($env:NO_COLOR) {
            Write-Host '══ Demo run options ══'
        } else {
            Write-Host '══ Demo run options ══' -ForegroundColor Cyan
        }
    }

    if (-not $resolvedMode) {
        $resolvedMode = Read-Choice -Prompt 'Mode' -Default 'fresh' -Allowed @('fresh', 'existing')
    }

    if ($resolvedMode -ne 'existing') {
        if (-not $resolvedProvider) {
            $resolvedProvider = Read-Choice -Prompt 'Provider' -Default 'blockfrost' -Allowed @('blockfrost', 'utxorpc')
        }
        if (-not $resolvedTld) {
            $entered = Read-Choice -Prompt 'TLD (blank = auto demo-<timestamp>)' -Default '' -Allowed @()
            if (-not [string]::IsNullOrWhiteSpace($entered)) {
                $resolvedTld = $entered.Trim()
            }
        }
        if (-not $resolvedSld) {
            $resolvedSld = Read-Choice -Prompt 'SLD' -Default 'www' -Allowed @()
        }
    }

    if (-not $level) {
        $level = Read-Choice -Prompt 'Log level' -Default 'normal' -Allowed @('quiet', 'normal', 'extensive')
    }

    if (-not $skipSet) {
        $skip = Read-YesNo -Prompt 'Skip tool installs / credential writes (guides only)?' -DefaultYes:$false
    }
    if (-not $clipSet -and $resolvedMode -ne 'existing') {
        $copyClip = Read-YesNo -Prompt 'Copy bootstrap faucet address to clipboard?' -DefaultYes:$true
        $noClip = -not $copyClip
    }

    if ($needAsk -and -not (Test-AssumeYes)) {
        if ($env:NO_COLOR) {
            Write-Host '════════════════════════'
        } else {
            Write-Host '════════════════════════' -ForegroundColor Cyan
        }
        Write-Host ''
    }

    return [pscustomobject]@{
        Mode        = $resolvedMode
        Provider    = $resolvedProvider
        Tld         = $resolvedTld
        Sld         = $resolvedSld
        LogLevel    = $level
        SkipInstall = $skip
        NoClipboard = $noClip
    }
}

$flags = Resolve-DemoFlags
# Apply skip-install choice before binary resolve/build prompts.
if ($flags.SkipInstall) {
    $SkipInstall = $true
}

$DnsCli = Resolve-DnsCli

$level = [string]$flags.LogLevel
switch ($level.ToLowerInvariant()) {
    { $_ -in @('quiet', 'q', '0') } { $verbose = 1 }
    { $_ -in @('extensive', 'debug', 'verbose', 'v', 'ext', '4') } { $verbose = 4 }
    default { $verbose = 2 }
}

$cliArgs = @('-v', "$verbose", 'demo', 'run', '--demo-root', $DemoRoot)
if ($flags.Mode) { $cliArgs += @('--mode', $flags.Mode) }
if ($flags.Provider -and $flags.Mode -ne 'existing') { $cliArgs += @('--provider', $flags.Provider) }
if ($flags.Tld) { $cliArgs += @('--tld', $flags.Tld) }
if ($flags.Sld) { $cliArgs += @('--sld', $flags.Sld) }
$cliArgs += @('--log-level', $level.ToLowerInvariant())
if ($Yes -or (Test-AssumeYes)) { $cliArgs += '--yes' }
if ($flags.SkipInstall) { $cliArgs += '--skip-install' }
if ($flags.NoClipboard) { $cliArgs += '--no-clipboard' }

Write-Host "dns-cli $($cliArgs -join ' ')"
& $DnsCli @cliArgs
exit $LASTEXITCODE
