# Backup completion shutdown contract v1

Status: Windows release `0.3.17-webdav18`.

New DEX clients use [device pairing](POWER_DEVICE_PAIRING_V1.md): anonymous DAV supported, one-time native Windows approval, no setup script or copied token. The static credential flow below is legacy compatibility. Linux advertises unsupported.
No ordinary DAV client can request shutdown. The feature is disabled by default.

## Android / Shell / DEX handoff

Capability discovery (read-only, no authentication): `GET /api/v1/capabilities`.
Require JSON `server == "SpeedBackup Server"`, `protocol.major == 1`, and
`features.backup_shutdown_v1 == true` before offering the option. Missing/false,
invalid JSON, a different protocol or HTTP failure means unavailable. This is a
capability declaration, not proof of server identity or enabled configuration;
use the same user-configured trusted origin and TLS verification as completion.
Never probe with POST. Only the dedicated credential authorizes completion.

1. Enable only by explicit user choice for the whole backup run. Create a random UUID job ID once at run start; retain it through notification retries.
2. Confirm the entire run succeeded, all workers exited, and uploads/final MOVE/index commits finished. Failure, cancellation, interruption and partial completion must NOT send completion.
3. Read a **separate shutdown credential** configured by the user. Do not reuse DAV Basic auth or the administrator/API token. Never place this credential in command arguments, logs, URLs, diagnostic bundles or exported public settings. Read it in-process from a private file/input.
4. Use the configured server origin (scheme/host/port), not a share directory. Do not silently send credentials to a discovered/replaced server. Require verified HTTPS for Internet connections; HTTP is only for an explicitly trusted LAN. Disable redirects and set bounded connect/read/total timeouts.
5. Send:

```http
POST /api/v1/power/complete
Authorization: Bearer <dedicated shutdown credential>
Content-Type: application/json

{"job_id":"<random UUID retained across retries>","result":"success"}
```

`job_id`: 16–96 ASCII letters, digits, hyphen, underscore or dot; a random UUID is recommended. Body limit 2048 bytes. Unknown fields are rejected.

Response: 202 for a newly accepted request; 200 for an already accepted ID. JSON job includes `job_id`, `state`, `preview`, `deadline_ms` (Unix milliseconds, 0 if not counting), `message`. A historical replay may return only `job_id` and `state: closed`. **202 does not mean the computer has shut down.** A desktop launch failure returns an accepted job with `state: error`; do not retry with a new ID.

401 means invalid credential. 400 means invalid request. 409 means disabled/unsupported, another pending request, or unavailable state storage. Do not retry 4xx automatically. For network timeouts/5xx, retry with the same ID at most three times with backoff (e.g. 2, 5, 10 seconds).

Optional status:

```http
GET /api/v1/power/jobs/<job_id>
Authorization: Bearer <same credential>
```

Returns 200, 401 or 404. States: `waiting`, `countdown`, `deferred`, `cancelled`, `shutting_down`, `error`, `closed`; `simulated` is preview-only. A connection lost while shutting down is expected but not proof of successful power-off. Report completion notification failure separately from successful backup; preserve backup logs and exit status.

## Server and desktop behavior

### Explicit manual decision after backup OR restore (power2)

The Android user may explicitly choose shutdown after **any settled outcome**, including
failure/partial completion/cancellation. This is separate from automatic successful-backup completion.
Workers, final writes and cleanup must have exited before prompting. Volume-down/no answer/timeout
must not send a request. Never mislabel a failed run as `result: success`.

Read-only discovery: `GET /api/v1/capabilities`, require the same server/protocol checks above
and `features.manual_shutdown_v1 == true`. Missing/false means unavailable.

```http
POST /api/v1/power/manual
Authorization: Bearer <dedicated shutdown credential>
Content-Type: application/json

{"job_id":"<stable UUID>","operation":"backup","outcome":"failed","user_confirmed":true,"preview":false}
```

`operation` is `backup` or `restore`. `outcome` is `success`, `failed`, `partial`, or `cancelled`.
`user_confirmed` must be the JSON boolean true and represents an actual affirmative user decision.
Do not include `result`. Unique-ID, response codes, status endpoint, bounded retries, active-transfer
gate, desktop countdown and cancellation rules remain unchanged. The desktop displays the real outcome.

For integration tests, explicitly set **`preview: true`**: it uses the same authorized native
desktop and status flow but cannot call OS shutdown. It is allowed while the shutdown setting is
disabled, still requires the separate credential, and consumes the job ID. Reusing this preview ID
with `preview: false` cannot turn it into a real shutdown. Keep test and real operation IDs distinct.
If the helper/desktop cannot start, expect `state: error` and no power action.

Offline UI preview: `speedbackup-notify.exe --preview` (no server/network/power action).
Test artifacts: `deliverables/SpeedBackup_power2_test` in the shared workspace.

- Settings → 備份完成後關機: generate/rotate a credential (shown once), enable, sync the current appearance, preview. Rotating disables shutdown and cancels a pending request. Administrator **session** required; generic API token is insufficient.
- Preview exercises the real UI and timer but cannot invoke the OS power-off function.
- The bundled native WinForms/GDI+ helper uses OPPO Sans Regular and the selected WebUI theme/accent/material. No Edge, Chrome, WebView2 or downloaded browser runtime is used. It runs on the Windows .NET Framework supplied with supported Windows 10/11 systems. Click “將目前外觀套用到桌面提示” to save the chosen appearance for this computer. Other browser preferences remain local.
- The installed LocalSystem service launches `speedbackup-notify.exe` under the active console user's token/environment. Portable mode launches under its own interactive session. No desktop/helper means an error and no shutdown. RDP-only and multi-user desktop selection are not supported in this first version. Lock/disconnect/minimize/close pauses the native prompt's heartbeat.
- All DAV requests and session/object transfers hold the power activity gate until the handler returns. Any new transfer resets the countdown. Once shutdown has been handed to Windows, further data requests return 503 until the service restarts.
- Countdown begins at 60 seconds only with no active data requests and a recent desktop heartbeat. Explicit hiding/closing asks the server to pause; unexpected disconnection pauses after 10 seconds. Reconnection starts a fresh 60 seconds. Requests expire after one hour, including deferrals. Delay adds ten minutes; cancel is terminal.
- Accepted IDs are persisted before launching the desktop. Service stop/restart never resumes a shutdown request; old IDs are never rescheduled. IDs cannot be removed by token rotation. The 10,000-ID limit fails closed rather than evicting replay protection.
- Deadline executes `InitiateSystemShutdownExW(timeout=0, forceAppsClosed=false, reboot=false)`. Application veto/unsaved documents can prevent Windows shutdown. After OS handoff, the app cannot offer cancel; `shutting_down` means submitted, not verified power-off. Never use `shutdown.exe /s /t 60`, whose nonzero timeout can imply force-closing.
- Desktop capability is separate, temporary, loopback-only and passed in the native helper's launch argument fragment (never an HTTP URL or request log). It grants only this pending job's actions. The helper only accepts `http://127.0.0.1:<port>/api/v1/power/desktop#sb1_<64 hex>` and rejects redirects/proxies.

## Validation scope

All automated power execution tests use injected stubs. Do not test real shutdown on a workstation with active work. Service-context launching and actual OS shutdown require a dedicated Windows VM/hardware acceptance test; neither is implied by compile/unit/browser success.

Windows API reference: https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-initiatesystemshutdownexw
