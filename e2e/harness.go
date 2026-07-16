//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	VigilBin    = "/home/chhu/projects/msl5/Virgil/vigil"
	FixturesDir = "/home/chhu/projects/msl5/Virgil/e2e/fixtures"
	NodeBin     = "/home/chhu/.nvm/versions/node/v24.14.0/bin/node"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func RunVigil(args ...string) Result {
	return RunCmd(VigilBin, args...)
}

func RunCmd(name string, args ...string) Result {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ServiceIsActive(t *testing.T, name string) bool {
	t.Helper()
	res := RunCmd("systemctl", "is-active", "--quiet", name+".service")
	return res.ExitCode == 0
}

func ServiceIsEnabled(t *testing.T, name string) bool {
	t.Helper()
	res := RunCmd("systemctl", "is-enabled", "--quiet", name+".service")
	return res.ExitCode == 0
}

func ServicePID(name string) string {
	res := RunCmd("systemctl", "show", "-p", "MainPID", "--value", name+".service")
	return strings.TrimSpace(res.Stdout)
}

func PortOpen(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res := RunCmd("curl", "-sf", fmt.Sprintf("http://localhost:%d/", port))
		if res.ExitCode == 0 {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func PortClosed(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res := RunCmd("curl", "-sf", fmt.Sprintf("http://localhost:%d/", port))
		if res.ExitCode != 0 {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func CrontabContains(substr string) bool {
	res := RunCmd("crontab", "-l")
	return strings.Contains(res.Stdout, substr)
}

func SiteEnabled(name string) bool {
	symlink := filepath.Join("/etc/nginx/sites-enabled", name+".conf")
	return FileExists(symlink)
}

func SiteConfigExists(name string) bool {
	cfg := filepath.Join("/etc/nginx/sites-available", name+".conf")
	return FileExists(cfg)
}

func WaitForServiceActive(t *testing.T, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ServiceIsActive(t, name) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("service %q did not become active within %v", name, timeout)
}

func WaitForServiceInactive(t *testing.T, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !ServiceIsActive(t, name) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("service %q did not become inactive within %v", name, timeout)
}

func WaitForPortOpen(t *testing.T, port int, timeout time.Duration) {
	t.Helper()
	if !PortOpen(port, timeout) {
		t.Fatalf("port %d did not open within %v", port, timeout)
	}
}

func WaitForPortClosed(t *testing.T, port int, timeout time.Duration) {
	t.Helper()
	if !PortClosed(port, timeout) {
		t.Fatalf("port %d did not close within %v", port, timeout)
	}
}

func UnitFileContains(t *testing.T, name, substr string) {
	t.Helper()
	path := filepath.Join("/etc/systemd/system", name+".service")
	data, err := ReadFile(path)
	require.NoError(t, err, "reading unit file %s", path)
	require.Contains(t, data, substr, "unit file should contain %q", substr)
}

func LogrotateConfigContains(t *testing.T, name, substr string) {
	t.Helper()
	path := filepath.Join("/etc/logrotate.d", "vigil-"+name)
	data, err := ReadFile(path)
	require.NoError(t, err, "reading logrotate config %s", path)
	require.Contains(t, data, substr, "logrotate config should contain %q", substr)
}

func VigilStoreDir() string {
	if dir := os.Getenv("VIRGIL_HOME"); dir != "" {
		return dir
	}
	if os.Geteuid() == 0 {
		return "/etc/vigil"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "vigil")
}

func AppStoreFile(name string) string {
	return filepath.Join(VigilStoreDir(), "apps", name+".json")
}

func CronStoreFile(name string) string {
	return filepath.Join(VigilStoreDir(), "cron", name+".json")
}

func LogStoreFile(name string) string {
	return filepath.Join(VigilStoreDir(), "logs", name+".json")
}

func CleanupCron(t *testing.T, name string) {
	t.Helper()
	res := RunVigil("cron", "remove", name)
	if res.ExitCode != 0 {
		t.Logf("cleanup cron remove %s: %s", name, res.Stderr)
		return
	}
}

func CleanupAll(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		CleanupApp(t, name)
		CleanupCron(t, name)
	}
}

func RequireSuccess(t *testing.T, res Result, msg string) {
	t.Helper()
	if res.ExitCode != 0 {
		t.Fatalf("%s: exit=%d stdout=%q stderr=%q", msg, res.ExitCode, res.Stdout, res.Stderr)
	}
}

func RequireError(t *testing.T, res Result, msg string) {
	t.Helper()
	if res.ExitCode == 0 {
		t.Fatalf("%s: expected error but got success (stdout=%q)", msg, res.Stdout)
	}
}

func WaitForNginxReady(t *testing.T, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res := RunCmd("curl", "-sf", "http://localhost:80/")
		if res.ExitCode == 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Log("nginx did not become ready within timeout; continuing anyway")
}

func NginxConfigValid(t *testing.T) bool {
	t.Helper()
	res := RunCmd("nginx", "-t")
	return res.ExitCode == 0
}

// ReadLogs reads from a reader until empty and returns the string.
func ReadLogs(r io.ReadCloser) string {
	data, _ := io.ReadAll(r)
	r.Close()
	return string(data)
}

func init() {
	if os.Getenv("VIGIL_HOME") == "" && os.Geteuid() == 0 {
		os.Setenv("VIRGIL_HOME", "/etc/vigil")
	}
}
