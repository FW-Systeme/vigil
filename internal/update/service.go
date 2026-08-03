package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FW-Systeme/Virgil/internal/nginx"
	"github.com/FW-Systeme/Virgil/internal/process"
)

type service struct {
	store     process.Store
	restart   RestartFunc
	nginx     nginx.Client
	stdout    io.Writer
	logOutput bool
}

func NewService(store process.Store, restart RestartFunc, nginx nginx.Client, stdout io.Writer, logOutput bool) Service {
	return &service{store: store, restart: restart, nginx: nginx, stdout: stdout, logOutput: logOutput}
}

func (s *service) Update(ctx context.Context, name string, version string) error {
	p, err := s.store.Load(name)
	if err != nil {
		return fmt.Errorf("loading process: %w", err)
	}

	workingDir := p.WorkingDir
	if workingDir == "" {
		return fmt.Errorf("working_dir not set")
	}

	releasesDir := filepath.Join(workingDir, "releases")
	incomingDir := filepath.Join(workingDir, "incoming")
	sharedDir := filepath.Join(workingDir, "shared")
	currentSymlink := filepath.Join(workingDir, "current")

	for _, d := range []string{workingDir, releasesDir, sharedDir, incomingDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating dir %s: %w", d, err)
		}
	}

	unlock, err := lock(workingDir)
	if err != nil {
		return err
	}
	defer unlock()

	var fw io.Writer
	if s.logOutput {
		logPath := filepath.Join(workingDir, ".vigil-update.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		defer f.Close()
		fw = f
	}

	log := NewLogger(s.stdout, fw)

	log.Log("lock.acquired", nil)
	defer func() {
		log.Log("lock.released", nil)
	}()

	if version == "" {
		version, err = findVersion(incomingDir)
		if err != nil {
			log.Log("update.failed", map[string]any{"error": err.Error(), "step": "version.resolve"})
			return err
		}
		log.Log("version.resolve", map[string]any{"version": version, "source": "incoming"})
	} else {
		log.Log("version.resolve", map[string]any{"version": version, "source": "flag"})
	}

	pkgPath := filepath.Join(incomingDir, version+".tar.gz")
	if err := verifyIntegrity(pkgPath); err != nil {
		log.Log("integrity.failed", map[string]any{"error": err.Error()})
		return err
	}
	log.Log("integrity.checked", nil)

	releaseDir := filepath.Join(releasesDir, version)
	if _, err := os.Stat(releaseDir); err == nil {
		os.RemoveAll(releaseDir)
	}
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		log.Log("update.failed", map[string]any{"error": err.Error(), "step": "mkdir.release"})
		return fmt.Errorf("creating release dir: %w", err)
	}

	log.Log("extract.start", map[string]any{"version": version})
	if err := extractTarGz(pkgPath, releaseDir); err != nil {
		log.Log("extract.failed", map[string]any{"error": err.Error()})
		os.RemoveAll(releaseDir)
		return err
	}
	log.Log("extract.done", map[string]any{"version": version})

	if !p.BundledDeps {
		log.Log("deps.install_start", nil)
		if err := installDeps(releaseDir, p.InstallCmd); err != nil {
			log.Log("deps.install_failed", map[string]any{"error": err.Error()})
			os.RemoveAll(releaseDir)
			return err
		}
		log.Log("deps.install_done", nil)
	} else {
		log.Log("deps.skip", map[string]any{"reason": "bundled"})
	}

	log.Log("shared.link_start", nil)
	if err := linkShared(sharedDir, releaseDir); err != nil {
		log.Log("shared.link_failed", map[string]any{"error": err.Error()})
		os.RemoveAll(releaseDir)
		return fmt.Errorf("linking shared: %w", err)
	}
	log.Log("shared.link_done", nil)

	oldTarget := ""
	if current, err := os.Readlink(currentSymlink); err == nil {
		oldTarget = current
	}

	log.Log("symlink.switch", map[string]any{"version": version})
	if err := switchSymlink(currentSymlink, releaseDir); err != nil {
		log.Log("symlink.switch_failed", map[string]any{"error": err.Error()})
		os.RemoveAll(releaseDir)
		return fmt.Errorf("switching symlink: %w", err)
	}

	nginxUpdated := false
	var nginxBackup []byte

	if p.Type == process.TypeStatic && s.nginx != nil {
		nginxConfigPath := filepath.Join(releaseDir, "nginx.conf")
		if _, err := os.Stat(nginxConfigPath); err == nil {
			currentConfig := nginx.SiteConfigPath(name)
			if data, rerr := os.ReadFile(currentConfig); rerr == nil {
				nginxBackup = data
			}

			if err := s.nginx.EnableSiteFromFile(name, nginxConfigPath); err != nil {
				log.Log("nginx.enable_failed", map[string]any{"error": err.Error()})
				rollbackSymlink(currentSymlink, oldTarget)
				os.RemoveAll(releaseDir)
				return fmt.Errorf("applying nginx site config: %w", err)
			}
			if err := s.nginx.Reload(ctx); err != nil {
				s.restoreNginxBackup(ctx, name, workingDir, nginxBackup, log)
				log.Log("nginx.reload_failed", map[string]any{"error": err.Error()})
				rollbackSymlink(currentSymlink, oldTarget)
				os.RemoveAll(releaseDir)
				return fmt.Errorf("reloading nginx after site config update: %w", err)
			}
			nginxUpdated = true
			log.Log("nginx.updated", nil)
		}
	}

	if !nginxUpdated {
		log.Log("service.restart_start", nil)
		if err := s.restart(ctx, name); err != nil {
			log.Log("service.restart_failed", map[string]any{"error": err.Error()})
			rollbackSymlink(currentSymlink, oldTarget)
			if rerr := s.restart(ctx, name); rerr != nil {
				log.Log("rollback.restart_failed", map[string]any{"error": rerr.Error()})
			}
			os.RemoveAll(releaseDir)
			return fmt.Errorf("restart after update: %w", err)
		}
		log.Log("service.restart_done", nil)
	}

	log.Log("smoke_test.start", nil)
	if err := runScript(p.SmokeTestScript, releaseDir); err != nil {
		log.Log("smoke_test.failed", map[string]any{"error": err.Error()})
		log.Log("rollback.start", nil)
		if nginxUpdated {
			s.restoreNginxBackup(ctx, name, workingDir, nginxBackup, log)
			rollbackSymlink(currentSymlink, oldTarget)
		} else {
			rollbackSymlink(currentSymlink, oldTarget)
			if err := s.restart(ctx, name); err != nil {
				log.Log("rollback.restart_failed", map[string]any{"error": err.Error()})
			}
		}
		log.Log("rollback.done", nil)
		return ErrRolledBack
	}
	log.Log("smoke_test.passed", nil)

	if err := cleanupReleases(releasesDir, version, 3); err != nil {
		log.Log("cleanup.failed", map[string]any{"error": err.Error()})
		return fmt.Errorf("cleaning up releases: %w", err)
	}
	log.Log("cleanup.done", nil)

	log.Log("update.complete", map[string]any{"version": version})
	return nil
}

