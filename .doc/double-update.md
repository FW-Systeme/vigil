# Doppelte Update-Ausführung: Analyse-Auftrag für Agenten

## Problem

Beim Deployment eines Updates wird `vigil update` **zweimal** ausgeführt. Der zweite
Aufruf scheitert mit `Error: update lock held`, weil die Lock-Datei `.vigil.lock`
nicht gelöscht wurde.

## Beobachtete Logs (Verschlüsselung: von User bereitgestellt)

Aus `update-watchdog`-Verzeichnis (`/mnt/ssd/msl5/branches/update/update-watchdog`):

```
$ sudo rm -rf .vigil.lock
$ sudo vigil update update-update-watchdog --log-output .
# Erfolgreicher Lauf (12:14:43)
Updated "update-update-watchdog"
```

Zwei Log-Dateien nach dem Lauf:

| Datei | Zeitstempel | PID | Ergebnis |
|-------|-------------|-----|----------|
| `vigil-update-20260817-120645-3808288.log` | 12:06:45 | 3808288 | **Fehler: update lock held** |
| `vigil-update-20260817-121443-3810307.log` | 12:14:43 | 3810307 | Erfolg (manueller Lauf) |

## Befunde

1. **Virgil selbst hat KEINE Retry-/Re-Invocation-Logik.** `cmd/vigil/main.go` ruft
   `cli.Execute()` genau einmal auf. Kein `exec.Command`, keine Goroutine, kein
   HTTP-Endpoint, keine cron-Trigger im Code.
2. Der fehlgeschlagene Lauf (12:06:45) fand **8 Minuten vor** dem manuellen Lauf statt.
   Zu diesem Zeitpunkt war die Lock-Datei bereits vorhanden (von einem früheren
   abgestürzten Prozess).
3. Die Lock-Datei wird von `internal/update/service.go` geschrieben (enthält PID).
   **Bug war:** PID wurde nie validiert. Abgestürzter Prozess (SIGKILL/OOM/panic)
   hinterlässt die Lock-Datei permanent → alle weiteren Updates blockiert bis zur
   manuellen Löschung. **Dieser Bug wurde bereits gefixt** (siehe unten).

## Bereits umgesetzter Fix

In `internal/update/service.go`:
- `removeStaleLock(lockPath)` prüft die PID in der Lock-Datei via
  `syscall.Kill(pid, 0)` (Signal 0). `ESRCH` = Prozess tot → Lock ist stale →
  Datei wird entfernt, Update kann fortfahren.
- Ungültige/leere PID (`<= 0`, nicht-numerisch) → als stale behandelt.
- Lebende PID → `ErrLocked` wie bisher.
- Doppel-Check vor Entfernung schützt gegen Race mit parallel startendem Updater.
- Tests in `internal/update/service_test.go`: `TestLock_StaleLockRemoved`,
  `TestLock_InvalidPIDRemoved`, `TestLock_EmptyPIDRemoved`,
  `TestLock_ActiveLockBlocks`; `TestUpdate_ErrLocked` nutzt jetzt `os.Getpid()`.

## Offene Frage: Wer triggert den zweiten Aufruf?

Der eigentliche Trigger der doppelten Ausführung ist **nicht in diesem Repo** zu
finden. Der zu untersuchende Aufruf betrifft den **update-watchdog-Service**
(separates Projekt, wird von vigil gemanagt; App-Name `update-update-watchdog`).
Bitte folgenden Code untersuchen:

### Mögliche Ursachen (Checkliste)

1. **Watchdog-Polling:** Pollt der Watchdog `incoming/` und ruft `vigil update`
   in einer Schleife/Ticker auf? Mehrfache Poll-Zyklen könnten den Update zweimal
   anstoßen, wenn `incoming/*.tar.gz` nach dem ersten Update nicht aufgeräumt wird.
2. **Systemd-Restart:** Unit hat `Restart=on-failure`. Crasht der Watchdog,
   startet systemd ihn neu → er könnte beim Start erneut `vigil update` auslösen.
3. **Cron-Job:** Gibt es einen cron-Eintrag (`vigil cron` oder extern), der das
   Update periodisch ausführt?
4. **CI/CD / Deployment-Skript:** Wird `vigil update` vom Deployment-Pipeline-Skript
   mehrfach aufgerufen (z.B. zweimal mit `--version` und ohne)?
5. **incoming/-Verzeichnis:** Nach erfolgreichem Update: Wird das `.tar.gz`-Paket
   aus `incoming/` entfernt? Wenn nicht, triggert jeder weitere `vigil update`
   denselben Version erneut.

### Empfohlene nächste Schritte

1. `incoming/`-Inhalt nach erfolgreichem Update prüfen (Paket noch da?).
2. Systemd-Unit des Watchdogs ansehen (`systemctl cat update-update-watchdog`),
   ob `Restart`/`ExecStartPre`/`ExecStartPost` `vigil update` enthalten.
3. Crontab prüfen (`crontab -l`, `vigil cron list`) auf wiederkehrende Update-Jobs.
4. Watchdog-Log der betroffenen Zeit 12:06-12:14 prüfen, ob dort `vigil update`
   zweimal in kurzer Zeit aufgerufen wurde.

## Relevante Dateien (Virgil-Repo)

| Datei | Zweck |
|-------|-------|
| `internal/update/service.go` | Lock-Logik + Update-Orchestrierung |
| `internal/update/service_test.go` | Tests (inkl. neuer Stale-Lock-Tests) |
| `internal/update/contract.go` | `ErrLocked` etc. |
| `internal/cli/update.go` | `vigil update` Command |
| `internal/process/manager.go` | systemd-Unit-Generierung (`Restart=on-failure`) |
