package nginx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
)

// NB: Tests that access package-level availablePath/enabledPath vars
// CANNOT use t.Parallel() because setupTempDirs monkey-patches them.
// Only pure-function tests without path var access may be parallel.

func TestNew(t *testing.T) {
	t.Parallel()
	c, err := New()
	require.NoError(t, err)
	require.NotNil(t, c)

	_, ok := c.(*client)
	assert.True(t, ok)
}

func TestClose(t *testing.T) {
	t.Parallel()
	c := &client{}
	err := c.Close()
	assert.NoError(t, err)
}

func TestConfigTemplate(t *testing.T) {
	t.Parallel()
	tmpl, err := template.New("site").Parse(configTemplate)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, configData{Port: 8080, Domain: "example.com", Root: "/var/www", Name: "myapp"})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "listen 8080;")
	assert.Contains(t, output, "server_name example.com;")
	assert.Contains(t, output, "root /var/www;")
	assert.Contains(t, output, "index index.html;")
	assert.Contains(t, output, "access_log /var/log/nginx/myapp.access.log;")
}

// setupTempDirs creates temp dirs mimicking /etc/nginx structure.
// Returns cleanup func that restores original path functions.
// Caller MUST NOT use t.Parallel() — package-level vars are used.
func setupTempDirs(t *testing.T) (cli *client, availableDir, enabledDir string, cleanup func()) {
	t.Helper()

	availableDir = t.TempDir()
	enabledDir = t.TempDir()

	origAvailable := availablePath
	origEnabled := enabledPath

	availablePath = func(name string) string {
		return filepath.Join(availableDir, name+".conf")
	}
	enabledPath = func(name string) string {
		return filepath.Join(enabledDir, name+".conf")
	}

	cleanup = func() {
		availablePath = origAvailable
		enabledPath = origEnabled
	}

	return &client{}, availableDir, enabledDir, cleanup
}

func TestClient_EnableSite(t *testing.T) {
	c, availDir, enabledDir, cleanup := setupTempDirs(t)
	defer cleanup()

	err := c.EnableSite("myapp", 8080, "example.com", "/var/www")
	require.NoError(t, err)

	// Verify config file in sites-available
	confPath := filepath.Join(availDir, "myapp.conf")
	data, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "listen 8080;")
	assert.Contains(t, string(data), "server_name example.com;")
	assert.Contains(t, string(data), "root /var/www;")
	assert.Contains(t, string(data), "index index.html;")

	// Verify symlink in sites-enabled
	linkPath := filepath.Join(enabledDir, "myapp.conf")
	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, confPath, linkTarget)
}

func TestClient_EnableSite_ReplaceExisting(t *testing.T) {
	c, availDir, enabledDir, cleanup := setupTempDirs(t)
	defer cleanup()

	// Pre-populate both dirs
	confPath := filepath.Join(availDir, "myapp.conf")
	err := os.WriteFile(confPath, []byte("old"), 0600)
	require.NoError(t, err)

	linkPath := filepath.Join(enabledDir, "myapp.conf")
	err = os.Symlink(confPath, linkPath)
	require.NoError(t, err)

	// Re-enable with different config
	err = c.EnableSite("myapp", 3000, "new.example.com", "/var/new")
	require.NoError(t, err)

	data, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "listen 3000;")
	assert.Contains(t, string(data), "server_name new.example.com;")
}

func TestClient_EnableSite_DefaultPathFails(t *testing.T) {
	c := &client{}
	err := c.EnableSite("test-site-xyz", 8080, "example.com", "/var/www")
	assert.Error(t, err)
}

func TestClient_EnableSite_SymlinkCreateError(t *testing.T) {
	c, availDir, enabledDir, cleanup := setupTempDirs(t)
	defer cleanup()

	// Create config + symlink successfully first
	err := c.EnableSite("myapp", 8080, "example.com", "/var/www")
	require.NoError(t, err)

	// Remove the symlink so EnableSite hits Symlink (not Remove)
	linkPath := filepath.Join(enabledDir, "myapp.conf")
	err = os.Remove(linkPath)
	require.NoError(t, err)

	// Make enabledDir read-only — Symlink will fail
	err = os.Chmod(enabledDir, 0555)
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(enabledDir, 0755)
	}()

	err = c.EnableSite("myapp", 8080, "example.com", "/var/www")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating symlink")

	// Verify config file still exists (created before symlink attempt)
	confPath := filepath.Join(availDir, "myapp.conf")
	_, err = os.Stat(confPath)
	assert.NoError(t, err)
}

func TestClient_EnableSite_SymlinkRemoveError(t *testing.T) {
	c, _, enabledDir, cleanup := setupTempDirs(t)
	defer cleanup()

	// Create config + symlink
	err := c.EnableSite("myapp", 8080, "example.com", "/var/www")
	require.NoError(t, err)

	// Make enabledDir read-only — Remove during replace will fail
	err = os.Chmod(enabledDir, 0555)
	require.NoError(t, err)
	defer func() {
		_ = os.Chmod(enabledDir, 0755)
	}()

	err = c.EnableSite("myapp", 8080, "example.com", "/var/www")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing existing symlink")
}

