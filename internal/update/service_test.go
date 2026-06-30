package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chris576/vigil/internal/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	process.Store
	p   process.Process
	err error
}

func (m *mockStore) Load(name string) (process.Process, error) {
	return m.p, m.err
}

func createTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestUpdate_NoWorkingDir(t *testing.T) {
	svc := NewService(&mockStore{p: process.Process{
		Name:            "app",
		SmokeTestScript: "/nonexistent",
	}}, nil)
	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "working_dir")
}

func TestUpdate_ErrLocked(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".vigil.lock")
	os.WriteFile(lockPath, []byte("12345\n"), 0644)

	store := &mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: "/nonexistent/smoke.sh",
	}}
	svc := NewService(store, nil)
	err := svc.Update(context.Background(), "app", "v1.0.0")
	assert.ErrorIs(t, err, ErrLocked)
}

func TestUpdate_ErrNoPackage(t *testing.T) {
	dir := t.TempDir()
	store := &mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: "/nonexistent/smoke.sh",
	}}
	svc := NewService(store, nil)
	err := svc.Update(context.Background(), "app", "")
	assert.ErrorIs(t, err, ErrNoPackage)
}

func TestUpdate_ErrIntegrity(t *testing.T) {
	dir := t.TempDir()
	incomingDir := filepath.Join(dir, "incoming")
	os.MkdirAll(incomingDir, 0755)

	pkgPath := filepath.Join(incomingDir, "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("corrupt data"), 0644)

	hash := sha256.Sum256([]byte("different data"))
	sumPath := pkgPath + ".sha256"
	os.WriteFile(sumPath, []byte(hex.EncodeToString(hash[:])), 0644)

	store := &mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: "/nonexistent/smoke.sh",
	}}
	svc := NewService(store, nil)
	err := svc.Update(context.Background(), "app", "v1.0.0")
	assert.ErrorIs(t, err, ErrIntegrity)
}

func TestUpdate_Success(t *testing.T) {
	dir := t.TempDir()
	setupDir(t, dir)
	tarData := createTarGz(t, map[string]string{
		"server.js":     `console.log("ok");`,
		"package.json":  `{"name":"app"}`,
	})
	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, tarData, 0644)

	script := filepath.Join(dir, "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	sharedDir := filepath.Join(dir, "shared")
	os.WriteFile(filepath.Join(sharedDir, ".env"), []byte("KEY=val\n"), 0644)

	restarted := false
	svc := NewService(&mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: script,
		BundledDeps:     true,
	}}, func(ctx context.Context, name string) error {
		restarted = true
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.NoError(t, err)
	assert.True(t, restarted)

	releaseDir := filepath.Join(dir, "releases", "v1.0.0")
	assert.DirExists(t, releaseDir)
	assert.FileExists(t, filepath.Join(releaseDir, "server.js"))

	current, err := os.Readlink(filepath.Join(dir, "current"))
	require.NoError(t, err)
	assert.Equal(t, releaseDir, current)

	envSymlink, err := os.Readlink(filepath.Join(releaseDir, ".env"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(sharedDir, ".env"), envSymlink)
}

func TestUpdate_AutoDetectVersion(t *testing.T) {
	dir := t.TempDir()
	setupDir(t, dir)
	tarData := createTarGz(t, map[string]string{"data": "x"})
	pkgPath := filepath.Join(dir, "incoming", "v2.0.0.tar.gz")
	os.WriteFile(pkgPath, tarData, 0644)

	script := filepath.Join(dir, "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	svc := NewService(&mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: script,
		BundledDeps:     true,
	}}, func(ctx context.Context, name string) error {
		return nil
	})

	err := svc.Update(context.Background(), "app", "")
	require.NoError(t, err)

	releaseDir := filepath.Join(dir, "releases", "v2.0.0")
	assert.DirExists(t, releaseDir)
}

