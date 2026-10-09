function Resolve-IsccPath {
    $candidates = @($env:ISCC)
    $command = Get-Command iscc.exe -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($command) { $candidates += $command.Source }
    foreach ($base in @(${env:ProgramFiles(x86)}, $env:ProgramFiles, (Join-Path $env:LOCALAPPDATA 'Programs'))) {
        if ($base) {
            $candidates += (Join-Path $base 'Inno Setup 6/ISCC.exe')
            $candidates += (Join-Path $base 'Inno Setup 7/ISCC.exe')
        }
    }
    foreach ($path in $candidates) {
        if ($path -and (Test-Path -LiteralPath $path)) { return (Resolve-Path -LiteralPath $path).Path }
    }
    return $null
}

function Get-ServerVersion([string]$Root) {
    $line = Select-String -LiteralPath (Join-Path $Root 'internal/sbserver/model.go') -Pattern 'ServerVersion\s*=\s*"([^"]+)"' | Select-Object -First 1
    if (-not $line) { throw 'Cannot determine ServerVersion' }
    return $line.Matches[0].Groups[1].Value
}

function Build-NativePowerPrompt([string]$Root, [string]$Output) {
    $csc = Join-Path $env:WINDIR 'Microsoft.NET/Framework64/v4.0.30319/csc.exe'
    if (-not (Test-Path -LiteralPath $csc)) { throw 'Windows .NET Framework compiler is required for the native notification helper.' }
    $font = Join-Path $Root 'internal\sbserver\web\oppo-sans-4-6c7d5864c661.ttf'
    $source = Join-Path $Root 'desktop\PowerPrompt.cs'
    & $csc /nologo /target:winexe /platform:x64 /optimize+ "/out:$Output" /reference:System.Drawing.dll /reference:System.Windows.Forms.dll /reference:System.Net.Http.dll /reference:System.Web.Extensions.dll "/resource:$font,OPPO.ttf" $source
    if ($LASTEXITCODE -ne 0) { throw 'Native notification helper build failed' }
}

function Build-ServerInstaller([string]$Root, [string]$Version) {
    $compiler = Resolve-IsccPath
    if (-not $compiler) { throw 'Install Inno Setup 6.7 or newer, or set ISCC to the compiler path.' }
    $exe = Join-Path $Root 'dist/windows-amd64/speedbackup-server.exe'
    $actual = & $exe version
    if ($LASTEXITCODE -ne 0 -or $actual -notlike "SpeedBackup Server $Version protocol *") {
        throw 'Prebuilt EXE version does not match the source; rebuild before packaging.'
    }
    & $compiler /Q "/DMyAppVersion=$Version" (Join-Path $Root 'packaging/windows/speedbackup-server.iss')
    if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed: $LASTEXITCODE" }
    Get-FileHash -Algorithm SHA256 (Join-Path $Root "dist/windows-setup/SpeedBackup_Server_Setup_v${Version}_windows_x64.exe")
}
