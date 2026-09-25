$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
& (Join-Path $Root 'build_windows.ps1') -RequireInstaller
