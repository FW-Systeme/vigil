#!/bin/bash
source "$(dirname "$0")/lib.sh"

name=ex-static-site
port=8146
CLEANUP_APPS+=("$name")

echo "=== static: lifecycle ==="
if ( set -e
    vigil add "$name" \
        --type static \
        --build-dir "$EXAMPLE_DIR/static" \
        --port "$port" \
        --nginx-domain "test.local" \
        --nginx-path "$EXAMPLE_DIR/static" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh"

    vigil start "$name"
    sleep 1
    resp=$(curl -s -H "Host: test.local" "http://localhost:$port/")
    [ "$resp" = "<h1>vigil e2e test</h1>" ]

    vigil list >/dev/null

    vigil stop "$name"
    sleep 1

    vigil remove "$name"
    [ ! -f "$VIRGIL_HOME/apps/$name.json" ]
    [ ! -f "/etc/nginx/sites-available/$name.conf" ]
); then
    pass "static lifecycle"
else
    fail "static lifecycle"
fi