func TestUpdate_RollbackOnSmokeTestFailure(t *testing.T) {
	dir := t.TempDir()
	setupDir(t, dir)

	oldRelease := filepath.Join(dir, "releases", "v0.9.0")
	os.MkdirAll(oldRelease, 0755)
	os.WriteFile(filepath.Join(oldRelease, "server.js"), []byte("old"), 0644)

	currentSymlink := filepath.Join(dir, "current")
	os.Symlink(oldRelease, currentSymlink)

	tarData := createTarGz(t, map[string]string{"server.js": "new"})
	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, tarData, 0644)

	script := filepath.Join(dir, "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 1
`)

	restartCount := 0
	svc := NewService(&mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: script,
		BundledDeps:     true,
	}}, func(ctx context.Context, name string) error {
		restartCount++
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	assert.ErrorIs(t, err, ErrRolledBack)

	current, _ := os.Readlink(currentSymlink)
	assert.Equal(t, oldRelease, current, "symlink should point back to old release")
	assert.Equal(t, 2, restartCount, "restart called: once for new, once for rollback")
}

func TestUpdate_BundledDepsSkipsNpmCI(t *testing.T) {
	dir := t.TempDir()
	setupDir(t, dir)
	tarData := createTarGz(t, map[string]string{
		"server.js":     `console.log("ok");`,
		"node_modules/x": "preinstalled",
	})
	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, tarData, 0644)

	script := filepath.Join(dir, "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	svc := NewService(&mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: script,
		BundledDeps:     true,
	}}, func(ctx context.Context, name string) error {
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.NoError(t, err)

	releaseDir := filepath.Join(dir, "releases", "v1.0.0")
	assert.FileExists(t, filepath.Join(releaseDir, "node_modules/x"))
}

func TestUpdate_CleanupOldReleases(t *testing.T) {
	dir := t.TempDir()
	setupDir(t, dir)

	for _, v := range []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0"} {
		os.MkdirAll(filepath.Join(dir, "releases", v), 0755)
	}

	currentSymlink := filepath.Join(dir, "current")
	os.Symlink(filepath.Join(dir, "releases", "v1.3.0"), currentSymlink)

	tarData := createTarGz(t, map[string]string{"data": "x"})
	pkgPath := filepath.Join(dir, "incoming", "v2.0.0.tar.gz")
	os.WriteFile(pkgPath, tarData, 0644)

	script := filepath.Join(dir, "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	svc := NewService(&mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: script,
		BundledDeps:     true,
	}}, func(ctx context.Context, name string) error {
		return nil
	})

	err := svc.Update(context.Background(), "app", "v2.0.0")
	require.NoError(t, err)

	assert.DirExists(t, filepath.Join(dir, "releases", "v2.0.0"))
	assert.DirExists(t, filepath.Join(dir, "releases", "v1.3.0"))
	assert.DirExists(t, filepath.Join(dir, "releases", "v1.2.0"))
	assert.NoDirExists(t, filepath.Join(dir, "releases", "v1.1.0"))
	assert.NoDirExists(t, filepath.Join(dir, "releases", "v1.0.0"))
}

func TestUpdate_SharedSymlinks(t *testing.T) {
	dir := t.TempDir()
	setupDir(t, dir)

	sharedDir := filepath.Join(dir, "shared")
	os.WriteFile(filepath.Join(sharedDir, ".env"), []byte("KEY=val\n"), 0644)
	os.WriteFile(filepath.Join(sharedDir, "config.json"), []byte("{}"), 0644)

	tarData := createTarGz(t, map[string]string{"server.js": "content"})
	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, tarData, 0644)

	script := filepath.Join(dir, "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	svc := NewService(&mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: script,
		BundledDeps:     true,
	}}, func(ctx context.Context, name string) error {
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.NoError(t, err)

	releaseDir := filepath.Join(dir, "releases", "v1.0.0")
	for _, name := range []string{".env", "config.json"} {
		target, err := os.Readlink(filepath.Join(releaseDir, name))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(sharedDir, name), target)
	}
}

func TestUpdate_RestartFailure(t *testing.T) {
	dir := t.TempDir()
	setupDir(t, dir)

	oldRelease := filepath.Join(dir, "releases", "v0.9.0")
	os.MkdirAll(oldRelease, 0755)
	currentSymlink := filepath.Join(dir, "current")
	os.Symlink(oldRelease, currentSymlink)

	tarData := createTarGz(t, map[string]string{"server.js": "new"})
	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, tarData, 0644)

	script := filepath.Join(dir, "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	restartCount := 0
	svc := NewService(&mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: script,
		BundledDeps:     true,
	}}, func(ctx context.Context, name string) error {
		restartCount++
		if restartCount == 1 {
			return fmt.Errorf("restart failed")
		}
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restart")

	current, _ := os.Readlink(currentSymlink)
	assert.Equal(t, oldRelease, current, "symlink rolled back after restart failure")
}

func TestFindVersion(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "v1.0.0.tar.gz"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(dir, "v1.0.0.tar.gz.sha256"), []byte("sum"), 0644)

	v, err := findVersion(dir)
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", v)
}

func TestFindVersion_Empty(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)

	_, err := findVersion(dir)
	assert.ErrorIs(t, err, ErrNoPackage)
}

func TestFindVersion_MissingDir(t *testing.T) {
	_, err := findVersion(t.TempDir() + "/nonexistent")
	assert.ErrorIs(t, err, ErrNoPackage)
}

func TestVerifyIntegrity_Mismatch(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "v1.0.0.tar.gz")
	os.WriteFile(pkg, []byte("data"), 0644)

	hash := sha256.Sum256([]byte("wrong"))
	os.WriteFile(pkg+".sha256", []byte(hex.EncodeToString(hash[:])), 0644)

	err := verifyIntegrity(pkg)
	assert.ErrorIs(t, err, ErrIntegrity)
}

func TestVerifyIntegrity_Match(t *testing.T) {
	dir := t.TempDir()
	data := []byte("data")
	pkg := filepath.Join(dir, "v1.0.0.tar.gz")
	os.WriteFile(pkg, data, 0644)

	hash := sha256.Sum256(data)
	os.WriteFile(pkg+".sha256", []byte(hex.EncodeToString(hash[:])), 0644)

	err := verifyIntegrity(pkg)
	assert.NoError(t, err)
}

func TestVerifyIntegrity_NoSumFile(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "v1.0.0.tar.gz")
	os.WriteFile(pkg, []byte("data"), 0644)

	err := verifyIntegrity(pkg)
	assert.NoError(t, err)
}

func TestExtractTarGz(t *testing.T) {
	dest := t.TempDir()
	tarData := createTarGz(t, map[string]string{
		"app/server.js":   "content",
		"app/lib/util.js": "util",
	})
	src := filepath.Join(dest, "pkg.tar.gz")
	os.WriteFile(src, tarData, 0644)

	err := extractTarGz(src, filepath.Join(dest, "out"))
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dest, "out", "app", "server.js"))
	assert.FileExists(t, filepath.Join(dest, "out", "app", "lib", "util.js"))
}

func TestExtractTarGz_InvalidGzip(t *testing.T) {
	dest := t.TempDir()
	src := filepath.Join(dest, "bad.tar.gz")
	os.WriteFile(src, []byte("not gzip"), 0644)

	err := extractTarGz(src, filepath.Join(dest, "out"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gzip")
}

func TestExtractTarGz_WithDirEntry(t *testing.T) {
	dest := t.TempDir()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	tw.WriteHeader(&tar.Header{Name: "subdir/", Typeflag: tar.TypeDir, Mode: 0755})
	tw.WriteHeader(&tar.Header{Name: "subdir/file.txt", Size: int64(4), Mode: 0644})
	tw.Write([]byte("data"))

	tw.Close()
	gz.Close()

	src := filepath.Join(dest, "pkg.tar.gz")
	os.WriteFile(src, buf.Bytes(), 0644)

	err := extractTarGz(src, filepath.Join(dest, "out"))
	require.NoError(t, err)

	assert.DirExists(t, filepath.Join(dest, "out", "subdir"))
	assert.FileExists(t, filepath.Join(dest, "out", "subdir", "file.txt"))
}

func TestExtractTarGz_RejectsPathTraversal(t *testing.T) {
	dest := t.TempDir()
	tarData := createTarGz(t, map[string]string{
		"../escape.txt": "bad",
	})
	src := filepath.Join(dest, "pkg.tar.gz")
	os.WriteFile(src, tarData, 0644)

	err := extractTarGz(src, filepath.Join(dest, "out"))
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(dest, "escape.txt"))
}

func TestSwitchSymlink_FailsOnInvalidTarget(t *testing.T) {
	// Symlink to empty string should fail
	err := switchSymlink(t.TempDir()+"/link", "")
	require.Error(t, err)
}

func TestInstallDeps_NoPackageJSON(t *testing.T) {
	err := installDeps(t.TempDir())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDepsFailed)
}

func TestSwitchSymlink(t *testing.T) {
	dir := t.TempDir()
	symPath := filepath.Join(dir, "link")
	target := filepath.Join(dir, "target")

	err := switchSymlink(symPath, target)
	require.NoError(t, err)

	got, err := os.Readlink(symPath)
	require.NoError(t, err)
	assert.Equal(t, target, got)
}

func TestRollbackSymlink_WithTarget(t *testing.T) {
	dir := t.TempDir()
	symPath := filepath.Join(dir, "link")
	oldTarget := filepath.Join(dir, "old")
	os.Symlink(oldTarget, symPath)

	rollbackSymlink(symPath, oldTarget)
	got, err := os.Readlink(symPath)
	require.NoError(t, err)
	assert.Equal(t, oldTarget, got)
}

func TestRollbackSymlink_EmptyTarget(t *testing.T) {
	rollbackSymlink("/nonexistent/link", "")
}

func TestRollbackSymlink_OldTargetGone(t *testing.T) {
	// Should not panic when old target directory doesn't exist
	dir := t.TempDir()
	symPath := filepath.Join(dir, "link")
	oldTarget := filepath.Join(dir, "nonexistent")
	os.Symlink(oldTarget, symPath)

	rollbackSymlink(symPath, oldTarget)
	got, err := os.Readlink(symPath)
	require.NoError(t, err)
	assert.Equal(t, oldTarget, got)
}

func TestLock_WritePID(t *testing.T) {
	dir := t.TempDir()
	unlock, err := lock(dir)
	require.NoError(t, err)
	require.NotNil(t, unlock)

	lockPath := filepath.Join(dir, ".vigil.lock")
	data, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\n")

	unlock()
	_, err = os.Stat(lockPath)
	assert.True(t, os.IsNotExist(err))
}

func TestLinkShared(t *testing.T) {
	sharedDir := t.TempDir()
	releaseDir := t.TempDir()

	os.WriteFile(filepath.Join(sharedDir, "file1.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(sharedDir, "file2.txt"), []byte("b"), 0644)

	err := linkShared(sharedDir, releaseDir)
	require.NoError(t, err)

	target1, _ := os.Readlink(filepath.Join(releaseDir, "file1.txt"))
	assert.Equal(t, filepath.Join(sharedDir, "file1.txt"), target1)
	target2, _ := os.Readlink(filepath.Join(releaseDir, "file2.txt"))
	assert.Equal(t, filepath.Join(sharedDir, "file2.txt"), target2)
}

func TestLinkShared_OverwritesExisting(t *testing.T) {
	sharedDir := t.TempDir()
	releaseDir := t.TempDir()

	os.WriteFile(filepath.Join(sharedDir, "config.json"), []byte(`{"shared":true}`), 0644)
	os.WriteFile(filepath.Join(releaseDir, "config.json"), []byte(`{"old":true}`), 0644)

	err := linkShared(sharedDir, releaseDir)
	require.NoError(t, err)

	target, err := os.Readlink(filepath.Join(releaseDir, "config.json"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(sharedDir, "config.json"), target)
}

func TestLinkShared_SharedNotExist(t *testing.T) {
	err := linkShared(t.TempDir()+"/nonexistent", t.TempDir())
	require.NoError(t, err)
}

func TestLinkShared_SymlinkError(t *testing.T) {
	sharedDir := t.TempDir()
	os.WriteFile(filepath.Join(sharedDir, "file.txt"), []byte("data"), 0644)

	// releaseDir is a file, not a dir → symlink creation fails
	releaseDir := t.TempDir()
	releaseFile := filepath.Join(releaseDir, "not-a-dir")
	os.WriteFile(releaseFile, []byte("x"), 0644)

	err := linkShared(sharedDir, releaseFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlinking")
}

func TestUpdate_ErrExtract(t *testing.T) {
	dir := t.TempDir()
	setupDir(t, dir)
	pkgPath := filepath.Join(dir, "incoming", "v1.0.0.tar.gz")
	os.WriteFile(pkgPath, []byte("valid tar.gz data"), 0644)
	// Remove package so extractTarGz fails but verifyIntegrity passes (no .sha256)
	os.Remove(pkgPath)

	script := filepath.Join(dir, "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)

	svc := NewService(&mockStore{p: process.Process{
		Name:            "app",
		WorkingDir:      dir,
		SmokeTestScript: script,
		BundledDeps:     true,
	}}, func(ctx context.Context, name string) error {
		return nil
	})

	err := svc.Update(context.Background(), "app", "v1.0.0")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoPackage)
	assert.NotErrorIs(t, err, ErrIntegrity)
}

func TestCleanupReleases(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)

	versions := []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0"}
	for _, v := range versions {
		os.MkdirAll(filepath.Join(dir, v), 0755)
	}

	err := cleanupReleases(dir, "v1.4.0", 3)
	require.NoError(t, err)

	assert.DirExists(t, filepath.Join(dir, "v1.4.0"))
	assert.DirExists(t, filepath.Join(dir, "v1.3.0"))
	assert.DirExists(t, filepath.Join(dir, "v1.2.0"))
	assert.NoDirExists(t, filepath.Join(dir, "v1.1.0"))
	assert.NoDirExists(t, filepath.Join(dir, "v1.0.0"))
}

func TestCleanupReleases_NoneToDelete(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1.0.0"), 0755)
	os.MkdirAll(filepath.Join(dir, "v1.1.0"), 0755)

	err := cleanupReleases(dir, "v1.1.0", 5)
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, "v1.0.0"))
	assert.DirExists(t, filepath.Join(dir, "v1.1.0"))
}

func TestCleanupReleases_NonDirEntry(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)
	os.MkdirAll(filepath.Join(dir, "v2.0.0"), 0755)
	os.MkdirAll(filepath.Join(dir, "v2.0.1"), 0755)
	// Add a non-directory entry alongside release dirs
	os.WriteFile(filepath.Join(dir, "README.txt"), []byte("info"), 0644)

	err := cleanupReleases(dir, "v2.0.1", 3)
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, "v2.0.0"))
	assert.DirExists(t, filepath.Join(dir, "v2.0.1"))
	assert.FileExists(t, filepath.Join(dir, "README.txt"), "non-dir entries should be preserved")
}

func TestCleanupReleases_NonExistentDir(t *testing.T) {
	err := cleanupReleases("/nonexistent/path", "v1.0.0", 3)
	require.NoError(t, err)
}

func TestCleanupReleases_Boundary(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1.0.0"), 0755)
	os.MkdirAll(filepath.Join(dir, "v1.0.1"), 0755)
	os.MkdirAll(filepath.Join(dir, "v1.0.2"), 0755)
	err := cleanupReleases(dir, "v1.0.2", 3)
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, "v1.0.0"))
	assert.DirExists(t, filepath.Join(dir, "v1.0.1"))
	assert.DirExists(t, filepath.Join(dir, "v1.0.2"))
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.1.0", "v1.0.0", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.10.0", "v1.9.0", 1},
		{"1.0.0", "v1.0.0", 0},
	}
	for _, tc := range tests {
		got := compareVersions(tc.a, tc.b)
		if tc.want < 0 {
			assert.True(t, got < 0, "expected %s < %s, got %d", tc.a, tc.b, got)
		} else if tc.want > 0 {
			assert.True(t, got > 0, "expected %s > %s, got %d", tc.a, tc.b, got)
		} else {
			assert.Equal(t, 0, got, "expected %s == %s", tc.a, tc.b)
		}
	}
}

func TestRunScript_ExitCode(t *testing.T) {
	script := filepath.Join(t.TempDir(), "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 1
`)
	err := runScript(script, t.TempDir())
	assert.ErrorIs(t, err, ErrSmokeTest)
}

func TestRunScript_Success(t *testing.T) {
	script := filepath.Join(t.TempDir(), "smoke.sh")
	writeScript(t, script, `#!/bin/sh
exit 0
`)
	err := runScript(script, t.TempDir())
	assert.NoError(t, err)
}

func setupDir(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, "releases"), 0755)
	os.MkdirAll(filepath.Join(dir, "shared"), 0755)
	os.MkdirAll(filepath.Join(dir, "incoming"), 0755)
}

func writeScript(t *testing.T, path, content string) {
	t.Helper()
	os.WriteFile(path, []byte(content), 0755)

	abs, err := filepath.Abs(path)
	require.NoError(t, err)

	cmd := exec.Command("sh", "-c", fmt.Sprintf("command -v %s", abs))
	if err := cmd.Run(); err != nil {
		t.Skipf("script not executable with sh: %v", err)
	}
}
