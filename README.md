# Vigil

Vigil ist ein leichtgewichtiger CLI-Prozessmanager – eine PM2-Alternative für Linux, die auf systemd und nginx aufsetzt.

Verwaltet Backend-Apps (Node.js, Go, Python, …) und Static-Apps durch Erzeugung von systemd-Units bzw. nginx-Site-Konfigurationen.

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

### Backend-App registrieren

Typ `app` (oder `node`) für beliebige Backend-Sprachen:

```bash
# Node.js
sudo vigil add my-api \
  --type app \
  --entry /opt/myapp/server.js \
  --port 3000 \
  --working-dir /opt/myapp \
  --smoke-test-script /opt/myapp/smoke.sh \
  --install-cmd "npm ci --production --ignore-scripts"

# Go-Binary (mit Build vor Start)
sudo vigil add my-service \
  --type app \
  --command "/opt/myapp/bin --port 8080" \
  --build-cmd "go build -o /opt/myapp/bin ." \
  --port 8080 \
  --working-dir /opt/myapp \
  --smoke-test-script /opt/myapp/smoke.sh

# Python
sudo vigil add my-bot \
  --type app \
  --command "python /opt/bot/main.py" \
  --port 9000 \
  --working-dir /opt/bot \
  --smoke-test-script /opt/bot/smoke.sh \
  --install-cmd "pip install -r requirements.txt"
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
| `--type` | `string` | ja* | `app`, `node` oder `static` |
| `--port` | `int` | ja* | Port der App |
| `--entry` | `string` | bei `node` | Einstiegsskript (z.B. `server.js`) |
| `--build-dir` | `string` | bei `static` | Build-Verzeichnis (z.B. `dist/`) |
| `--working-dir` | `string` | nein | Arbeitsverzeichnis |
| `--env-file` | `string` | nein | Pfad zur Environment-Datei |
| `--command` | `string` | nein | Custom `ExecStart` (z.B. `"/opt/app/bin --port 3000"`). Überschreibt `--entry` |
| `--build-cmd` | `string` | nein | Build-Befehl, läuft in `--working-dir` vor jedem Start (z.B. `"go build -o /opt/app/bin ."`) |
| `--nginx-domain` | `string` | nein | nginx `server_name` |
| `--nginx-path` | `string` | nein | nginx `root`-Pfad |
| `--nginx-config` | `string` | nein | Pfad zu benutzerdefinierter nginx-Config (überschreibt Auto-Generierung) |
| `--config` | `string` | nein | Pfad zur ecosystem.json |
| `--force` | `bool` | nein | Überschreibt existierende App |
| `--smoke-test-script` | `string` | **ja** | Pfad zum Smoke-Test-Skript (aktiviert Release-Management) |
| `--install-cmd` | `string` | bei Backend | Installationsbefehl (z.B. `"npm ci --production"`). Pflicht wenn `--bundled-deps` nicht gesetzt |
| `--bundled-deps` | `bool` | nein | Abhängigkeiten sind im Paket enthalten (skippt `--install-cmd`) |

\* `--type` und `--port` sind nur Pflicht, wenn ohne `--config` gearbeitet wird.

**Beispiele:**

```bash
# Einfache Node-App
vigil add my-api --type node --entry app.js --port 3000 \
  --smoke-test-script ./smoke.sh --install-cmd "npm ci --production"

# Static-App mit nginx-Domain
vigil add my-site --type static --build-dir dist --port 8080 --nginx-domain example.com --nginx-path /var/www/example

# Mit Arbeitsverzeichnis und Env-File
vigil add my-api --type node --entry server.js --port 4000 --working-dir /app --env-file /app/.env \
  --smoke-test-script /app/smoke.sh --install-cmd "npm ci --production"

# Aus ecosystem.json (alle Apps)
vigil add --config ecosystem.json

# Nur eine bestimmte App aus ecosystem.json
vigil add my-api --config ecosystem.json

# Vorhandene App überschreiben
vigil add my-api --type node --entry app.js --port 3000 \
  --smoke-test-script ./smoke.sh --install-cmd "npm ci --production" --force