func TestClient_EnableSiteFromFile(t *testing.T) {
	c, availDir, enabledDir, cleanup := setupTempDirs(t)
	defer cleanup()

	// Create a custom config file
	configPath := filepath.Join(t.TempDir(), "my-custom.conf")
	err := os.WriteFile(configPath, []byte("custom nginx config"), 0644)
	require.NoError(t, err)

	err = c.EnableSiteFromFile("myapp", configPath)
	require.NoError(t, err)

	// Verify config was copied to sites-available
	confPath := filepath.Join(availDir, "myapp.conf")
	data, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Equal(t, "custom nginx config", string(data))

	// Verify symlink in sites-enabled
	linkPath := filepath.Join(enabledDir, "myapp.conf")
	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, confPath, linkTarget)
}

func TestClient_EnableSiteFromFile_NonExistentSource(t *testing.T) {
	c := &client{}
	err := c.EnableSiteFromFile("myapp", "/nonexistent/path.conf")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config")
}

func TestClient_EnableSiteFromFile_WriteError(t *testing.T) {
	// Create a valid config file
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "custom.conf")
	os.WriteFile(configPath, []byte("config"), 0644)

	// Use a read-only available dir → WriteFile fails
	roDir := t.TempDir()
	os.Chmod(roDir, 0555)

	c := &client{}
	origAvailable := availablePath
	availablePath = func(name string) string { return filepath.Join(roDir, name+".conf") }
	defer func() { availablePath = origAvailable }()

	err := c.EnableSiteFromFile("myapp", configPath)
	require.Error(t, err)
	os.Chmod(roDir, 0755)
}

