$ErrorActionPreference = "Stop"
$Here = Split-Path -Parent $MyInvocation.MyCommand.Path
$Bin = Join-Path $Here "speedbackup-server.exe"
$Root = if ($env:SPEEDBACKUP_SERVER_ROOT) { $env:SPEEDBACKUP_SERVER_ROOT } else { Join-Path $Here "SpeedBackupData" }
$Listen = if ($env:SPEEDBACKUP_SERVER_LISTEN) { $env:SPEEDBACKUP_SERVER_LISTEN } else { "0.0.0.0:8765" }
& $Bin serve --root $Root --listen $Listen
exit $LASTEXITCODE
