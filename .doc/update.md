# Feature: Update / Release-Management

## Beschreibung

Zero-Downtime Deployments fuer Apps mit Smoke-Test. Laedt `.tar.gz`-Pakete aus `incoming/`, extrahiert sie in versionierte Release-Ordner, schaltet atomar per Symlink um und rollt bei Fehler zurueck.

## Architektur

```
internal/cli/update.go          <- "vigil update" cobra command
internal/update/
├── contract.go                 <- Service interface, RestartFunc, Fehler-Sentinels
├── service.go                  <- Update-Orchestrator (13 Steps + Rollback)
├── logger.go                   <- JSON-Line Logger
├── logger_test.go
└── service_test.go
```

### Typen

```go
// internal/update/contract.go
type Service interface {
    Update(ctx context.Context, name string, version string) error
}
type RestartFunc func(ctx context.Context, name string) error

// internal/update/logger.go
type Logger struct {
    // schreibt JSON-Zeilen nach stdout (optional) und Datei (optional)
}
func NewLogger(w, f io.Writer) *Logger
func (l *Logger) Log(event string, fields map[string]any)
```

### Logger

| Event | Fields | Beschreibung |
|-------|--------|-------------|
| `lock.acquired` | — | Lock gesetzt |
| `lock.released` | — | Lock aufgehoben (defer) |
| `version.resolve` | `version`, `source=incoming\|flag` | Version bestimmt |
| `integrity.checked` | — | SHA256-Pruefung bestanden |
| `integrity.failed` | `error` | SHA256-Mismatch |
| `extract.start` | `version` | Extraktion beginnt |
| `extract.done` | `version` | Extraktion beendet |
| `extract.failed` | `error` | Extraktion fehlgeschlagen |
| `deps.install_start` | — | install_cmd start |
| `deps.install_done` | — | install_cmd beendet |
| `deps.install_failed` | `error` | install_cmd Fehler |
| `deps.skip` | `reason=bundled` | Keine Dep-Installation |
| `shared.link_start` | — | Shared-Daten verlinken |
| `shared.link_done` | — | Shared-Daten verlinkt |
| `shared.link_failed` | `error` | Symlink-Fehler |
| `symlink.switch` | `version` | current-Symlink umschalten |
| `symlink.switch_failed` | `error` | Symlink-Schalter-Fehler |
| `nginx.updated` | — | nginx Config aus Release aktiviert (static type) |
| `nginx.enable_failed` | `error` | nginx EnableSite Fehler |
| `nginx.reload_failed` | `error` | nginx Reload Fehler |
| `service.restart_start` | — | systemd restart |
| `service.restart_done` | — | systemd restart beendet |
| `service.restart_failed` | `error` | systemd restart Fehler |
| `smoke_test.start` | — | Smoke-Test laeuft |
| `smoke_test.passed` | — | Smoke-Test bestanden |
| `smoke_test.failed` | `error` | Smoke-Test Fehler → Rollback |
| `rollback.start` | — | Rollback beginnt |
| `rollback.done` | — | Rollback beendet |
| `rollback.restart_failed` | `error` | Restart waehrend Rollback fehlgeschlagen |
| `rollback.nginx_backup_write_failed` | `error` | nginx Backup-Datei schreiben fehlgeschlagen |
| `rollback.nginx_restore_failed` | `error` | nginx Config wiederherstellen fehlgeschlagen |
| `rollback.nginx_reload_failed` | `error` | nginx Reload nach Rollback fehlgeschlagen |
| `rollback.nginx_backup_remove_failed` | `error` | nginx Backup-Datei loeschen fehlgeschlagen |
| `cleanup.done` | — | Alte Releases geloescht |
| `cleanup.failed` | `error` | Cleanup-Fehler |
| `update.complete` | `version` | Update erfolgreich |
| `update.failed` | `error`, `step` | Allgemeiner Fehler (mkdir, version etc.) |

## API

### `update.NewService`

```go
func NewService(
    store     process.Store,
    restart   RestartFunc,
    nginx     nginx.Client,
    stdout    io.Writer,   // nil = kein stdout
    logOutput bool,        // true = logge in .vigil-update.log
) Service
```

### `Service.Update`

```go
func (s *service) Update(ctx context.Context, name string, version string) error
```

Rueckgabe-Sentinels:
- `ErrLocked` — Lock bereits gehalten
- `ErrNoPackage` — kein .tar.gz in incoming/
- `ErrIntegrity` — SHA256-Mismatch
- `ErrSmokeTest` — Smoke-Test fehlgeschlagen
- `ErrDepsFailed` — install_cmd fehlgeschlagen
- `ErrRolledBack` — Update fehlgeschlagen, erfolgreich zurueckgerollt

