#!/usr/bin/env sh
set -eu
BASE_URL="${BASE_URL:-http://127.0.0.1:8765}"
TOKEN="${SPEEDBACKUP_SERVER_TOKEN:?set SPEEDBACKUP_SERVER_TOKEN}"
AUTH="Authorization: Bearer $TOKEN"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
DEVICE="smoke-$(date +%s)-$$"
PROFILE="default"

json_post() {
  path="$1"
  body="$2"
  curl -fsS -H "$AUTH" -H 'Content-Type: application/json' -d "$body" "$BASE_URL$path"
}

echo "[1/10] capabilities"
curl -fsS "$BASE_URL/api/v1/capabilities" > "$TMP/capabilities.json"
grep -q '"server":"SpeedBackup Server"' "$TMP/capabilities.json"

echo "[2/10] status/auth"
curl -fsS -H "$AUTH" "$BASE_URL/api/v1/status" > "$TMP/status.json"
grep -q '"version"' "$TMP/status.json"

echo "[3/10] create isolated session"
BODY="{\"device_id\":\"$DEVICE\",\"profile_id\":\"$PROFILE\",\"base_generation\":null}"
SESSION_JSON="$(json_post /api/v1/sessions "$BODY")"
SESSION_ID="$(printf '%s' "$SESSION_JSON" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')"
[ -n "$SESSION_ID" ]

printf '%s' 'SpeedBackup Server resumable smoke payload 0123456789abcdefghijklmnopqrstuvwxyz' > "$TMP/payload.bin"
SIZE="$(wc -c < "$TMP/payload.bin" | tr -d '[:space:]')"
SHA="$(sha256sum "$TMP/payload.bin" | awk '{print $1}')"
FIRST=$((SIZE / 2))
SECOND=$((SIZE - FIRST))
dd if="$TMP/payload.bin" of="$TMP/chunk1.bin" bs=1 count="$FIRST" status=none
dd if="$TMP/payload.bin" of="$TMP/chunk2.bin" bs=1 skip="$FIRST" count="$SECOND" status=none

echo "[4/10] resumable PATCH part 1"
curl -fsS -X PATCH -H "$AUTH" -H 'Content-Type: application/octet-stream' \
  -H "Upload-Offset: 0" -H "Upload-Length: $SIZE" \
  --data-binary @"$TMP/chunk1.bin" \
  "$BASE_URL/api/v1/sessions/$SESSION_ID/objects/$SHA" > "$TMP/part1.json"
grep -q '"complete":false' "$TMP/part1.json"

echo "[5/10] HEAD resume state"
curl -fsSI -H "$AUTH" "$BASE_URL/api/v1/sessions/$SESSION_ID/objects/$SHA" > "$TMP/head.txt"
grep -qi "^upload-offset: $FIRST" "$TMP/head.txt"
grep -qi "^upload-length: $SIZE" "$TMP/head.txt"

echo "[6/10] resumable PATCH part 2 + verify"
curl -fsS -X PATCH -H "$AUTH" -H 'Content-Type: application/octet-stream' \
  -H "Upload-Offset: $FIRST" -H "Upload-Length: $SIZE" \
  --data-binary @"$TMP/chunk2.bin" \
  "$BASE_URL/api/v1/sessions/$SESSION_ID/objects/$SHA" > "$TMP/part2.json"
grep -q '"complete":true' "$TMP/part2.json"
curl -fsS -H "$AUTH" -X POST "$BASE_URL/api/v1/sessions/$SESSION_ID/verify" > "$TMP/verify.json"
grep -q '"verified":true' "$TMP/verify.json"

echo "[7/10] commit generation"
COMMIT_BODY="{\"commit_id\":\"smoke-commit\",\"base_generation\":null,\"entries\":[{\"path\":\"smoke/payload.bin\",\"size\":$SIZE,\"sha256\":\"$SHA\",\"kind\":\"other\"}]}"
COMMIT_JSON="$(json_post "/api/v1/sessions/$SESSION_ID/commit" "$COMMIT_BODY")"
printf '%s' "$COMMIT_JSON" > "$TMP/commit.json"
grep -q '"committed":true' "$TMP/commit.json"

echo "[8/10] idempotent commit replay"
REPLAY_JSON="$(json_post "/api/v1/sessions/$SESSION_ID/commit" "$COMMIT_BODY")"
printf '%s' "$REPLAY_JSON" > "$TMP/replay.json"
grep -q '"idempotent_replay":true' "$TMP/replay.json"

echo "[9/10] manifest + object download integrity"
curl -fsS -H "$AUTH" "$BASE_URL/api/v1/manifests/current?device_id=$DEVICE&profile_id=$PROFILE" > "$TMP/manifest.json"
grep -q "$SHA" "$TMP/manifest.json"
curl -fsS -H "$AUTH" "$BASE_URL/api/v1/objects/$SHA" -o "$TMP/download.bin"
DOWN_SHA="$(sha256sum "$TMP/download.bin" | awk '{print $1}')"
[ "$DOWN_SHA" = "$SHA" ]

echo "[10/10] safe cleanup dry-run"
json_post /api/v1/admin/cleanup '{"dry_run":true}' > "$TMP/cleanup.json"
grep -q '"dry_run":true' "$TMP/cleanup.json"

echo "SMOKE OK device=$DEVICE sha256=$SHA"