# Go-Binary mit Build und Smoke-Test
vigil add my-service --type app \
  --command "/opt/myapp/bin --port 8080" \
  --build-cmd "go build -o /opt/myapp/bin ." \
  --port 8080 --working-dir /opt/myapp \
  --smoke-test-script /opt/myapp/smoke.sh

# Mit Smoke-Test-Skript (aktiviert Release-Management)
vigil add my-api --type node --entry server.js --port 3000 \
  --working-dir /opt/myapp \
  --smoke-test-script /opt/myapp/smoke.sh \
  --install-cmd "npm ci --production --ignore-scripts"
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

Führt ein Release-Update für eine App mit konfiguriertem `--smoke-test-script` durch.

```bash
# Update auf bestimmte Version
vigil update my-api --version v1.2.0

# Version automatisch aus incoming/ ermitteln
vigil update my-api
```

**Flags:**

| Flag | Typ | Beschreibung |
|------|-----|-------------|
| `--version` | `string` | Zielversion (z.B. `v1.2.0`). Wird leer gelassen, scannt Vigil `incoming/` nach `.tar.gz`-Dateien |
| `--quiet` | `bool` | Fortschrittsausgabe unterdrücken (nur Fehler und finales Ergebnis anzeigen) |

**Ablauf:**

```
 1. Lock         ← .vigil.lock verhindert parallele Updates
 2. Dirs         ← releases/, shared/, incoming/ anlegen
 3. Version      ← aus --version oder Auto-Detekt in incoming/
 4. Integrität   ← SHA256-Prüfung (falls .sha256-Datei vorhanden)
 5. Extract      ← Archiv via tar entpacken (von Virgil selbst, kein App-Skript)
 6. Deps         ← install_cmd ausführen (falls nicht --bundled-deps)
 7. Shared-Links ← Symlinks aus shared/ in release-Dir
 8. Symlink      ← current → releases/<version> (atomar)
 9. Restart      ← systemd restart / nginx reload
10. Smoke-Test   → ./smoke.sh <release-dir>
11. Rollback?    ← Bei Fehler: Symlink zurück, Restart, Abbruch
12. Cleanup      ← Alte Releases löschen (keep=3, fix)
13. Unlock       ← .vigil.lock entfernen
```

---

### `--log-output` (Globaler Flag)

Jeder Befehl kann mit `--log-output <pfad>` gestartet werden, um stdout und stderr in eine Logdatei zu schreiben.

```bash
# In Verzeichnis mit automatischem Dateinamen
vigil start my-api --log-output /var/log/vigil/

# Exakter Dateipfad
vigil update my-api --log-output /var/log/vigil/update-my-api.log
```

**Automatischer Dateiname:** `vigil-<befehl>-<YYYYMMDD-HHMMSS>-<PID>.log`

| Verhalten | Beschreibung |
|-----------|-------------|
| Pfad existiert und ist Verzeichnis | Logdatei wird automatisch im Verzeichnis erzeugt |
| Pfad endet mit `/` | Verzeichnis wird angelegt, automatischer Dateiname |
| Exakter Dateipfad (z.B. `.log`) | Logdatei an exakt diesem Pfad (Elternverzeichnis wird angelegt) |
| Wiederholter Aufruf mit gleichem Pfad | Ausgabe wird angehängt (append) |

---

### `vigil init`

Generiert eine `ecosystem.json`-Vorlage.

```bash
vigil init
vigil init --output mein-projekt.json
```

