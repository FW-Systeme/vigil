//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStartNonExistent(t *testing.T) {
	res := RunVigil("start", "nonexistent-app-e2e")
	RequireError(t, res, "start nonexistent")
}

func TestRemoveNonExistent(t *testing.T) {
	res := RunVigil("remove", "nonexistent-app-e2e")
	RequireError(t, res, "remove nonexistent")
}

func TestAddDuplicate(t *testing.T) {
	name := "e2e-err-dup"
	port := 3110
	cleanupDefer(t, name)

	addNodeApp(t, name, port)

	res := RunVigil("add", name,
		"--type=app",
		fmt.Sprintf("--command=%s %s/example-project/app/server.js", NodeBin, FixturesDir),
		fmt.Sprintf("--port=%d", port+1),
		fmt.Sprintf("--working-dir=%s/example-project/app", FixturesDir),
		fmt.Sprintf("--smoke-test-script=%s/example-project/smoke-pass.sh", FixturesDir),
	)
	RequireError(t, res, "add duplicate")
	assert.Contains(t, res.Stderr, "already exists")
}

func TestStopAlreadyStopped(t *testing.T) {
	name := "e2e-err-stop"
	port := 3111
	cleanupDefer(t, name)

	addNodeApp(t, name, port)

	res := RunVigil("stop", name)
	RequireSuccess(t, res, "vigil stop (first)")

	res = RunVigil("stop", name)
	RequireSuccess(t, res, "vigil stop (second)")
}
