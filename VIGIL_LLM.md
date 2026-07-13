# Vigil — LLM Context

## What
Lightweight CLI process manager for Linux. PM2 alternative. Backend on systemd, static sites on nginx. Written in Go.

## Key Properties
- Manage apps (Node.js/Go/Python/any binary) via systemd units
- Manage static sites via nginx site configs
- Release management (zero-downtime deploys) for apps with smoke test
- JSON store: `/etc/vigil/apps/<name>.json` (root) or `~/.config/vigil/apps/<name>.json`
- Apps stored as individual JSON files, atomic writes (temp file + os.Rename)

## Types
| Type | Backend | Mechanism |
|------|---------|-----------|
| `app` / `node` | systemd | systemd unit file, DBus control |
| `static` | nginx | `/etc/nginx/sites-{available,enabled}/` |

## Commands

### `vigil add <name>`
Register app. Flags:
- `--type app|static` (required)
- `--port <int>` (required)
- `--entry <path>` entry script (app type)
- `--working-dir <path>`
- `--command <string>` custom ExecStart (overrides --entry)
- `--build-cmd <string>` build cmd before start
- `--smoke-test-script <path>` enable release management
- `--bundled-deps` skip npm ci if node_modules in archive
- `--build-dir <path>` (static type)
- `--nginx-domain/--nginx-path/--nginx-config` (static type)
- `--env-file <path>`
- `--force` overwrite existing
- `--config <path>` ecosystem.json bulk add

### `vigil remove <name>`
Stop, disable unit/site, delete configs, remove from store.

### `vigil list`
List registered apps. Output: `name type port status`

### `vigil start|stop|restart <name>`
Control app. app type → systemctl. static type → nginx reload.

### `vigil update <name>`
Release update for apps with --smoke-test-script. Flags:
- `--version <string>` explicit version. Auto-detect from incoming/ if empty.
- `--quiet` suppress progress output.

### `vigil init`
Generate ecosystem.json template.

## Update Process (Release Management)

### Prerequisites
- App must have `--smoke-test-script` set
- Smoke test script: `smoke.sh <release-dir>` exit 0 = success

### Directory Structure
```
<working-dir>/
├── releases/v1.0.0/   extracted releases
├── shared/             persistent data (.env, config) symlinked into each release
├── incoming/           upload .tar.gz packages here
├── current → releases/v1.0.0/  atomic symlink
└── .vigil.lock         lock file (PID inside)
```

### Update Steps (13 steps)
1. Lock `.vigil.lock` (fail if held)
2. Ensure dirs: releases/, shared/, incoming/
3. Resolve version (--version flag or scan incoming/ for `*.tar.gz`)
4. SHA256 integrity check (if `.sha256` file present)
5. Extract tar.gz into `releases/<version>/`
6. npm ci --production --ignore-scripts (skip if --bundled-deps)
7. Symlink shared/ files into release dir
8. Atomic symlink switch: current → new release
9. Restart service (systemd)
10. Run smoke test script
11. Rollback on failure: revert symlink, restart old, delete new release
12. Cleanup old releases (keep newest 3)
13. Remove lock

### Incoming Package Requirements
| Requirement | Details |
|-------------|---------|
| Filename | `<version>.tar.gz` (version extracted from filename) |
| Format | gzip-compressed tar, flat structure (no wrapping dir) |
| SHA256 | Optional: `<file>.tar.gz.sha256` with hex hash |
| Contents | App code + package.json + package-lock.json (if npm deps needed) |
| bundled-deps | If true, node_modules/ in archive, skip npm ci |

**Critical**: Archive must be FLAT. Wrapping folder corrupts paths.
```
✅ server.js, package.json, lib/
❌ my-app-v1.0.0/{server.js, package.json}
```

### Console Output (example)
```
Lock acquired
Using version v1.2.0
Integrity check passed
Extracting v1.2.0.tar.gz...
Installing dependencies (npm ci)...
Linking shared data...
Switching symlink to v1.2.0...
Restarting service...
Running smoke test...
Smoke test passed
Cleaned up old releases
```

### Rollback
Triggered by: restart failure OR smoke test non-zero. Reverts symlink, restarts old release, keeps failed release dir for analysis.

## Architecture
```
cmd/vigil/main.go → internal/cli/ (cobra commands)
  ├── internal/process/  manager + JSON store + validation
  │   ├── internal/systemd/ DBus control
  │   └── internal/nginx/   site config management
  └── internal/update/   release orchestrator
```
