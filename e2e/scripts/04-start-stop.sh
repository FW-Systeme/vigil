#!/bin/bash
source "$(dirname "$0")/lib.sh"

name=ex-startstop-app
port=3143
CLEANUP_APPS+=("$name")
dir=$(mktemp_dir "$name" "$port")

echo "=== start/stop: node app lifecycle ==="
if ( set -e
    vigil add "$name" \
        --type app \
        --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
        --port "$port" \
        --working-dir "$dir" \
        --env-file "$dir/shared/.env" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
        --bundled-deps

    vigil start "$name"
    wait_for_service_active "$name"
    wait_for_port "$port"
    resp=$(curl -sf "http://localhost:$port/")
    [ "$resp" = "ok" ]

    vigil stop "$name"
    wait_for_service_inactive "$name"
    wait_for_port_closed "$port"

    vigil start "$name"
    wait_for_service_active "$name"
    wait_for_port "$port"

    vigil restart "$name"
    wait_for_service_active "$name"
    wait_for_port "$port"
); then
    pass "start/stop/restart node app"
else
    fail "start/stop/restart node app"
fi

echo "=== start/stop: static app ==="
if ( set -e
    vigil add ex-startstop-site \
        --type static \
        --build-dir "$EXAMPLE_DIR/static" \
        --port 8143 \
        --nginx-domain "test.local" \
        --nginx-path "$EXAMPLE_DIR/static" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh"

    vigil start ex-startstop-site
    sleep 1
    resp=$(curl -s -H "Host: test.local" "http://localhost:8143/")
    [ "$resp" = "<h1>vigil e2e test</h1>" ]

    vigil stop ex-startstop-site
    sleep 1
    [ -z "$(curl -s -H "Host: test.local" "http://localhost:8143/" 2>/dev/null)" ] || [ "$(curl -s -o /dev/null -w '%{http_code}' -H 'Host: test.local' http://localhost:8143/)" = "404" ]
); then
    pass "start/stop static app"
else
    fail "start/stop static app"
fi
