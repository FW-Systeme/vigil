#!/bin/bash
source "$(dirname "$0")/lib.sh"

name=ex-update-app
port=3147
CLEANUP_APPS+=("$name")
proj="$WORK_ROOT/update-proj"
appdir="$proj/app"
incoming="$appdir/incoming"
smoke="$proj/smoke.sh"

echo "=== update: setup project ==="
if ( set -e
    mkdir -p "$appdir/shared" "$incoming"
    echo "PORT=$port" > "$appdir/shared/.env"
    cat > "$smoke" <<'EOF'
#!/bin/bash
[ -f "$1/server.js" ]
EOF
    chmod +x "$smoke"

    vigil add "$name" \
        --type app \
        --command "node server.js" \
        --port "$port" \
        --working-dir "$appdir" \
        --env-file "$appdir/shared/.env" \
        --smoke-test-script "$smoke" \
        --bundled-deps
); then
    pass "update setup project"
else
    fail "update setup project"
fi

echo "=== update: integrity failure ==="
if ( set -e
    mkdir -p /tmp/vigil-e2e-rel-0.9.0
    cp "$EXAMPLE_DIR/app/server.js" /tmp/vigil-e2e-rel-0.9.0/server.js
    echo "0.9.0" > /tmp/vigil-e2e-rel-0.9.0/VERSION
    tar -czf "$incoming/v0.9.0.tar.gz" -C /tmp/vigil-e2e-rel-0.9.0 .
    sha256sum "$incoming/v0.9.0.tar.gz" | awk '{print $1}' > "$incoming/v0.9.0.tar.gz.sha256"
    # corrupt the checksum
    echo "0000000000000000000000000000000000000000000000000000000000000000" > "$incoming/v0.9.0.tar.gz.sha256"

    if vigil_capture update "$name" --version v0.9.0 >/dev/null 2>&1; then
        false
    fi
    out=$(vigil_capture update "$name" --version v0.9.0 || true)
    echo "$out" | grep -qi "integrity"
    [ ! -d "$appdir/releases/v0.9.0" ]
); then
    pass "update integrity failure"
else
    fail "update integrity failure"
fi

echo "=== update: successful release ==="
if ( set -e
    mkdir -p /tmp/vigil-e2e-rel-1.0.0
    cp "$EXAMPLE_DIR/app/server.js" /tmp/vigil-e2e-rel-1.0.0/server.js
    echo "1.0.0" > /tmp/vigil-e2e-rel-1.0.0/VERSION
    tar -czf "$incoming/v1.0.0.tar.gz" -C /tmp/vigil-e2e-rel-1.0.0 .
    sha256sum "$incoming/v1.0.0.tar.gz" | awk '{print $1}' > "$incoming/v1.0.0.tar.gz.sha256"

    vigil update "$name" --version v1.0.0
    [ "$(cat "$appdir/releases/v1.0.0/VERSION")" = "1.0.0" ]
    [ "$(readlink "$appdir/current")" = "$appdir/releases/v1.0.0" ]
    wait_for_service_active "$name"
    wait_for_port "$port"
    resp=$(curl -sf "http://localhost:$port/")
    [ -n "$resp" ]
); then
    pass "update successful release"
else
    fail "update successful release"
fi

echo "=== update: auto-detect version from incoming ==="
if ( set -e
    rm -f "$incoming"/*.tar.gz "$incoming"/*.sha256
    mkdir -p /tmp/vigil-e2e-rel-1.1.0
    cp "$EXAMPLE_DIR/app/server.js" /tmp/vigil-e2e-rel-1.1.0/server.js
    echo "1.1.0" > /tmp/vigil-e2e-rel-1.1.0/VERSION
    tar -czf "$incoming/v1.1.0.tar.gz" -C /tmp/vigil-e2e-rel-1.1.0 .
    sha256sum "$incoming/v1.1.0.tar.gz" | awk '{print $1}' > "$incoming/v1.1.0.tar.gz.sha256"

    vigil update "$name"
    [ "$(cat "$appdir/releases/v1.1.0/VERSION")" = "1.1.0" ]
    [ "$(readlink "$appdir/current")" = "$appdir/releases/v1.1.0" ]
); then
    pass "update auto-detect version"
else
    fail "update auto-detect version"
fi

echo "=== update: JSON output contains app name ==="
if ( set -e
    rm -f "$incoming"/*.tar.gz "$incoming"/*.sha256
    mkdir -p /tmp/vigil-e2e-rel-1.2.0
    cp "$EXAMPLE_DIR/app/server.js" /tmp/vigil-e2e-rel-1.2.0/server.js
    echo "1.2.0" > /tmp/vigil-e2e-rel-1.2.0/VERSION
    tar -czf "$incoming/v1.2.0.tar.gz" -C /tmp/vigil-e2e-rel-1.2.0 .
    sha256sum "$incoming/v1.2.0.tar.gz" | awk '{print $1}' > "$incoming/v1.2.0.tar.gz.sha256"

    out=$(vigil_capture update "$name" --version v1.2.0)
    echo "$out" | grep -q "\"app\":\"$name\""
    echo "$out" | grep -q "\"event\":\"update.complete\""
); then
    pass "update JSON contains app name"
else
    fail "update JSON contains app name"
fi
