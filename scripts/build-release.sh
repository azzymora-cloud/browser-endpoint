#!/usr/bin/env bash
# Cross-compile the Windows agent and build ThinkCentreEndpoint-1.1.0.msi
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="1.1.0"
STAGE="$ROOT/dist/stage"
MSI="$ROOT/dist/ThinkCentreEndpoint-${VERSION}.msi"

cd "$ROOT/agent"
go test ./...
go get golang.org/x/sys@v0.25.0
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$STAGE/thinkcentre-agent.exe" .

mkdir -p "$STAGE/scripts" "$STAGE/guacamole/init" "$STAGE/cloudflare" "$STAGE/installer"
cp "$ROOT/docker-compose.yml" "$ROOT/docker-compose.tunnel.yml" "$ROOT/.env.example" \
   "$ROOT/README.md" "$ROOT/LICENSE" "$ROOT/NOTICE" "$STAGE/"
cp "$ROOT/installer/ThinkCentre-Endpoint.url" "$STAGE/"
cp "$ROOT/scripts/"*.ps1 "$STAGE/scripts/"
cp "$ROOT/scripts/generate-initdb.sh" "$STAGE/scripts/" 2>/dev/null || true
cp "$ROOT/guacamole/connection-template.md" "$STAGE/guacamole/"
cp "$ROOT/guacamole/init/"*.sql "$STAGE/guacamole/init/"
cp "$ROOT/cloudflare/config.yml.example" "$STAGE/cloudflare/"

python3 "$ROOT/scripts/generate-wxs.py" "$STAGE" "$ROOT/dist/ThinkCentreEndpoint.wxs"
# wixl resolves File/@Source relative to the .wxs location unless absolute.
# generate-wxs.py writes absolute paths, which wixl accepts.
wixl --arch x64 -o "$MSI" "$ROOT/dist/ThinkCentreEndpoint.wxs"
echo "Built $MSI"
ls -lh "$MSI" "$STAGE/thinkcentre-agent.exe"
