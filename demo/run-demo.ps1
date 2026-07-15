# =============================================================================
# run-demo.ps1 — Resumable Cardano Preprod end-to-end demo for dns-cli
# =============================================================================
#
# Purpose
#   Drive a full lifecycle on Preprod: wallets → funding → system prepare/init →
#   register/activate TLD → mint/update SLD. State is stored under runtime/ so
#   interrupted runs can resume without re-submitting confirmed steps.
#
# Modes
#   fresh     Create wallets, deploy contracts, and submit the full tx chain.
#   existing  Offline validation + print historical Preprod fixture evidence
#             (no live submissions).
#
# Providers
#   blockfrost  Needs DNS_CLI_BLOCKFROST_PROJECT_ID
#   utxorpc     Needs DNS_CLI_UTXORPC_URL (optional DNS_CLI_UTXORPC_HEADERS)
#
# Logging
#   -LogLevel Quiet|Normal|Extensive   or   -ExtensiveLogging
#   Env: DEMO_LOG_LEVEL=quiet|normal|extensive   /   DEMO_EXTENSIVE_LOGGING=1
#   Extensive also passes -v/--verbose to dns-cli (see Resolve-CliVerbosity).
#
# Prerequisites
#   -Yes / DEMO_ASSUME_YES=1     auto-approve install/set prompts
#   -SkipInstall                 only print guides; do not install or write .env
#   Credentials may be saved to runtime/.env (gitignored).
#
# Security
#   Fixture/runtime keys are Preprod test material only — never use on mainnet.
#
# Usage
#   .\run-demo.ps1 [-Mode fresh|existing] [-Provider blockfrost|utxorpc]
#                  [-Tld NAME] [-Sld NAME] [-ExtensiveLogging] [-LogLevel ...]
#                  [-Yes] [-SkipInstall]
# =============================================================================

[CmdletBinding()]
param(
    # fresh = live lifecycle; existing = historical fixture inspection only
    [ValidateSet('fresh', 'existing')]
    [string] $Mode,

    # Chain data provider used in bootstrap/bound configs
    [ValidateSet('blockfrost', 'utxorpc')]
    [string] $Provider,

    # TLD / SLD labels (fresh default TLD is demo-<timestamp>, SLD is www)
    [string] $Tld,
    [string] $Sld,

    # Quiet=errors only, Normal=default progress, Extensive=debug + dns-cli -v
    [ValidateSet('Quiet', 'Normal', 'Extensive')]
    [string] $LogLevel,

    # Shortcut for -LogLevel Extensive (also honored via DEMO_EXTENSIVE_LOGGING=1)
    [switch] $ExtensiveLogging,

    # Auto-answer Yes to prerequisite install/set prompts (also DEMO_ASSUME_YES=1)
    [switch] $Yes,

    # Only print install guides; never run installers or write credentials
    [switch] $SkipInstall
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# --- Paths & constants (all relative to this demo/ folder) ----------------------
$DemoRoot = $PSScriptRoot
$RuntimeDir = Join-Path $DemoRoot 'runtime'          # generated wallets/artifacts/state
$StateFile = Join-Path $RuntimeDir 'state.json'      # resumable confirmed-step ledger
$StateSchema = Join-Path $DemoRoot 'state.schema.json'
$RecordsFile = Join-Path $DemoRoot 'records.json'    # DNS records for update-sld
$EnvFile = Join-Path $RuntimeDir '.env'              # local credentials (gitignored via runtime/)
$ToolsDir = Join-Path $RuntimeDir 'tools'            # optional local tool installs
$CliRoot = Split-Path $DemoRoot -Parent              # dns-cli module root
$ApolloDir = Join-Path (Split-Path $CliRoot -Parent) 'apollo'
$FaucetUrl = 'https://docs.cardano.org/cardano-testnets/tools/faucet/'
$MinBootstrapLovelace = [int64]150000000             # 150 ADA before starting fund tx
$PollSeconds = 20                                    # balance poll interval
$BlockfrostDashboard = 'https://blockfrost.io/dashboard'
$AikenInstallDocs = 'https://aiken-lang.org/installation-instructions'
$GoInstallDocs = 'https://go.dev/dl/'

# --- Script-scoped run state ---------------------------------------------------
$script:DnsCli = $null
$script:State = $null
$script:BootstrapConfig = $null
$script:BoundConfig = $null
$script:DeploymentJson = $null
$script:ProofBundle = $null
$script:EffectiveMode = $null
$script:EffectiveProvider = $null
$script:EffectiveTld = $null
$script:EffectiveSld = $null
$script:ModeSet = $PSBoundParameters.ContainsKey('Mode')
$script:ProviderSet = $PSBoundParameters.ContainsKey('Provider')
$script:TldSet = $PSBoundParameters.ContainsKey('Tld')
$script:SldSet = $PSBoundParameters.ContainsKey('Sld')
$script:LogLevelName = 'Normal'   # Quiet | Normal | Extensive
$script:CliVerbose = 2            # dns-cli -v level (0–4); raised under Extensive

# -----------------------------------------------------------------------------
# Logging helpers
#   Write-DemoLog     always-on progress (suppressed in Quiet)
#   Write-DemoExt     extensive/debug detail (paths, timings, redacted env, JSON)
#   Write-DemoError   always printed before throw
# -----------------------------------------------------------------------------

function Initialize-DemoLogging {
    # Resolve precedence: -ExtensiveLogging > -LogLevel > DEMO_EXTENSIVE_LOGGING > DEMO_LOG_LEVEL > Normal
    if ($ExtensiveLogging) {
        $script:LogLevelName = 'Extensive'
    }
    elseif ($PSBoundParameters.ContainsKey('LogLevel') -and -not [string]::IsNullOrWhiteSpace($LogLevel)) {
        $script:LogLevelName = $LogLevel
    }
    elseif ($env:DEMO_EXTENSIVE_LOGGING -match '^(1|true|yes|on)$') {
        $script:LogLevelName = 'Extensive'
    }
    elseif (-not [string]::IsNullOrWhiteSpace($env:DEMO_LOG_LEVEL)) {
        switch -Regex ($env:DEMO_LOG_LEVEL.Trim().ToLowerInvariant()) {
            '^(0|quiet|q)$' { $script:LogLevelName = 'Quiet' }
            '^(2|extensive|debug|verbose|v|ext)$' { $script:LogLevelName = 'Extensive' }
            default { $script:LogLevelName = 'Normal' }
        }
    }
    else {
        $script:LogLevelName = 'Normal'
    }

    # Map runner log level → dns-cli --verbose (PersistentPreRun -v 0..4)
    switch ($script:LogLevelName) {
        'Quiet' { $script:CliVerbose = 1 }
        'Normal' { $script:CliVerbose = 2 }
        'Extensive' { $script:CliVerbose = 4 }
        default { $script:CliVerbose = 2 }
    }
}

function Write-DemoLog {
    param([string] $Message)
    if ($script:LogLevelName -eq 'Quiet') { return }
    $ts = Get-Date -Format 'HH:mm:ss'
    Write-Host "[demo $ts] $Message"
}

function Write-DemoExt {
    param([string] $Message)
    if ($script:LogLevelName -ne 'Extensive') { return }
    $ts = Get-Date -Format 'HH:mm:ss.fff'
    Write-Host "[demo:ext $ts] $Message" -ForegroundColor DarkCyan
}

function Write-DemoError {
    param([string] $Message)
    $ts = Get-Date -Format 'HH:mm:ss'
    Write-Host "[demo $ts] ERROR: $Message" -ForegroundColor Red
}

function Throw-Demo {
    param([string] $Message)
    Write-DemoError $Message
    throw "[demo] ERROR: $Message"
}

function Get-RedactedEnvSummary {
    # Never print secret values; only presence / length for provider credentials.
    $lines = @()
    foreach ($name in @(
            'CLI', 'DEMO_MODE', 'DEMO_PROVIDER', 'DEMO_LOG_LEVEL', 'DEMO_EXTENSIVE_LOGGING',
            'DNS_CLI_BLOCKFROST_PROJECT_ID', 'DNS_CLI_UTXORPC_URL', 'DNS_CLI_UTXORPC_HEADERS'
        )) {
        $val = [Environment]::GetEnvironmentVariable($name)
        if ([string]::IsNullOrEmpty($val)) {
            $lines += "  $name=<unset>"
        }
        elseif ($name -match 'PROJECT_ID|HEADERS|KEY|SECRET|TOKEN|PASSWORD') {
            $lines += "  $name=<set len=$($val.Length)>"
        }
        else {
            $lines += "  $name=$val"
        }
    }
    return ($lines -join "`n")
}

function Write-DemoStartupBanner {
    Write-DemoLog "LogLevel=$($script:LogLevelName) dns-cli -v $($script:CliVerbose)"
    Write-DemoExt "DemoRoot=$DemoRoot"
    Write-DemoExt "RuntimeDir=$RuntimeDir"
    Write-DemoExt "StateFile=$StateFile"
    Write-DemoExt "PSVersion=$($PSVersionTable.PSVersion) OS=$([System.Environment]::OSVersion.VersionString)"
    Write-DemoExt ("Environment:`n" + (Get-RedactedEnvSummary))
}

# =============================================================================
# Prerequisites — detect, optionally install/set with consent, else guide
# =============================================================================

function Confirm-DemoYes {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Prompt
    )
    if ($Yes -or ($env:DEMO_ASSUME_YES -match '^(1|true|yes|on)$')) {
        Write-DemoLog "$Prompt -> yes (auto)"
        return $true
    }
    $ans = Read-Host "$Prompt [y/N]"
    return ($ans -match '^[Yy]$')
}

