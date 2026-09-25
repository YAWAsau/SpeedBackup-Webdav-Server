param(
    [Parameter(Mandatory=$true)][string]$Setup,
    [Parameter(Mandatory=$true)][string]$TestRoot,
    [string]$LegacyExe
)
$ErrorActionPreference = 'Stop'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
if (-not ([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'This isolated installation test requires Administrator rights.'
}
$ServiceName = 'SpeedBackupServerReview'
if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) { throw 'Test service already exists; refusing to overwrite it.' }
if (Test-Path -LiteralPath $TestRoot) { throw 'Test directory already exists; use a new empty path.' }
$null = New-Item -ItemType Directory -Path $TestRoot
$Install = Join-Path $TestRoot 'Program Files 測試'
$Data = Join-Path $TestRoot 'Backup Data 測試'
$Exe = Join-Path $Install 'speedbackup-server.exe'
$Port = 18765
$Results = [ordered]@{}
$Setup = (Resolve-Path -LiteralPath $Setup).Path

function Run-Exe([string]$File, [string[]]$Arguments) {
    & $File @Arguments | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "$File failed: $LASTEXITCODE" }
}
function Run-Setup([string]$Stage, [switch]$ExpectFailure, [string]$AutoStart = '') {
    $log = Join-Path $TestRoot ($Stage + '.log')
    $setupArgs = @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/SP-',
        ('/DIR="' + $Install + '"'), ('/DATA="' + $Data + '"'), ('/PORT=' + $Port),
        ('/LOG="' + $log + '"'))
    if ($AutoStart) { $setupArgs += ('/AUTOSTART=' + $AutoStart) }
    $process = Start-Process -FilePath $Setup -ArgumentList $setupArgs -WindowStyle Hidden -Wait -PassThru
    if ($ExpectFailure) {
        if ($process.ExitCode -eq 0) { throw 'Installer reported success while service could not start' }
        return
    }
    if ($process.ExitCode -ne 0) { throw "$Stage failed: $($process.ExitCode); see $log" }
    if ((Get-Service $ServiceName).Status -ne 'Running') { throw "$Stage service not running" }
    $caps = Invoke-RestMethod "http://127.0.0.1:$Port/api/v1/capabilities"
    if ($caps.server -ne 'SpeedBackup Server') { throw 'Unexpected HTTP server' }
}
try {
    Run-Setup 'fresh-install'
    $Results.fresh_install_and_start = $true
    if (-not (Test-Path -LiteralPath (Join-Path $Install 'speedbackup-picker.exe'))) { throw 'Native picker missing' }
    $origin = "http://127.0.0.1:$Port"
    $headers = @{'X-SB-Admin'='1';'Origin'=$origin}
    $login = @{username='review';password='x'} | ConvertTo-Json
    $null = Invoke-RestMethod "$origin/api/v1/auth/setup" -Method Post -Headers $headers -Body $login -ContentType application/json -SessionVariable AdminSession
    $state = Invoke-RestMethod "$origin/api/v1/admin/autostart" -WebSession $AdminSession
    if (-not $state.manageable) { throw 'Installed service cannot manage its boot policy' }
    $pidBefore = (Get-CimInstance Win32_Service -Filter "Name='$ServiceName'").ProcessId
    foreach ($enabled in @($false,$true)) {
        $body = @{enabled=$enabled} | ConvertTo-Json
        $state = Invoke-RestMethod "$origin/api/v1/admin/autostart" -Method Post -Headers $headers -Body $body -ContentType application/json -WebSession $AdminSession
        if ($state.enabled -ne $enabled) { throw 'Autostart API readback mismatch' }
        if ((Get-CimInstance Win32_Service -Filter "Name='$ServiceName'").ProcessId -ne $pidBefore) { throw 'Autostart API restarted service' }
    }
    $picker = Invoke-RestMethod "$origin/api/v1/admin/directories/picker" -WebSession $AdminSession
    if (-not $picker.available) { throw 'Installed native picker unavailable' }
    $job = Invoke-RestMethod "$origin/api/v1/admin/directories/picker" -Method Post -Headers $headers -Body (@{directory=$Data}|ConvertTo-Json) -ContentType application/json -WebSession $AdminSession
    if (-not $job.launch_url.StartsWith('speedbackup-picker-review://choose/?port=')) { throw 'Test protocol not isolated or not canonical' }
    $null = Invoke-RestMethod "$origin/api/v1/native-picker/$($job.id)"
    $null = Invoke-RestMethod "$origin/api/v1/native-picker/$($job.id)" -Method Post -Body (@{path=$Data}|ConvertTo-Json) -ContentType application/json
    $selected = Invoke-RestMethod "$origin/api/v1/admin/directories/picker/$($job.id)" -WebSession $AdminSession
    if ($selected.state -ne 'selected' -or $selected.path -ne $Data) { throw 'Native callback did not reach administrator' }
    $Results.admin_autostart_and_picker_bridge = $true

    if ((Get-Service $ServiceName).StartType -ne 'Automatic') { throw 'Fresh startup default is not automatic' }
    Run-Exe $Exe @('service','autostart','--enabled=false')
    if ((Get-Service $ServiceName).Status -ne 'Running') { throw 'Boot choice stopped current service' }
    Run-Setup 'upgrade-manual-startup'
    if ((Get-Service $ServiceName).StartType -ne 'Manual') { throw 'Upgrade lost manual startup' }
    Run-Setup 'choose-automatic-startup' -AutoStart 'true'
    if ((Get-Service $ServiceName).StartType -ne 'Automatic') { throw 'Explicit startup selection was ignored' }
    $Results.startup_selection_and_upgrade_preservation = $true
    $tokenPath = Join-Path $Data 'FIRST_RUN_TOKEN.txt'
    $token = [IO.File]::ReadAllText($tokenPath).Trim()
    if (-not $token.StartsWith('sb1_')) { throw 'Token file missing or invalid' }
    $env:BASE_URL = "http://127.0.0.1:$Port"
    $env:SPEEDBACKUP_SERVER_TOKEN = $token
    & (Join-Path $PSScriptRoot 'smoke_windows.ps1')
    $Results.api_upload_commit_download = $true
    Run-Exe $Exe @('service','start')
    Run-Exe $Exe @('service','stop')
    Run-Exe $Exe @('service','stop')
    $Results.idempotent_start_stop = $true
    $listener = [Net.Sockets.TcpListener]::Create($Port)
    $listener.ExclusiveAddressUse = $true
    $listener.Start()
    try {
        $failedStart = Start-Process -FilePath $Exe -ArgumentList @('service','start') -WindowStyle Hidden -Wait -PassThru -RedirectStandardError (Join-Path $TestRoot 'expected-port-conflict.txt')
        if ($failedStart.ExitCode -eq 0) { throw 'Service start incorrectly succeeded on occupied port' }
        $Results.port_conflict_reports_failure = $true
        Run-Setup 'expected-install-conflict' -ExpectFailure
        $Results.installer_failure_exit_code = $true
    } finally { $listener.Stop() }
    Run-Exe $Exe @('service','start')
    if ($LegacyExe) {
        Run-Exe $Exe @('service','stop')
        Copy-Item -LiteralPath $LegacyExe -Destination $Exe -Force
        Run-Exe $Exe @('service','start')
        $Results.legacy_binary_started = $true
    }
    Run-Setup 'upgrade-reinstall'
    if ([IO.File]::ReadAllText($tokenPath).Trim() -ne $token) { throw 'Upgrade changed token' }
    $Results.upgrade_preserves_token = $true
    if ($LegacyExe) { $Results.go5_upgrade = $true }
    $before = @(Get-ChildItem -LiteralPath (Join-Path $Data 'data/.speedbackup-server/manifests') -Recurse -File).Count
    if ($before -eq 0) { throw 'Smoke manifest missing before uninstall' }
    $uninstall = Join-Path $Install 'unins000.exe'
    $process = Start-Process -FilePath $uninstall -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART') -WindowStyle Hidden -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw 'Uninstaller failed' }
    if (Get-Service $ServiceName -ErrorAction SilentlyContinue) { throw 'Service survived uninstall' }
    if ((Test-Path -LiteralPath $Exe) -or -not (Test-Path -LiteralPath $tokenPath)) { throw 'Uninstall retention contract failed' }
    $after = @(Get-ChildItem -LiteralPath (Join-Path $Data 'data/.speedbackup-server/manifests') -Recurse -File).Count
    if ($before -ne $after) { throw 'Uninstall removed backup manifests' }
    if (Test-Path 'HKLM:\Software\Classes\speedbackup-picker-review') { throw 'Picker protocol survived uninstall' }
    $Results.uninstall_preserves_data = $true
} catch {
    $Results.error = $_.Exception.Message
    throw
} finally {
    $env:SPEEDBACKUP_SERVER_TOKEN = ''
    $Results | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $TestRoot 'results.json') -Encoding UTF8
    # Only the independently named test service may be cleaned up here.
    if (Get-Service $ServiceName -ErrorAction SilentlyContinue) {
        & $Exe service uninstall | Out-Null
    }
    if ($Results.uninstall_preserves_data) {
        $key = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::LocalMachine,[Microsoft.Win32.RegistryView]::Registry64)
        $key.DeleteSubKeyTree('Software\SpeedBackup Server Review',$false)
        $key.Close()
    }
}
$Results | ConvertTo-Json
