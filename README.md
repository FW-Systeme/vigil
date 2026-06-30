# Vigil

Vigil ist ein leichtgewichtiger CLI-Prozessmanager – eine PM2-Alternative für Linux, die auf systemd und nginx aufsetzt.

Verwaltet Node.js- und Static-Apps durch Erzeugung von systemd-Units bzw. nginx-Site-Konfigurationen.

## Installation

```bash
go install github.com/chris576/vigil/cmd/vigil@latest
```

Oder nach Clonen des Repos:

```bash
go build -o vigil ./cmd/vigil
sudo mv vigil /usr/local/bin/
```

**Voraussetzungen:**

- Linux mit systemd (für Node-Apps)
- nginx installiert (für Static-Apps)
- Go 1.22+

## Kurzstart

### Node-App registrieren

```bash
sudo vigil add my-api \
  --type node \
  --entry /opt/myapp/server.js \
  --port 3000 \
  --working-dir /opt/myapp \
  --env-file /opt/myapp/.env
```

### Static-App registrieren

```bash
sudo vigil add my-site \
  --type static \
  --build-dir /opt/mysite/dist \
  --port 8080 \
  --nginx-domain example.com \
  --nginx-path /var/www/example
```

### Status prüfen

```bash
vigil list
```

### App starten/stoppen/neustarten

```bash
vigil start my-api
vigil stop my-api
vigil restart my-api
```

### App entfernen

```bash
vigil remove my-api
```

---

## Kommandos im Detail

### `vigil add [name]`

Registriert eine neue App.

**Flags:**

| Flag | Typ | Pflicht | Beschreibung |
|------|-----|---------|-------------|
| `--type` | `string` | ja* | `node` oder `static` |
| `--port` | `int` | ja* | Port der App |
| `--entry` | `string` | bei `node` | Einstiegsskript (z.B. `server.js`) |
| `--build-dir` | `string` | bei `static` | Build-Verzeichnis (z.B. `dist/`) |
| `--working-dir` | `string` | nein | Arbeitsverzeichnis |
| `--env-file` | `string` | nein | Pfad zur Environment-Datei |
| `--nginx-domain` | `string` | nein | nginx `server_name` |
| `--nginx-path` | `string` | nein | nginx `root`-Pfad |
| `--config` | `string` | nein | Pfad zur ecosystem.json |
| `--force` | `bool` | nein | Überschreibt existierende App |
| `--update-script` | `string` | nein | Pfad zum Update-Skript (aktiviert Release-Management) |
| `--incoming-dir` | `string` | nein | Verzeichnis für hochgeladene Update-Pakete (Default: `<working-dir>/incoming`) |
| `--keep-releases` | `int` | nein | Anzahl der zu behaltenden Releases (Default: 3) |

\* `--type` und `--port` sind nur Pflicht, wenn ohne `--config` gearbeitet wird.

**Beispiele:**

```bash
# Einfache Node-App
vigil add my-api --type node --entry app.js --port 3000

# Static-App mit nginx-Domain
vigil add my-site --type static --build-dir dist --port 8080 --nginx-domain example.com --nginx-path /var/www/example

# Mit Arbeitsverzeichnis und Env-File
vigil add my-api --type node --entry server.js --port 4000 --working-dir /app --env-file /app/.env

# Aus ecosystem.json (alle Apps)
vigil add --config ecosystem.json

# Nur eine bestimmte App aus ecosystem.json
vigil add my-api --config ecosystem.json

# Vorhandene App überschreiben
vigil add my-api --type node --entry app.js --port 3000 --force

# Mit Update-Skript (aktiviert Release-Management)
vigil add my-api --type node --entry server.js --port 3000 \
  --working-dir /opt/myapp \
  --update-script /opt/myapp/update.sh \
  --incoming-dir /opt/myapp/incoming \
  --keep-releases 5
```

---

### `vigil remove <name>`

Entfernt eine registrierte App inklusive systemd-Unit bzw. nginx-Site-Konfiguration.

```bash
vigil remove my-api
```

**Aktion:** Stoppt die App, deaktiviert die Unit/Site, löscht die Konfigurationsdateien und entfernt den Eintrag aus dem Store.

---

### `vigil list`

Listet alle registrierten Apps auf.

```bash
vigil list
```

**Ausgabe:**
```
my-api               node    port 3000    active
my-site              static  port 8080    active
```

---

### `vigil start <name>`

Startet eine registrierte App.

```bash
vigil start my-api
```

- **Node-Apps:** Startet die systemd-Unit
- **Static-Apps:** Erzeugt die nginx-Site-Konfiguration und lädt nginx neu

---

### `vigil stop <name>`