function Show-DemoGuide {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Title,
        [Parameter(Mandatory = $true)]
        [string[]] $Lines
    )
    Write-Host ''
    Write-Host "-------- How to get: $Title --------" -ForegroundColor Yellow
    foreach ($line in $Lines) {
        Write-Host "  $line"
    }
    Write-Host '----------------------------------------'
    Write-Host ''
}

function Import-DemoEnvFile {
    # Load KEY=VALUE pairs from runtime/.env into this process (no shell injection).
    if (-not (Test-Path -LiteralPath $EnvFile)) {
        Write-DemoExt "no env file at $EnvFile"
        return
    }
    Write-DemoLog "Loading credentials from $EnvFile"
    foreach ($raw in Get-Content -LiteralPath $EnvFile) {
        $line = $raw.Trim()
        if ([string]::IsNullOrWhiteSpace($line) -or $line.StartsWith('#')) { continue }
        if ($line -notmatch '^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$') { continue }
        $key = $Matches[1]
        $val = $Matches[2].Trim()
        if (($val.StartsWith('"') -and $val.EndsWith('"')) -or ($val.StartsWith("'") -and $val.EndsWith("'"))) {
            $val = $val.Substring(1, $val.Length - 2)
        }
        if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($key))) {
            [Environment]::SetEnvironmentVariable($key, $val, 'Process')
            Set-Item -Path "Env:$key" -Value $val
            Write-DemoExt "imported $key from .env (len=$($val.Length))"
        }
        else {
            Write-DemoExt "keeping existing process env for $key"
        }
    }
}

function Save-DemoEnvVar {
    param(
        [Parameter(Mandatory = $true)][string] $Name,
        [Parameter(Mandatory = $true)][string] $Value
    )
    [Environment]::SetEnvironmentVariable($Name, $Value, 'Process')
    Set-Item -Path "Env:$Name" -Value $Value
    if (-not (Test-Path -LiteralPath $RuntimeDir)) {
        New-Item -ItemType Directory -Path $RuntimeDir -Force | Out-Null
    }
    $lines = @()
    if (Test-Path -LiteralPath $EnvFile) {
        $lines = @(Get-Content -LiteralPath $EnvFile | Where-Object {
                $_ -notmatch ("^\s*{0}\s*=" -f [regex]::Escape($Name))
            })
    }
    $lines += "$Name=$Value"
    [System.IO.File]::WriteAllText($EnvFile, (($lines -join "`n") + "`n"))
    Write-DemoLog "Saved $Name to $EnvFile (process env updated)"
}

