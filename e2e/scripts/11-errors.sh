#!/bin/bash
source "$(dirname "$0")/lib.sh"

name=ex-err-app
port=3148
CLEANUP_APPS+=("$name")
dir=$(mktemp_dir "$name" "$port")

echo "=== error: duplicate app add ==="
if ( set -e
    vigil add "$name" \
        --type app \
        --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
        --port "$port" \
        --working-dir "$dir" \
        --env-file "$dir/shared/.env" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
        --bundled-deps

    out=$(vigil_capture add "$name" \
        --type app \
        --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
        --port "$port" \
        --working-dir "$dir" \
        --env-file "$dir/shared/.env" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
        --bundled-deps || true)
    echo "$out" | grep -qi "already exists"
); then
    pass "error duplicate app add"
else
    fail "error duplicate app add"
fi

echo "=== error: missing command ==="
if ( set -e
    out=$(vigil_capture add ex-err-nocmd \
        --type app \
        --port 8147 \
        --working-dir "$dir" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
        --bundled-deps || true)
    echo "$out" | grep -qi "command"
); then
    pass "error missing command"
else
    fail "error missing command"
fi

echo "=== error: update lock held ==="
if ( set -e
    echo $$ > "$dir/.vigil.lock"

    out=$(vigil_capture update "$name" --version v1.0.0 || true)
    echo "$out" | grep -qi "lock"
    rm -f "$dir/.vigil.lock"
); then
    pass "error update lock held"
else
    fail "error update lock held"
fi

echo "=== error: update no package ==="
if ( set -e
    out=$(vigil_capture update "$name" || true)
    echo "$out" | grep -qi "no package"
); then
    pass "error update no package"
else
    fail "error update no package"
fi

echo "=== error: invalid cron schedule ==="
if ( set -e
    out=$(vigil_capture cron add bad-schedule --schedule "not-a-schedule" --command "/bin/true" || true)
    [ -n "$out" ]
); then
    pass "error invalid cron schedule"
else
    fail "error invalid cron schedule"
fi