**Erzeugt (`type: app`):**
```json
{
  "name": "my-app",
  "type": "app",
  "command": "./my-app",
  "port": 3000
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
| `type` | `string` | **ja** | `"app"`, `"node"` oder `"static"` |
| `port` | `int` | **ja** | Port (muss > 0 sein) |
| `entry` | `string` | bei `node` | Einstiegsskript (z.B. `"app.js"`) |
| `build_dir` | `string` | bei `static` | Build-Verzeichnis (z.B. `"dist"`) |
| `env_file` | `string` | nein | Pfad zur `.env`-Datei |
| `working_dir` | `string` | nein | Arbeitsverzeichnis der App |
| `nginx_domain` | `string` | nein | nginx `server_name` |
| `nginx_path` | `string` | nein | nginx `root`-Pfad |
| `nginx_config` | `string` | nein | Pfad zu benutzerdefinierter nginx-Config (überschreibt Auto-Generierung) |
| `command` | `string` | nein | Custom `ExecStart` (z.B. `"/opt/app/bin --port 8080"`) |
| `build_cmd` | `string` | nein | Build-Befehl vor Start (z.B. `"go build -o /opt/app/bin ."`) |
| `smoke_test_script` | `string` | **ja** | Pfad zum Smoke-Test-Skript |
| `install_cmd` | `string` | bei Backend | Installationsbefehl (z.B. `"npm ci --production"`). Pflicht wenn `bundled_deps` false |
| `bundled_deps` | `bool` | nein | Abhängigkeiten im Paket enthalten (skippt `install_cmd`) |
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

### Backend-Apps (type: "app" oder "node")

Vigil erzeugt eine systemd-Unit-Datei unter `/etc/systemd/system/<name>.service`:

```ini
[Unit]
Description=Vigil: my-api
After=network.target

[Service]
Type=simple
ExecStart=/opt/app/bin --port 3000
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Die Unit wird via DBus aktiviert und gestartet. systemd übernimmt das Restart-Verhalten, Logging (`journalctl`) und Prozess-Isolation.

**`--command`**: Wird gesetzt, nutzt Vigil diesen Wert als `ExecStart`. Sonst automatisch `/usr/bin/node <entry>` (rückwärtskompatibel zu `--type node`).

**`--build-cmd`**: Wird vor jedem Start/Neustart synchron in `WorkingDir` ausgeführt. Schlägt der Build fehl, startet der Service nicht. Nützlich für Kompilierung (Go, Rust, …) oder Dependency-Installation.

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

Apps mit konfiguriertem `--smoke-test-script` nutzen das integrierte Release-Management von Vigil.

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
Beim Add mit `--smoke-test-script` passt Vigil die Unit automatisch an:

```ini
[Service]
WorkingDirectory=/opt/myapp/current
ExecStart=/usr/bin/node server.js          # Entry relativ zu current/
EnvironmentFile=/opt/myapp/shared/.env     # Env aus shared/
```

### Smoke-Test-Skript-Schnittstelle

Das Skript ist ein einzelnes ausführbares Skript, das als einziges Argument das Release-Verzeichnis erhält:

```bash
./smoke.sh <release-dir>
```

**Exit-Codes:** `0` = Erfolg, `≠0` = Fehler → Rollback.

- Entpacken und Abhängigkeiten (`install_cmd`) übernimmt Vigil selbst.
- Der Smoke-Test läuft **nach** dem Symlink-Switch und dem Restart.
- Ein Fehler löst automatisch Rollback auf die vorherige Version aus.

**Beispiel-Skript:**

```bash
#!/bin/bash
set -euo pipefail

RELEASE_DIR="$1"
cd "$RELEASE_DIR"

# Prüfe Health-Endpunkt der neuen Version
curl -sf http://localhost:3000/health > /dev/null
```

### Update-Paket-Format

Ein Update-Paket ist eine `.tar.gz`-Datei, die den gesamten App-Code enthält.

#### Anforderungen an das Incoming-Paket

| Kriterium | Anforderung |
|-----------|-------------|
| **Dateiname** | `<version>.tar.gz` – z.B. `v1.2.0.tar.gz`. Der Teil vor `.tar.gz` wird als Versionsname für das Release-Verzeichnis verwendet |
| **SHA256-Prüfsumme (optional)** | Datei `<version>.tar.gz.sha256` mit hex-kodiertem SHA256-Hash. Fehlt die Datei, wird die Integritätsprüfung übersprungen. Bei einem Mismatch bricht Vigil mit `integrity check failed` ab |
| **Archiv-Format** | Gültiges gzip-komprimiertes tar-Archiv (.tar.gz) |
| **Archiv-Inhalt** | Applikationscode **flach** – kein Wrapping-Ordner (siehe unten) |
| **Pfad-Traversal** | Einträge mit `../` im Pfad werden von Vigil stillschweigend ignoriert (Sicherheit) |

