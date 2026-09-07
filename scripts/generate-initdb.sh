#!/usr/bin/env bash
# Refresh official Guacamole Postgres schema files (Apache-2.0) for this tag.
set -euo pipefail

TAG="${1:-1.6.0}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/guacamole/init"
BASE="https://raw.githubusercontent.com/apache/guacamole-client/${TAG}/extensions/guacamole-auth-jdbc/modules/guacamole-auth-jdbc-postgresql/schema"

mkdir -p "$DEST"
curl -fsSL "$BASE/001-create-schema.sql" -o "$DEST/001-create-schema.sql"
curl -fsSL "$BASE/002-create-admin-user.sql" -o "$DEST/002-create-admin-user.sql"
echo "Wrote $DEST/001-create-schema.sql and 002-create-admin-user.sql from guacamole-client $TAG"
echo "Keep 003-seed-thinkcentre.sql; it is project-specific."
