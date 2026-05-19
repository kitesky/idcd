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
mv "$DEST_FILE.tmp" "$DEST_FILE"

echo "→ installed: $DEST_FILE"
echo "  size: $(du -h "$DEST_FILE" | cut -f1)"
echo ""
echo "next: restart your local stack — start-local-stack.sh auto-detects"
echo "the mmdb and writes geoip_db_path into the agent config."
