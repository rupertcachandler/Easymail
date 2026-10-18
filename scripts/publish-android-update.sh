#!/bin/bash
# Publish an Android APK as an over-the-air update for BOI.
# Usage: scripts/publish-android-update.sh <version> <apk-path> [notes]
#
# Requires the Ed25519 signing key at $BOI_UPDATE_KEY (default
# ~/.config/boi-updates/ed25519.key) — the matching PUBLIC half is pinned in
# internal/app/update_android.go; never commit or ship the private key.
# Uploads to $BOI_UPDATE_HOST (default root@mail.btl.io:/var/www/boi/).
set -euo pipefail

VER="${1:?version required}"
APK="${2:?apk path required}"
NOTES="${3:-}"
KEY="${BOI_UPDATE_KEY:-$HOME/.config/boi-updates/ed25519.key}"
DEST="${BOI_UPDATE_HOST:-root@mail.btl.io:/var/www/boi}"

[ -f "$KEY" ] || { echo "signing key not found at $KEY" >&2; exit 1; }
[ -f "$APK" ] || { echo "apk not found at $APK" >&2; exit 1; }

SHA=$(sha256sum "$APK" | cut -d' ' -f1)
# Ed25519 one-shot signing needs a real (seekable) file — a pipe fails with
# "unable to determine file size for oneshot operation".
SIG=$(printf '%s' "$SHA" > /tmp/.boi-sign.msg && openssl pkeyutl -sign -inkey "$KEY" -rawin -in /tmp/.boi-sign.msg | base64 -w0; rm -f /tmp/.boi-sign.msg)
APKNAME="boi-${VER}.apk"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
cp "$APK" "$TMP/$APKNAME"
cat > "$TMP/manifest.json" << EOF
{"version":"$VER","url":"http://mail.btl.io/boi/$APKNAME","sha256":"$SHA","notes":"$(echo "$NOTES" | sed 's/"/\\"/g')","sig":"$SIG"}
EOF
cat > "$TMP/boi.apk.sha256" << EOF
$SHA  $APKNAME
EOF

scp -q "$TMP/$APKNAME" "$TMP/manifest.json" "$TMP/boi.apk.sha256" "$DEST/"
# Keep a stable "latest" name too; manifest version governs the offer.
ssh "$(echo "$DEST" | cut -d: -f1)" "cp /var/www/boi/$APKNAME /var/www/boi/boi.apk && chmod 644 /var/www/boi/*"
echo "published $VER ($SHA)"
echo "manifest: http://mail.btl.io/boi/manifest.json"
