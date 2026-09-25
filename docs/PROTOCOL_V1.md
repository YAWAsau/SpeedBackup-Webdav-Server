# SpeedBackup Server Protocol v1

狀態：`v1.0 draft / server v0.2.0-go2 implemented subset`  
手機端基底：SpeedBackup r647（尚未接入此 backend）

## 1. 目標

SpeedBackup Server 是 SpeedBackup 專用遠端儲存 backend。它不是 WebDAV server，也不模擬 WebDAV。

核心目標：

- 明確協議能力，不做 vendor 猜測。
- server-side `size + SHA-256` 完整性驗證。
- session staging，未 commit 資料永不成為正式 generation。
- immutable generation manifest；commit 原子發布。
- 可續傳 object upload。
- manifest diff 由 server 回覆。
- generation guard 防止舊 client 覆蓋新 generation。
- app_details seedless/taint policy 可由 server 統一計算。
- 多裝置、多 profile namespace 隔離。

## 2. 傳輸與認證

### 2.1 HTTP/S

v1 支援 HTTP。正式跨不可信網路部署應由 HTTPS reverse proxy 或後續 server native TLS 提供 TLS。

### 2.2 Bearer Token

除 `GET /api/v1/capabilities` 與 WebAdmin static files 外，API 都要求：

```http
Authorization: Bearer sb1_<random-token>
```

server 只保存 token 的 SHA-256，不保存明文 token。

### 2.3 Namespace

client 提供：

```text
device_id
profile_id
```

v1 僅允許 ASCII：`A-Z a-z 0-9 . _ -`，長度 1..96。

邏輯 object path 使用 `/`，必須是 relative path，禁止：

```text
/absolute
\\windows\\path
..
.
empty // segment
control characters
```

client path 永遠不直接拼接為 server filesystem path；正式物件使用 CAS SHA-256 儲存。

## 3. Storage model

```text
BackupRoot/
└─ .speedbackup-server/
   ├─ config/
   │  └─ 00000000000000000001.json ...
   ├─ server.lock
   ├─ sessions/
   │  └─ <session_id>/
   │     ├─ meta/
   │     │  └─ 00000000000000000001.json ...
   │     └─ objects/
   │        ├─ <sha256>.part
   │        └─ <sha256>.ready
   ├─ manifests/
   │  └─ <device_id>/<profile_id>/generations/
   │     ├─ 00000000000000000001.json
   │     ├─ 00000000000000000002.json
   │     └─ ...
   ├─ objects/
   │  └─ <sha[0:2]>/<sha[2:4]>/<64hex>
   └─ audit/
      └─ events.jsonl
```

### 3.1 為什麼沒有 mutable `current` pointer

Windows 與 Linux 對 rename-over-existing / symlink 行為不同。v1 不更新 `current.json`；目前 generation 定義為：

```text
max(valid immutable generation filename)
```

commit 最後一步只建立新的、原先不存在的 generation manifest。

因此 crash 邊界：

- 上傳中 crash：只留 `.part`。
- hash 完成後 crash：只留 `.ready`。
- CAS promote 後、generation publish 前 crash：只留 orphan CAS blob，可安全 GC。
- generation manifest atomic rename 完成：該 generation 正式可見。
- manifest publish 後、session state 寫回前 crash：相同 `session_id + commit_id` 重試會視為 idempotent replay。

不會出現「半個正式 generation」。

## 4. Capabilities

```http
GET /api/v1/capabilities
```

不需 Token。server v0.2.0-go2：

```json
{
  "server": "SpeedBackup Server",
  "version": "0.2.0-go2",
  "protocol": { "major": 1, "minor": 0 },
  "features": {
    "atomic_publish": true,
    "content_addressed_objects": true,
    "resumable_upload": true,
    "sha256_verify": true,
    "manifest_diff": true,
    "generation_guard": true,
    "appdetails_audit": true,
    "orphan_cleanup_safe": true,
    "append_only_metadata": true,
    "web_admin": true,
    "zh_tw_zh_cn": true
  }
}
```

兼容規則：

- protocol major 不同：client 必須拒絕。
- major 相同、minor 不同：依 feature capability 決策。
- 不允許用 Server header / port / URL pattern 猜功能。

## 5. ManifestEntry

```json
{
  "path": "app/com.example/base.tar.zst",
  "size": 1234567,
  "sha256": "64hex...",
  "kind": "app_payload",
  "app_id": "com.example"
}
```

`kind` / `app_id` 是 optional metadata；hash/size/path 是完整性與 generation contract。

建議 `kind`：

```text
app_payload
app_details
wifi
media
settings
other
```

## 6. Manifest diff

```http
POST /api/v1/manifests/diff
Authorization: Bearer ...
Content-Type: application/json
```

Request：

```json
{
  "device_id": "xiaomi13",
  "profile_id": "default",
  "entries": [ ... ]
}
```

Response：

```json
{
  "base_generation": 18,
  "upload_required_sha256": ["..."],
  "unchanged_paths": ["..."],
  "changed_paths": ["..."],
  "remote_only_paths": ["..."]
}
```

注意：`changed_paths` 不等於 `upload_required`。如果新 path 指向 CAS 已存在 hash，client 不需重傳 blob。

## 7. Session

### 7.1 建立

```http
POST /api/v1/sessions
```

```json
{
  "device_id": "xiaomi13",
  "profile_id": "default",
  "base_generation": 18
}
```

`base_generation` 必須和 server current generation 完全一致；首次備份為 `null`。

不一致回 `409 Conflict`。

### 7.2 查詢

```http
GET /api/v1/sessions/{session_id}
```

## 8. Object upload

Object identity 是 SHA-256，而不是 remote path。

