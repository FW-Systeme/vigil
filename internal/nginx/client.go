package nginx

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"text/template"
)

var (
	_            Client = (*client)(nil)
	tailCmd             = "tail"
	logrotateDir        = "/etc/logrotate.d"
)

type client struct{}

func New() (Client, error) {
	return &client{}, nil
}

const configTemplate = `server {
	listen {{.Port}};
	server_name {{.Domain}};
	root {{.Root}};
	index index.html;
	access_log /var/log/nginx/{{.Name}}.access.log;
}
`

type configData struct {
	Port   int
	Domain string
	Root   string
	Name   string
}

func (c *client) EnableSiteFromFile(name string, configPath string) error {
	confPath := availablePath(name)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("enabling nginx site from file: reading config: %w", err)
	}
	if err := os.WriteFile(confPath, data, 0644); err != nil {
		return fmt.Errorf("enabling nginx site from file: writing config: %w", err)
	}

	linkPath := enabledPath(name)
	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("enabling nginx site from file: removing existing symlink: %w", err)
		}
	}
	if err := os.Symlink(confPath, linkPath); err != nil {
		return fmt.Errorf("enabling nginx site from file: creating symlink: %w", err)
	}
	return nil
}

func (c *client) EnableSite(name string, port int, domain, root string) error {
	tmpl, err := template.New("site").Parse(configTemplate)
	if err != nil {
		return fmt.Errorf("enabling nginx site: parsing template: %w", err)
	}

	confPath := availablePath(name)
	f, err := os.Create(confPath)
	if err != nil {
		return fmt.Errorf("enabling nginx site: creating config: %w", err)
	}
	if err := tmpl.Execute(f, configData{Port: port, Domain: domain, Root: root, Name: name}); err != nil {
		f.Close()
		return fmt.Errorf("enabling nginx site: writing config: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("enabling nginx site: closing config: %w", err)
	}

	linkPath := enabledPath(name)
	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("enabling nginx site: removing existing symlink: %w", err)
		}
	}

	if err := os.Symlink(confPath, linkPath); err != nil {
		return fmt.Errorf("enabling nginx site: creating symlink: %w", err)
	}

	return nil
}

func (c *client) DisableSite(name string) error {
	if err := os.Remove(enabledPath(name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("disabling nginx site: %w", err)
	}
	return nil
}

func (c *client) RemoveSiteConfig(name string) error {
	if err := os.Remove(availablePath(name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing nginx site config: %w", err)
	}
	return nil
}

func (c *client) SiteEnabled(name string) (bool, error) {
	if _, err := os.Stat(enabledPath(name)); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("checking nginx site enabled: %w", err)
	}
	return true, nil
}

func (c *client) Reload(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "nginx", "-s", "reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reloading nginx: %w", err)
	}
	return nil
}

func (c *client) Close() error {
	return nil
}

func (c *client) LogFile(name string) string {
	return "/var/log/nginx/" + name + ".access.log"
}

func (c *client) Logs(ctx context.Context, name string, lines int, follow bool) (io.ReadCloser, error) {
	path := c.LogFile(name)
	if follow {
		args := []string{"-f", "-n", strconv.Itoa(lines)}
		cmd := exec.CommandContext(ctx, tailCmd, args...) //nolint:gosec
		cmd.Args = append(cmd.Args, path)
		r, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("creating tail pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("starting tail: %w", err)
		}
		return r, nil
	}

	if lines <= 0 {
		// cat entire file
		cmd := exec.CommandContext(ctx, "cat", path) //nolint:gosec
		r, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("creating cat pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("starting cat: %w", err)
		}
		return r, nil
	}

	cmd := exec.CommandContext(ctx, tailCmd, "-n", strconv.Itoa(lines), path) //nolint:gosec
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating tail pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting tail: %w", err)
	}
	return r, nil
}

func (c *client) SetupLogging(name string, logPath string, maxSize string, rotate int) error {
	content := fmt.Sprintf(`%s {
	size %s
	rotate %d
	compress
	missingok
	notifempty
	copytruncate
}
`, logPath, maxSize, rotate)

	logrotatePath := filepath.Join(logrotateDir, "vigil-"+name)
	if err := os.MkdirAll(logrotateDir, 0755); err != nil {
		return fmt.Errorf("creating logrotate dir: %w", err)
	}
	return os.WriteFile(logrotatePath, []byte(content), 0644) //nolint:gosec
}

func (c *client) RemoveLogging(name string) error {
	logrotatePath := filepath.Join(logrotateDir, "vigil-"+name)
	err := os.Remove(logrotatePath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SiteConfigPath returns the filesystem path for the nginx site config file.
func SiteConfigPath(name string) string {
	return availablePath(name)
}

var availablePath = func(name string) string {
	base := os.Getenv("VIRGIL_NGINX_AVAILABLE_DIR")
	if base == "" {
		base = "/etc/nginx/sites-available"
	}
	return filepath.Join(base, name+".conf")
}

var enabledPath = func(name string) string {
	base := os.Getenv("VIRGIL_NGINX_ENABLED_DIR")
	if base == "" {
		base = "/etc/nginx/sites-enabled"
	}
	return filepath.Join(base, name+".conf")
}
