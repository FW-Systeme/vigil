package systemd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
)

var (
	_        Client = (*client)(nil)
	errNoConn       = errors.New("systemd dbus connection not available")

	journalctlCmd = "journalctl"
	systemctlCmd  = "systemctl"
	logrotateDir  string
)

func init() {
	logrotateDir = os.Getenv("VIRGIL_LOGROTATE_DIR")
	if logrotateDir == "" {
		logrotateDir = "/etc/logrotate.d"
	}
}

type client struct {
	conn *dbus.Conn
}

func New() (Client, error) {
	conn, err := dbus.NewSystemConnectionContext(context.Background())
	if err != nil {
		return nil, err
	}
	return &client{conn: conn}, nil
}

var unitPath = func(name string) string {
	base := os.Getenv("VIRGIL_SYSTEMD_DIR")
	if base == "" {
		base = "/etc/systemd/system"
	}
	return filepath.Join(base, name+".service")
}

func (c *client) StartUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	return c.jobWait(ctx, func(ch chan<- string) (int, error) {
		return c.conn.StartUnitContext(ctx, name+".service", "replace", ch)
	})
}

func (c *client) StopUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	return c.jobWait(ctx, func(ch chan<- string) (int, error) {
		return c.conn.StopUnitContext(ctx, name+".service", "replace", ch)
	})
}

func (c *client) RestartUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	return c.jobWait(ctx, func(ch chan<- string) (int, error) {
		return c.conn.ReloadOrRestartUnitContext(ctx, name+".service", "replace", ch)
	})
}

func jobWait(ctx context.Context, fn func(chan<- string) (int, error)) error {
	ch := make(chan string, 1)
	_, err := fn(ch)
	if err != nil {
		return err
	}
	select {
	case result := <-ch:
		if result != "done" {
			return fmt.Errorf("job failed: %s", result)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *client) jobWait(ctx context.Context, fn func(chan<- string) (int, error)) error {
	return jobWait(ctx, fn)
}

func (c *client) EnableUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	_, _, err := c.conn.EnableUnitFilesContext(ctx, []string{unitPath(name)}, false, false)
	return err
}

func (c *client) DisableUnit(ctx context.Context, name string) error {
	if c.conn == nil {
		return errNoConn
	}
	_, err := c.conn.DisableUnitFilesContext(ctx, []string{unitPath(name)}, false)
	return err
}

func (c *client) UnitStatus(ctx context.Context, name string) (activeState, subState string, err error) {
	if c.conn == nil {
		return "", "", errNoConn
	}
	units, err := c.conn.ListUnitsByNamesContext(ctx, []string{name + ".service"})
	if err != nil {
		return "", "", err
	}
	if len(units) == 0 {
		return "inactive", "dead", nil
	}
	return units[0].ActiveState, units[0].SubState, nil
}

func (c *client) CreateUnitFile(name string, content []byte) error {
	return os.WriteFile(unitPath(name), content, 0644) //nolint:gosec
}

func (c *client) RemoveUnitFile(name string) error {
	err := os.Remove(unitPath(name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (c *client) Reload(ctx context.Context) error {
	if c.conn == nil {
		return errNoConn
	}
	return c.conn.ReloadContext(ctx)
}

func (c *client) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}

func (c *client) Logs(ctx context.Context, name string, lines int, follow bool) (io.ReadCloser, error) {
	args := []string{"-u", name + ".service", "-o", "cat"}
	if lines > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", lines))
	}
	if follow {
		args = append(args, "-f")
	}
	cmd := exec.CommandContext(ctx, journalctlCmd, args...)
	cmd.Stderr = nil
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating journalctl pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting journalctl: %w", err)
	}
	return r, nil
}

func (c *client) SetupLogging(ctx context.Context, name string, logPath, maxSize string, rotate int) error {
	unit := unitPath(name)
	data, err := os.ReadFile(unit)
	if err != nil {
		return fmt.Errorf("reading unit file: %w", err)
	}
	content := string(data)

	if strings.Contains(content, "StandardOutput=") || strings.Contains(content, "StandardError=") {
		return nil
	}

	lines := strings.Split(content, "\n")
	serviceLine := -1
	nextSectionLine := len(lines)
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "[Service]" {
			serviceLine = i
		} else if serviceLine >= 0 && strings.HasPrefix(trim, "[") {
			nextSectionLine = i
			break
		}
	}

	var out []string
	if serviceLine < 0 {
		out = make([]string, 0, len(lines)+3)
		out = append(out, lines...)
		out = append(out, "[Service]",
			"StandardOutput=append:"+logPath,
			"StandardError=append:"+logPath)
	} else {
		for i, line := range lines {
			if i == nextSectionLine {
				out = append(out,
					"StandardOutput=append:"+logPath,
					"StandardError=append:"+logPath)
			}
			out = append(out, line)
		}
	}

	if err := os.WriteFile(unit, []byte(strings.Join(out, "\n")), 0644); err != nil { //nolint:gosec
		return fmt.Errorf("writing unit file: %w", err)
	}

	if err := c.writeLogrotate(name, logPath, maxSize, rotate); err != nil {
		return err
	}

	return c.reloadAndRestart(ctx, name)
}

func (c *client) RemoveLogging(ctx context.Context, name string) error {
	unit := unitPath(name)
	data, err := os.ReadFile(unit)
	if err != nil {
		return fmt.Errorf("reading unit file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "StandardOutput=") || strings.HasPrefix(trimmed, "StandardError=") {
			continue
		}
		cleaned = append(cleaned, line)
	}

	if err := os.WriteFile(unit, []byte(strings.Join(cleaned, "\n")), 0644); err != nil { //nolint:gosec
		return fmt.Errorf("writing unit file: %w", err)
	}

	logrotatePath := filepath.Join(logrotateDir, "vigil-"+name)
	if err := os.Remove(logrotatePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing logrotate config: %w", err)
	}

	return c.reloadAndRestart(ctx, name)
}

func (c *client) writeLogrotate(name, logPath, maxSize string, rotate int) error {
	content := fmt.Sprintf(`%s {
	size %s
	rotate %d
	compress
	missingok
	notifempty
	copytruncate
}
`, logPath, maxSize, rotate)

	path := filepath.Join(logrotateDir, "vigil-"+name)
	if err := os.MkdirAll(logrotateDir, 0755); err != nil {
		return fmt.Errorf("creating logrotate dir: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0644) //nolint:gosec
}

func (c *client) reloadAndRestart(ctx context.Context, name string) error {
	reload := exec.CommandContext(ctx, systemctlCmd, "daemon-reload")                          //nolint:gosec
	if out, err := reload.CombinedOutput(); err != nil {
		return fmt.Errorf("daemon-reload failed: %w\n%s", err, string(out))
	}
	restart := exec.CommandContext(ctx, systemctlCmd, "restart", name+".service")              //nolint:gosec
	if out, err := restart.CombinedOutput(); err != nil {
		return fmt.Errorf("restart failed: %w\n%s", err, string(out))
	}
	return nil
}


