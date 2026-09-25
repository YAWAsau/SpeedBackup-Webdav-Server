$ErrorActionPreference = "Stop"

# Keep output readable on zh-TW/zh-CN Windows Terminal/PowerShell where sc.exe
# may otherwise mix OEM code pages with UTF-8 text.
try {
    [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
    $OutputEncoding = [Console]::OutputEncoding
} catch {}

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Restart-Elevated {
    $script = $PSCommandPath
    if (-not $script) { throw "Cannot determine script path for elevation" }
    Write-Host "SpeedBackup Server service installation requires Administrator privileges. Requesting UAC elevation..."
    $args = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", ('"' + $script + '"')
    ) -join " "
    $proc = Start-Process -FilePath "powershell.exe" -ArgumentList $args -Verb RunAs -Wait -PassThru
    if ($null -ne $proc -and $proc.ExitCode -ne 0) {
        throw "Elevated installer exited with code $($proc.ExitCode)"
    }
    exit 0
}

function Invoke-Checked {
    param(
        [Parameter(Mandatory=$true)][string]$FilePath,
        [Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments
    )
    Write-Host "> $FilePath $($Arguments -join ' ')"
    & $FilePath @Arguments
    $code = $LASTEXITCODE
    if ($null -ne $code -and $code -ne 0) {
        throw "Command failed with exit code ${code}: $FilePath $($Arguments -join ' ')"
    }
}

if (-not (Test-IsAdministrator)) {
    Restart-Elevated
}

$Exe = Join-Path $PSScriptRoot "speedbackup-server.exe"
if (-not (Test-Path -LiteralPath $Exe)) { throw "speedbackup-server.exe not found beside this script" }

$Data = Join-Path $env:ProgramData "SpeedBackup Server"
$Root = Join-Path $Data "data"
$TokenFile = Join-Path $Data "FIRST_RUN_TOKEN.txt"
New-Item -ItemType Directory -Force -Path $Data, $Root | Out-Null

Invoke-Checked $Exe "init" "--root" $Root "--token-file" $TokenFile
Invoke-Checked $Exe "service" "install" "--root" $Root "--listen" "0.0.0.0:8765" "--token-file" $TokenFile
$current = Get-CimInstance Win32_Service -Filter "Name='SpeedBackupServer'" -ErrorAction SilentlyContinue
$defaultAuto = -not $current -or $current.StartMode -eq 'Auto'
$defaultLabel = if ($defaultAuto) { 'Y' } else { 'N' }
$choice = Read-Host "Start automatically at boot? Y/N (Enter keeps $defaultLabel)"
if ($choice -notmatch '^(?i:y|n)?$') { throw 'Enter Y or N' }
$auto = if ($choice) { $choice -match '^(?i:y)$' } else { $defaultAuto }
Invoke-Checked $Exe "service" "autostart" "--enabled=$($auto.ToString().ToLowerInvariant())"
Invoke-Checked $Exe "service" "start"

Write-Host ""
Write-Host "SpeedBackup Server service installed and started."
Write-Host "WebAdmin: http://127.0.0.1:8765/web/admin"
if (Test-Path -LiteralPath $TokenFile) { Write-Host "First-run token file: $TokenFile" }
Write-Host ""
Write-Host "Check service status:"
Write-Host "  Get-Service SpeedBackupServer"