### 8.1 單次 PUT

```http
PUT /api/v1/sessions/{session_id}/objects/{sha256}
Upload-Length: 1234567
Content-Type: application/octet-stream
```

也接受 `X-SpeedBackup-Size` 或 `Content-Length` 作為總長度。

完成時 server 重新讀檔計算 SHA-256；不相等回 `409` 並刪除 `.part`。

### 8.2 續傳 PATCH

先查：

```http
HEAD /api/v1/sessions/{session_id}/objects/{sha256}
```

Response headers：

```text
Upload-Offset: <received bytes>
Upload-Length: <total bytes>
Upload-Complete: 0|1
```

續傳：

```http
PATCH /api/v1/sessions/{session_id}/objects/{sha256}
Upload-Offset: <exact current offset>
Upload-Length: <total size>
```

server 只接受 append-at-exact-offset；offset 不一致回 `409`，client 必須重新 HEAD，不可盲 append。

### 8.3 Session 內並行上傳

v0.2.0-go2 允許同一 session 的不同 SHA-256 object 並行上傳；同一 object 由 object lock 串行化。

Session finalization 使用 read/write gate：

```text
upload                   -> shared/read gate
verify / commit / delete  -> exclusive/write gate
```

因此 verify/commit/delete 一定等待正在傳輸的 object request 結束，不會在 upload body 尚未完成時驗證、發布或刪除 session。每個 upload request 在開始接收 body **之前**先把 `expected_size` 寫入新的 append-only session metadata version；若 request body 中途斷線，HEAD 用 `.part` 實際檔案大小回 offset、用 session metadata 回原始 total。每次 upload 進度落盤時，uploads map 都在 metadata lock 下重新讀取、合併，再發布新的 immutable metadata version，避免並行 object 完成時互相覆蓋進度。

同一 SHA-256 object 的 `Upload-Length` 在 session 內固定；續傳時不得改變。`.part` 若已大於宣告總長度或 `.ready` 大小與 hash identity 不一致，server 回 `409`。

## 9. Verify

```http
POST /api/v1/sessions/{session_id}/verify
```

重新檢查 session `.ready` object 的 size/hash。

## 10. Commit

```http
POST /api/v1/sessions/{session_id}/commit
```

```json
{
  "commit_id": "client-generated-idempotency-key",
  "base_generation": 18,
  "entries": [ ...完整目標 manifest... ]
}
```

規則：

1. `request.base_generation == session.base_generation == server current generation`。
2. 每個 manifest entry 的 hash 必須已存在 CAS 或本 session `.ready`。
3. promote session objects 到 CAS。
4. 計算 canonical manifest SHA-256。
5. 在同目錄寫 temp generation manifest、`fsync`，再 rename 到新的 immutable generation filename（destination 原先不存在，因此 Windows/Linux 不需要 replace-existing 語義）。
6. 最後才把 session 標成 committed，並以新的 append-only metadata version 發布。

相同 `session_id + commit_id` 在 generation 已發布後重送：回同一 generation，`idempotent_replay=true`。

## 11. Current manifest / object download

```http
GET /api/v1/manifests/current?device_id=...&profile_id=...
GET /api/v1/objects/{sha256}
```

手機恢復先取得 manifest，再依 hash 下載 object。

## 12. app_details audit

```http
POST /api/v1/appdetails/audit
```

Request：

```json
{
  "seed_ok": false,
  "stage_apps": ["com.a", "com.b"],
  "seed_apps": [],
  "payload_apps": ["com.a", "com.b", "old.or.foreign"]
}
```

v1 policy 對齊 SpeedBackup r647：

```text
seed_ok=true:
  scope = stage ∪ seed
  stage payload missing -> BLOCK
  seed app missing from stage -> BLOCK

seed_ok=false:
  scope = stage
  stage non-empty -> seedlessRepair=1
  payload - scope 非空 -> seedlessTainted=1
  允許 repair，但 taint 必須被上層保留
```

server response 會回：

```text
allowed
seedless_repair
seedless_tainted
missing_stage_payload_apps
missing_seed_apps
ignored_remote_payload_apps
```

手機端未接入前，r647 現有防護維持不變。

## 13. Safe orphan cleanup

```http
POST /api/v1/admin/cleanup
```

Dry-run：

```json
{ "dry_run": true }
```

實刪：

```json
{
  "dry_run": false,
  "confirm": "DELETE_ORPHAN_OBJECTS"
}
```

server 在 commit global lock 下重新掃描所有 generation manifest；**任何 generation JSON 無法讀取、schema 不符或 hash 非法時直接拒絕 cleanup（fail-closed）**。只有完整掃描成功後才刪除：

```text
CAS object hash ∉ union(all generation referenced hashes)
```

因此不依賴 client 傳入 delete path，也不使用 tainted app_details 清單判斷 CAS GC。

## 14. WebAdmin

```text
/web/admin
```

WebAdmin 與 REST API 同 HTTP server/binding，操作方式參考 SFTPGo WebAdmin，但 UI 為 SpeedBackup 自行實作，不使用 SFTPGo 現行受限制的 WebUI theme/template。

內建：

- 繁體中文 `zh-TW`
- 简体中文 `zh-CN`
- Dashboard
- Devices / Profiles
- Sessions
- Manifest browser
- CAS storage / cleanup
- Audit events
- Token rotation

Token 僅保存於 browser `sessionStorage`；語言偏好保存於 `localStorage`。

## 15. v1 後續但不影響 wire compatibility

- native TLS / certificate management
- per-device scoped token
- session expiry / automatic abandoned session cleanup
- generation retention policy
- upload concurrency limits / bandwidth controls
- WebAdmin service install controls
- Android `remote_type=speedbackup_server`
