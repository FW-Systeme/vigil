#!/bin/bash
source "$(dirname "$0")/lib.sh"

name=ex-json-app
port=3150
CLEANUP_APPS+=("$name")
dir=$(mktemp_dir "$name" "$port")
eco="/tmp/vigil-eco-app.json"

echo "=== json: app lifecycle (single object) ==="
if ( set -e
    cat > "$eco" <<EOF
{
  "name": "$name",
  "type": "app",
  "command": "$NODE_BIN $EXAMPLE_DIR/app/server.js",
  "port": $port,
  "working_dir": "$dir",
  "env_file": "$dir/shared/.env",
  "smoke_test_script": "$EXAMPLE_DIR/smoke-pass.sh",
  "bundled_deps": true
}
EOF

    vigil add --config "$eco"
    out=$(vigil_capture list)
    echo "$out" | grep -q "$name"

    vigil start "$name"
    wait_for_service_active "$name"
    wait_for_port "$port"
    resp=$(curl -sf "http://localhost:$port/")
    [ "$resp" = "ok" ]

    logs=$(vigil_capture logs "$name" --lines=5 || true)
    [ -n "$logs" ]

    vigil stop "$name"
    wait_for_service_inactive "$name"

    vigil remove "$name"
    [ ! -f "$VIRGIL_HOME/apps/$name.json" ]
    [ ! -f "/etc/systemd/system/$name.service" ]
); then
    pass "json app lifecycle (single object)"
else
    fail "json app lifecycle (single object)"
fi

site=ex-json-site
port=8150
CLEANUP_APPS+=("$site")
eco2="/tmp/vigil-eco-static.json"

echo "=== json: static lifecycle (apps array) ==="
if ( set -e
    cat > "$eco2" <<EOF
{
  "apps": [
    {
      "name": "$site",
      "type": "static",
      "build_dir": "$EXAMPLE_DIR/static",
      "port": $port,
      "nginx_domain": "test.local",
      "nginx_path": "$EXAMPLE_DIR/static",
      "smoke_test_script": "$EXAMPLE_DIR/smoke-pass.sh"
    }
  ]
}
EOF

    vigil add --config "$eco2"
    out=$(vigil_capture list)
    echo "$out" | grep -q "$site"

    vigil start "$site"
    sleep 1
    resp=$(curl -s -H "Host: test.local" "http://localhost:$port/")
    [ "$resp" = "<h1>vigil e2e test</h1>" ]

    vigil stop "$site"

    vigil remove "$site"
    [ ! -f "$VIRGIL_HOME/apps/$site.json" ]
    [ ! -f "/etc/nginx/sites-available/$site.conf" ]
); then
    pass "json static lifecycle (apps array)"
else
    fail "json static lifecycle (apps array)"
fi

bulk_app=ex-json-bulk-app
bulk_site=ex-json-bulk-site
app_port=3151
site_port=8151
CLEANUP_APPS+=("$bulk_app" "$bulk_site")
app_dir=$(mktemp_dir "$bulk_app" "$app_port")
eco3="/tmp/vigil-eco-bulk.json"

echo "=== json: bulk mixed add (apps array) ==="
if ( set -e
    cat > "$eco3" <<EOF
{
  "apps": [
    {
      "name": "$bulk_app",
      "type": "app",
      "command": "$NODE_BIN $EXAMPLE_DIR/app/server.js",
      "port": $app_port,
      "working_dir": "$app_dir",
      "env_file": "$app_dir/shared/.env",
      "smoke_test_script": "$EXAMPLE_DIR/smoke-pass.sh",
      "bundled_deps": true
    },
    {
      "name": "$bulk_site",
      "type": "static",
      "build_dir": "$EXAMPLE_DIR/static",
      "port": $site_port,
      "nginx_domain": "test.local",
      "nginx_path": "$EXAMPLE_DIR/static",
      "smoke_test_script": "$EXAMPLE_DIR/smoke-pass.sh"
    }
  ]
}
EOF

    vigil add --config "$eco3"
    out=$(vigil_capture list)
    echo "$out" | grep -q "$bulk_app"
    echo "$out" | grep -q "$bulk_site"

    vigil start "$bulk_app"
    wait_for_service_active "$bulk_app"
    wait_for_port "$app_port"
    resp=$(curl -sf "http://localhost:$app_port/")
    [ "$resp" = "ok" ]

    vigil start "$bulk_site"
    sleep 1
    resp=$(curl -s -H "Host: test.local" "http://localhost:$site_port/")
    [ "$resp" = "<h1>vigil e2e test</h1>" ]

    vigil remove "$bulk_app"
    vigil remove "$bulk_site"
    [ ! -f "$VIRGIL_HOME/apps/$bulk_app.json" ]
    [ ! -f "/etc/nginx/sites-available/$bulk_site.conf" ]
); then
    pass "json bulk mixed add"