Stoppt eine laufende App.

```bash
vigil stop my-api
```

- **Node-Apps:** Stoppt die systemd-Unit
- **Static-Apps:** Entfernt den nginx-Site-Symlink und lädt nginx neu

---

### `vigil restart <name>`

Startet eine App neu.

```bash
vigil restart my-api
```

- **Node-Apps:** Führt `systemctl restart` aus
- **Static-Apps:** Deaktiviert und aktiviert die nginx-Site neu

---

### `vigil update <name>`

Führt ein Release-Update für eine App mit konfiguriertem `--update-script` durch.

```bash
# Update auf bestimmte Version
vigil update my-api --version v1.2.0

# Version automatisch aus incoming/ ermitteln
vigil update my-api

# keep-releases temporär überschreiben
vigil update my-api --version v1.2.0 --keep-releases 5
```

**Flags:**

| Flag | Typ | Beschreibung |
|------|-----|-------------|
| `--version` | `string` | Zielversion (z.B. `v1.2.0`). Wird leer gelassen, scannt Vigil `incoming/` nach `.tar.gz`-Dateien |
| `--keep-releases` | `int` | Überschreibt `keep_releases` aus der Config (Default aus Config oder 3) |

**Ablauf:**

```
 1. Lock         ← .vigil.lock verhindert parallele Updates
 2. Dirs         ← releases/, shared/, incoming/ anlegen
 3. Version      ← aus --version oder Auto-Detekt in incoming/
 4. Integrität   ← SHA256-Prüfung (falls .sha256-Datei vorhanden)
 5. Extract      → ./update.sh extract <package> <release-dir>
 6. Deps         → ./update.sh deps <release-dir>
 7. Migrate      → ./update.sh migrate <release-dir> <shared-dir>
 8. Shared-Links ← Symlinks aus shared/ in release-Dir
 9. Pre-Check    → ./update.sh health-check <release-dir> (optional)
10. Symlink      ← current → releases/<version> (atomar)
11. Restart      ← systemd restart / nginx reload
12. Health-Check → ./update.sh health-check <release-dir>
13. Rollback?    ← Bei Fehler: Symlink zurück, Restart, Abbruch
14. Cleanup      ← Alte Releases löschen (keep_releases)
15. Unlock       ← .vigil.lock entfernen
```

---

### `vigil init`

Generiert eine `ecosystem.json`-Vorlage.

```bash
vigil init
vigil init --output mein-projekt.json
```

**Erzeugt:**
```json
{
  "name": "my-app",
  "type": "node",
  "port": 3000,
  "entry": "./app.js",
  "build_dir": "",
  "env_file": "",
  "working_dir": "",
  "nginx_domain": "",
  "nginx_path": "",
  "update_script": "",
  "incoming_dir": "",
  "keep_releases": 3,
  "created_at": "2025-01-01T00:00:00Z",
  "enabled": true
}
```

---

### `vigil version`

Zeigt die installierte Version an.

```bash
vigil version
```

---

## Ecosystem-JSON (ecosystem.json)

Die `ecosystem.json` erlaubt es, mehrere Apps auf einmal zu registrieren. Das Format ist an PM2 angelehnt.

### Einzelner Prozess

```json
{
  "name": "my-api",
  "type": "node",
  "port": 3000,
  "entry": "./app.js",
  "build_dir": "",
  "env_file": "/opt/myapp/.env",
  "working_dir": "/opt/myapp",
  "nginx_domain": "",
  "nginx_path": "",
  "enabled": true
}
```

### Mehrere Prozesse (apps-Array)

```json
{
  "apps": [
    {
      "name": "api",
      "type": "node",
      "entry": "server.js",
      "port": 3000,
      "working_dir": "/opt/api",
      "env_file": "/opt/api/.env"
    },
    {
      "name": "frontend",
      "type": "static",
      "build_dir": "/opt/frontend/dist",
      "port": 8080,
      "nginx_domain": "example.com",
      "nginx_path": "/var/www/example"
    },
    {
      "name": "admin",
      "type": "static",
      "build_dir": "/opt/admin/build",
      "port": 8081,
      "nginx_domain": "admin.example.com",
      "nginx_path": "/var/www/admin"
    }
  ]
}
```

### JSON-Felder

