#!/bin/bash
source "$(dirname "$0")/lib.sh"

echo "=== list: empty home ==="
if ( set -e
    out=$(vigil_capture list)
    echo "$out" | grep -q "No apps registered"
); then
    pass "list empty home"
else
    fail "list empty home"
fi

name=ex-list-app
port=3144
CLEANUP_APPS+=("$name")
dir=$(mktemp_dir "$name" "$port")

echo "=== list: registered apps ==="
if ( set -e
    vigil add "$name" \
        --type app \
        --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
        --port "$port" \
        --working-dir "$dir" \
        --env-file "$dir/shared/.env" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
        --bundled-deps
    vigil add ex-list-site \
        --type static \
        --build-dir "$EXAMPLE_DIR/static" \
        --port 8144 \
        --nginx-domain "test.local" \
        --nginx-path "$EXAMPLE_DIR/static" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh"
    out=$(vigil_capture list)
    echo "$out" | grep -q "$name"
    echo "$out" | grep -q "ex-list-site"
    echo "$out" | grep -q "port $port"
    echo "$out" | grep -q "active"
); then
    pass "list registered apps"
else
    fail "list registered apps"
fi
