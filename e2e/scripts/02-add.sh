#!/bin/bash
source "$(dirname "$0")/lib.sh"

name=ex-add-app
port=3140
CLEANUP_APPS+=("$name")
dir=$(mktemp_dir "$name" "$port")

echo "=== add: node app ==="
if ( set -e
    vigil add "$name" \
        --type app \
        --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
        --port "$port" \
        --working-dir "$dir" \
        --env-file "$dir/shared/.env" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
        --bundled-deps
    [ -f "$VIRGIL_HOME/apps/$name.json" ]
    [ -f "/etc/systemd/system/$name.service" ]
); then
    pass "add node app"
else
    fail "add node app"
fi

echo "=== add: static app ==="
if ( set -e
    vigil add ex-add-site \
        --type static \
        --build-dir "$EXAMPLE_DIR/static" \
        --port 8140 \
        --nginx-domain "test.local" \
        --nginx-path "$EXAMPLE_DIR/static" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh"
    [ -f "$VIRGIL_HOME/apps/ex-add-site.json" ]
    [ -f "/etc/nginx/sites-available/ex-add-site.conf" ]
); then
    pass "add static app"
else
    fail "add static app"
fi

echo "=== add: from ecosystem config ==="
if ( set -e
    vigil add --config "$EXAMPLE_DIR/ecosystem.json"
    out=$(vigil_capture list)
    echo "$out" | grep -q "dummy-app"
    vigil remove dummy-app
); then
    pass "add from ecosystem config"
else
    fail "add from ecosystem config"
fi

echo "=== add: validation error missing smoke script ==="
if ( set -e
    out=$(vigil_capture add ex-add-nosmoke \
        --type app \
        --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
        --port 8141 \
        --working-dir "$dir" \
        --bundled-deps || true)
    echo "$out" | grep -qi "smoke"
); then
    pass "validation error: missing smoke script"
else
    fail "validation error: missing smoke script"
fi
