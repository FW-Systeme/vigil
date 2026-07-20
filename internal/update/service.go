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
	store   process.Store
	restart RestartFunc
	nginx   nginx.Client
	out     io.Writer
}

func NewService(store process.Store, restart RestartFunc, nginx nginx.Client, out io.Writer) Service {
	if out == nil {
		out = io.Discard
	}
	return &service{store: store, restart: restart, nginx: nginx, out: out}
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

	unlock, err := lock(workingDir)
	if err != nil {
		return err
	}
	defer unlock()
	fmt.Fprintln(s.out, "  Lock acquired")

	for _, d := range []string{releasesDir, sharedDir, incomingDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating dir %s: %w", d, err)
		}
	}

	if version == "" {
		version, err = findVersion(incomingDir)
		if err != nil {
			return err
		}
		fmt.Fprintf(s.out, "  Found package %s in incoming/\n", version)
	} else {
		fmt.Fprintf(s.out, "  Using version %s\n", version)
	}

	pkgPath := filepath.Join(incomingDir, version+".tar.gz")
	if err := verifyIntegrity(pkgPath); err != nil {
		return err
	}
	fmt.Fprintln(s.out, "  Integrity check passed")

	releaseDir := filepath.Join(releasesDir, version)
	if _, err := os.Stat(releaseDir); err == nil {
		os.RemoveAll(releaseDir)
	}
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		return fmt.Errorf("creating release dir: %w", err)
	}

	fmt.Fprintf(s.out, "  Extracting %s...\n", version+".tar.gz")
	if err := extractTarGz(pkgPath, releaseDir); err != nil {
		os.RemoveAll(releaseDir)
		return err
	}

	if !p.BundledDeps {
		fmt.Fprintln(s.out, "  Installing dependencies (npm ci)...")
		if err := installDeps(releaseDir); err != nil {
			os.RemoveAll(releaseDir)
			return err
		}
	} else {
		fmt.Fprintln(s.out,  "  Skipping npm ci (bundled deps)")
	}

	fmt.Fprintln(s.out, "  Linking shared data...")
	if err := linkShared(sharedDir, releaseDir); err != nil {
		os.RemoveAll(releaseDir)
		return fmt.Errorf("linking shared: %w", err)
	}

	oldTarget := ""
	if current, err := os.Readlink(currentSymlink); err == nil {
		oldTarget = current
	}

	fmt.Fprintf(s.out, "  Switching symlink to %s...\n", version)
	if err := switchSymlink(currentSymlink, releaseDir); err != nil {
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
				rollbackSymlink(currentSymlink, oldTarget)
				os.RemoveAll(releaseDir)
				return fmt.Errorf("applying nginx site config: %w", err)
			}
			if err := s.nginx.Reload(ctx); err != nil {
				if nginxBackup != nil {
					bf := filepath.Join(workingDir, ".vigil-nginx-backup")
					os.WriteFile(bf, nginxBackup, 0644)
					s.nginx.EnableSiteFromFile(name, bf)
					s.nginx.Reload(ctx)
					os.Remove(bf)
				}
				rollbackSymlink(currentSymlink, oldTarget)
				os.RemoveAll(releaseDir)
				return fmt.Errorf("reloading nginx after site config update: %w", err)
			}
			nginxUpdated = true
			fmt.Fprintln(s.out, "  Updated nginx site config")
		}
	}

	if !nginxUpdated {
		fmt.Fprintln(s.out, "  Restarting service...")
		if err := s.restart(ctx, name); err != nil {
			rollbackSymlink(currentSymlink, oldTarget)
			s.restart(ctx, name)
			os.RemoveAll(releaseDir)
			return fmt.Errorf("restart after update: %w", err)
		}
	}

	fmt.Fprintln(s.out, "  Running smoke test...")
	if err := runScript(p.SmokeTestScript, releaseDir); err != nil {
		fmt.Fprintln(s.out, "  Smoke test failed - rolling back...")
		if nginxUpdated {
			if nginxBackup != nil {
				bf := filepath.Join(workingDir, ".vigil-nginx-backup")
				os.WriteFile(bf, nginxBackup, 0644)
				s.nginx.EnableSiteFromFile(name, bf)
				s.nginx.Reload(ctx)
				os.Remove(bf)
			}
			rollbackSymlink(currentSymlink, oldTarget)
		} else {
			rollbackSymlink(currentSymlink, oldTarget)
			s.restart(ctx, name)
		}
		return ErrRolledBack
	}
	fmt.Fprintln(s.out, "  Smoke test passed")

	if err := cleanupReleases(releasesDir, version, 3); err != nil {
		return fmt.Errorf("cleaning up releases: %w", err)
	}
	fmt.Fprintln(s.out, "  Cleaned up old releases")

	return nil
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

		target := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			of, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(of, tr); err != nil {
				of.Close()
				return err
			}
			of.Close()
		}
	}
	return nil
}

func installDeps(releaseDir string) error {
	cmd := exec.Command("npm", "ci", "--production", "--ignore-scripts")
	cmd.Dir = releaseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: npm ci: %v", ErrDepsFailed, err)
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
	os.Rename(tmp, symlinkPath)
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
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
		}
		if numA != numB {
			return numA - numB
		}
	}
	return 0
}
