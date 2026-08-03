#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VIGIL_BIN="${VIGIL_BIN:-/usr/local/bin/vigil}"
LOG_DIR="${LOG_DIR:-/logs}"
EXAMPLE_DIR="/e2e/fixtures/example-project"
NODE_BIN="${VIRGIL_E2E_NODE_BIN:-$(command -v node)}"
NGINX_AVAILABLE=false

systemctl is-active --quiet nginx 2>/dev/null && NGINX_AVAILABLE=true

SCRIPT_NAME="$(basename "$0")"
VIRGIL_HOME="/tmp/vigil-e2e/${SCRIPT_NAME%.sh}"
WORK_ROOT="/tmp/vigil-e2e-work/${SCRIPT_NAME%.sh}"
RESULT_DIR="${RESULT_DIR:-/tmp/e2e-results}"
CLI_LOG="$LOG_DIR/cli.log"

mkdir -p "$VIRGIL_HOME" "$RESULT_DIR"
rm -rf "$VIRGIL_HOME"/* "$WORK_ROOT"

# mktemp_dir <name> <port> — build an isolated working dir for a backend app
mktemp_dir() {
    local dir="$WORK_ROOT/$1"
    mkdir -p "$dir/current" "$dir/shared"
    echo "PORT=$2" > "$dir/shared/.env"
    echo "$dir"
}

PASS=0; FAIL=0; SKIP=0
CLEANUP_APPS=()

vigil() {
    echo "\$ vigil $*" >> "$CLI_LOG"
    VIRGIL_HOME="$VIRGIL_HOME" "$VIGIL_BIN" "$@" >> "$CLI_LOG" 2>&1
}

vigil_capture() {
    echo "\$ vigil $*" >> "$CLI_LOG"
    local out
    out=$(VIRGIL_HOME="$VIRGIL_HOME" "$VIGIL_BIN" "$@" 2>&1)
    local rc=$?
    echo "$out" >> "$CLI_LOG"
    echo "$out"
    return "$rc"
}

pass() { echo "  PASS $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL $1"; FAIL=$((FAIL + 1)); }
skip() { echo "  SKIP $1"; SKIP=$((SKIP + 1)); }

cleanup() {
    set +e
    for app in "${CLEANUP_APPS[@]}"; do
        VIRGIL_HOME="$VIRGIL_HOME" "$VIGIL_BIN" remove "$app" >/dev/null 2>&1
        rm -f "/etc/systemd/system/$app.service" "/etc/nginx/sites-available/$app.conf" "/etc/nginx/sites-enabled/$app.conf"
    done
    nginx -s reload >/dev/null 2>&1
    systemctl daemon-reload >/dev/null 2>&1
    rm -rf "$VIRGIL_HOME"
}

result() {
    echo "$PASS $FAIL $SKIP" > "$RESULT_DIR/$SCRIPT_NAME"
}

trap 'cleanup; result' EXIT

wait_for_service_active() {
    local name="$1"
    for i in $(seq 1 40); do
        systemctl is-active --quiet "$name.service" 2>/dev/null && return 0
        sleep 0.5
    done
    return 1
}

wait_for_service_inactive() {
    local name="$1"
    for i in $(seq 1 40); do
        systemctl is-active --quiet "$name.service" 2>/dev/null || return 0
        sleep 0.5
    done
    return 1
}

wait_for_port() {
    local port="$1"
    for i in $(seq 1 40); do
        curl -sf "http://localhost:$port/" >/dev/null 2>&1 && return 0
        sleep 0.5
    done
    return 1
}

wait_for_port_closed() {
    local port="$1"
    for i in $(seq 1 40); do
        curl -sf "http://localhost:$port/" >/dev/null 2>&1 || return 0
        sleep 0.5
    done
    return 1
}