func (s *service) restoreNginxBackup(ctx context.Context, name, workingDir string, nginxBackup []byte, log *Logger) {
	if len(nginxBackup) == 0 {
		return
	}
	bf := filepath.Join(workingDir, ".vigil-nginx-backup")
	if err := os.WriteFile(bf, nginxBackup, 0600); err != nil {
		log.Log("rollback.nginx_backup_write_failed", map[string]any{"error": err.Error()})
		return
	}
	if err := s.nginx.EnableSiteFromFile(name, bf); err != nil {
		log.Log("rollback.nginx_restore_failed", map[string]any{"error": err.Error()})
	}
	if err := s.nginx.Reload(ctx); err != nil {
		log.Log("rollback.nginx_reload_failed", map[string]any{"error": err.Error()})
	}
	if err := os.Remove(bf); err != nil {
		log.Log("rollback.nginx_backup_remove_failed", map[string]any{"error": err.Error()})
	}
}

func lock(workingDir string) (func(), error) {
	lockPath := filepath.Join(workingDir, ".vigil.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("creating lock: %w", err)
	}
	fmt.Fprintf(f, "%d\n", os.Getpid())
	f.Close()
	return func() { os.Remove(lockPath) }, nil
}

func findVersion(incomingDir string) (string, error) {
	entries, err := os.ReadDir(incomingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoPackage
		}
		return "", fmt.Errorf("reading incoming dir: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".sha256") {
			return strings.TrimSuffix(name, ".tar.gz"), nil
		}
	}
	return "", ErrNoPackage
}

