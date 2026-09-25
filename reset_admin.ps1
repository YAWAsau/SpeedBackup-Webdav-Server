param(
    [Parameter(Mandatory=$true)][string]$Root,
    [string]$Username = 'admin'
)
$ErrorActionPreference = 'Stop'
$exe = Join-Path $PSScriptRoot 'speedbackup-server.exe'
if (-not (Test-Path -LiteralPath $exe)) { throw 'Place this script beside speedbackup-server.exe.' }
Write-Host 'Stop the server/service first. Backup accounts and files will be preserved.'
$secret = Read-Host 'New administrator password' -AsSecureString
$buffer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secret)
$previousEncoding = $OutputEncoding
try {
    $OutputEncoding = [Text.UTF8Encoding]::new($false)
    $plain = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($buffer)
    $plain | & $exe admin-reset --root $Root --username $Username --password-stdin
    if ($LASTEXITCODE -ne 0) { throw 'Administrator reset failed. Check the data root and stop the running server.' }
} finally {
    $OutputEncoding = $previousEncoding
    $plain = $null
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($buffer)
    $secret.Dispose()
}
