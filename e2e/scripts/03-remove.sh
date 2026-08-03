#!/bin/bash
source "$(dirname "$0")/lib.sh"

name=ex-rm-app
port=3142
CLEANUP_APPS+=("$name")
dir=$(mktemp_dir "$name" "$port")

echo "=== remove: node app ==="
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
    vigil remove "$name"
    [ ! -f "$VIRGIL_HOME/apps/$name.json" ]
    [ ! -f "/etc/systemd/system/$name.service" ]
    out=$(vigil_capture list)
    echo "$out" | grep -q "No apps registered"
); then
    pass "remove node app"
else
    fail "remove node app"
fi

echo "=== remove: static app ==="
if ( set -e
    vigil add ex-rm-site \
        --type static \
        --build-dir "$EXAMPLE_DIR/static" \
        --port 8142 \
        --nginx-domain "test.local" \
        --nginx-path "$EXAMPLE_DIR/static" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh"
    vigil remove ex-rm-site
    [ ! -f "$VIRGIL_HOME/apps/ex-rm-site.json" ]
    [ ! -f "/etc/nginx/sites-available/ex-rm-site.conf" ]
); then
    pass "remove static app"
else
    fail "remove static app"
fi