| Feld | Typ | Pflicht | Beschreibung |
|------|-----|---------|-------------|
| `name` | `string` | **ja** | Name der App (eindeutig) |
| `type` | `string` | **ja** | `"node"` oder `"static"` |
| `port` | `int` | **ja** | Port (muss > 0 sein) |
| `entry` | `string` | bei `node` | Einstiegsskript (z.B. `"app.js"`) |
| `build_dir` | `string` | bei `static` | Build-Verzeichnis (z.B. `"dist"`) |
| `env_file` | `string` | nein | Pfad zur `.env`-Datei |
| `working_dir` | `string` | nein | Arbeitsverzeichnis der App |
| `nginx_domain` | `string` | nein | nginx `server_name` |
| `nginx_path` | `string` | nein | nginx `root`-Pfad |
| `update_script` | `string` | nein | Pfad zum Update-Skript (aktiviert Release-Management) |
| `incoming_dir` | `string` | nein | Verzeichnis für hochgeladene Update-Pakete (Default: `<working_dir>/incoming`) |
| `keep_releases` | `int` | nein | Anzahl zu behaltender alter Releases (Default: 3) |
| `enabled` | `bool` | nein | Ob die App aktiv ist (default: `false`) |

### Nutzung

```bash
# Alle Apps aus der Datei registrieren
vigil add --config ecosystem.json

# Eine bestimmte App aus der Datei registrieren
vigil add api --config ecosystem.json
vigil add frontend --config ecosystem.json
```

Fehlerhafte Apps werden übersprungen (mit Warnung), die restlichen werden registriert.

---

## Architektur

```
cmd/vigil/main.go
  └─ internal/cli/            ← Cobra-CLI
       ├─ internal/process/   ← Manager + Store + Process-Typen
       │    ├─ internal/systemd/  ← DBus-Client (Node-Apps)
       │    └─ internal/nginx/    ← Site-Config-Management (Static-Apps)
       └─ internal/update/    ← Update-Orchestrator (Release-Management)
```

### Komponenten

| Komponente | Aufgabe |
|---|---|
| **CLI** (`internal/cli/`) | Cobra-Commands, Context-Injection |
| **Process** (`internal/process/`) | Manager-Logik, JSON-Store, Validierung |
| **Update** (`internal/update/`) | Release-Management: Lock, Entpacken, Migrationen, Symlink-Switch, Rollback, Cleanup |
| **systemd** (`internal/systemd/`) | DBus-Verbindung für systemd-Unit-Lifecycle |
| **nginx** (`internal/nginx/`) | nginx-Site-Konfiguration (sites-available/-enabled) |

---

## Funktionsweise

### Node-Apps (type: "node")

Vigil erzeugt eine systemd-Unit-Datei unter `/etc/systemd/system/<name>.service`:

```ini
[Unit]
Description=Vigil: my-api
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/myapp
ExecStart=/usr/bin/node /opt/myapp/server.js
Restart=on-failure
RestartSec=5
EnvironmentFile=/opt/myapp/.env

[Install]
WantedBy=multi-user.target
```

Die Unit wird via DBus aktiviert und gestartet. systemd übernimmt das Restart-Verhalten, Logging (`journalctl`) und Prozess-Isolation.

### Static-Apps (type: "static")

Vigil erzeugt eine nginx-Site-Konfiguration unter `/etc/nginx/sites-available/<name>.conf`:

```nginx
server {
    listen 8080;
    server_name example.com;
    root /var/www/example;
    index index.html;
}
```

Ein Symlink `/etc/nginx/sites-enabled/<name>.conf` → `sites-available/<name>.conf` aktiviert die Site. nginx wird neu geladen.

---

## Update-Prozess (Release-Management)

Apps mit konfiguriertem `--update-script` nutzen das integrierte Release-Management von Vigil.

### Verzeichnisstruktur

```
<working-dir>/
├── releases/
│   ├── v1.0.0/          ← eine Version pro Ordner
│   ├── v1.1.0/
│   └── v1.2.0/
├── shared/               ← persistente Daten (.env, DB, Logs)
├── incoming/             ← Ziel für hochgeladene Update-Pakete
└── current → releases/v1.2.0/   ← Symlink (aktivierte Version)
```

- **`releases/`** — jede Version bekommt einen eigenen Ordner
- **`current`** — Symlink, zeigt immer auf die aktive Version
- **`shared/`** — persistente Daten, die Updates überleben
- **`incoming/`** — Zielort für übertragene `.tar.gz`-Pakete

Die systemd-Unit zeigt auf `<working-dir>/current`, nie auf eine konkrete Version.
Beim Add mit `--update-script` passt Vigil die Unit automatisch an:

```ini
[Service]
WorkingDirectory=/opt/myapp/current
ExecStart=/usr/bin/node server.js          # Entry relativ zu current/
EnvironmentFile=/opt/myapp/shared/.env     # Env aus shared/
```

### Update-Skript-Schnittstelle

Das Skript erhält Subcommands mit Argumenten. Der Exit-Code bestimmt den Erfolg:

