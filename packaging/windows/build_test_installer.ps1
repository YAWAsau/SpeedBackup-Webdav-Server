$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
. (Join-Path $PSScriptRoot 'build_common.ps1')
$Version = Get-ServerVersion $Root
$Compiler = Resolve-IsccPath
if (-not $Compiler) { throw 'Set ISCC to Inno Setup 6.7+ compiler.' }
Push-Location $Root
try {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $flags = '-s -w -X speedbackup-server/internal/desktop.Protocol=speedbackup-picker-review -X speedbackup-server/internal/service.WindowsServiceName=SpeedBackupServerReview -X "speedbackup-server/internal/service.WindowsServiceDisplayName=SpeedBackup Server Review"'
    go build -trimpath "-ldflags=$flags" -o dist/windows-review/speedbackup-server.exe ./cmd/speedbackup-server
    if ($LASTEXITCODE -ne 0) { throw 'Test binary build failed' }
    go build -trimpath "-ldflags=$flags -H=windowsgui" -o dist/windows-review/speedbackup-picker.exe ./cmd/speedbackup-picker
    if ($LASTEXITCODE -ne 0) { throw 'Test picker build failed' }
    & $Compiler /Q '/DMyPickerProtocol=speedbackup-picker-review'  "/DMyAppVersion=$Version" '/DMyAppName=SpeedBackup Server Review' '/DMyServiceName=SpeedBackupServerReview' '/DMyAppId={{7B5EC9B2-EFB8-47ED-9C86-D754E94B95B6}' '/DMyBinaryDir=..\..\dist\windows-review' '/DMyOutputDir=..\..\dist\windows-review-setup' packaging/windows/speedbackup-server.iss
    if ($LASTEXITCODE -ne 0) { throw 'Test installer build failed' }
} finally { Pop-Location }
