---
description: >
  Orchestriert den kompletten Feature-Workflow im Virgil-Go-Projekt.
  Erstellt Branch → Shared Contract (Go Interface) → Implement + Test (parallel) →
  Quality (golangci-lint + Coverage) → Unit + E2E-Tests lokal → Merge in main.
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

### Phase 5: Test-Validierung
- Unit-Tests ausführen: `go test -race -count=1 ./...`
- E2E-Tests ausführen: `make e2e-run 2>&1`
- Prüfe Resultat beider Testläufe
- Bei Fehlschlag: Fix-Schleife (max 3 Iterationen):
  1. Fehlerlogs analysieren
  2. **Implementer + Tester** mit konkreter Fehlerbeschreibung neu starten
  3. **Quality-Ensurance** erneut laufen lassen
  4. Zurück zu Phase 5
- Erst bei bestandenen Tests → Phase 6

### Phase 6: Lokale Integration
- `git checkout main`
- `git pull origin main`
- `git merge feature/<name>`
- Bei Konflikten: lösen, `git add` und `git commit`
- `git push origin main`
- Feature-Branch lokal löschen: `git branch -d feature/<name>`
