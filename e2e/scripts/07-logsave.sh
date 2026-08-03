#!/bin/bash
source "$(dirname "$0")/lib.sh"

name=ex-logsave-app
port=3146
CLEANUP_APPS+=("$name")
dir=$(mktemp_dir "$name" "$port")

echo "=== logsave: enable/status/disable node app ==="
if ( set -e
    vigil add "$name" \
        --type app \
        --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
        --port "$port" \
        --working-dir "$dir" \
        --env-file "$dir/shared/.env" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
        --bundled-deps

    vigil logsave enable "$name" --output /tmp/logsave --max-size 5M --rotate 2
    out=$(vigil_capture logsave status "$name")
    echo "$out" | grep -q "Enabled:   true"
    echo "$out" | grep -q "/tmp/logsave/$name.log"

    out=$(vigil_capture logsave disable "$name" || true)
    echo "$out" | grep -qi "Disabled log saving"
); then
    pass "logsave enable/status/disable"
else
    fail "logsave enable/status/disable"
fi

echo "=== logsave: status unknown app ==="
if ( set -e
    out=$(vigil_capture logsave status no-such-app || true)
    echo "$out" | grep -qi "error"
); then
    pass "logsave status unknown app"
else
    fail "logsave status unknown app"
fi
