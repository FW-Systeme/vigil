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
- `--smoke-test-script <path>` (required) enable release management
- `--install-cmd <string>` dependency install command (required for backend if --bundled-deps is false)
- `--bundled-deps` skip dependency installation if deps are in archive
- `--build-dir <path>` (static type)
- `--nginx-domain/--nginx-path/--nginx-config` (static type)
- `--env-file <path>`
- `--kill-mode <mode>` systemd KillMode: `process`, `control-group`, `mixed`, or `none` (default: empty = systemd default `control-group`). Use `process` for self-updating apps to prevent systemd from killing the update process during restart.
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
- `--quiet` suppress stdout output.
- `--log-output <path>` write JSON-line log to file or directory (see below).

#### --log-output Semantics

| Path Type | Behavior | Example |
|-----------|----------|---------|
| Existing directory | Auto-name: `vigil-update-<YYYYMMDD-HHMMSS>-<PID>.log` | `--log-output .` → `./vigil-update-20260817-121443-3810307.log` |
| Non-existing path (no trailing `/`) | Treat as file, create parent dirs | `--log-output /var/log/vigil/update.log` |
| Path with trailing `/` | Treat as directory, create it, auto-name inside | `--log-output /var/log/vigil/` → `/var/log/vigil/vigil-update-20260817-121443-3810307.log` |

**Watchdog integration:** Pass explicit file path (`--log-output /path/to/update-<jobId>-<app>.log`) and read that exact file to check update status.

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
├── .vigil.lock         lock file (PID inside, stale-lock detection active)
└── vigil-update-*.log  JSON-line update logs (when --log-output is used with directory path)
```

### Update Steps (13 steps)
1. Lock `.vigil.lock` (fail if held by live process; stale locks from dead processes are auto-removed via PID check)
2. Ensure dirs: releases/, shared/, incoming/
3. Resolve version (--version flag or scan incoming/ for `*.tar.gz`)
4. SHA256 integrity check (if `.sha256` file present)
5. Extract tar.gz into `releases/<version>/`
6. Run install_cmd (skip if --bundled-deps)
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
| Contents | App code + dependency manifests (e.g. package.json + package-lock.json) |
| bundled-deps | If true, deps in archive, skip install_cmd |

**Critical**: Archive must be FLAT. Wrapping folder corrupts paths.
```
✅ server.js, package.json, lib/
❌ my-app-v1.0.0/{server.js, package.json}
```

### Console Output (example)

Output is structured JSON lines (one per event):

```jsonl
{"ts":"2026-07-21T10:30:01Z","event":"lock.acquired"}
{"ts":"2026-07-21T10:30:01Z","event":"version.resolve","fields":{"version":"v1.2.0","source":"incoming"}}
{"ts":"2026-07-21T10:30:02Z","event":"integrity.checked"}
{"ts":"2026-07-21T10:30:02Z","event":"extract.start","fields":{"version":"v1.2.0"}}
{"ts":"2026-07-21T10:30:05Z","event":"extract.done","fields":{"version":"v1.2.0"}}
{"ts":"2026-07-21T10:30:05Z","event":"deps.install_start"}
{"ts":"2026-07-21T10:30:12Z","event":"deps.install_done"}
{"ts":"2026-07-21T10:30:12Z","event":"shared.link_done"}
{"ts":"2026-07-21T10:30:12Z","event":"symlink.switch","fields":{"version":"v1.2.0"}}
{"ts":"2026-07-21T10:30:13Z","event":"service.restart_done"}
{"ts":"2026-07-21T10:30:14Z","event":"smoke_test.passed"}
{"ts":"2026-07-21T10:30:14Z","event":"cleanup.done"}
{"ts":"2026-07-21T10:30:14Z","event":"update.complete","fields":{"version":"v1.2.0"}}
{"ts":"2026-07-21T10:30:14Z","event":"lock.released"}
```

Error/rollback path:
```jsonl
{"ts":"2026-07-21T10:30:14Z","event":"smoke_test.failed","fields":{"error":"exit status 1"}}
{"ts":"2026-07-21T10:30:14Z","event":"rollback.start"}
{"ts":"2026-07-21T10:30:14Z","event":"rollback.done"}
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
