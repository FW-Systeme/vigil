---
description: Run e2e test suite in container, report structured failures
---

## Task
Run vigil e2e tests in isolated Docker container. Report pass/fail per test with structured output.

## Steps
1. **Prerequisites**: Run `docker info` to verify Docker daemon is running. If `vigil-e2e` image missing, run `make e2e-build` from project root.
2. **Execute**: Run `make e2e-run 2>&1` from project root.
3. **Parse output**: Scan `go test` output for:
   - Total test count (`=== RUN`, `--- PASS`, `--- FAIL`, `--- SKIP`)
   - For each FAIL: extract test name, error message, file:line reference
   - Ignore container startup/cleanup noise, focus on test results
4. **Report** in this exact format:
   ```
   ## E2E Validation Report
   Tests: <total> | Pass: <passed> | Fail: <failed> | Skip: <skipped>

   ### Failures
   - `TestFoo`: <error message> (<file_test.go>:<line>)

   ### Details
   <full failure output, last 50 lines of relevant logs>

   ## Result: PASSED|FAILED
   ```
5. **Cleanup**: Ensure cleanup runs even on failure: `docker stop vigil-e2e-run 2>/dev/null; docker rm vigil-e2e-run 2>/dev/null`
6. **Exit**: 0 if PASSED, 1 if FAILED.
