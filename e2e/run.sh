#!/bin/bash
set -euo pipefail

LOG_DIR="${LOG_DIR:-/logs}"

# --- Container entrypoint (PID 1) ---
# systemd requires PID 1. Hand off to it; systemd boots multi-user.target
# and starts vigil-e2e.service, which re-invokes this script as a oneshot
# unit to run the test suite automatically on container start.
if [ "$$" -eq 1 ]; then
    exec /lib/systemd/systemd
fi

# --- Test runner (started by vigil-e2e.service after boot) ---
mkdir -p "$LOG_DIR" "$LOG_DIR/nginx" /tmp/e2e-results
: > "$LOG_DIR/e2e.log"
: > "$LOG_DIR/cli.log"

# mirror all output to console + /logs/e2e.log
exec > >(tee -a "$LOG_DIR/e2e.log") 2>&1

# redirect nginx logs into the mirrored volume
sed -i 's|/var/log/nginx/access.log|/logs/nginx/access.log|g; s|/var/log/nginx/error.log|/logs/nginx/error.log|g' /etc/nginx/nginx.conf
if nginx -t >/dev/null 2>&1; then
    nginx -s reload >/dev/null 2>&1 || true
fi

echo "=========================================="
echo "  Virgil e2e test suite"
echo "  started: $(date -u +%FT%TZ)"
echo "=========================================="

rc=0
for script in /e2e/scripts/*.sh; do
    [ -r "$script" ] || continue
    [ "$(basename "$script")" = "lib.sh" ] && continue
    echo ""
    echo ">>> $(basename "$script")"
    if ! bash "$script"; then
        echo ">>> $(basename "$script"): FAILED"
        rc=1
    fi
done

pass=0; fail=0; skip=0
for f in /tmp/e2e-results/*; do
    [ -f "$f" ] || continue
    read -r p fa s < "$f" || true
    pass=$((pass + ${p:-0})); fail=$((fail + ${fa:-0})); skip=$((skip + ${s:-0}))
done

echo ""
echo "=========================================="
echo "  Results: $pass passed, $fail failed, $skip skipped"
echo "=========================================="

if [ "$fail" -eq 0 ]; then
    echo "PASS" > "$LOG_DIR/result"
else
    echo "FAIL" > "$LOG_DIR/result"
fi

# stop the container; host verdict is read from the mirrored $LOG_DIR/result
systemctl poweroff --no-block || true
exit "$rc"
