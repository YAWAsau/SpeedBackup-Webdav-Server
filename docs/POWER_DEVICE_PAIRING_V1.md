# Device shutdown pairing v1

Capability: `features.manual_shutdown_device_pairing_v1 == true` (Windows).
Anonymous DAV is supported; DAV credentials are never used for power pairing.
Administrator enables pairing, approves a device once, and separately enables real shutdown.

All POST bodies require application/json, no redirects, same trusted origin. Use verified HTTPS outside explicitly trusted LAN. Keep P-256 private key in Android root-private storage across runs. No setup script.

1. POST /api/v1/power/challenge: {"public_key":"base64url unpadded X509 SPKI DER P-256","device_name":"phone label","job_id":"UUID","scope":"power:manual"}.
200: {"challenge_id":"opaque","message":"exact ASCII message","expires_unix":seconds,"device_id":"64 lowercase hex SHA256 of SPKI"}.
Challenge TTL 120 seconds. Sign exact UTF-8 message bytes with SHA256withECDSA; signature ASN.1 DER, base64url unpadded. Do not append newline.
2. POST /api/v1/power/pair: {"challenge_id":"...","signature":"..."}.
Challenge consumed on every signature attempt. New valid device: 202 {"state":"approval_required","device_id":"..."}.
Show full fingerprint on phone for comparison with WebAdmin. First approval uses the existing independent native Windows prompt (no browser dependency); WebAdmin is also available. Compare full fingerprint before approving. Pending request expires in 10 minutes; polling does not extend expiry. Repeat challenge+pair every 3 seconds, at most 120 seconds, with same key/job. Timeout means no shutdown; reuse key next run.
Approved device: 200 {"token":"opaque","expires_unix":seconds,"scope":"power:manual","job_id":"..."}. Token TTL 7200 seconds, memory-only, exact job bound.
3. Use token only for POST /api/v1/power/manual and GET /api/v1/power/jobs/{same job_id}. Existing explicit user confirmation and manual body apply. No complete/admin/DAV privilege.

Errors: 400 malformed input/key/scope; 401 expired/consumed challenge or invalid signature; 403 pairing disabled/revoked device/job owner mismatch; 409 unsupported/unavailable; 429 capacity. Never retry an ambiguous shutdown response with a new job ID.

Admin /api/v1/admin/power actions: enable_pairing, disable_pairing, approve_device/revoke_device with device_id, revoke_pairs. GET includes pairing_enabled and devices (id/name/approved/revoked/expires_unix). Revocation cancels pending paired shutdown. Restart retains approved devices and job tombstones but invalidates all grants/challenges. Real shutdown defaults off.

Exact signed message format (ASCII, LF separators, no final newline):
```text
SpeedBackup power device v1
{challenge_id}
{device_id}
{job_id}
power:manual
```
Clients verify every field and purpose before signing.

`delete_device` with device_id deletes remembered approval, invalidates that device grants/challenges/desktop approval capability and cancels pending shutdown. Next request with the same key returns approval_required and requires fresh approval. Retain job tombstones: deleted device cannot replay an old accepted job as a new shutdown.