function Test-DemoCommand {
    param([Parameter(Mandatory = $true)][string] $Name)
    return $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

function Prepend-DemoToolsPath {
    if (Test-Path -LiteralPath $ToolsDir) {
        $env:Path = "$ToolsDir;$env:Path"
        Write-DemoExt "PATH prepended with $ToolsDir"
    }
    $aikenHome = Join-Path $env:USERPROFILE '.aiken\bin'
    if (Test-Path -LiteralPath $aikenHome) {
        $env:Path = "$aikenHome;$env:Path"
        Write-DemoExt "PATH prepended with $aikenHome"
    }
}

function Find-DnsCliCandidate {
    # Returns a path string or $null.
    if ($env:CLI -and (Test-Path -LiteralPath $env:CLI)) { return $env:CLI }
    $exe = Join-Path $CliRoot 'dns-cli.exe'
    $bin = Join-Path $CliRoot 'dns-cli'
    if (Test-Path -LiteralPath $exe) { return $exe }
    if (Test-Path -LiteralPath $bin) { return $bin }
    $cmd = Get-Command dns-cli -ErrorAction SilentlyContinue
    if ($null -ne $cmd) { return $cmd.Source }
    return $null
}

function Install-DnsCliFromSource {
    Write-DemoLog 'Attempting to build dns-cli from source...'
    if (-not (Test-DemoCommand 'go')) {
        Show-DemoGuide -Title 'Go toolchain' -Lines @(
            "Go is required to build dns-cli. Install from $GoInstallDocs",
            'Then reopen this terminal and re-run the demo.',
            "Expected Apollo checkout at: $ApolloDir"
        )
        return $false
    }
    if (-not (Test-Path -LiteralPath (Join-Path $ApolloDir 'go.mod'))) {
        Show-DemoGuide -Title 'Apollo v2 checkout' -Lines @(
            "dns-cli go.mod replace expects Apollo at: $ApolloDir",
            'Clone it next to the dns-cli module:',
            "  git clone https://github.com/Salvionied/apollo.git `"$ApolloDir`"",
            '  cd apollo && git checkout b2f56d0c6e9d22316b6938feeb325bdbab3846d2',
            'Then re-run this demo.'
        )
        return $false
    }
    $outName = if ($env:OS -match 'Windows') { 'dns-cli.exe' } else { 'dns-cli' }
    $out = Join-Path $CliRoot $outName
    Push-Location $CliRoot
    try {
        Write-DemoLog "go build -o $out ./cmd/dns-cli"
        & go build -o $out ./cmd/dns-cli
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $out)) {
            Show-DemoGuide -Title 'dns-cli build failed' -Lines @(
                "Manual build from $CliRoot :",
                "  go build -o $outName ./cmd/dns-cli",
                'See docs/installation.md for Apollo + Go version pins.'
            )
            return $false
        }
    }
    finally {
        Pop-Location
    }
    $env:CLI = $out
    Write-DemoLog "Built dns-cli: $out"
    return $true
}

function Ensure-DnsCli {
    $found = Find-DnsCliCandidate
    if ($found) {
        $script:DnsCli = $found
        Write-DemoLog "Using CLI: $($script:DnsCli)"
        return
    }

    Write-DemoLog 'dns-cli binary not found.'
    Show-DemoGuide -Title 'dns-cli' -Lines @(
        "Set CLI to a built binary, or build from $CliRoot :",
        '  go build -o dns-cli.exe ./cmd/dns-cli',
        "Apollo must exist at $ApolloDir (see go.mod replace).",
        'Docs: docs/installation.md'
    )

    if ($SkipInstall) {
        Throw-Demo 'dns-cli missing and -SkipInstall was set'
    }
    if (-not (Confirm-DemoYes -Prompt 'Build dns-cli from source now?')) {
        Throw-Demo 'dns-cli is required. Build it or set CLI=path\to\dns-cli.exe'
    }
    if (-not (Install-DnsCliFromSource)) {
        Throw-Demo 'could not build dns-cli; follow the guide above'
    }
    $script:DnsCli = Find-DnsCliCandidate
    if (-not $script:DnsCli) {
        Throw-Demo 'dns-cli still missing after build attempt'
    }
    Write-DemoLog "Using CLI: $($script:DnsCli)"
}

function Install-AikenInteractive {
    Write-DemoLog 'Attempting Aiken install...'
    Prepend-DemoToolsPath
    if (Test-DemoCommand 'aiken') { return $true }

    $isWin = ($env:OS -match 'Windows') -or ($PSVersionTable.PSEdition -eq 'Desktop')
    if ($isWin) {
        if (-not (Confirm-DemoYes -Prompt 'Run official Windows Aiken installer (irm https://windows.aiken-lang.org | iex)?')) {
            return $false
        }
        try {
            Write-DemoLog 'Downloading/running Windows Aiken installer...'
            Invoke-Expression (Invoke-RestMethod 'https://windows.aiken-lang.org')
        }
        catch {
            Write-DemoLog "Installer failed: $_"
            return $false
        }
    }
    elseif (Test-DemoCommand 'curl') {
        if (-not (Confirm-DemoYes -Prompt 'Install aikup via https://install.aiken-lang.org then run aikup?')) {
            return $false
        }
        bash -lc 'curl --proto "=https" --tlsv1.2 -LsSf https://install.aiken-lang.org | sh && export PATH="$HOME/.aiken/bin:$PATH" && aikup' 2>&1 | ForEach-Object { Write-DemoExt $_ }
    }
    elseif (Test-DemoCommand 'cargo') {
        if (-not (Confirm-DemoYes -Prompt 'Install aiken with cargo install aiken? (may take several minutes)')) {
            return $false
        }
        & cargo install aiken
        if ($LASTEXITCODE -ne 0) { return $false }
    }
    else {
        return $false
    }

    Prepend-DemoToolsPath
    # Refresh command lookup
    $env:Path = [System.Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' +
        [System.Environment]::GetEnvironmentVariable('Path', 'User') + ';' + $env:Path
    Prepend-DemoToolsPath
    return (Test-DemoCommand 'aiken')
}

function Ensure-Aiken {
    Prepend-DemoToolsPath
    if (Test-DemoCommand 'aiken') {
        $cmd = Get-Command aiken
        Write-DemoLog "Using aiken: $($cmd.Source)"
        Write-DemoExt "aiken version: $((& aiken --version) 2>&1 | Out-String)".Trim()
        return
    }

    Write-DemoLog 'aiken not found on PATH (required for fresh mode system prepare).'
    Show-DemoGuide -Title 'Aiken' -Lines @(
        "Install docs: $AikenInstallDocs",
        'Windows:  powershell -c "irm https://windows.aiken-lang.org | iex"',
        'macOS/Linux: curl --proto "=https" --tlsv1.2 -LsSf https://install.aiken-lang.org | sh && aikup',
        'Or: cargo install aiken  (requires Rust)',
        'Then ensure aiken is on PATH and re-run.'
    )

    if ($SkipInstall) {
        Throw-Demo 'aiken missing and -SkipInstall was set'
    }
    if (-not (Confirm-DemoYes -Prompt 'Try to install Aiken now?')) {
        Throw-Demo 'aiken is required for fresh mode. Install it and re-run.'
    }
    if (-not (Install-AikenInteractive)) {
        Throw-Demo 'Aiken install did not succeed. Follow the guide above, then re-run.'
    }
    Write-DemoLog "Using aiken: $((Get-Command aiken).Source)"
}

function Read-SecretValue {
    param(
        [Parameter(Mandatory = $true)][string] $Prompt,
        [switch] $AllowEmpty
    )
    $val = Read-Host $Prompt
    if (-not $AllowEmpty -and [string]::IsNullOrWhiteSpace($val)) {
        return $null
    }
    return $val
}

function Ensure-ProviderCredentials {
    switch ($script:EffectiveProvider) {
        'blockfrost' {
            if (-not [string]::IsNullOrWhiteSpace($env:DNS_CLI_BLOCKFROST_PROJECT_ID)) {
                Write-DemoExt "DNS_CLI_BLOCKFROST_PROJECT_ID is set (len=$($env:DNS_CLI_BLOCKFROST_PROJECT_ID.Length))"
                return
            }
            Write-DemoLog 'Missing DNS_CLI_BLOCKFROST_PROJECT_ID for Blockfrost Preprod.'
            Show-DemoGuide -Title 'Blockfrost project id' -Lines @(
                "Open $BlockfrostDashboard and create/select a Preprod project.",
                'Copy the project ID (looks like preprod...).',
                'Set for this shell:  $env:DNS_CLI_BLOCKFROST_PROJECT_ID = ''preprod...''',
                "Or let this script save it to $EnvFile"
            )
            if ($SkipInstall) {
                Throw-Demo 'DNS_CLI_BLOCKFROST_PROJECT_ID missing and -SkipInstall was set'
            }
            if (-not (Confirm-DemoYes -Prompt 'Enter Blockfrost project id now and save to runtime/.env?')) {
                Throw-Demo 'DNS_CLI_BLOCKFROST_PROJECT_ID is required for -Provider blockfrost'
            }
            $id = Read-SecretValue -Prompt 'Blockfrost Preprod project id'
            if ([string]::IsNullOrWhiteSpace($id)) {
                Throw-Demo 'empty project id; cannot continue'
            }
            Save-DemoEnvVar -Name 'DNS_CLI_BLOCKFROST_PROJECT_ID' -Value $id.Trim()
        }
        'utxorpc' {
            if (-not [string]::IsNullOrWhiteSpace($env:DNS_CLI_UTXORPC_URL)) {
                Write-DemoExt "DNS_CLI_UTXORPC_URL=$($env:DNS_CLI_UTXORPC_URL)"
                return
            }
            Write-DemoLog 'Missing DNS_CLI_UTXORPC_URL for UTxO RPC provider.'
            Show-DemoGuide -Title 'UTxO RPC endpoint' -Lines @(
                'Set DNS_CLI_UTXORPC_URL to your Preprod UTxO RPC base URL (https://...).',
                'Optional DNS_CLI_UTXORPC_HEADERS as Key=Value,Key2=Value2.',
                'Example:  $env:DNS_CLI_UTXORPC_URL = ''https://preprod.example/...'''
            )
            if ($SkipInstall) {
                Throw-Demo 'DNS_CLI_UTXORPC_URL missing and -SkipInstall was set'
            }
            if (-not (Confirm-DemoYes -Prompt 'Enter UTxO RPC URL now and save to runtime/.env?')) {
                Throw-Demo 'DNS_CLI_UTXORPC_URL is required for -Provider utxorpc'
            }
            $url = Read-SecretValue -Prompt 'DNS_CLI_UTXORPC_URL'
            if ([string]::IsNullOrWhiteSpace($url)) {
                Throw-Demo 'empty URL; cannot continue'
            }
            Save-DemoEnvVar -Name 'DNS_CLI_UTXORPC_URL' -Value $url.Trim()
            if (Confirm-DemoYes -Prompt 'Also set optional DNS_CLI_UTXORPC_HEADERS?') {
                $hdr = Read-SecretValue -Prompt 'DNS_CLI_UTXORPC_HEADERS (Key=Value,...)' -AllowEmpty
                if (-not [string]::IsNullOrWhiteSpace($hdr)) {
                    Save-DemoEnvVar -Name 'DNS_CLI_UTXORPC_HEADERS' -Value $hdr.Trim()
                }
            }
        }
        default { Throw-Demo "unsupported provider: $($script:EffectiveProvider)" }
    }
}

function Invoke-DemoPrerequisites {
    param([ValidateSet('fresh', 'existing')][string] $ModeName)
    Write-DemoLog "Checking prerequisites for mode=$ModeName ..."
    if (-not (Test-Path -LiteralPath $RuntimeDir)) {
        New-Item -ItemType Directory -Path $RuntimeDir -Force | Out-Null
    }
    Import-DemoEnvFile
    Prepend-DemoToolsPath
    Ensure-DnsCli
    if ($ModeName -eq 'fresh') {
        Ensure-ProviderCredentials
        Ensure-Aiken
    }
    Write-DemoLog 'Prerequisites OK'
}

function Format-CliArgForLog {
    param([string] $Arg)
    if ($Arg -match '\s') { return ('"{0}"' -f $Arg) }
    return $Arg
}

function Resolve-Cli {
    # Prefer explicit CLI=, then sibling ../dns-cli(.exe), then PATH.
    # Prefer Invoke-DemoPrerequisites / Ensure-DnsCli for interactive setup.
    Write-DemoExt 'Resolve-Cli: locating dns-cli binary'
    $found = Find-DnsCliCandidate
    if (-not $found) {
        Throw-Demo 'dns-cli binary not found. Set CLI=... or build ../dns-cli.exe (or re-run so prerequisites can offer a build)'
    }
    $script:DnsCli = $found
    if (-not (Test-Path -LiteralPath $script:DnsCli)) {
        Throw-Demo "CLI not found: $($script:DnsCli)"
    }
    Write-DemoLog "Using CLI: $($script:DnsCli)"
    if ($script:LogLevelName -eq 'Extensive') {
        try {
            $item = Get-Item -LiteralPath $script:DnsCli
            Write-DemoExt "CLI size=$($item.Length) bytes mtime=$($item.LastWriteTimeUtc.ToString('o'))"
        }
        catch {
            Write-DemoExt "CLI stat failed: $_"
        }
    }
}

function Invoke-DnsCliJson {
    # Run dns-cli with --output json and -v. stdout must stay JSON-only (tint goes to stderr).
    param(
        [Parameter(Mandatory = $true)]
        [string[]] $CliArgs
    )
    $argList = @('-v', [string]$script:CliVerbose) + @($CliArgs) + @('--output', 'json')
    $display = ($CliArgs | ForEach-Object { Format-CliArgForLog $_ }) -join ' '
    Write-DemoLog ("dns-cli " + $display)
    Write-DemoExt ("full argv: dns-cli " + (($argList | ForEach-Object { Format-CliArgForLog $_ }) -join ' '))

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    # Do not merge stderr: dns-cli writes tint/verbose logs there; JSON body is on stdout.
    $stdout = & $script:DnsCli @argList
    $sw.Stop()
    $code = $LASTEXITCODE

    $text = if ($null -eq $stdout) {
        ''
    }
    elseif ($stdout -is [array]) {
        ($stdout | ForEach-Object { [string]$_ }) -join "`n"
    }
    else {
        [string]$stdout
    }
    $text = $text.Trim()

    Write-DemoExt ("exit=$code duration_ms=$($sw.ElapsedMilliseconds) stdout_bytes=$($text.Length)")
    if ($code -ne 0) {
        if (-not [string]::IsNullOrWhiteSpace($text)) {
            $tailLen = [Math]::Min(2000, $text.Length)
            Write-DemoExt ("stdout tail:`n" + $text.Substring($text.Length - $tailLen))
        }
        Throw-Demo "dns-cli failed (exit $code): $display"
    }
    if ([string]::IsNullOrWhiteSpace($text)) {
        Write-DemoExt 'empty JSON stdout (ok for some commands)'
        return $null
    }

    try {
        $obj = $text | ConvertFrom-Json
    }
    catch {
        $tailLen = [Math]::Min(1500, $text.Length)
        Write-DemoExt ("JSON parse failed; raw tail:`n" + $text.Substring($text.Length - $tailLen))
        Throw-Demo "failed to parse dns-cli JSON for: $display"
    }
    if ($script:LogLevelName -eq 'Extensive') {
        try {
            $preview = ($obj | ConvertTo-Json -Depth 6 -Compress)
            if ($preview.Length -gt 1200) { $preview = $preview.Substring(0, 1200) + '...(truncated)' }
            Write-DemoExt "json: $preview"
        }
        catch {
            Write-DemoExt 'json preview failed'
        }
    }
    return $obj
}

function Invoke-DnsCliQuiet {
    param(
        [Parameter(Mandatory = $true)]
        [string[]] $CliArgs
    )
    $null = Invoke-DnsCliJson -CliArgs $CliArgs
}

function Test-ProviderEnv {
    # Interactive credential setup (also called from Invoke-DemoPrerequisites).
    Ensure-ProviderCredentials
}

function New-EmptyStep {
    # Placeholder for a not-yet-confirmed lifecycle step in state.json.
    return [pscustomobject]@{ txId = ''; manifest = '' }
}

function New-DefaultState {
    # Initial resumable ledger: schemaVersion 1 with empty confirmed.* steps.
    param(
        [string] $TldName,
        [string] $SldName,
        [string] $ModeName,
        [string] $ProviderName
    )
    Write-DemoExt "New-DefaultState tld=$TldName sld=$SldName mode=$ModeName provider=$ProviderName"
    return [pscustomobject]@{
        schemaVersion = 1
        mode          = $ModeName
        network       = 'preprod'
        provider      = $ProviderName
        tld           = $TldName
        sld           = $SldName
        confirmed     = [pscustomobject]@{
            fund      = New-EmptyStep
            deploy    = New-EmptyStep
            register  = New-EmptyStep
            activate  = New-EmptyStep
            mintSld   = New-EmptyStep
            updateSld = New-EmptyStep
        }
    }
}

function Save-StateAtomic {
    # Write via .tmp + Move-Item so a crash mid-write cannot leave a half file.
    $tmp = "$StateFile.tmp"
    $dir = Split-Path -Parent $StateFile
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    $json = $script:State | ConvertTo-Json -Depth 10
    [System.IO.File]::WriteAllText($tmp, $json + "`n")
    Move-Item -LiteralPath $tmp -Destination $StateFile -Force
    Write-DemoExt "Save-StateAtomic wrote $StateFile ($($json.Length) bytes)"
}

function Get-StepTxId {
    # Return confirmed tx id for a step key, or '' if missing / empty.
    param([string] $Key)
    $confirmed = $script:State.confirmed
    $step = $confirmed | Select-Object -ExpandProperty $Key -ErrorAction SilentlyContinue
    if ($null -eq $step) { return '' }
    return [string]$step.txId
}

function Set-StepConfirmed {
    # Persist a confirmed on-chain step so resume skips rebuild/resubmit.
    param(
        [string] $Key,
        [string] $TxId,
        [string] $Manifest
    )
    $entry = [pscustomobject]@{
        txId     = $TxId
        manifest = $Manifest
    }
    switch ($Key) {
        'fund' { $script:State.confirmed.fund = $entry }
        'deploy' { $script:State.confirmed.deploy = $entry }
        'register' { $script:State.confirmed.register = $entry }
        'activate' { $script:State.confirmed.activate = $entry }
        'mintSld' { $script:State.confirmed.mintSld = $entry }
        'updateSld' { $script:State.confirmed.updateSld = $entry }
        default { Throw-Demo "unknown confirmed step key: $Key" }
    }
    Save-StateAtomic
    Write-DemoLog "Confirmed step ${Key}: txId=${TxId}"
    Write-DemoExt "manifest=$Manifest"
}

function Initialize-State {
    # Load runtime/state.json if present; otherwise create defaults.
    # Explicit CLI flags / env vars override stored values where intended.
    Write-DemoExt 'Initialize-State: begin'
    if (-not (Test-Path -LiteralPath $RuntimeDir)) {
        New-Item -ItemType Directory -Path $RuntimeDir -Force | Out-Null
        Write-DemoExt "created RuntimeDir=$RuntimeDir"
    }

    if ($script:ModeSet) {
        $script:EffectiveMode = $Mode
    }
    elseif ($env:DEMO_MODE) {
        $script:EffectiveMode = $env:DEMO_MODE
    }
    else {
        $script:EffectiveMode = 'fresh'
    }

    if ($script:ProviderSet) {
        $script:EffectiveProvider = $Provider
    }
    elseif ($env:DEMO_PROVIDER) {
        $script:EffectiveProvider = $env:DEMO_PROVIDER
    }
    else {
        $script:EffectiveProvider = 'blockfrost'
    }

    if ($script:SldSet) {
        $script:EffectiveSld = $Sld
    }
    else {
        $script:EffectiveSld = 'www'
    }

    $defaultTld = 'demo-' + (Get-Date -Format 'yyyyMMddHHmmss')

    if (Test-Path -LiteralPath $StateFile) {
        Write-DemoExt "loading existing state from $StateFile"
        $raw = Get-Content -LiteralPath $StateFile -Raw
        $script:State = $raw | ConvertFrom-Json
        if ([int]$script:State.schemaVersion -ne 1) {
            Throw-Demo 'unsupported state.schemaVersion (want 1)'
        }
        if (-not $script:TldSet) {
            $script:EffectiveTld = [string]$script:State.tld
        }
        else {
            $script:EffectiveTld = $Tld
        }
        if (-not $script:SldSet) {
            $script:EffectiveSld = [string]$script:State.sld
            if ([string]::IsNullOrWhiteSpace($script:EffectiveSld)) {
                $script:EffectiveSld = 'www'
            }
        }
        if (-not $script:ModeSet -and -not $env:DEMO_MODE) {
            $script:EffectiveMode = [string]$script:State.mode
        }
        if (-not $script:ProviderSet -and -not $env:DEMO_PROVIDER) {
            $script:EffectiveProvider = [string]$script:State.provider
        }
        Write-DemoExt ("resume confirmed keys: fund=$(Get-StepTxId fund) deploy=$(Get-StepTxId deploy) register=$(Get-StepTxId register) activate=$(Get-StepTxId activate) mintSld=$(Get-StepTxId mintSld) updateSld=$(Get-StepTxId updateSld)")
    }
    else {
        Write-DemoExt 'no state file; creating fresh defaults'
        if ($script:TldSet) {
            $script:EffectiveTld = $Tld
        }
        else {
            $script:EffectiveTld = $defaultTld
        }
        $script:State = New-DefaultState -TldName $script:EffectiveTld -SldName $script:EffectiveSld `
            -ModeName $script:EffectiveMode -ProviderName $script:EffectiveProvider
    }

    if ($script:EffectiveMode -notin @('fresh', 'existing')) {
        Throw-Demo "invalid mode: $($script:EffectiveMode)"
    }
    if ($script:EffectiveProvider -notin @('blockfrost', 'utxorpc')) {
        Throw-Demo "invalid provider: $($script:EffectiveProvider)"
    }

    $script:State.mode = $script:EffectiveMode
    $script:State.provider = $script:EffectiveProvider
    $script:State.tld = $script:EffectiveTld
    $script:State.sld = $script:EffectiveSld
    $script:State.network = 'preprod'
    Save-StateAtomic
    Write-DemoLog "State: $StateFile (mode=$($script:EffectiveMode) provider=$($script:EffectiveProvider) tld=$($script:EffectiveTld) sld=$($script:EffectiveSld))"
}

function Read-PaymentAddr {
    # payment.addr is a single bech32 line written by `wallet create`.
    param([string] $WalletDir)
    $path = Join-Path $WalletDir 'payment.addr'
    Write-DemoExt "Read-PaymentAddr: $path"
    if (-not (Test-Path -LiteralPath $path)) {
        Throw-Demo "missing payment.addr: $path"
    }
    $addr = (Get-Content -LiteralPath $path -Raw).Trim()
    if ([string]::IsNullOrWhiteSpace($addr)) {
        Throw-Demo "empty payment.addr in $WalletDir"
    }
    Write-DemoExt "addr=$addr"
    return $addr
}

function Ensure-Wallets {
    # Create the four Preprod actors once; skip any wallet that already has addr+skey.
    Write-DemoExt 'Ensure-Wallets: bootstrap/registrar/tld-owner/sld-owner'
    foreach ($name in @('bootstrap', 'registrar', 'tld-owner', 'sld-owner')) {
        $out = Join-Path $RuntimeDir "wallets\$name"
        $addr = Join-Path $out 'payment.addr'
        $skey = Join-Path $out 'payment.skey'
        if ((Test-Path -LiteralPath $addr) -and (Test-Path -LiteralPath $skey)) {
            Write-DemoLog "Wallet exists: $name"
            Write-DemoExt "reuse $out"
            continue
        }
        Write-DemoLog "Creating wallet: $name"
        Invoke-DnsCliQuiet -CliArgs @(
            'wallet', 'create',
            '--name', $name,
            '--network', 'preprod',
            '--format', 'key-envelope',
            '--out-dir', $out
        )
    }
}

function Write-BootstrapConfig {
    # Temporary profile used only for faucet wait, fund, and system init.
    # Contract placeholders are REPLACE_ME until `system bind` after deploy confirms.
    Write-DemoExt 'Write-BootstrapConfig: begin'
    $out = Join-Path $RuntimeDir 'config\bootstrap.json'
    $cfgDir = Split-Path -Parent $out
    if (-not (Test-Path -LiteralPath $cfgDir)) {
        New-Item -ItemType Directory -Path $cfgDir -Force | Out-Null
    }

    $bootstrapAddr = Read-PaymentAddr (Join-Path $RuntimeDir 'wallets\bootstrap')
    $registrarAddr = Read-PaymentAddr (Join-Path $RuntimeDir 'wallets\registrar')
    $tldOwnerAddr = Read-PaymentAddr (Join-Path $RuntimeDir 'wallets\tld-owner')
    $sldOwnerAddr = Read-PaymentAddr (Join-Path $RuntimeDir 'wallets\sld-owner')

    if ($script:EffectiveProvider -eq 'blockfrost') {
        $providerObj = [ordered]@{
            type         = 'blockfrost'
            baseURL      = 'https://cardano-preprod.blockfrost.io/api/v0'
            projectIdEnv = 'DNS_CLI_BLOCKFROST_PROJECT_ID'
        }
    }
    else {
        $providerObj = [ordered]@{
            type        = 'utxorpc'
            baseUrlEnv  = 'DNS_CLI_UTXORPC_URL'
            headersEnv  = 'DNS_CLI_UTXORPC_HEADERS'
        }
    }

    $doc = [ordered]@{
        version        = 1
        defaultProfile = 'preprod'
        profiles       = [ordered]@{
            preprod = [ordered]@{
                network   = [ordered]@{
                    name          = 'preprod'
                    id            = 0
                    magic         = 1
                    explorerTxURL = 'https://preprod.cexplorer.io/tx/{txId}'
                }
                provider  = $providerObj
                contracts = [ordered]@{
                    blueprintPath          = '../../fixtures/contracts/plutus.json'
                    tldRegistrarAddress    = 'addr_test1...'
                    tldReferenceAddress    = 'addr_test1...'
                    sldReferenceAddress    = 'addr_test1...'
                    tldRegistrarPolicyId   = 'REPLACE_ME'
                    tldReferencePolicyId   = 'REPLACE_ME'
                    sldReferencePolicyId   = 'REPLACE_ME'
                    referenceUtxos         = [ordered]@{
                        tldRegistrar = 'REPLACE_ME_TXHASH#0'
                        tldReference = 'REPLACE_ME_TXHASH#1'
                        sldReference = 'REPLACE_ME_TXHASH#2'
                    }
                }
                actors    = [ordered]@{
                    bootstrap = [ordered]@{
                        address        = $bootstrapAddr
                        signingKeyFile = '../wallets/bootstrap/payment.skey'
                    }
                    registrar = [ordered]@{
                        address        = $registrarAddr
                        signingKeyFile = '../wallets/registrar/payment.skey'
                    }
                    tldOwner  = [ordered]@{
                        address        = $tldOwnerAddr
                        signingKeyFile = '../wallets/tld-owner/payment.skey'
                    }
                    sldOwner  = [ordered]@{
                        address        = $sldOwnerAddr
                        signingKeyFile = '../wallets/sld-owner/payment.skey'
                    }
                }
                transaction = [ordered]@{
                    ttlSlots              = 300
                    confirmationTimeout   = '10m'
                    pollInterval          = '5s'
                    artifactDir           = '../artifacts'
                    maxDatumBytes         = 4000
                }
            }
        }
    }

    $tmp = "$out.tmp"
    [System.IO.File]::WriteAllText($tmp, (($doc | ConvertTo-Json -Depth 12) + "`n"))
    Move-Item -LiteralPath $tmp -Destination $out -Force
    $script:BootstrapConfig = $out
    Write-DemoLog "Wrote $($script:BootstrapConfig)"
    Invoke-DnsCliQuiet -CliArgs @('config', 'validate', '--config', $script:BootstrapConfig)
}

function Wait-ForBootstrapFunds {
    # Poll until bootstrap has >= 150 ADA so fund+init have enough Lovelace.
    $addr = Read-PaymentAddr (Join-Path $RuntimeDir 'wallets\bootstrap')
    Write-DemoLog "Waiting for bootstrap balance >= $MinBootstrapLovelace lovelace (150 ADA)"
    Write-DemoLog "Fund address via Preprod faucet: $FaucetUrl"
    Write-DemoLog "Bootstrap address: $addr"
    $attempt = 0
    while ($true) {
        $attempt++
        Write-DemoExt "balance poll attempt=$attempt"
        $result = Invoke-DnsCliJson -CliArgs @(
            'wallet', 'balance',
            '--config', $script:BootstrapConfig,
            '--actor', 'bootstrap'
        )
        $lovelace = [int64]$result.data.lovelace
        if ($lovelace -ge $MinBootstrapLovelace) {
            Write-DemoLog "Bootstrap funded: $lovelace lovelace"
            return
        }
        Write-DemoLog "Current lovelace=$lovelace; sleep ${PollSeconds}s (faucet: $FaucetUrl)"
        Start-Sleep -Seconds $PollSeconds
    }
}

function Ensure-ProofAndPrepare {
    # Generate HNS proof once; run system prepare (Aiken) once for deployment.json.
    Write-DemoExt 'Ensure-ProofAndPrepare: begin'
    $proofsDir = Join-Path $RuntimeDir 'proofs'
    $contractsDir = Join-Path $RuntimeDir 'contracts'
    foreach ($d in @($proofsDir, $contractsDir)) {
        if (-not (Test-Path -LiteralPath $d)) {
            New-Item -ItemType Directory -Path $d -Force | Out-Null
        }
    }

    $script:ProofBundle = Join-Path $proofsDir 'proof-bundle.json'
    if (-not (Test-Path -LiteralPath $script:ProofBundle)) {
        Write-DemoLog "Generating proof bundle for tld=$($script:EffectiveTld)"
        Invoke-DnsCliQuiet -CliArgs @(
            'proof', 'generate',
            '--tld', $script:EffectiveTld,
            '--out-dir', $proofsDir
        )
    }
    else {
        Write-DemoLog "Proof bundle exists: $($script:ProofBundle)"
    }

    $registrarHns = Join-Path $proofsDir 'registrar.hns'
    if (-not (Test-Path -LiteralPath $registrarHns)) {
        Throw-Demo 'missing registrar.hns after proof generate'
    }
    Write-DemoExt "registrar.hns=$registrarHns"

    $script:DeploymentJson = Join-Path $contractsDir 'deployment.json'
    if (-not (Test-Path -LiteralPath $script:DeploymentJson)) {
        $aiken = Get-Command aiken -ErrorAction SilentlyContinue
        if ($null -eq $aiken) {
            Throw-Demo 'aiken is required on PATH for fresh mode (system prepare)'
        }
        Write-DemoLog 'Running system prepare (aiken build + parameterize)'
        Write-DemoExt "aiken=$($aiken.Source)"
        Invoke-DnsCliQuiet -CliArgs @(
            'system', 'prepare',
            '--blueprint', (Join-Path $DemoRoot 'fixtures\contracts'),
            '--registrar-hns-key', $registrarHns,
            '--stake-key', (Join-Path $RuntimeDir 'wallets\bootstrap\stake.vkey'),
            '--network', 'preprod',
            '--out-dir', $contractsDir
        )
    }
    else {
        Write-DemoLog "Deployment exists: $($script:DeploymentJson)"
    }
}

function Show-Preflight {
    # Operator checkpoint before any signing/submit (allocations are fixed by design).
    Write-Host ''
    Write-Host '======== Preprod demo preflight ========'
    Write-Host "DEMO_ROOT:     $DemoRoot"
    Write-Host "CLI:           $($script:DnsCli)"
    Write-Host "logLevel:      $($script:LogLevelName) (dns-cli -v $($script:CliVerbose))"
    Write-Host "mode:          $($script:EffectiveMode)"
    Write-Host "provider:      $($script:EffectiveProvider)"
    Write-Host "tld / sld:     $($script:EffectiveTld) / $($script:EffectiveSld)"
    Write-Host "bootstrap cfg: $($script:BootstrapConfig)"
    Write-Host "deployment:    $($script:DeploymentJson)"
    Write-Host "proof:         $($script:ProofBundle)"
    Write-Host "records:       $RecordsFile"
    Write-Host "artifacts:     $(Join-Path $RuntimeDir 'artifacts')"
    Write-Host "state:         $StateFile"
    Write-Host "schema:        $StateSchema"
    Write-Host 'Funding:       registrar=30 ADA, tldOwner=50 ADA, sldOwner=30 ADA (+5 ADA collateral each)'
    Write-Host '========================================'
    Write-Host ''
}

function Confirm-Proceed {
    $ans = Read-Host 'Proceed with Preprod submissions? [y/N]'
    if ($ans -notmatch '^[Yy]$') {
        Write-DemoLog 'Aborted before submissions.'
        exit 0
    }
    Write-DemoExt 'operator confirmed; starting submissions'
}

function Get-BoundConfigPath {
    return (Join-Path $RuntimeDir "config\$($script:EffectiveProvider).json")
}

function Ensure-BoundConfig {
    # After deploy confirms, stamp policy IDs / ref UTxOs into the provider template.
    $out = Get-BoundConfigPath
    $deployTx = Get-StepTxId -Key 'deploy'
    if ([string]::IsNullOrWhiteSpace($deployTx)) {
        Throw-Demo 'cannot bind: deploy not confirmed'
    }
    $base = Join-Path $DemoRoot "config\$($script:EffectiveProvider).template.json"
    if (-not (Test-Path -LiteralPath $base)) {
        Throw-Demo "missing base template: $base"
    }
    Write-DemoLog "Binding provider config -> $out"
    Write-DemoExt "base=$base deployTx=$deployTx"
    Invoke-DnsCliQuiet -CliArgs @(
        'system', 'bind',
        '--base-config', $base,
        '--deployment', $script:DeploymentJson,
        '--tx-id', $deployTx,
        '--actor-dir', (Join-Path $RuntimeDir 'wallets'),
        '--provider', $script:EffectiveProvider,
        '--out', $out,
        '--force'
    )
    $script:BoundConfig = $out
}

# Shared submit cycle: sign -> submit -> status --wait, then persist state.
function Invoke-SubmitCycle {
    param(
        [string] $StepKey,
        [string] $Actor,
        [string] $Unsigned,
        [string] $Signed,
        [string] $Manifest,
        [string] $Config
    )

    Write-DemoExt "Invoke-SubmitCycle step=$StepKey actor=$Actor"
    Write-DemoExt "unsigned=$Unsigned signed=$Signed manifest=$Manifest config=$Config"
    foreach ($p in @($Unsigned, $Config)) {
        if (-not (Test-Path -LiteralPath $p)) {
            Throw-Demo "submit cycle missing path for ${StepKey}: $p"
        }
        if ($script:LogLevelName -eq 'Extensive') {
            $item = Get-Item -LiteralPath $p
            Write-DemoExt "exists $p size=$($item.Length)"
        }
    }

    Write-DemoLog "Signing $StepKey as $Actor"
    Invoke-DnsCliQuiet -CliArgs @(
        'tx', 'sign',
        '--config', $Config,
        '--tx', $Unsigned,
        '--actor', $Actor,
        '--out', $Signed
    )

    Write-DemoLog "Submitting $StepKey"
    $submit = Invoke-DnsCliJson -CliArgs @(
        'tx', 'submit',
        '--config', $Config,
        '--tx', $Signed
    )
    $txId = [string]$submit.txId
    if ([string]::IsNullOrWhiteSpace($txId)) {
        Throw-Demo "submit returned empty txId for $StepKey"
    }

    Write-DemoLog "Waiting for confirmation: $txId"
    Invoke-DnsCliQuiet -CliArgs @(
        'tx', 'status',
        '--config', $Config,
        '--tx-id', $txId,
        '--manifest', $Manifest,
        '--wait'
    )

    Set-StepConfirmed -Key $StepKey -TxId $txId -Manifest $Manifest
}

function Invoke-FreshSubmissions {
    # Ordered, resumable steps. Each skips when state.confirmed.<key>.txId is set.
    Write-DemoExt 'Invoke-FreshSubmissions: begin'
    $artifacts = Join-Path $RuntimeDir 'artifacts'
    if (-not (Test-Path -LiteralPath $artifacts)) {
        New-Item -ItemType Directory -Path $artifacts -Force | Out-Null
    }
    $config = $script:BootstrapConfig

    # --- fund: split bootstrap ADA to registrar / tldOwner / sldOwner ---
    if ([string]::IsNullOrWhiteSpace((Get-StepTxId -Key 'fund'))) {
        $prefix = Join-Path $artifacts '00-fund'
        Write-DemoLog 'Building wallet fund'
        Invoke-DnsCliQuiet -CliArgs @(
            'wallet', 'fund',
            '--config', $config,
            '--from-actor', 'bootstrap',
            '--allocation', 'registrar=30000000',
            '--allocation', 'tldOwner=50000000',
            '--allocation', 'sldOwner=30000000',
            '--collateral', '5000000',
            '--out', $prefix
        )
        Invoke-SubmitCycle -StepKey 'fund' -Actor 'bootstrap' `
            -Unsigned "$prefix.unsigned.json" -Signed "$prefix.signed.json" `
            -Manifest "$prefix.manifest.json" -Config $config
    }
    else {
        Write-DemoLog 'Skipping fund (already confirmed)'
        Write-DemoExt ("fund txId=" + (Get-StepTxId -Key 'fund'))
    }

    # --- deploy: publish parameterized reference scripts (system init) ---
    if ([string]::IsNullOrWhiteSpace((Get-StepTxId -Key 'deploy'))) {
        $prefix = Join-Path $artifacts '01-deploy'
        Write-DemoLog 'Building system init'
        Invoke-DnsCliQuiet -CliArgs @(
            'system', 'init',
            '--config', $config,
            '--deployment', $script:DeploymentJson,
            '--actor', 'bootstrap',
            '--out', $prefix
        )
        Invoke-SubmitCycle -StepKey 'deploy' -Actor 'bootstrap' `
            -Unsigned "$prefix.unsigned.json" -Signed "$prefix.signed.json" `
            -Manifest "$prefix.manifest.json" -Config $config
    }
    else {
        Write-DemoLog 'Skipping deploy (already confirmed)'
        Write-DemoExt ("deploy txId=" + (Get-StepTxId -Key 'deploy'))
    }

    # Bound config carries real policy IDs / ref UTxOs for later protocol txs.
    Ensure-BoundConfig
    $config = $script:BoundConfig

    # --- register TLD (registrar signature + HNS proof) ---
    if ([string]::IsNullOrWhiteSpace((Get-StepTxId -Key 'register'))) {
        $prefix = Join-Path $artifacts '02-register'
        Write-DemoLog 'Building register-tld'
        Invoke-DnsCliQuiet -CliArgs @(
            'registrar', 'register-tld',
            '--config', $config,
            '--tld', $script:EffectiveTld,
            '--proof', $script:ProofBundle,
            '--out', $prefix
        )
        Invoke-SubmitCycle -StepKey 'register' -Actor 'registrar' `
            -Unsigned "$prefix.unsigned.json" -Signed "$prefix.signed.json" `
            -Manifest "$prefix.manifest.json" -Config $config
    }
    else {
        Write-DemoLog 'Skipping register (already confirmed)'
        Write-DemoExt ("register txId=" + (Get-StepTxId -Key 'register'))
    }

    # --- activate TLD (owner claim) ---
    if ([string]::IsNullOrWhiteSpace((Get-StepTxId -Key 'activate'))) {
        $prefix = Join-Path $artifacts '03-activate'
        Write-DemoLog 'Building activate-tld'
        Invoke-DnsCliQuiet -CliArgs @(
            'owner', 'activate-tld',
            '--config', $config,
            '--tld', $script:EffectiveTld,
            '--proof', $script:ProofBundle,
            '--out', $prefix
        )
        Invoke-SubmitCycle -StepKey 'activate' -Actor 'tldOwner' `
            -Unsigned "$prefix.unsigned.json" -Signed "$prefix.signed.json" `
            -Manifest "$prefix.manifest.json" -Config $config
    }
    else {
        Write-DemoLog 'Skipping activate (already confirmed)'
        Write-DemoExt ("activate txId=" + (Get-StepTxId -Key 'activate'))
    }

    # --- mint SLD under the TLD ---
    if ([string]::IsNullOrWhiteSpace((Get-StepTxId -Key 'mintSld'))) {
        $prefix = Join-Path $artifacts '04-mint-sld'
        Write-DemoLog 'Building mint-sld'
        Invoke-DnsCliQuiet -CliArgs @(
            'owner', 'mint-sld',
            '--config', $config,
            '--tld', $script:EffectiveTld,
            '--sld', $script:EffectiveSld,
            '--sld-owner', 'sldOwner',
            '--out', $prefix
        )
        Invoke-SubmitCycle -StepKey 'mintSld' -Actor 'tldOwner' `
            -Unsigned "$prefix.unsigned.json" -Signed "$prefix.signed.json" `
            -Manifest "$prefix.manifest.json" -Config $config
    }
    else {
        Write-DemoLog 'Skipping mintSld (already confirmed)'
        Write-DemoExt ("mintSld txId=" + (Get-StepTxId -Key 'mintSld'))
    }

    # --- replace SLD DNS records from demo/records.json ---
    if ([string]::IsNullOrWhiteSpace((Get-StepTxId -Key 'updateSld'))) {
        if (-not (Test-Path -LiteralPath $RecordsFile)) {
            Throw-Demo "missing records file: $RecordsFile"
        }
        $prefix = Join-Path $artifacts '05-update-sld'
        Write-DemoLog 'Building update-sld'
        Write-DemoExt "records=$RecordsFile"
        Invoke-DnsCliQuiet -CliArgs @(
            'owner', 'update-sld',
            '--config', $config,
            '--tld', $script:EffectiveTld,
            '--sld', $script:EffectiveSld,
            '--records', $RecordsFile,
            '--out', $prefix
        )
        Invoke-SubmitCycle -StepKey 'updateSld' -Actor 'sldOwner' `
            -Unsigned "$prefix.unsigned.json" -Signed "$prefix.signed.json" `
            -Manifest "$prefix.manifest.json" -Config $config
    }
    else {
        Write-DemoLog 'Skipping updateSld (already confirmed)'
        Write-DemoExt ("updateSld txId=" + (Get-StepTxId -Key 'updateSld'))
    }
}

function Show-SuccessSummary {
    $updateTx = Get-StepTxId -Key 'updateSld'
    Write-Host ''
    Write-Host '======== Demo complete ========'
    Write-Host "tld / sld:  $($script:EffectiveTld) / $($script:EffectiveSld)"
    Write-Host "provider:   $($script:EffectiveProvider)"
    Write-Host "logLevel:   $($script:LogLevelName)"
    Write-Host "config:     $(Get-BoundConfigPath)"
    Write-Host "fund:       $(Get-StepTxId -Key 'fund')"
    Write-Host "deploy:     $(Get-StepTxId -Key 'deploy')"
    Write-Host "register:   $(Get-StepTxId -Key 'register')"
    Write-Host "activate:   $(Get-StepTxId -Key 'activate')"
    Write-Host "mintSld:    $(Get-StepTxId -Key 'mintSld')"
    Write-Host "updateSld:  $updateTx"
    Write-Host "state:      $StateFile"
    Write-Host "Explorer:   https://preprod.cexplorer.io/tx/$updateTx"
    Write-Host '==============================='
}

function Invoke-ExistingMode {
    # No chain writes — validate historical config and print fixture evidence.
    Write-DemoExt 'Invoke-ExistingMode: begin'
    $cfg = Join-Path $DemoRoot "config\existing.$($script:EffectiveProvider).json"
    if (-not (Test-Path -LiteralPath $cfg)) {
        Throw-Demo "missing existing config: $cfg"
    }
    Write-DemoLog "Existing mode: validating $cfg offline"
    try {
        Invoke-DnsCliQuiet -CliArgs @('config', 'validate', '--config', $cfg)
        Write-DemoLog 'Offline validation OK'
    }
    catch {
        Write-DemoLog 'WARNING: offline validation reported errors (historical fixture may share actor addresses).'
        Write-DemoExt "validation error: $_"
    }

    Write-Host ''
    Write-Host '======== Historical Preprod deployment ========'
    Write-Host "Config: $cfg"
    Write-Host 'Init tx: ef635b55fce6abc39cd4c843722d9d574cb719114e224f2cd1c8747d5abfc19e'
    Write-Host ''
    Write-Host '| Role          | Ref | Policy ID                                                      |'
    Write-Host '|---------------|-----|----------------------------------------------------------------|'
    Write-Host '| tldRegistrar  | #0  | ea32305e62561a0c0bb69588a936afb6fabd0fb4d2cc2a6c67363e9d |'
    Write-Host '| tldReference  | #1  | 694cb48da919e928b3e51c4648f051326ac150eaa9436792ec7a6e35 |'
    Write-Host '| sldReference  | #2  | 96512d4c426d912ba453014e74a57d655dfb3980154c4de106f69320 |'
    Write-Host ''
    Write-Host ("Addresses: " + (Join-Path $DemoRoot 'fixtures\preprod\validators\*.addr'))
    Write-Host ("Tx evidence: " + (Join-Path $DemoRoot 'fixtures\preprod\tx\') + ' (not replayable)')
    Write-Host 'No submissions performed in existing mode.'
    Write-Host '==============================================='
}

# =============================================================================
# main — orchestration
# =============================================================================
Initialize-DemoLogging
Write-DemoStartupBanner
Import-DemoEnvFile
Initialize-State

# Mode-aware tool/credential setup (prompt to install/set, else print guides).
Invoke-DemoPrerequisites -ModeName $script:EffectiveMode

if ($script:EffectiveMode -eq 'existing') {
    Invoke-ExistingMode
    exit 0
}

foreach ($dir in @(
        (Join-Path $RuntimeDir 'wallets'),
        (Join-Path $RuntimeDir 'config'),
        (Join-Path $RuntimeDir 'proofs'),
        (Join-Path $RuntimeDir 'contracts'),
        (Join-Path $RuntimeDir 'artifacts'),
        $ToolsDir
    )) {
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        Write-DemoExt "mkdir $dir"
    }
}

Ensure-Wallets
Write-BootstrapConfig
Wait-ForBootstrapFunds
Ensure-ProofAndPrepare
Show-Preflight
Confirm-Proceed
Invoke-FreshSubmissions
Show-SuccessSummary
Write-DemoExt 'main: complete'
