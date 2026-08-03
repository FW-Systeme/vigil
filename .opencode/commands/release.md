---
description: >
  Erzeugt ein neues Release im Virgil-Go-Projekt.
  Prüft Qualität lokal (Tests, Coverage, Lint), inkrementiert Version (SemVer), taggt und pusht.
  CI baut darauf das Binary und erstellt ein GitHub Release.
agent: build
---

# Release Coordinator

Du erstellst ein neues Release für das Virgil-Go-Projekt.

## Argument
$ARGUMENTS

Erwartet wird: `major`, `minor`, `patch` oder eine explizite Version wie `v1.2.3`.

## Workflow

### Phase 1: Qualität prüfen

Führe Qualitätsgates aus. **Bei Fehlern sofort abbrechen** – User muss manuell korrigieren.

1. `golangci-lint run ./...` → Exit-Code 0 erforderlich
2. `go test -race -count=1 ./...` → alle Tests grün
3. Coverage-Check:
   ```bash
   go test -coverprofile=coverage.out ./...
   awk '/^total:/ {gsub(/%/,"",$3); if ($3+0 < 85) \
     {printf "FAIL: coverage %.1f%% < 85%%\n", $3; exit 1} \
     else {printf "PASS: coverage %.1f%%\n", $3}}' \
     <(go tool cover -func=coverage.out)
   ```
4. `go vet ./...`

### Phase 2: Aktuelle Version ermitteln

```bash
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
```

Parst den aktuellen Tag in `$MAJOR`, `$MINOR`, `$PATCH`.

### Phase 3: Neue Version bestimmen

- `major` → MAJOR+1, MINOR=0, PATCH=0
- `minor` → MINOR+1, PATCH=0
- `patch` → PATCH+1
- `vX.Y.Z` → direkt übernehmen (muss höher als aktuell sein)

Neue Version = `v$MAJOR.$MINOR.$PATCH`

### Phase 4: User bestätigen lassen

Nutze das `question`-Tool, um den User die neue Version bestätigen zu lassen.

**Falls User ablehnt**: Abbruch mit Hinweis.

### Phase 5: Tag erstellen

```bash
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"
```

### Phase 6: Pushen

```bash
git push origin main --tags
```

### Phase 7: Ergebnis melden

Push erfolgreich. CI baut Binaries und erstellt GitHub Release automatisch.

- Release `$NEW_VERSION`
- CI-Workflow läuft auf `main` mit Tag `$NEW_VERSION`
- Binary-Download und Release-Notes nach CI-Durchlauf auf GitHub verfügbar
