#!/bin/bash
source "$(dirname "$0")/lib.sh"

name=ex-logs-app
port=3145
CLEANUP_APPS+=("$name")
dir=$(mktemp_dir "$name" "$port")

echo "=== logs: node app (journalctl) ==="
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
    curl -sf "http://localhost:$port/" >/dev/null

    logs=$(vigil_capture logs "$name" --lines=5 || true)
    echo "$logs" | grep -q "listening on"

    vigil logs "$name" --lines=5 -o /tmp/vigil-logs-app.out
    [ -s /tmp/vigil-logs-app.out ]
); then
    pass "logs node app"
else
    fail "logs node app"
fi

name=ex-logs-site
CLEANUP_APPS+=("$name")

echo "=== logs: static app (nginx) ==="
if ( set -e
    vigil add "$name" \
        --type static \
        --build-dir "$EXAMPLE_DIR/static" \
        --port 8145 \
        --nginx-domain "test.local" \
        --nginx-path "$EXAMPLE_DIR/static" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh"
    vigil start "$name"
    sleep 1
    curl -s -H "Host: test.local" "http://localhost:8145/" >/dev/null

    logs=$(vigil_capture logs "$name" --lines=5 || true)
    for i in $(seq 1 5); do
        [ -n "$logs" ] && break
        sleep 0.3
        logs=$(vigil_capture logs "$name" --lines=5 || true)
    done
    [ -n "$logs" ]
); then
    pass "logs static app"
else
    fail "logs static app"
fi
