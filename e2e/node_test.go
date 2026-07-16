//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddNodeApp(t *testing.T) {
	name := "e2e-node-add"
	port := 3091
	cleanupDefer(t, name)

	res := RunVigil("add", name,
		"--type=app",
		fmt.Sprintf("--command=%s %s/server.js", NodeBin, FixturesDir+"/dummy-app"),
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--working-dir=%s/dummy-app", FixturesDir),
		fmt.Sprintf("--smoke-test-script=%s/smoke-pass.sh", FixturesDir),
	)
	RequireSuccess(t, res, "vigil add node")
	assert.Contains(t, res.Stdout, "Registered app")

	assert.True(t, FileExists(AppStoreFile(name)), "store file should exist")
	unitFile := filepath.Join("/etc/systemd/system", name+".service")
	assert.True(t, FileExists(unitFile), "unit file should exist")
	UnitFileContains(t, name, fmt.Sprintf("WorkingDirectory=%s/dummy-app", FixturesDir))

	WaitForServiceActive(t, name, 10*time.Second)
	assert.True(t, ServiceIsEnabled(t, name), "service should be enabled")

	WaitForPortOpen(t, port, 10*time.Second)
}

func TestStopNodeApp(t *testing.T) {
	name := "e2e-node-stop"
	port := 3092
	cleanupDefer(t, name)

	addNodeApp(t, name, port)

	res := RunVigil("stop", name)
	RequireSuccess(t, res, "vigil stop")

	WaitForServiceInactive(t, name, 10*time.Second)
	assert.False(t, ServiceIsActive(t, name), "service should be inactive")
	assert.True(t, PortClosed(port, 5*time.Second), "port should be closed")
}

func TestStartNodeApp(t *testing.T) {
	name := "e2e-node-start"
	port := 3093
	cleanupDefer(t, name)

	addNodeApp(t, name, port)

	res := RunVigil("stop", name)
	RequireSuccess(t, res, "vigil stop")
	WaitForServiceInactive(t, name, 10*time.Second)

	res = RunVigil("start", name)
	RequireSuccess(t, res, "vigil start")

	WaitForServiceActive(t, name, 10*time.Second)
	WaitForPortOpen(t, port, 10*time.Second)
}

func TestRestartNodeApp(t *testing.T) {
	name := "e2e-node-restart"
	port := 3094
	cleanupDefer(t, name)

	addNodeApp(t, name, port)
	WaitForServiceActive(t, name, 10*time.Second)

	oldPID := ServicePID(name)
	require.NotEmpty(t, oldPID, "should have a PID")

	res := RunVigil("restart", name)
	RequireSuccess(t, res, "vigil restart")

	WaitForServiceActive(t, name, 10*time.Second)
	newPID := ServicePID(name)
	require.NotEmpty(t, newPID, "should have a new PID")
	WaitForPortOpen(t, port, 10*time.Second)
	_ = oldPID
	_ = newPID
}

func TestRemoveNodeApp(t *testing.T) {
	name := "e2e-node-remove"
	port := 3095

	addNodeApp(t, name, port)
	WaitForServiceActive(t, name, 10*time.Second)

	res := RunVigil("remove", name)
	RequireSuccess(t, res, "vigil remove")

	assert.False(t, FileExists(AppStoreFile(name)), "store file should be deleted")
	unitFile := filepath.Join("/etc/systemd/system", name+".service")
	assert.False(t, FileExists(unitFile), "unit file should be deleted")

	res = RunCmd("systemctl", "is-active", name+".service")
	assert.NotEqual(t, 0, res.ExitCode, "service should not be active")
}

func TestNodeAppLogs(t *testing.T) {
	name := "e2e-node-logs"
	port := 3096
	cleanupDefer(t, name)

	addNodeApp(t, name, port)
	WaitForServiceActive(t, name, 10*time.Second)

	res := RunVigil("logs", name, "--lines=5")
	RequireSuccess(t, res, "vigil logs")
	assert.NotEmpty(t, res.Stdout, "logs should not be empty")
}