## CLI

### `vigil update <name>`

| Flag | Typ | Default | Beschreibung |
|------|-----|---------|-------------|
| `--version` | string | `""` | Explizite Version (sonst auto-detect aus incoming/) |
| `--quiet` | bool | `false` | Unterdrueckt stdout-Ausgabe |
| `--log-output` | bool | `false` | Schreibt JSON-Log nach `<working-dir>/.vigil-update.log` (append) |

Bei `--log-output` wird der Logger als Dual-Writer konfiguriert: stdout (sofern nicht `--quiet`) + Logdatei.

## Update-Prozess (13 Steps)

1. **Lock** — `.vigil.lock` anlegen (PID reinschreiben)
2. **Dirs** — `releases/`, `shared/`, `incoming/` sicherstellen
3. **Version** — Flag parsen oder `incoming/*.tar.gz` scannen
4. **Integrity** — SHA256-Checksum pruefen (falls `.sha256` vorhanden)
5. **Extract** — tar.gz nach `releases/<version>/` entpacken
6. **Deps** — `install_cmd` ausfuehren (skip bei `bundled-deps`)
7. **Shared** — Dateien aus `shared/` in Release-Dir symlinken
8. **Symlink** — atomarer Switch: `current` → neue Release
9a. **nginx** (static) — `nginx.conf` aus Release aktivieren, nginx reload
9b. **Restart** (app) — systemd service neustarten
10. **Smoke Test** — `smoke.sh <release-dir>` ausfuehren
11. **Rollback** (bei Fehler) — Symlink revert, ggf. nginx restore, Restart altes Release
12. **Cleanup** — aelteste Releases loeschen (max. 3 behalten)
13. **Unlock** — `.vigil.lock` entfernen (defer)

## Logging

### Format

Jede Zeile ist ein JSON-Objekt mit `ts` (RFC3339 UTC), `event` (String), `fields` (optional map):

```json
{"ts":"2026-07-21T10:30:01Z","event":"lock.acquired"}
{"ts":"2026-07-21T10:30:02Z","event":"extract.start","fields":{"version":"v1.2.0"}}
```

### Modi

| --quiet | --log-output | stdout | Datei |
|---------|-------------|--------|-------|
| — | — | JSON | — |
| ja | — | — | — |
| — | ja | JSON | `<working-dir>/.vigil-update.log` |
| ja | ja | — | `<working-dir>/.vigil-update.log` |

### Aufrufender Prozess

Aufrufender Prozess kann stdout parsen oder `--log-output` nutzen und die Datei tailen. Beide Streams sind atomar pro Zeile (Mutex-geschuetzt).

```bash
# Maschinenlesbar via stdout
vigil update my-app --version v1.2.0 | while read line; do
  event=$(echo "$line" | jq -r '.event')
  echo "Step: $event"
done

# Oder Logdatei pollen
vigil update my-app --version v1.2.0 --log-output &
tail -f /app/.vigil-update.log | jq '.event'
```

## Verzeichnisstruktur

```
<working-dir>/
├── releases/v1.0.0/    extrahierte Releases
├── releases/v1.1.0/
├── shared/              persistente Daten (.env, configs), symlinked in Releases
├── incoming/            Uploads: <version>.tar.gz + optional .sha256
├── current → releases/v1.1.0/  atomischer Symlink
├── .vigil.lock          Lock-Datei (PID)
└── .vigil-update.log    Update-Log (nur bei --log-output)
```

## Abhaengigkeiten

| Dependency | Verwendung |
|---|---|
| `archive/tar`, `compress/gzip` | tar.gz entpacken |
| `crypto/sha256` | Integritaetspruefung |
| `os/exec` | install_cmd, smoke.sh, systemctl (indirekt via RestartFunc) |
| `github.com/FW-Systeme/Virgil/internal/process` | Store, Process-Typ |
| `github.com/FW-Systeme/Virgil/internal/nginx` | nginx Config-Aktivierung (static type) |
| `github.com/stretchr/testify` | Tests |

## Teststrategie

- Unit-Tests fuer jeden Step isoliert (Lock, FindVersion, VerifyIntegrity, Extract, Cleanup etc.)
- Integrationstests fuer kompletten Update-Flow in temp dirs
- Mock-Store + Mock-RestartFunc fuer service.Update() Tests
- Logger separat getestet (Single/Dual/Nil Writer, Concurrent, JSON-Valid)
- Rollback-Pfade explizit getestet (Smoke-Test-Failure, Restart-Failure)
- nginx-Config-Update + Rollback via recorderNginx mock
