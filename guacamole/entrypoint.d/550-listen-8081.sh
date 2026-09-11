#!/bin/bash
#
# Cloudflare and the host publish :8080. nginx (session HUD) owns that port
# in this network namespace; Tomcat must sit on 8081 behind it.
#

if [ -z "${CATALINA_BASE:-}" ] || [ ! -f "$CATALINA_BASE/conf/server.xml" ]; then
    echo "550-listen-8081: CATALINA_BASE/server.xml missing" >&2
    exit 1
fi

sed -i 's/Connector port="8080"/Connector port="8081"/' "$CATALINA_BASE/conf/server.xml"

if ! grep -q 'Connector port="8081"' "$CATALINA_BASE/conf/server.xml"; then
    echo "550-listen-8081: failed to move Tomcat HTTP connector to 8081" >&2
    exit 1
fi

echo "Tomcat HTTP connector listening on 8081 (nginx owns 8080)"
