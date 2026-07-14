# Resumable Preprod demo runner for dns-cli (PowerShell).
# Usage: .\run-demo.ps1 [-Mode fresh|existing] [-Provider blockfrost|utxorpc] [-Tld NAME] [-Sld NAME]
[CmdletBinding()]
param(
    [ValidateSet('fresh', 'existing')]
    [string] $Mode,

    [ValidateSet('blockfrost', 'utxorpc')]
    [string] $Provider,

    [string] $Tld,

    [string] $Sld
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$DemoRoot = $PSScriptRoot
$RuntimeDir = Join-Path $DemoRoot 'runtime'
$StateFile = Join-Path $RuntimeDir 'state.json'
$StateSchema = Join-Path $DemoRoot 'state.schema.json'
$RecordsFile = Join-Path $DemoRoot 'records.json'
$FaucetUrl = 'https://docs.cardano.org/cardano-testnets/tools/faucet/'
$MinBootstrapLovelace = [int64]150000000
$PollSeconds = 20

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

function Write-DemoLog {
    param([string] $Message)
    Write-Host "[demo] $Message"
}

function Throw-Demo {
    param([string] $Message)
    throw "[demo] ERROR: $Message"
}

function Resolve-Cli {
    if ($env:CLI) {
        $script:DnsCli = $env:CLI
    }
    else {
        $exe = Join-Path (Split-Path $DemoRoot -Parent) 'dns-cli.exe'
        $bin = Join-Path (Split-Path $DemoRoot -Parent) 'dns-cli'
        if (Test-Path -LiteralPath $exe) {
            $script:DnsCli = $exe
        }
        elseif (Test-Path -LiteralPath $bin) {
            $script:DnsCli = $bin
        }
        else {
            $cmd = Get-Command dns-cli -ErrorAction SilentlyContinue
            if ($null -eq $cmd) {
                Throw-Demo 'dns-cli binary not found. Set CLI=... or build ../dns-cli.exe'
            }
            $script:DnsCli = $cmd.Source
        }
    }
    if (-not (Test-Path -LiteralPath $script:DnsCli)) {
        Throw-Demo "CLI not found: $($script:DnsCli)"
    }
    Write-DemoLog "Using CLI: $($script:DnsCli)"
}

function Invoke-DnsCliJson {
    param(
        [Parameter(Mandatory = $true)]
        [string[]] $CliArgs
    )
    $argList = @($CliArgs) + @('--output', 'json')
    Write-DemoLog ("dns-cli " + ($CliArgs -join ' '))
    $stdout = & $script:DnsCli @argList
    if ($LASTEXITCODE -ne 0) {
        Throw-Demo "dns-cli failed (exit $LASTEXITCODE): $($CliArgs -join ' ')"
    }
    $text = if ($stdout -is [array]) { $stdout -join "`n" } else { [string]$stdout }
    if ([string]::IsNullOrWhiteSpace($text)) {
        return $null
    }
    return ($text | ConvertFrom-Json)
}

function Invoke-DnsCliQuiet {
    param(
        [Parameter(Mandatory = $true)]
        [string[]] $CliArgs
    )
    $null = Invoke-DnsCliJson -CliArgs $CliArgs
}

function Test-ProviderEnv {
    switch ($script:EffectiveProvider) {
        'blockfrost' {
            if ([string]::IsNullOrWhiteSpace($env:DNS_CLI_BLOCKFROST_PROJECT_ID)) {
                Throw-Demo 'DNS_CLI_BLOCKFROST_PROJECT_ID is required for -Provider blockfrost'
            }
        }
        'utxorpc' {
            if ([string]::IsNullOrWhiteSpace($env:DNS_CLI_UTXORPC_URL)) {
                Throw-Demo 'DNS_CLI_UTXORPC_URL is required for -Provider utxorpc'
            }
        }
        default { Throw-Demo "unsupported provider: $($script:EffectiveProvider)" }
    }
}

function New-EmptyStep {
    return [pscustomobject]@{ txId = ''; manifest = '' }
}

function New-DefaultState {
    param(
        [string] $TldName,
        [string] $SldName,
        [string] $ModeName,
        [string] $ProviderName
    )
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
    $tmp = "$StateFile.tmp"
    $dir = Split-Path -Parent $StateFile
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    $json = $script:State | ConvertTo-Json -Depth 10
    [System.IO.File]::WriteAllText($tmp, $json + "`n")
    Move-Item -LiteralPath $tmp -Destination $StateFile -Force
}

function Get-StepTxId {
    param([string] $Key)
    $confirmed = $script:State.confirmed
    $step = $confirmed | Select-Object -ExpandProperty $Key -ErrorAction SilentlyContinue
    if ($null -eq $step) { return '' }
    return [string]$step.txId
}

function Set-StepConfirmed {
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
}

function Initialize-State {
    if (-not (Test-Path -LiteralPath $RuntimeDir)) {
        New-Item -ItemType Directory -Path $RuntimeDir -Force | Out-Null
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
    }
    else {
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
    param([string] $WalletDir)
    $path = Join-Path $WalletDir 'payment.addr'
    if (-not (Test-Path -LiteralPath $path)) {
        Throw-Demo "missing payment.addr: $path"
    }
    $addr = (Get-Content -LiteralPath $path -Raw).Trim()
    if ([string]::IsNullOrWhiteSpace($addr)) {
        Throw-Demo "empty payment.addr in $WalletDir"
    }
    return $addr
}

function Ensure-Wallets {
    foreach ($name in @('bootstrap', 'registrar', 'tld-owner', 'sld-owner')) {
        $out = Join-Path $RuntimeDir "wallets\$name"
        $addr = Join-Path $out 'payment.addr'
        $skey = Join-Path $out 'payment.skey'
        if ((Test-Path -LiteralPath $addr) -and (Test-Path -LiteralPath $skey)) {
            Write-DemoLog "Wallet exists: $name"
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
    $addr = Read-PaymentAddr (Join-Path $RuntimeDir 'wallets\bootstrap')
    Write-DemoLog "Waiting for bootstrap balance >= $MinBootstrapLovelace lovelace (150 ADA)"
    Write-DemoLog "Fund address via Preprod faucet: $FaucetUrl"
    Write-DemoLog "Bootstrap address: $addr"
    while ($true) {
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

    $script:DeploymentJson = Join-Path $contractsDir 'deployment.json'
    if (-not (Test-Path -LiteralPath $script:DeploymentJson)) {
        $aiken = Get-Command aiken -ErrorAction SilentlyContinue
        if ($null -eq $aiken) {
            Throw-Demo 'aiken is required on PATH for fresh mode (system prepare)'
        }
        Write-DemoLog 'Running system prepare (aiken build + parameterize)'
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
    Write-Host ''
    Write-Host '======== Preprod demo preflight ========'
    Write-Host "DEMO_ROOT:     $DemoRoot"
    Write-Host "CLI:           $($script:DnsCli)"
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
}

function Get-BoundConfigPath {
    return (Join-Path $RuntimeDir "config\$($script:EffectiveProvider).json")
}

function Ensure-BoundConfig {
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
    $artifacts = Join-Path $RuntimeDir 'artifacts'
    if (-not (Test-Path -LiteralPath $artifacts)) {
        New-Item -ItemType Directory -Path $artifacts -Force | Out-Null
    }
    $config = $script:BootstrapConfig

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
    }

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
    }

    Ensure-BoundConfig
    $config = $script:BoundConfig

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
    }

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
    }

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
    }

    if ([string]::IsNullOrWhiteSpace((Get-StepTxId -Key 'updateSld'))) {
        if (-not (Test-Path -LiteralPath $RecordsFile)) {
            Throw-Demo "missing records file: $RecordsFile"
        }
        $prefix = Join-Path $artifacts '05-update-sld'
        Write-DemoLog 'Building update-sld'
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
    }
}

function Show-SuccessSummary {
    $updateTx = Get-StepTxId -Key 'updateSld'
    Write-Host ''
    Write-Host '======== Demo complete ========'
    Write-Host "tld / sld:  $($script:EffectiveTld) / $($script:EffectiveSld)"
    Write-Host "provider:   $($script:EffectiveProvider)"
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

# --- main ---
Resolve-Cli
Initialize-State

if ($script:EffectiveMode -eq 'existing') {
    Invoke-ExistingMode
    exit 0
}

Test-ProviderEnv

$aikenCmd = Get-Command aiken -ErrorAction SilentlyContinue
if ($null -eq $aikenCmd) {
    Throw-Demo 'aiken is required on PATH for fresh mode'
}

foreach ($dir in @(
        (Join-Path $RuntimeDir 'wallets'),
        (Join-Path $RuntimeDir 'config'),
        (Join-Path $RuntimeDir 'proofs'),
        (Join-Path $RuntimeDir 'contracts'),
        (Join-Path $RuntimeDir 'artifacts')
    )) {
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
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