| Subcommand | Argumente | Beschreibung |
|------------|-----------|-------------|
| `extract` | `<package-path> <release-dir>` | `.tar.gz` entpacken |
| `deps` | `<release-dir>` | Abhängigkeiten installieren (npm ci o.ä.) |
| `migrate` | `<release-dir> <shared-dir>` | Datenbank-Migrationen |
| `health-check` | `<release-dir>` | Smoke-Test der neuen Version |

**Exit-Codes:** `0` = Erfolg, `≠0` = Abbruch. Bei `health-check` nach dem Restart löst ein Fehler automatisch **Rollback** aus.

**Beispiel-Skript:**

```bash
#!/bin/bash
set -euo pipefail

ACTION="$1"
RELEASE_DIR="${3:-$2}"

case "$ACTION" in
  extract)
    tar -xzf "$2" -C "$3"
    ;;
  deps)
    cd "$2" && npm ci --production
    ;;
  migrate)
    cd "$2" && node migrate.js
    ;;
  health-check)
    # Smoke-Test: prüfe Health-Endpunkt
    curl -sf http://localhost:3000/health > /dev/null
    ;;
esac
```

### Update-Paket-Format

Ein Update-Paket ist eine `.tar.gz`-Datei, die den gesamten App-Code (inkl. `package.json`, Frontend-Build etc.) enthält:

```
<working-dir>/incoming/
├── v1.2.0.tar.gz          ← App-Code als Archiv
└── v1.2.0.tar.gz.sha256   ← Optional: SHA256-Prüfsumme
```

Der Dateiname (ohne `.tar.gz`) wird als Version verwendet. Liegt eine `.sha256`-Datei neben dem Paket, prüft Vigil die Integrität vor dem Entpacken.

### Lock-Mechanismus

Eine Lock-Datei `<working-dir>/.vigil.lock` verhindert parallele Updates – sowohl über SSH als auch über die Web-App. Bei einem laufenden Update schlägt ein zweiter Aufruf sofort mit `update lock held` fehl.

### Rollback

Schlägt der `health-check` nach dem Neustart fehl, setzt Vigil den `current`-Symlink automatisch auf die vorherige Version zurück und startet den Service erneut. Die fehlgeschlagene Version bleibt zur Analyse im `releases/`-Verzeichnis erhalten.

### Berechtigungen

Falls die Web-App das Update triggert, sollten erhöhte Rechte ausschließlich auf das Update-Skript beschränkt sein (sudoers-Eintrag für genau dieses Skript, nicht pauschal für Vigil).

---

## Speicherort

Jede App wird als einzelne JSON-Datei gespeichert. Schreibvorgänge sind atomar (Temp-Datei + `os.Rename`).

| Benutzer | Speicherpfad |
|----------|-------------|
| **root** (UID 0) | `/etc/vigil/apps/<name>.json` |
| **Non-Root** | `~/.config/vigil/apps/<name>.json` |

**Beispiel `/etc/vigil/apps/my-api.json`:**

```json
{
  "name": "my-api",
  "type": "node",
  "port": 3000,
  "entry": "/opt/myapp/server.js",
  "build_dir": "",
  "env_file": "/opt/myapp/.env",
  "working_dir": "/opt/myapp",
  "nginx_domain": "",
  "nginx_path": "",
  "created_at": "2025-01-15T10:30:00Z",
  "enabled": true,
  "update_script": "/opt/myapp/update.sh",
  "incoming_dir": "/opt/myapp/incoming",
  "keep_releases": 3
}
```

---

## Fehlerbehandlung

- **`add` mit `--config`:** Fehlerhafte Apps werden übersprungen. Am Ende wird die Anzahl der erfolgreichen und fehlgeschlagenen Registrierungen ausgegeben. Bei mindestens einem Fehler gibt der Befehl einen Exit-Code `!= 0` zurück.
- **Doppelte Apps:** Ohne `--force` wird `add` einen Fehler ausgeben, wenn die App bereits existiert.
- **Validierung:** Vor dem Speichern wird jedes `Process`-Objekt validiert (Pflichtfelder je Type).
- **Atomare Writes:** Der Store schreibt in eine Temp-Datei und führt dann `os.Rename` aus – bei Absturz während des Schreibens bleibt die alte Konfiguration erhalten.

---

## nginx-Troubleshooting

Falls nginx nach `vigil start` / `vigil stop` nicht neu lädt:

```bash
# nginx-Konfiguration testen
nginx -t

# Manuell neu laden
nginx -s reload

# Status prüfen
systemctl status nginx
```

Vigil ruft `nginx -s reload` auf. Schlägt dies fehl (weil z.B. die Konfiguration fehlerhaft ist), wird der Fehler zurückgegeben.