func TestLogSaveEnableDisable(t *testing.T) {
	name := "e2e-node-logsave"
	port := 3097
	cleanupDefer(t, name)

	addNodeApp(t, name, port)

	res := RunVigil("logsave", "enable", name, "--max-size=5M", "--rotate=2", "--output=/var/log/vigil")
	RequireSuccess(t, res, "vigil logsave enable")

	assert.True(t, FileExists(LogStoreFile(name)), "log store file should exist")
	LogrotateConfigContains(t, name, "maxsize 5M")
	LogrotateConfigContains(t, name, "rotate 2")

	res = RunVigil("logsave", "status", name)
	RequireSuccess(t, res, "vigil logsave status")
	assert.Contains(t, res.Stdout, "Enabled")

	res = RunVigil("logsave", "disable", name)
	RequireSuccess(t, res, "vigil logsave disable")

	assert.False(t, FileExists(LogStoreFile(name)), "log store file should be deleted")
}

func TestNodeAppWithBuildCommand(t *testing.T) {
	name := "e2e-node-build"
	port := 3098
	cleanupDefer(t, name)

	markerFile := "/tmp/vigil-e2e-build-marker"
	os.Remove(markerFile)

	res := RunVigil("add", name,
		"--type=app",
		fmt.Sprintf("--command=%s %s/dummy-app/server.js", NodeBin, FixturesDir),
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--working-dir=%s/dummy-app", FixturesDir),
		fmt.Sprintf("--smoke-test-script=%s/smoke-pass.sh", FixturesDir),
		fmt.Sprintf("--build-cmd=touch %s", markerFile),
	)
	RequireSuccess(t, res, "vigil add with build-cmd")

	assert.True(t, FileExists(markerFile), "build marker should exist")
	os.Remove(markerFile)
	CleanupApp(t, name)
}

func TestNodeAppWithCustomCommand(t *testing.T) {
	name := "e2e-node-customcmd"
	port := 3099
	cleanupDefer(t, name)

	cmdStr := fmt.Sprintf("%s %s/dummy-app/server.js", NodeBin, FixturesDir)
	res := RunVigil("add", name,
		"--type=app",
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--command=%s", cmdStr),
		fmt.Sprintf("--smoke-test-script=%s/smoke-pass.sh", FixturesDir),
	)
	RequireSuccess(t, res, "vigil add with custom command")

	UnitFileContains(t, name, fmt.Sprintf("ExecStart=%s", cmdStr))
	WaitForServiceActive(t, name, 10*time.Second)
	WaitForPortOpen(t, port, 10*time.Second)
}

func TestNodeAppWithEnvFile(t *testing.T) {
	name := "e2e-node-envfile"
	port := 3100
	cleanupDefer(t, name)

	envFile := "/tmp/vigil-e2e-envfile"
	require.NoError(t, os.WriteFile(envFile, []byte("PORT=3100\n"), 0644))

	res := RunVigil("add", name,
		"--type=app",
		fmt.Sprintf("--command=%s %s/dummy-app/server.js", NodeBin, FixturesDir),
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--working-dir=%s/dummy-app", FixturesDir),
		fmt.Sprintf("--smoke-test-script=%s/smoke-pass.sh", FixturesDir),
		fmt.Sprintf("--env-file=%s", envFile),
	)
	RequireSuccess(t, res, "vigil add with env-file")

	UnitFileContains(t, name, fmt.Sprintf("EnvironmentFile=%s", envFile))
	os.Remove(envFile)
}

func TestAddWithoutSmokeScript(t *testing.T) {
	name := "e2e-node-nosmoke"
	port := 3101

	res := RunVigil("add", name,
		"--type=app",
		fmt.Sprintf("--command=%s %s/dummy-app/server.js", NodeBin, FixturesDir),
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--working-dir=%s/dummy-app", FixturesDir),
	)
	RequireError(t, res, "add without smoke script")
	assert.Contains(t, res.Stderr, "smoke_test_script")
}

func addNodeApp(t *testing.T, name string, port int) {
	t.Helper()
	res := RunVigil("add", name,
		"--type=app",
		fmt.Sprintf("--command=%s %s/dummy-app/server.js", NodeBin, FixturesDir),
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--working-dir=%s/dummy-app", FixturesDir),
		fmt.Sprintf("--smoke-test-script=%s/smoke-pass.sh", FixturesDir),
	)
	RequireSuccess(t, res, "vigil add node")
}

func CleanupApp(t *testing.T, name string) {
	t.Helper()
	RunVigil("remove", name)
	RunCmd("systemctl", "stop", name+".service")
	RunCmd("systemctl", "disable", "--now", name+".service")
	os.Remove(filepath.Join("/etc/systemd/system", name+".service"))
}

func cleanupDefer(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		CleanupApp(t, name)
	})
}
