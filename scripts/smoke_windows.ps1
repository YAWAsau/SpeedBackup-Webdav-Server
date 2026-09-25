$ErrorActionPreference = "Stop"
$BaseUrl = if ($env:BASE_URL) { $env:BASE_URL } else { "http://127.0.0.1:8765" }
$Token = $env:SPEEDBACKUP_SERVER_TOKEN
if (-not $Token) { throw "set SPEEDBACKUP_SERVER_TOKEN" }
$Headers = @{ Authorization = "Bearer $Token" }
$Device = "smoke-" + [guid]::NewGuid().ToString("N")
$Profile = "default"

function Hex-Sha256([byte[]]$Bytes) {
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $hash = $sha.ComputeHash($Bytes)
        return (($hash | ForEach-Object { $_.ToString("x2") }) -join "")
    } finally { $sha.Dispose() }
}

function Send-Chunk([string]$SessionId, [string]$Hash, [byte[]]$Bytes, [long]$Offset, [long]$Total) {
    Add-Type -AssemblyName System.Net.Http
    $client = [System.Net.Http.HttpClient]::new()
    try {
        $req = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::new("PATCH"), "$BaseUrl/api/v1/sessions/$SessionId/objects/$Hash")
        $req.Headers.Authorization = [System.Net.Http.Headers.AuthenticationHeaderValue]::new("Bearer", $Token)
        $req.Headers.Add("Upload-Offset", [string]$Offset)
        $req.Headers.Add("Upload-Length", [string]$Total)
        $content = [System.Net.Http.ByteArrayContent]::new($Bytes)
        $content.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::new("application/octet-stream")
        $req.Content = $content
        $res = $client.SendAsync($req).GetAwaiter().GetResult()
        $text = $res.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        if (-not $res.IsSuccessStatusCode) { throw "PATCH failed: $([int]$res.StatusCode) $text" }
        return ($text | ConvertFrom-Json)
    } finally { $client.Dispose() }
}

Write-Host "[1/9] capabilities + status/auth"
$caps = Invoke-RestMethod "$BaseUrl/api/v1/capabilities"
if ($caps.server -ne "SpeedBackup Server") { throw "unexpected capabilities response" }
$status = Invoke-RestMethod "$BaseUrl/api/v1/status" -Headers $Headers

Write-Host "[2/9] create isolated session"
$body = @{ device_id = $Device; profile_id = $Profile; base_generation = $null } | ConvertTo-Json
$s = Invoke-RestMethod "$BaseUrl/api/v1/sessions" -Method Post -Headers $Headers -ContentType "application/json" -Body $body
$SessionId = $s.session.id

Write-Host "[3/9] resumable PATCH part 1"
$Payload = [Text.Encoding]::UTF8.GetBytes("SpeedBackup Server resumable smoke payload 0123456789abcdefghijklmnopqrstuvwxyz")
$Size = [long]$Payload.Length
$Hash = Hex-Sha256 $Payload
$First = [int][Math]::Floor($Payload.Length / 2)
$Chunk1 = New-Object byte[] $First
[Array]::Copy($Payload, 0, $Chunk1, 0, $First)
$Chunk2 = New-Object byte[] ($Payload.Length - $First)
[Array]::Copy($Payload, $First, $Chunk2, 0, $Chunk2.Length)
$r1 = Send-Chunk $SessionId $Hash $Chunk1 0 $Size
if ($r1.complete) { throw "part 1 unexpectedly complete" }

Write-Host "[4/9] resumable PATCH part 2"
$r2 = Send-Chunk $SessionId $Hash $Chunk2 $First $Size
if (-not $r2.complete) { throw "part 2 did not complete" }

Write-Host "[5/9] verify"
$v = Invoke-RestMethod "$BaseUrl/api/v1/sessions/$SessionId/verify" -Method Post -Headers $Headers
if (-not $v.verified) { throw "verify failed" }

Write-Host "[6/9] commit"
$commitObject = @{
    commit_id = "smoke-commit"
    base_generation = $null
    entries = @(@{ path = "smoke/payload.bin"; size = $Size; sha256 = $Hash; kind = "other" })
}
$commitBody = $commitObject | ConvertTo-Json -Depth 8
$c = Invoke-RestMethod "$BaseUrl/api/v1/sessions/$SessionId/commit" -Method Post -Headers $Headers -ContentType "application/json" -Body $commitBody
if (-not $c.committed) { throw "commit failed" }

Write-Host "[7/9] idempotent replay"
$c2 = Invoke-RestMethod "$BaseUrl/api/v1/sessions/$SessionId/commit" -Method Post -Headers $Headers -ContentType "application/json" -Body $commitBody
if (-not $c2.idempotent_replay) { throw "commit replay was not idempotent" }

Write-Host "[8/9] manifest + download hash"
$m = Invoke-RestMethod "$BaseUrl/api/v1/manifests/current?device_id=$Device&profile_id=$Profile" -Headers $Headers
if ($m.entries[0].sha256 -ne $Hash) { throw "manifest hash mismatch" }
$tmp = [IO.Path]::GetTempFileName()
try {
    Invoke-WebRequest "$BaseUrl/api/v1/objects/$Hash" -Headers $Headers -OutFile $tmp | Out-Null
    $downHash = Hex-Sha256 ([IO.File]::ReadAllBytes($tmp))
    if ($downHash -ne $Hash) { throw "download hash mismatch" }
} finally { Remove-Item $tmp -Force -ErrorAction SilentlyContinue }

Write-Host "[9/9] safe cleanup dry-run"
$cleanup = Invoke-RestMethod "$BaseUrl/api/v1/admin/cleanup" -Method Post -Headers $Headers -ContentType "application/json" -Body '{"dry_run":true}'
if (-not $cleanup.dry_run) { throw "cleanup dry-run flag mismatch" }
Write-Host "SMOKE OK device=$Device sha256=$Hash"