func verifyIntegrity(pkgPath string) error {
	sumFile := pkgPath + ".sha256"
	expectedRaw, err := os.ReadFile(sumFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading checksum: %w", err)
	}
	expected := strings.TrimSpace(string(expectedRaw))

	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return fmt.Errorf("reading package: %w", err)
	}
	hash := sha256.Sum256(data)
	got := hex.EncodeToString(hash[:])

	if !strings.EqualFold(expected, got) {
		return ErrIntegrity
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening package: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading gzip: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Reject archive entries that try to escape the destination tree.
		if strings.Contains(header.Name, "..") || filepath.IsAbs(header.Name) {
			continue
		}
		//nolint:gosec // G305: path traversal is prevented by the prefix check below; the archive is trusted (integrity verified, written by the deployment pipeline).
		target := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			//nolint:gosec // G115: tar header Mode from integrity-verified archive.
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			//nolint:gosec // G115: tar header Mode from integrity-verified archive.
			of, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			//nolint:gosec // G110: decompression bomb is mitigated by the trusted archive (integrity verified, internal deployment).
			if _, err := io.Copy(of, tr); err != nil {
				of.Close()
				return err
			}
			of.Close()
		}
	}
	return nil
}

func installDeps(releaseDir string, installCmd string) error {
	cmd := exec.Command("sh", "-c", installCmd) //nolint:gosec // G204: intentional shell command configured by the operator.
	cmd.Dir = releaseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrDepsFailed, installCmd, err)
	}
	return nil
}

func runScript(script string, releaseDir string) error {
	cmd := exec.Command(script, releaseDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrSmokeTest, script, err)
	}
	return nil
}

func linkShared(sharedDir, releaseDir string) error {
	entries, err := os.ReadDir(sharedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		src := filepath.Join(sharedDir, e.Name())
		dst := filepath.Join(releaseDir, e.Name())
		if _, err := os.Lstat(dst); err == nil {
			os.Remove(dst)
		}
		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("symlinking %s: %w", e.Name(), err)
		}
	}
	return nil
}

func switchSymlink(symlinkPath, target string) error {
	tmp := symlinkPath + ".tmp"
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, symlinkPath)
}

func rollbackSymlink(symlinkPath, oldTarget string) {
	if oldTarget == "" {
		return
	}
	tmp := symlinkPath + ".tmp"
	if err := os.Symlink(oldTarget, tmp); err != nil {
		return
	}
	if err := os.Rename(tmp, symlinkPath); err != nil {
		return
	}
}

func cleanupReleases(releasesDir, currentVersion string, keep int) error {
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type release struct {
		name string
		info os.FileInfo
	}

	var releases []release
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == currentVersion {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		releases = append(releases, release{name: name, info: info})
	}

	sort.Slice(releases, func(i, j int) bool {
		return compareVersions(releases[i].name, releases[j].name) > 0
	})

	maxOld := keep - 1
	if maxOld < 0 {
		maxOld = 0
	}
	for i := maxOld; i < len(releases); i++ {
		os.RemoveAll(filepath.Join(releasesDir, releases[i].name))
	}
	return nil
}

func compareVersions(a, b string) int {
	a = strings.TrimLeft(a, "vV")
	b = strings.TrimLeft(b, "vV")

	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			_, _ = fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			_, _ = fmt.Sscanf(partsB[i], "%d", &numB)
		}
		if numA != numB {
			return numA - numB
		}
	}
	return 0
}
