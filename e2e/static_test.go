//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddStaticSite(t *testing.T) {
	name := "e2e-static-add"
	port := 8080
	cleanupStaticDefer(t, name)

	res := RunVigil("add", name,
		"--type=static",
		fmt.Sprintf("--port=%d", port),
		"--build-dir=/e2e/fixtures/nginx-site",
		"--nginx-domain=test.local",
		"--smoke-test-script=/e2e/fixtures/smoke-pass.sh",
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

	res := RunVigil("logs", name, "--lines=5")
	RequireSuccess(t, res, "vigil logs static")
	assert.NotEmpty(t, res.Stdout, "logs should not be empty")
}

func addStaticSite(t *testing.T, name string, port int) {
	t.Helper()
	res := RunVigil("add", name,
		"--type=static",
		fmt.Sprintf("--port=%d", port),
		"--build-dir=/e2e/fixtures/nginx-site",
		"--nginx-domain=test.local",
		"--smoke-test-script=/e2e/fixtures/smoke-pass.sh",
	)
	RequireSuccess(t, res, "vigil add static")
}

func cleanupStaticDefer(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		RunVigil("remove", name)
		os.Remove(filepath.Join("/etc/nginx/sites-available", name+".conf"))
		os.Remove(filepath.Join("/etc/nginx/sites-enabled", name+".conf"))
	})
}
