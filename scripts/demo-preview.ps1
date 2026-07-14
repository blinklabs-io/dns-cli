$ErrorActionPreference = "Stop"
$Cli = if ($env:CLI) { $env:CLI } else { ".\dns-cli.exe" }
$Config = if ($env:CONFIG) { $env:CONFIG } else { "dns-cli.json" }
$Tld = if ($env:TLD) { $env:TLD } else { "demo-$(Get-Date -UFormat %s)" }
$Proof = if ($env:PROOF) { $env:PROOF } else { "examples\proof-bundle.json" }
$Out = if ($env:OUT) { $env:OUT } else { "artifacts\demo" }

& $Cli config validate --config $Config
& $Cli registrar register-tld --config $Config --tld $Tld --proof $Proof --out "$Out-register"
Write-Host "Built: $Out-register.unsigned.json (sign and submit before continuing)"
