package process

import (
	"context"
	"fmt"

	"github.com/chris576/vigil/internal/nginx"
	"github.com/chris576/vigil/internal/systemd"
)

type Manager struct {
	store   Store
	systemd systemd.Client
	nginx   nginx.Client
}

func New(store Store, systemdClient systemd.Client, nginxClient nginx.Client) *Manager {
	return &Manager{
		store:   store,
		systemd: systemdClient,
		nginx:   nginxClient,
	}
}

func (m *Manager) AddProcess(ctx context.Context, p Process) error {
	if err := p.Validate(); err != nil {
		return err
	}

	if _, err := m.store.Load(p.Name); err == nil {
		return fmt.Errorf("process %q already exists", p.Name)
	}

	if err := m.store.Save(p); err != nil {
		return fmt.Errorf("saving process: %w", err)
	}

	switch p.Type {
	case TypeNode:
		content := unitContent(p)
		if err := m.systemd.CreateUnitFile(p.Name, content); err != nil {
			return fmt.Errorf("creating systemd unit: %w", err)
		}
		if err := m.systemd.EnableUnit(ctx, p.Name); err != nil {
			return fmt.Errorf("enabling systemd unit: %w", err)
		}
		if err := m.systemd.Reload(ctx); err != nil {
			return fmt.Errorf("reloading systemd: %w", err)
		}
	case TypeStatic:
		if err := m.nginx.EnableSite(p.Name, p.Port, p.NginxDomain, p.NginxPath); err != nil {
			return fmt.Errorf("enabling nginx site: %w", err)
		}
		if err := m.nginx.Reload(ctx); err != nil {
			return fmt.Errorf("reloading nginx: %w", err)
		}
	}

	return nil
}

func (m *Manager) RemoveProcess(ctx context.Context, name string) error {
	p, err := m.store.Load(name)
	if err != nil {
		return fmt.Errorf("loading process: %w", err)
	}

	switch p.Type {
	case TypeNode:
		if err := m.systemd.StopUnit(ctx, name); err != nil {
			return fmt.Errorf("stopping unit: %w", err)
		}
		if err := m.systemd.DisableUnit(ctx, name); err != nil {
			return fmt.Errorf("disabling unit: %w", err)
		}
		if err := m.systemd.RemoveUnitFile(name); err != nil {
			return fmt.Errorf("removing unit file: %w", err)
		}
		if err := m.systemd.Reload(ctx); err != nil {
			return fmt.Errorf("reloading systemd: %w", err)
		}
	case TypeStatic:
		if err := m.nginx.DisableSite(name); err != nil {
			return fmt.Errorf("disabling nginx site: %w", err)
		}
		if err := m.nginx.RemoveSiteConfig(name); err != nil {
			return fmt.Errorf("removing nginx config: %w", err)
		}
		if err := m.nginx.Reload(ctx); err != nil {
			return fmt.Errorf("reloading nginx: %w", err)
		}
	}

	if err := m.store.Delete(name); err != nil {
		return fmt.Errorf("deleting process config: %w", err)
	}

	return nil
}

func (m *Manager) ListProcesses(ctx context.Context) ([]Process, error) {
	processes, err := m.store.List()
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	return processes, nil
}

func (m *Manager) StartProcess(ctx context.Context, name string) error {
	p, err := m.store.Load(name)
	if err != nil {
		return fmt.Errorf("loading process: %w", err)
	}

	switch p.Type {
	case TypeNode:
		if err := m.systemd.StartUnit(ctx, name); err != nil {
			return fmt.Errorf("starting unit: %w", err)
		}
		return nil
	case TypeStatic:
		if err := m.nginx.EnableSite(p.Name, p.Port, p.NginxDomain, p.NginxPath); err != nil {
			return fmt.Errorf("enabling nginx site: %w", err)
		}
		if err := m.nginx.Reload(ctx); err != nil {
			return fmt.Errorf("reloading nginx: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown process type: %s", p.Type)
	}
}

func (m *Manager) StopProcess(ctx context.Context, name string) error {
	p, err := m.store.Load(name)
	if err != nil {
		return fmt.Errorf("loading process: %w", err)
	}

	switch p.Type {
	case TypeNode:
		if err := m.systemd.StopUnit(ctx, name); err != nil {
			return fmt.Errorf("stopping unit: %w", err)
		}
		return nil
	case TypeStatic:
		if err := m.nginx.DisableSite(name); err != nil {
			return fmt.Errorf("disabling nginx site: %w", err)
		}
		if err := m.nginx.Reload(ctx); err != nil {
			return fmt.Errorf("reloading nginx: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown process type: %s", p.Type)
	}
}

func (m *Manager) RestartProcess(ctx context.Context, name string) error {
	p, err := m.store.Load(name)
	if err != nil {
		return fmt.Errorf("loading process: %w", err)
	}

	switch p.Type {
	case TypeNode:
		if err := m.systemd.RestartUnit(ctx, name); err != nil {
			return fmt.Errorf("restarting unit: %w", err)
		}
		return nil
	case TypeStatic:
		if err := m.nginx.DisableSite(name); err != nil {
			return fmt.Errorf("disabling nginx site: %w", err)
		}
		if err := m.nginx.EnableSite(p.Name, p.Port, p.NginxDomain, p.NginxPath); err != nil {
			return fmt.Errorf("enabling nginx site: %w", err)
		}
		if err := m.nginx.Reload(ctx); err != nil {
			return fmt.Errorf("reloading nginx: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown process type: %s", p.Type)
	}
}

func (m *Manager) Status(ctx context.Context, name string) (activeState, subState string, err error) {
	p, err := m.store.Load(name)
	if err != nil {
		return "", "", fmt.Errorf("loading process: %w", err)
	}

	switch p.Type {
	case TypeNode:
		active, sub, err := m.systemd.UnitStatus(ctx, name)
		if err != nil {
			return "", "", fmt.Errorf("getting unit status: %w", err)
		}
		return active, sub, nil
	case TypeStatic:
		enabled, err := m.nginx.SiteEnabled(name)
		if err != nil {
			return "", "", fmt.Errorf("checking nginx site: %w", err)
		}
		if enabled {
			return "active", "enabled", nil
		}
		return "inactive", "disabled", nil
	default:
		return "", "", fmt.Errorf("unknown process type: %s", p.Type)
	}
}

func unitContent(p Process) []byte {
	content := fmt.Sprintf(`[Unit]
Description=Vigil: %s
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=/usr/bin/node %s
Restart=on-failure
RestartSec=5
`, p.Name, p.WorkingDir, p.Entry)

	if p.EnvFile != "" {
		content += fmt.Sprintf("EnvironmentFile=%s\n", p.EnvFile)
	}

	content += `
[Install]
WantedBy=multi-user.target
`
	return []byte(content)
}
