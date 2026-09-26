param([switch]$RequireInstaller)
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
. (Join-Path $PSScriptRoot 'packaging/windows/build_common.ps1')
$Version = Get-ServerVersion $PSScriptRoot
New-Item -ItemType Directory -Force -Path 'dist/windows-amd64' | Out-Null
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
go build -buildvcs=false -trimpath -ldflags='-s -w' -o 'dist/windows-amd64/speedbackup-server.exe' ./cmd/speedbackup-server
if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
go build -buildvcs=false -trimpath -ldflags='-s -w -H=windowsgui' -o 'dist/windows-amd64/speedbackup-picker.exe' ./cmd/speedbackup-picker
if ($LASTEXITCODE -ne 0) { throw 'Folder picker build failed' }
Copy-Item -LiteralPath 'packaging/windows/install_service_admin.ps1','README.md','LICENSE','THIRD_PARTY_NOTICES.txt','start_windows.ps1','reset_admin.ps1' -Destination 'dist/windows-amd64' -Force
if ((Resolve-IsccPath) -or $RequireInstaller) {
    Build-ServerInstaller $PSScriptRoot $Version
} else {
    Write-Host 'Portable EXE built. Install Inno Setup 6.7+ or set ISCC to build the installer.'
}
Get-FileHash -Algorithm SHA256 'dist/windows-amd64/speedbackup-server.exe'
