#!/usr/bin/env bash
# Download MaxMind GeoLite2-City.mmdb for the agent's traceroute hop geo lookup.
#
# Uses the P3TERX community mirror so we don't need a MaxMind license key:
#   https://github.com/P3TERX/GeoLite.mmdb
# The mirror tracks upstream daily. License compliance (GeoLite2 EULA) is
# still on the user — MaxMind's terms apply regardless of where you got the file.
#
# Usage:
#   ./scripts/download-geolite2.sh
#
# Output: apps/agent/data/GeoLite2-City.mmdb (gitignored)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST_DIR="$REPO_ROOT/apps/agent/data"
DEST_FILE="$DEST_DIR/GeoLite2-City.mmdb"

mkdir -p "$DEST_DIR"

URL="https://github.com/P3TERX/GeoLite.mmdb/releases/latest/download/GeoLite2-City.mmdb"

echo "→ downloading GeoLite2-City from P3TERX mirror..."
curl -fL --progress-bar "$URL" -o "$DEST_FILE.tmp"

# Sanity-check the download before installing — the mirror is HTTPS but
# unsigned, so a tampered or truncated file would otherwise be loaded by the
# agent's mmdb reader and either crash on parse or produce wrong geo. These
# two guards catch the realistic failure modes (mirror returning an error
# page; partial download).

# 1. Size floor: GeoLite2-City is consistently ~60 MB. A file under 50 MB is
#    almost certainly an HTML 404 / rate-limit page saved by curl.
SIZE_BYTES=$(wc -c < "$DEST_FILE.tmp" | tr -d ' ')
if [ "$SIZE_BYTES" -lt 52428800 ]; then
  rm -f "$DEST_FILE.tmp"
  echo "✗ download too small (${SIZE_BYTES} bytes) — mirror likely returned an error page" >&2
  exit 1
fi

# 2. MaxMind format marker: every valid mmdb ends with the literal
#    "\xab\xcd\xefMaxMind.com" sequence delimiting the metadata section.
#    Cheap structural check that catches wrong-file mirror swaps and
#    truncation without needing a license-keyed checksum.
if ! tail -c 2048 "$DEST_FILE.tmp" | grep -q "MaxMind.com" 2>/dev/null; then
  rm -f "$DEST_FILE.tmp"
  echo "✗ file does not contain MaxMind.com metadata marker — not a valid mmdb" >&2
  exit 1
fi

mv "$DEST_FILE.tmp" "$DEST_FILE"

echo "→ installed: $DEST_FILE"
echo "  size: $(du -h "$DEST_FILE" | cut -f1)"
echo ""
echo "next: restart your local stack — start-local-stack.sh auto-detects"
echo "the mmdb and writes geoip_db_path into the agent config."
