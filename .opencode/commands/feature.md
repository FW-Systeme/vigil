---
description: >
  Orchestriert den kompletten Feature-Workflow im Virgil-Go-Projekt.
  Erstellt Branch → Shared Contract (Go Interface) → Implement + Test (parallel) →
  Quality (golangci-lint + Coverage) → Push → CI-Status via GitHub MCP →
  ggf. Fix-Schleife.
agent: build
---

# Feature Coordinator

Du orchestrierst den Feature-Entwicklungs-Workflow für das Virgil-Go-Projekt.

## Feature Anforderung
$ARGUMENTS

## Workflow-Phasen

### Phase 1: Analyse & Contract
- Anforderung analysieren
- Scope bestimmen: welches Package unter `internal/` ist betroffen?
- Neues Package anlegen: `internal/<feature>/`
- **Shared Contract** als Go Interface erstellen: `internal/<feature>/contract.go`
  - Interface definiert die Verträge (Methoden-Signaturen)
  - Kommentare dokumentieren das Verhalten (Pre/Post-Conditions)
  - Beispiel:
    ```go
    // Service defines the contract for <feature>.
    type Service interface {
        // DoSomething processes input and returns the result.
        DoSomething(ctx context.Context, input Something) (Something, error)
    }
    ```
- Bestehende Architektur beachten
- Bei Unklarheiten: User befragen

### Phase 2: Branch erstellen
- Erstelle Branch: `git checkout -b feature/<kebab-case-name>`
- Committe den Contract: `git add -A && git commit -m "feat: add contract for <feature>"`

### Phase 3: Parallel-Implementierung
- Starte **Implementer** als Subagent (via `task`-Tool):
  - Übergib: Anforderung, Contract-Interface, Package-Pfad
- Starte **Tester** als Subagent (via `task`-Tool):
  - Übergib: Anforderung, Contract-Interface, Package-Pfad
- Beide parallel ausführen
- Ergebnisse einsammeln, ggf. Contract anpassen und einen der beiden erneut starten

### Phase 4: Qualität
- Starte **Quality-Ensurance** als Subagent (via `task`-Tool)
- Result: `golangci-lint run ./...` Exit-Code muss 0 sein
- Coverage: ≥85%
- Bei Fehlern: zurückschicken an Implementer/Tester

### Phase 4b: E2E Validation
- Rufe `/e2e-validate` auf
- Prüfe Resultat:
  - `PASSED` → weiter zu Phase 5
  - `FAILED` → Fix-Schleife (max 3 Iterationen):
    1. Logs aus `/e2e-validate` Report analysieren
    2. **Implementer + Tester** mit konkreter Fehlerbeschreibung neu starten
    3. Erneut `/e2e-validate`
- Erst bei PASSED → Phase 5

### Phase 5: Commit & Push
- `git add -A && git commit -m "feat: implement <feature>"`
- `git push origin feature/<name>`

### Phase 6: CI-Status via GitHub MCP
- Nutze GitHub MCP-Tool `get_workflow_run` oder `list_workflow_runs`
- Filter auf: Branch = `feature/<name>`, Workflow = `ci-dev.yml`
- Poll alle 30s bis `status = "completed"`
- Prüfe `conclusion`: `"success"` oder `"failure"`

### Phase 7: CI-Auswertung
- **success** → Meldung an User: ✅ Feature `<feature>` fertig implementiert
- **failure** → Fix-Schleife starten (maximal 5 Iterationen):
  1. CI-Logs analysieren (via MCP Tool zum Log-Download)
  2. **Implementer + Tester** mit konkreter Fehlerbeschreibung neu starten
  3. Änderungen aushandeln lassen
  4. **Quality-Ensurance** erneut laufen lassen
  5. Commit: `git add -A && git commit -m "fix: <feature> - <kurzbeschreibung>"`
  6. `git push origin feature/<name>`
  7. Zurück zu Phase 6
- Nach 5 Iterationen ohne Erfolg: User um manuelle Hilfe bitten
