#!/bin/bash
source "$(dirname "$0")/lib.sh"

logbase="/tmp/vigil-e2e-logtest"

echo "=== log-output: file created with version command ==="
logfile="$logbase/version.log"
if ( set -e
    rm -rf "$logbase"
    mkdir -p "$logbase"
    vigil version --log-output "$logfile"
    [ -f "$logfile" ]
    [ -s "$logfile" ]
); then
    pass "log-output file created for version"
else
    fail "log-output file created for version"
fi

echo "=== log-output: directory creates auto-named file ==="
if ( set -e
    rm -rf "$logbase-dir"
    vigil version --log-output "$logbase-dir/"
    count=$(find "$logbase-dir" -name "vigil-version-*.log" | wc -l)
    [ "$count" -ge 1 ]
); then
    pass "log-output directory creates auto-named file"
else
    fail "log-output directory creates auto-named file"
fi

echo "=== log-output: append on repeated call ==="
if ( set -e
    rm -rf "$logbase"
    mkdir -p "$logbase"
    vigil version --log-output "$logbase/append.log"
    size1=$(wc -c < "$logbase/append.log")
    vigil version --log-output "$logbase/append.log"
    size2=$(wc -c < "$logbase/append.log")
    [ "$size2" -gt "$size1" ]
); then
    pass "log-output append on repeated call"
else
    fail "log-output append on repeated call"
fi

echo "=== log-output: captures start command output ==="
name=ex-logout-app
port=3146
CLEANUP_APPS+=("$name")
dir=$(mktemp_dir "$name" "$port")
logfile="$logbase/start.log"
if ( set -e
    rm -rf "$logbase"
    mkdir -p "$logbase"
    vigil add "$name" \
        --type app \
        --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
        --port "$port" \
        --working-dir "$dir" \
        --env-file "$dir/shared/.env" \
        --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
        --bundled-deps
    vigil start "$name" --log-output "$logfile"
    grep -q "Started app" "$logfile"
); then
    pass "log-output captures start output"
else
    fail "log-output captures start output"
fi

echo "=== log-output: captures update output ==="
updateapp=ex-logout-update
updateport=3155
CLEANUP_APPS+=("$updateapp")
updir="$WORK_ROOT/update-logtest"
incoming="$updir/incoming"
smoke="$updir/smoke.sh"
if ( set -e
    rm -rf "$updir"
    mkdir -p "$updir/shared" "$incoming"
    echo "PORT=$updateport" > "$updir/shared/.env"
    cat > "$smoke" <<'SHEOF'
#!/bin/bash
[ -f "$1/server.js" ]
SHEOF
    chmod +x "$smoke"

    vigil add "$updateapp" \
        --type app \
        --command "node server.js" \
        --port "$updateport" \
        --working-dir "$updir" \
        --env-file "$updir/shared/.env" \
        --smoke-test-script "$smoke" \
        --bundled-deps

    mkdir -p /tmp/vigil-e2e-rel-logtest
    cp "$EXAMPLE_DIR/app/server.js" /tmp/vigil-e2e-rel-logtest/server.js
    tar -czf "$incoming/v1.0.0.tar.gz" -C /tmp/vigil-e2e-rel-logtest .
    sha256sum "$incoming/v1.0.0.tar.gz" | awk '{print $1}' > "$incoming/v1.0.0.tar.gz.sha256"

    out=$(vigil_capture update "$updateapp" --log-output "$logbase/update.log")
    echo "$out" | grep -q "\"app\":\"$updateapp\""
    [ -s "$logbase/update.log" ]
); then
    pass "log-output captures update output with app name"
else
    fail "log-output captures update output with app name"
fi
