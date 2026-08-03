#!/usr/bin/env bash
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VIGIL_BIN="${VIGIL_BIN:-/usr/local/bin/vigil}"
VIRGIL_HOME="/tmp/vigil-e2e-$$"
EXAMPLE_DIR="$SCRIPT_DIR/fixtures/example-project"
NODE_BIN="${VIRGIL_E2E_NODE_BIN:-$(command -v node)}"

if [ ! -x "$VIGIL_BIN" ]; then
    echo ">>> Building vigil..."
    (cd "$PROJECT_ROOT" && go build -o "$VIGIL_BIN" ./cmd/vigil/) || exit 1
fi

for cmd in curl systemctl; do
    command -v $cmd >/dev/null 2>&1 || { echo "Missing: $cmd"; exit 1; }
done

NGINX_AVAILABLE=false
systemctl is-active --quiet nginx 2>/dev/null && NGINX_AVAILABLE=true

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
PASS=0; FAIL=0; SKIP=0

if [ "$(id -u)" -ne 0 ]; then
    command -v sudo >/dev/null 2>&1 || { echo "Missing: sudo (not root)"; exit 1; }
    SUDO="sudo"
    vigil() { sudo VIRGIL_HOME="$VIRGIL_HOME" "$VIGIL_BIN" "$@"; }
else
    SUDO=""
    vigil() { VIRGIL_HOME="$VIRGIL_HOME" "$VIGIL_BIN" "$@"; }
fi

CLEANUP_APPS=()
cleanup() {
    set +e
    echo ">>> Cleanup"
    for app in "${CLEANUP_APPS[@]}"; do
        vigil remove "$app" >/dev/null 2>&1
        $SUDO rm -f "/etc/systemd/system/$app.service" "/etc/nginx/sites-available/$app.conf" "/etc/nginx/sites-enabled/$app.conf"
    done
    $SUDO nginx -s reload 2>/dev/null || true
    $SUDO systemctl daemon-reload 2>/dev/null || true
    rm -rf "$VIRGIL_HOME"
}
trap cleanup EXIT

pass() { echo -e "  ${GREEN}PASS${NC} $1"; ((PASS++)); }
fail() { echo -e "  ${RED}FAIL${NC} $1"; ((FAIL++)); }
skip() { echo -e "  ${YELLOW}SKIP${NC} $1"; ((SKIP++)); }

wait_for_service_active() {
    local name="$1"
    for i in $(seq 1 30); do
        systemctl is-active --quiet "$name.service" 2>/dev/null && return 0
        sleep 0.5
    done
    return 1
}

wait_for_service_inactive() {
    local name="$1"
    for i in $(seq 1 30); do
        systemctl is-active --quiet "$name.service" 2>/dev/null || return 0
        sleep 0.5
    done
    return 1
}

wait_for_port() {
    local port="$1"
    for i in $(seq 1 20); do
        curl -sf "http://localhost:$port/" >/dev/null 2>&1 && return 0
        sleep 0.5
    done
    return 1
}

wait_for_port_closed() {
    local port="$1"
    for i in $(seq 1 20); do
        curl -sf "http://localhost:$port/" >/dev/null 2>&1 || return 0
        sleep 0.5
    done
    return 1
}

# ============================
# Tests
# ============================

test_example_app_lifecycle() {
    local name="ex-app-lifecycle"
    local port=3130
    CLEANUP_APPS+=("$name")
    echo ""
    echo "=== App Lifecycle ($name) ==="

    if (
        set -e

        vigil add "$name" \
            --type app \
            --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
            --port "$port" \
            --working-dir "$EXAMPLE_DIR/app" \
            --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
            --bundled-deps

        vigil start "$name"
        wait_for_service_active "$name"
        wait_for_port "$port"

        resp=$(curl -sf "http://localhost:$port/")
        [ "$resp" = "ok" ]

        vigil stop "$name"
        wait_for_service_inactive "$name"

        vigil remove "$name"
        [ ! -f "$VIRGIL_HOME/apps/$name.json" ]
        [ ! -f "/etc/systemd/system/$name.service" ]
    ); then
        pass "App Lifecycle"
    else
        fail "App Lifecycle"
    fi
}

test_example_static_lifecycle() {
    local name="ex-static-lifecycle"
    local port=8085
    CLEANUP_APPS+=("$name")
    echo ""
    echo "=== Static Lifecycle ($name) ==="

    if ! $NGINX_AVAILABLE; then
        skip "Static Lifecycle"
        return
    fi

    if (
        set -e

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
        [ "$resp" = "<h1>Vigil Static Test</h1>" ]

        vigil stop "$name"

        vigil remove "$name"
        [ ! -f "$VIRGIL_HOME/apps/$name.json" ]
        [ ! -f "/etc/nginx/sites-available/$name.conf" ]
    ); then
        pass "Static Lifecycle"
    else
        fail "Static Lifecycle"
    fi
}