#### Inhaltliche Voraussetzungen

Das Paket muss enthalten, was die Applikation zum Laufen braucht – Vigil validiert den Inhalt **nicht** vor dem Entpacken.

| Szenario | Erforderlich im Archiv |
|----------|----------------------|
| `--bundled-deps=false` (Standard) | Abhängigkeiten im Archiv (`install_cmd` wird ausgeführt) |
| `--bundled-deps=true` | Abhängigkeiten bereits im Archiv, `install_cmd` wird übersprungen |
| Jede App | Einstiegsskript (z.B. `server.js`), Configs, statische Dateien etc. |

> **Fehlen Abhängigkeiten bei `--bundled-deps=false`, schlägt `install_cmd` fehl und Vigil bricht mit `dependency installation failed` ab.**

#### Archiv-Struktur — kein Wrapping-Ordner!

Vigil entpackt das Archiv **direkt** ins Release-Verzeichnis `releases/<version>/`. Enthält das Archiv einen Wrapping-Ordner (z.B. `my-app-v1.0.0/`), landen alle Dateien eine Ebene zu tief:

**✅ Richtig (flach) – Dateien auf oberster Ebene im Archiv:**
```
server.js
package.json
lib/
  utils.js
```

**❌ Falsch (mit Wrapping-Ordner):**
```
my-app-v1.0.0/
  server.js           ← landet in releases/v1.0.0/my-app-v1.0.0/server.js
  package.json            statt in releases/v1.0.0/server.js
```

#### Paket erstellen — Beispiel

```bash
# App-Code flach packen (von Build-Verzeichnis aus)
tar -czf v1.2.0.tar.gz -C ./build .

# Optional: SHA256-Prüfsumme erzeugen
sha256sum v1.2.0.tar.gz > v1.2.0.tar.gz.sha256

# Paket ins incoming/ legen
mv v1.2.0.tar.gz v1.2.0.tar.gz.sha256 <working-dir>/incoming/
```

#### Häufige Fehler beim Update

| Fehler | Ursache | Lösung |
|--------|---------|--------|
| `no package found in incoming dir` | Keine `.tar.gz`-Datei in `incoming/` | Paket (z.B. `v1.0.0.tar.gz`) nach `incoming/` legen |
| `integrity check failed` | SHA256-Hash stimmt nicht | `sha256sum` neu berechnen, `.sha256`-Datei aktualisieren |
| `reading gzip: ...` | Archiv ist kein gültiges gzip-Format | `file v1.0.0.tar.gz` prüfen, Archiv korrekt erstellen |
| Wrapping-Ordner (unauffällig) | App startet nicht, Pfade stimmen nicht | `tar -tzf v1.0.0.tar.gz` prüfen: keine führende Ordnerebene |
| `install_cmd` schlägt fehl | Abhängigkeiten im Archiv fehlen oder fehlerhaft | `install_cmd` korrekt konfigurieren, Abhängigkeiten im Build einbinden |

**`--bundled-deps`:** Enthält das Archiv bereits die Abhängigkeiten (z.B. `node_modules/`), kann Vigil mit `--bundled-deps=true` den Schritt `install_cmd` überspringen. Sonst führt Vigil den konfigurierten `install_cmd` aus.

#### Konsolenausgabe während des Updates

Vigil gibt den Fortschritt des 13-Schritte-Prozesses auf der Konsole aus:

```
  Lock acquired
  Using version v1.2.0
  Integrity check passed
  Extracting v1.2.0.tar.gz...
  Installing dependencies (install_cmd)...
  Linking shared data...
  Switching symlink to v1.2.0...
  Restarting service...
  Running smoke test...
  Smoke test passed
  Cleaned up old releases
Updated "myapp" to v1.2.0
```

Bei einem Fehler bricht Vigil mit einer Fehlermeldung ab und führt ggf. ein Rollback durch. Mit `--quiet` kann die Fortschrittsausgabe unterdrückt werden.

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
  "smoke_test_script": "/opt/myapp/smoke.sh",
  "bundled_deps": false,
  "install_cmd": "npm ci --production --ignore-scripts"
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
