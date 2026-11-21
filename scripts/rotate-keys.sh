#!/usr/bin/env bash
# rotate-keys.sh — Zero-downtime automated key & short-id rotation for Sirin Gateway
# Usage: sudo /opt/sirin/scripts/rotate-keys.sh [--force]
set -euo pipefail

LOG_FILE="/var/log/sirin/key-rotation.log"
XRAY_CONFIG="/usr/local/etc/xray/config.json"
SIRINAPI_CONFIG="/etc/sirin/sirinapi.toml"

log() {
  local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $1"
  echo "$msg"
  if [ -d "/var/log/sirin" ]; then
    echo "$msg" >> "$LOG_FILE"
  fi
}

if [ "$EUID" -ne 0 ]; then
  echo "ERROR: rotate-keys.sh must be run as root (sudo)" >&2
  exit 1
fi

log "Starting Sirin Gateway automated key rotation..."

if [ ! -f "$XRAY_CONFIG" ]; then
  log "ERROR: Xray config not found at $XRAY_CONFIG"
  exit 1
fi

# 1. Generate new 16-character hex short ID (8 bytes)
NEW_SHORT_ID=$(openssl rand -hex 8)
log "Generated new Reality shortId: $NEW_SHORT_ID"

# 2. Extract current primary shortId from Xray config using jq or python
if command -v jq &>/dev/null; then
  CURRENT_SHORT_ID=$(jq -r '.inbounds[0].streamSettings.realitySettings.shortIds[0] // empty' "$XRAY_CONFIG")
  
  if [ -n "$CURRENT_SHORT_ID" ] && [ "$CURRENT_SHORT_ID" != "null" ] && [ "$CURRENT_SHORT_ID" != "$NEW_SHORT_ID" ]; then
    log "Rotating shortIds: primary=$NEW_SHORT_ID, secondary=$CURRENT_SHORT_ID"
    # Keep current shortId as secondary so existing active clients continue working seamlessly
    TMP_JSON=$(mktemp)
    jq --arg newId "$NEW_SHORT_ID" --arg oldId "$CURRENT_SHORT_ID" '
      .inbounds |= map(
        if .streamSettings.realitySettings? then
          .streamSettings.realitySettings.shortIds = [$newId, $oldId, ""]
        else
          .
        end
      )
    ' "$XRAY_CONFIG" > "$TMP_JSON"
    mv "$TMP_JSON" "$XRAY_CONFIG"
    chmod 644 "$XRAY_CONFIG"
  fi
elif command -v python3 &>/dev/null; then
  python3 -c "
import json
with open('$XRAY_CONFIG', 'r') as f:
    cfg = json.load(f)
for ib in cfg.get('inbounds', []):
    rs = ib.get('streamSettings', {}).get('realitySettings', {})
    if 'shortIds' in rs:
        old_ids = [x for x in rs['shortIds'] if x]
        old_primary = old_ids[0] if old_ids else ''
        rs['shortIds'] = ['$NEW_SHORT_ID'] + ([old_primary] if old_primary else []) + ['']
with open('$XRAY_CONFIG', 'w') as f:
    json.dump(cfg, f, indent=2)
"
fi

# 3. Update sirinapi.toml with the new short ID
if [ -f "$SIRINAPI_CONFIG" ]; then
  if grep -q "vless_sid" "$SIRINAPI_CONFIG"; then
    sed -i "s/vless_sid = .*/vless_sid = \"$NEW_SHORT_ID\"/" "$SIRINAPI_CONFIG"
  else
    echo "vless_sid = \"$NEW_SHORT_ID\"" >> "$SIRINAPI_CONFIG"
  fi
  if grep -q "xhttp_sid" "$SIRINAPI_CONFIG"; then
    sed -i "s/xhttp_sid = .*/xhttp_sid = \"$NEW_SHORT_ID\"/" "$SIRINAPI_CONFIG"
  else
    echo "xhttp_sid = \"$NEW_SHORT_ID\"" >> "$SIRINAPI_CONFIG"
  fi
  log "Updated $SIRINAPI_CONFIG with new vless_sid & xhttp_sid"
fi

# 4. Gracefully reload or restart Xray container
if command -v docker &>/dev/null; then
  if docker ps --format '{{.Names}}' | grep -q "^sirin-xray$"; then
    log "Restarting sirin-xray container to apply rotated shortIds..."
    docker restart sirin-xray >/dev/null
    log "sirin-xray restarted successfully."
  fi
  if docker ps --format '{{.Names}}' | grep -q "^sirin-sirinapi$"; then
    log "Restarting sirin-sirinapi to refresh API parameters..."
    docker restart sirin-sirinapi >/dev/null
    log "sirin-sirinapi restarted successfully."
  fi
fi

log "Key rotation completed successfully. Active short ID: $NEW_SHORT_ID"
