#!/bin/bash
source "$(dirname "$0")/lib.sh"

echo "=== init: ecosystem template ==="
if ( set -e
    vigil init -o /tmp/vigil-init-eco.json
    [ -s /tmp/vigil-init-eco.json ]
    grep -q '"name"' /tmp/vigil-init-eco.json
    grep -q '"port"' /tmp/vigil-init-eco.json
); then
    pass "init ecosystem template"
else
    fail "init ecosystem template"
fi

echo "=== init: cronfile template ==="
if ( set -e
    vigil cron init -o /tmp/vigil-init-cron.json
    [ -s /tmp/vigil-init-cron.json ]
    grep -q '"crons"' /tmp/vigil-init-cron.json
); then
    pass "init cronfile template"
else
    fail "init cronfile template"
fi

echo "=== list: empty home ==="
if ( set -e
    out=$(vigil_capture list)
    echo "$out" | grep -q "No apps registered"
); then
    pass "list empty home"
else
    fail "list empty home"
fi
