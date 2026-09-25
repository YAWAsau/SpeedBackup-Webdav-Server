$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
. (Join-Path $PSScriptRoot 'build_common.ps1')
Build-ServerInstaller $Root (Get-ServerVersion $Root)