func TestClient_DisableSite(t *testing.T) {
	t.Run("non-existent returns nil", func(t *testing.T) {
		c := &client{}
		err := c.DisableSite("nonexistent")
		assert.NoError(t, err)
	})

	t.Run("existing symlink removed", func(t *testing.T) {
		c, _, enabledDir, cleanup := setupTempDirs(t)
		defer cleanup()

		linkPath := filepath.Join(enabledDir, "myapp.conf")
		err := os.Symlink("/tmp/fake.conf", linkPath)
		require.NoError(t, err)

		err = c.DisableSite("myapp")
		require.NoError(t, err)

		_, err = os.Stat(linkPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("remove fails returns error", func(t *testing.T) {
		c, _, enabledDir, cleanup := setupTempDirs(t)
		defer cleanup()

		linkPath := filepath.Join(enabledDir, "myapp.conf")
		err := os.Symlink("/tmp/fake.conf", linkPath)
		require.NoError(t, err)

		// Remove write permission — os.Remove will fail
		err = os.Chmod(enabledDir, 0555)
		require.NoError(t, err)
		defer func() { _ = os.Chmod(enabledDir, 0755) }()

		err = c.DisableSite("myapp")
		assert.Error(t, err)
	})
}

func TestClient_RemoveSiteConfig(t *testing.T) {
	t.Run("non-existent returns nil", func(t *testing.T) {
		c := &client{}
		err := c.RemoveSiteConfig("nonexistent")
		assert.NoError(t, err)
	})

	t.Run("existing config removed", func(t *testing.T) {
		c, availDir, _, cleanup := setupTempDirs(t)
		defer cleanup()

		confPath := filepath.Join(availDir, "myapp.conf")
		err := os.WriteFile(confPath, []byte("config"), 0600)
		require.NoError(t, err)

		err = c.RemoveSiteConfig("myapp")
		require.NoError(t, err)

		_, err = os.Stat(confPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("remove fails returns error", func(t *testing.T) {
		c, availDir, _, cleanup := setupTempDirs(t)
		defer cleanup()

		confPath := filepath.Join(availDir, "myapp.conf")
		err := os.WriteFile(confPath, []byte("config"), 0600)
		require.NoError(t, err)

		// Remove write permission — os.Remove fails
		err = os.Chmod(availDir, 0555)
		require.NoError(t, err)
		defer func() { _ = os.Chmod(availDir, 0755) }()

		err = c.RemoveSiteConfig("myapp")
		assert.Error(t, err)
	})
}

func TestClient_SiteEnabled(t *testing.T) {
	t.Run("non-existent returns false", func(t *testing.T) {
		c := &client{}
		enabled, err := c.SiteEnabled("nonexistent")
		require.NoError(t, err)
		assert.False(t, enabled)
	})

	t.Run("existing symlink returns true", func(t *testing.T) {
		c, _, enabledDir, cleanup := setupTempDirs(t)
		defer cleanup()

		// Create a real target so os.Stat follows symlink and succeeds
		targetDir := t.TempDir()
		targetPath := filepath.Join(targetDir, "target.conf")
		err := os.WriteFile(targetPath, []byte("config"), 0600)
		require.NoError(t, err)

		linkPath := filepath.Join(enabledDir, "myapp.conf")
		err = os.Symlink(targetPath, linkPath)
		require.NoError(t, err)

		enabled, err := c.SiteEnabled("myapp")
		require.NoError(t, err)
		assert.True(t, enabled)
	})

	t.Run("stat error returns error", func(t *testing.T) {
		c, _, enabledDir, cleanup := setupTempDirs(t)
		defer cleanup()

		// Create target in a dir, then remove search permission
		targetDir := t.TempDir()
		targetPath := filepath.Join(targetDir, "target.conf")
		err := os.WriteFile(targetPath, []byte("x"), 0600)
		require.NoError(t, err)

		// No execute bit = not searchable → os.Stat fails with EACCES
		err = os.Chmod(targetDir, 0644)
		require.NoError(t, err)
		defer func() { _ = os.Chmod(targetDir, 0700) }()

		linkPath := filepath.Join(enabledDir, "myapp.conf")
		err = os.Symlink(targetPath, linkPath)
		require.NoError(t, err)

		_, err = c.SiteEnabled("myapp")
		assert.Error(t, err)
	})
}

func TestClient_Reload_CanceledContext(t *testing.T) {
	t.Parallel()
	c := &client{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Reload(ctx)
	assert.Error(t, err)
}

func TestPaths_default(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) string
		want string
	}{
		{name: "availablePath", fn: availablePath, want: "/etc/nginx/sites-available/myapp.conf"},
		{name: "enabledPath", fn: enabledPath, want: "/etc/nginx/sites-enabled/myapp.conf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn("myapp")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLogFile(t *testing.T) {
	t.Parallel()
	c := &client{}
	path := c.LogFile("my-site")
	assert.Equal(t, "/var/log/nginx/my-site.access.log", path)
}

func TestLogs_NilReceiver(t *testing.T) {
	t.Parallel()
	c := &client{}
	r, err := c.Logs(context.Background(), "test", 10, false)
	require.NoError(t, err)
	require.NotNil(t, r)
	r.Close()
}

func TestLogs_CatPath_NonExistentFile(t *testing.T) {
	t.Parallel()
	c := &client{}
	r, err := c.Logs(context.Background(), "nonexistent-test-xyz", 0, false)
	require.NoError(t, err)
	defer r.Close()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestLogs_TailPath_NonExistentFile(t *testing.T) {
	t.Parallel()
	c := &client{}
	r, err := c.Logs(context.Background(), "nonexistent-test-xyz", 50, false)
	require.NoError(t, err)
	defer r.Close()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestLogs_FollowStartError(t *testing.T) {
	orig := tailCmd
	tailCmd = "/nonexistent-tail-binary-xyz"
	defer func() { tailCmd = orig }()

	c := &client{}
	_, err := c.Logs(context.Background(), "test", 10, true)
	require.Error(t, err)
}

func TestLogs_FollowPath_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "tail")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\""), 0755) //nolint:gosec
	require.NoError(t, err)

	orig := tailCmd
	tailCmd = script
	defer func() { tailCmd = orig }()

	c := &client{}
	r, err := c.Logs(context.Background(), "nonexistent-test-xyz", 10, true)
	require.NoError(t, err)
	defer r.Close()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	args := string(data)
	assert.Contains(t, args, "-n 10")
}

func TestSetupLogging_CreatesLogrotate(t *testing.T) {
	dir := t.TempDir()
	orig := logrotateDir
	logrotateDir = dir
	defer func() { logrotateDir = orig }()

	c := &client{}
	err := c.SetupLogging("my-app", "/var/log/nginx/my-app.access.log", "10M", 3)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "vigil-my-app"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "/var/log/nginx/my-app.access.log")
	assert.Contains(t, content, "size 10M")
	assert.Contains(t, content, "rotate 3")
	assert.Contains(t, content, "compress")
}

func TestRemoveLogging_RemovesLogrotate(t *testing.T) {
	dir := t.TempDir()
	orig := logrotateDir
	logrotateDir = dir
	defer func() { logrotateDir = orig }()

	rotatePath := filepath.Join(dir, "vigil-my-app")
	err := os.WriteFile(rotatePath, []byte("old config"), 0600)
	require.NoError(t, err)

	c := &client{}
	err = c.RemoveLogging("my-app")
	require.NoError(t, err)

	_, err = os.Stat(rotatePath)
	assert.True(t, os.IsNotExist(err))
}

func TestRemoveLogging_Idempotent(t *testing.T) {
	dir := t.TempDir()
	orig := logrotateDir
	logrotateDir = dir
	defer func() { logrotateDir = orig }()

	c := &client{}
	err := c.RemoveLogging("ghost")
	require.NoError(t, err)
}
