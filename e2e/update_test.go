//go:build e2e
// +build e2e

package e2e

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTarGz(srcDir, destFile string) error {
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return err
	}

	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
}

func writeChecksum(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(data)
	return os.WriteFile(filePath+".sha256", []byte(hex.EncodeToString(hash[:])+"\n"), 0644)
}

func TestUpdateFullCycle(t *testing.T) {
	name := "e2e-update"
	port := 3120
	version := "1.0.0"

	workDir := "/tmp/vigil-e2e-update-" + name
	require.NoError(t, os.MkdirAll(workDir, 0755))
	t.Cleanup(func() { os.RemoveAll(workDir) })

	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "server.js"),
		[]byte(fmt.Sprintf(`const http = require('http');
const port = %d;
http.createServer((req, res) => {
    res.writeHead(200, {'Content-Type': 'text/plain'});
    res.end('v1\n');
}).listen(port, () => console.log('v1 on ' + port));
`, port)),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "package.json"),
		[]byte(`{"name":"`+name+`","private":true}`),
		0644,
	))

	appDir := filepath.Join(workDir, "app-v2")
	require.NoError(t, os.MkdirAll(appDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(appDir, "server.js"),
		[]byte(fmt.Sprintf(`const http = require('http');
const port = %d;
http.createServer((req, res) => {
    res.writeHead(200, {'Content-Type': 'text/plain'});
    res.end('v2\n');
}).listen(port, () => console.log('v2 on ' + port));
`, port)),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(appDir, "package.json"),
		[]byte(`{"name":"`+name+`","private":true}`),
		0644,
	))

	pkgFile := filepath.Join(workDir, "incoming", version+".tar.gz")
	require.NoError(t, createTarGz(appDir, pkgFile))
	require.NoError(t, writeChecksum(pkgFile))

	res := RunVigil("add", name,
		"--type=node",
		"--entry=server.js",
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--working-dir=%s", workDir),
		"--smoke-test-script=/bin/true",
		"--bundled-deps",
	)
	RequireSuccess(t, res, "vigil add for update test")
	WaitForServiceActive(t, name, 10*time.Second)
	WaitForPortOpen(t, port, 10*time.Second)

	res = RunVigil("update", name, "--version", version)
	RequireSuccess(t, res, "vigil update")
	assert.Contains(t, res.Stdout, "Updated")

	releaseDir := filepath.Join(workDir, "releases", version)
	assert.True(t, FileExists(releaseDir), "release dir should exist")
	assert.True(t, FileExists(filepath.Join(releaseDir, "server.js")), "server.js in release")

	currentLink, err := os.Readlink(filepath.Join(workDir, "current"))
	require.NoError(t, err, "current symlink should exist")
	assert.Contains(t, currentLink, version, "current should point to release")

	RunVigil("remove", name)
	RunCmd("systemctl", "stop", name+".service")
	os.Remove(filepath.Join("/etc/systemd/system", name+".service"))
}

func TestUpdateLockPrevention(t *testing.T) {
	name := "e2e-update-lock"
	port := 3121
	version := "1.1.0"

	workDir := "/tmp/vigil-e2e-update-" + name
	require.NoError(t, os.MkdirAll(workDir, 0755))
	t.Cleanup(func() { os.RemoveAll(workDir) })

	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "server.js"),
		[]byte(fmt.Sprintf(`const http = require('http');
const port = %d;
http.createServer((req, res) => {
    res.writeHead(200, {'Content-Type': 'text/plain'});
    res.end('ok\n');
}).listen(port, () => console.log('ok on ' + port));
`, port)),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "package.json"),
		[]byte(`{"name":"`+name+`","private":true}`),
		0644,
	))

	appDir := filepath.Join(workDir, "app-v2")
	require.NoError(t, os.MkdirAll(appDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(appDir, "server.js"),
		[]byte(`const http = require('http'); http.createServer((req, res) => { res.writeHead(200); res.end('v2\n'); }).listen(`+fmt.Sprintf("%d", port)+`);`),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(appDir, "package.json"),
		[]byte(`{"name":"`+name+`","private":true}`),
		0644,
	))

	pkgFile := filepath.Join(workDir, "incoming", version+".tar.gz")
	require.NoError(t, createTarGz(appDir, pkgFile))
	require.NoError(t, writeChecksum(pkgFile))

	res := RunVigil("add", name,
		"--type=node",
		"--entry=server.js",
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--working-dir=%s", workDir),
		"--smoke-test-script=/bin/true",
		"--bundled-deps",
	)
	RequireSuccess(t, res, "vigil add for lock test")
	WaitForServiceActive(t, name, 10*time.Second)

	lockPath := filepath.Join(workDir, ".vigil.lock")
	require.NoError(t, os.WriteFile(lockPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644))

	res = RunVigil("update", name, "--version", version)
	RequireError(t, res, "update with lock held")

	os.Remove(lockPath)

	RunVigil("remove", name)
	RunCmd("systemctl", "stop", name+".service")
	os.Remove(filepath.Join("/etc/systemd/system", name+".service"))
}
