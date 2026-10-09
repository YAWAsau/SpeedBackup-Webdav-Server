param([string]$OutputDirectory)
$ErrorActionPreference='Stop'
$Root=(Resolve-Path (Join-Path $PSScriptRoot '..')).Path
if (-not $OutputDirectory) { $OutputDirectory=Join-Path $Root 'dist/native-power-evidence' }
$OutputDirectory=[IO.Path]::GetFullPath($OutputDirectory)
$exe=Join-Path $Root 'dist/windows-amd64/speedbackup-notify.exe'
$process=Start-Process -FilePath $exe -ArgumentList @('--render-preview',('"'+$OutputDirectory+'"')) -WindowStyle Hidden -PassThru -Wait
if ($process.ExitCode -ne 0) { throw "Native prompt test failed: $($process.ExitCode)" }
$report=Get-Content -LiteralPath (Join-Path $OutputDirectory 'native-check.txt') -Raw
if ($report -notmatch 'Defer / Cancel / Simulated shutdown: PASS' -or $report -notmatch 'OPPO Sans 4.0') { throw 'Native behavior/font validation missing' }
foreach ($theme in @('white','dark','black','system')) {
    if (-not (Test-Path -LiteralPath (Join-Path $OutputDirectory "native-$theme.png"))) { throw "Missing native render: $theme" }
}
$references=[Reflection.Assembly]::ReflectionOnlyLoadFrom($exe).GetReferencedAssemblies().Name
if ($references -match 'WebView|Cef|Chromium') { throw 'Browser runtime dependency detected' }
Write-Output $report