else
    fail "json bulk mixed add"
fi

cronfile="/tmp/vigil-eco-cron.json"

echo "=== json: cron add (single object) ==="
if ( set -e
    cat > "$cronfile" <<EOF
{
  "name": "json-cron-one",
  "schedule": "*/5 * * * *",
  "command": "echo json-cron-one >> /tmp/vigil-json-cron.log"
}
EOF

    vigil cron add --config "$cronfile"
    crontab -l 2>/dev/null | grep -q "vigil-cron:json-cron-one"
    out=$(vigil_capture cron status json-cron-one)
    echo "$out" | grep -q "Cron daemon: running"
    echo "$out" | grep -q "Status:   active"

    vigil cron disable json-cron-one
    out=$(vigil_capture cron status json-cron-one)
    echo "$out" | grep -q "Status:   disabled"

    vigil cron enable json-cron-one
    vigil cron remove json-cron-one
    crontab -l 2>/dev/null | grep -q "vigil-cron:json-cron-one" && exit 1 || true
); then
    pass "json cron add (single object)"
else
    fail "json cron add (single object)"
fi

cronfile2="/tmp/vigil-eco-cron2.json"

echo "=== json: cron bulk add (crons array) ==="
if ( set -e
    cat > "$cronfile2" <<EOF
{
  "crons": [
    {
      "name": "json-cron-a",
      "schedule": "0 3 * * *",
      "command": "/bin/true"
    },
    {
      "name": "json-cron-b",
      "schedule": "0 4 * * *",
      "command": "/bin/true"
    }
  ]
}
EOF

    vigil cron add --config "$cronfile2"
    crontab -l 2>/dev/null | grep -q "vigil-cron:json-cron-a"
    crontab -l 2>/dev/null | grep -q "vigil-cron:json-cron-b"
    out=$(vigil_capture cron list)
    echo "$out" | grep -q "json-cron-a"
    echo "$out" | grep -q "json-cron-b"

    vigil cron remove json-cron-a
    vigil cron remove json-cron-b
    crontab -l 2>/dev/null | grep -q "vigil-cron:json-cron-a" && exit 1 || true
); then
    pass "json cron bulk add (crons array)"
else
    fail "json cron bulk add (crons array)"
fi

filter_a=ex-json-f-a
filter_b=ex-json-f-b
port_a=3152
port_b=3153
CLEANUP_APPS+=("$filter_a" "$filter_b")
dir_a=$(mktemp_dir "$filter_a" "$port_a")
dir_b=$(mktemp_dir "$filter_b" "$port_b")
eco4="/tmp/vigil-eco-filter.json"

echo "=== json: add with name filter ==="
if ( set -e
    cat > "$eco4" <<EOF
{
  "apps": [
    {
      "name": "$filter_a",
      "type": "app",
      "command": "$NODE_BIN $EXAMPLE_DIR/app/server.js",
      "port": $port_a,
      "working_dir": "$dir_a",
      "env_file": "$dir_a/shared/.env",
      "smoke_test_script": "$EXAMPLE_DIR/smoke-pass.sh",
      "bundled_deps": true
    },
    {
      "name": "$filter_b",
      "type": "app",
      "command": "$NODE_BIN $EXAMPLE_DIR/app/server.js",
      "port": $port_b,
      "working_dir": "$dir_b",
      "env_file": "$dir_b/shared/.env",
      "smoke_test_script": "$EXAMPLE_DIR/smoke-pass.sh",
      "bundled_deps": true
    }
  ]
}
EOF

    vigil add --config "$eco4" "$filter_a"
    out=$(vigil_capture list)
    echo "$out" | grep -q "$filter_a"
    echo "$out" | grep -q "$filter_b" && exit 1 || true

    vigil remove "$filter_a"
); then
    pass "json add with name filter"
else
    fail "json add with name filter"
fi
