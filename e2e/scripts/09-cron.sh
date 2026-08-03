#!/bin/bash
source "$(dirname "$0")/lib.sh"

echo "=== cron: list empty ==="
if ( set -e
    out=$(vigil_capture cron list)
    echo "$out" | grep -q "No cron jobs configured"
); then
    pass "cron list empty"
else
    fail "cron list empty"
fi

echo "=== cron: add from config file ==="
if ( set -e
    vigil cron add --config "$EXAMPLE_DIR/cronfile.json"
    crontab -l 2>/dev/null | grep -q "vigil-cron:test-cron"
    out=$(vigil_capture cron list)
    echo "$out" | grep -q "test-cron"
); then
    pass "cron add from config"
else
    fail "cron add from config"
fi

echo "=== cron: status active ==="
if ( set -e
    out=$(vigil_capture cron status test-cron)
    echo "$out" | grep -q "Cron daemon: running"
    echo "$out" | grep -q "Status:   active"
); then
    pass "cron status active"
else
    fail "cron status active"
fi

echo "=== cron: duplicate add error ==="
if ( set -e
    out=$(vigil_capture cron add test-cron --schedule "0 3 * * *" --command "/bin/true" || true)
    echo "$out" | grep -qi "already exists"
); then
    pass "cron duplicate add error"
else
    fail "cron duplicate add error"
fi

echo "=== cron: disable/enable ==="
if ( set -e
    vigil cron disable test-cron
    out=$(vigil_capture cron status test-cron)
    echo "$out" | grep -q "Status:   disabled"

    vigil cron enable test-cron
    out=$(vigil_capture cron status test-cron)
    echo "$out" | grep -q "Status:   active"
); then
    pass "cron disable/enable"
else
    fail "cron disable/enable"
fi

echo "=== cron: remove ==="
if ( set -e
    vigil cron remove test-cron
    crontab -l 2>/dev/null | grep -q "vigil-cron:test-cron" && exit 1 || true
    out=$(vigil_capture cron list)
    echo "$out" | grep -q "No cron jobs configured"
); then
    pass "cron remove"
else
    fail "cron remove"
fi