test_example_ecosystem_bulk() {
    local app_name="ex-eco-app"
    local static_name="ex-eco-site"
    local app_port=3140
    local static_port=8087
    CLEANUP_APPS+=("$app_name" "$static_name")
    local eco_file="/tmp/vigil-eco-$$.json"
    echo ""
    echo "=== Ecosystem Bulk Add ==="

    if ! $NGINX_AVAILABLE; then
        skip "Ecosystem Bulk Add"
        return
    fi

    if (
        set -e

        cat > "$eco_file" << ECOEOF
{
  "apps": [
    {
      "name": "$app_name",
      "type": "app",
      "command": "$NODE_BIN $EXAMPLE_DIR/app/server.js",
      "port": $app_port,
      "working_dir": "$EXAMPLE_DIR/app",
      "smoke_test_script": "$EXAMPLE_DIR/smoke-pass.sh",
      "bundled_deps": true
    },
    {
      "name": "$static_name",
      "type": "static",
      "build_dir": "$EXAMPLE_DIR/static",
      "port": $static_port,
      "nginx_domain": "test.local",
      "nginx_path": "$EXAMPLE_DIR/static",
      "smoke_test_script": "$EXAMPLE_DIR/smoke-pass.sh"
    }
  ]
}
ECOEOF

        vigil add --config "$eco_file"

        list=$(vigil list 2>&1)
        echo "$list" | grep -q "$app_name"
        echo "$list" | grep -q "$static_name"

        vigil remove "$app_name"
        vigil remove "$static_name"
        rm -f "$eco_file"
    ); then
        pass "Ecosystem Bulk Add"
    else
        fail "Ecosystem Bulk Add"
    fi
}

test_example_app_logs() {
    local name="ex-app-logs"
    local port=3131
    CLEANUP_APPS+=("$name")
    echo ""
    echo "=== App Logs ($name) ==="

    if (
        set -e

        vigil add "$name" \
            --type app \
            --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
            --port "$port" \
            --working-dir "$EXAMPLE_DIR/app" \
            --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
            --bundled-deps

        vigil start "$name"
        wait_for_service_active "$name"
        wait_for_port "$port"

        curl -sf "http://localhost:$port/" >/dev/null

        logs=$(vigil logs "$name" --lines=5 2>&1)
        [ -n "$logs" ]

        vigil remove "$name"
    ); then
        pass "App Logs"
    else
        fail "App Logs"
    fi
}

test_example_static_logs() {
    local name="ex-static-logs"
    local port=8086
    CLEANUP_APPS+=("$name")
    echo ""
    echo "=== Static Logs ($name) ==="

    if ! $NGINX_AVAILABLE; then
        skip "Static Logs"
        return
    fi

    if (
        set -e

        vigil add "$name" \
            --type static \
            --build-dir "$EXAMPLE_DIR/static" \
            --port "$port" \
            --nginx-domain "test.local" \
            --nginx-path "$EXAMPLE_DIR/static" \
            --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh"

        vigil start "$name"
        sleep 1

        curl -s -H "Host: test.local" "http://localhost:$port/" >/dev/null

        logs=$(vigil logs "$name" --lines=5 2>&1)
        for i in $(seq 1 5); do
            [ -n "$logs" ] && break
            sleep 0.3
            logs=$(vigil logs "$name" --lines=5 2>&1)
        done
        [ -n "$logs" ]

        vigil remove "$name"
    ); then
        pass "Static Logs"
    else
        fail "Static Logs"
    fi
}

test_example_error_duplicate() {
    local name="ex-err-dup"
    local port=3132
    CLEANUP_APPS+=("$name")
    echo ""
    echo "=== Error: Duplicate Add ($name) ==="

    if (
        set -e

        vigil add "$name" \
            --type app \
            --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
            --port "$port" \
            --working-dir "$EXAMPLE_DIR/app" \
            --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
            --bundled-deps

        out=$(vigil add "$name" \
            --type app \
            --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
            --port "$port" \
            --working-dir "$EXAMPLE_DIR/app" \
            --smoke-test-script "$EXAMPLE_DIR/smoke-pass.sh" \
            --bundled-deps 2>&1 || true)

        echo "$out" | grep -qi "already exists"

        vigil remove "$name"
    ); then
        pass "Error: Duplicate Add"
    else
        fail "Error: Duplicate Add"
    fi
}

test_example_error_no_smoke() {
    local name="ex-err-nosmoke"
    local port=3133
    echo ""
    echo "=== Error: Missing Smoke Test Script ==="

    if (
        set -e

        out=$(vigil add "$name" \
            --type app \
            --command "$NODE_BIN $EXAMPLE_DIR/app/server.js" \
            --port "$port" \
            --working-dir "$EXAMPLE_DIR/app" \
            --bundled-deps 2>&1 || true)

        echo "$out" | grep -qi "smoke_test_script"
    ); then
        pass "Error: Missing Smoke Test Script"
    else
        fail "Error: Missing Smoke Test Script"
    fi
}

# ============================
# Run
# ============================

echo ""
echo "=========================================="
echo "  Virgil e2e — example-project"
echo "  VIGIL_BIN=$VIGIL_BIN"
echo "  VIRGIL_HOME=$VIRGIL_HOME"
echo "=========================================="

test_example_app_lifecycle
test_example_static_lifecycle
test_example_ecosystem_bulk
test_example_app_logs
test_example_static_logs
test_example_error_duplicate
test_example_error_no_smoke

echo ""
echo "=========================================="
echo "  Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "=========================================="

[ "$FAIL" -eq 0 ]
