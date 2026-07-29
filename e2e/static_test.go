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

func TestAddStaticSite(t *testing.T) {
	name := "e2e-static-add"
	port := 8080
	cleanupStaticDefer(t, name)

	res := RunVigil("add", name,
		"--type=static",
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--build-dir=%s/example-project/static", FixturesDir),
		"--nginx-domain=test.local",
		fmt.Sprintf("--nginx-path=%s/example-project/static", FixturesDir),
		fmt.Sprintf("--smoke-test-script=%s/example-project/smoke-pass.sh", FixturesDir),
	)
	RequireSuccess(t, res, "vigil add static")
	assert.Contains(t, res.Stdout, "Registered app")

	assert.True(t, FileExists(AppStoreFile(name)), "store file should exist")
	assert.True(t, SiteConfigExists(name), "nginx config should exist")
	assert.True(t, SiteEnabled(name), "site should be enabled")

	assert.True(t, NginxConfigValid(t), "nginx config should be valid")
}

func TestRemoveStaticSite(t *testing.T) {
	name := "e2e-static-remove"
	port := 8081

	addStaticSite(t, name, port)

	res := RunVigil("remove", name)
	RequireSuccess(t, res, "vigil remove static")

	assert.False(t, FileExists(AppStoreFile(name)), "store file should be deleted")
	assert.False(t, SiteConfigExists(name), "nginx config should be deleted")
	assert.False(t, SiteEnabled(name), "site should be disabled")
}

func TestStaticSiteRestart(t *testing.T) {
	name := "e2e-static-restart"
	port := 8082
	cleanupStaticDefer(t, name)

	addStaticSite(t, name, port)
	assert.True(t, SiteEnabled(name), "site should be enabled before restart")

	res := RunVigil("restart", name)
	RequireSuccess(t, res, "vigil restart static")
	assert.True(t, SiteEnabled(name), "site should be enabled after restart")
	assert.True(t, NginxConfigValid(t), "nginx config should be valid")
}

func TestStaticSiteLogs(t *testing.T) {
	name := "e2e-static-logs"
	port := 8083
	cleanupStaticDefer(t, name)

	addStaticSite(t, name, port)

	res := RunCmd("curl", "-s", "-H", "Host: test.local", fmt.Sprintf("http://localhost:%d/", port))
	require.Equal(t, 0, res.ExitCode, "curl should succeed")
	require.NotEmpty(t, res.Stdout, "curl response should not be empty")

	res = RunVigil("logs", name, "--lines=5")
	RequireSuccess(t, res, "vigil logs static")
	// retry in case nginx hasn't flushed the log entry yet
	for i := 0; i < 5 && res.Stdout == ""; i++ {
		time.Sleep(200 * time.Millisecond)
		res = RunVigil("logs", name, "--lines=5")
	}
	assert.NotEmpty(t, res.Stdout, "logs should not be empty")
}

func addStaticSite(t *testing.T, name string, port int) {
	t.Helper()
	res := RunVigil("add", name,
		"--type=static",
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--build-dir=%s/example-project/static", FixturesDir),
		"--nginx-domain=test.local",
		fmt.Sprintf("--nginx-path=%s/example-project/static", FixturesDir),
		fmt.Sprintf("--smoke-test-script=%s/example-project/smoke-pass.sh", FixturesDir),
	)
	RequireSuccess(t, res, "vigil add static")

	res = RunVigil("start", name)
	RequireSuccess(t, res, "vigil start static")
}

func cleanupStaticDefer(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		RunVigil("remove", name)
		os.Remove(filepath.Join("/etc/nginx/sites-available", name+".conf"))
		os.Remove(filepath.Join("/etc/nginx/sites-enabled", name+".conf"))
	})
}
