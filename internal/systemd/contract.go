package systemd

import (
	"context"
	"io"
)

// Client defines the interface for systemd unit management and log access.
type Client interface {
	StartUnit(ctx context.Context, name string) error
	StopUnit(ctx context.Context, name string) error
	RestartUnit(ctx context.Context, name string) error
	EnableUnit(ctx context.Context, name string) error
	DisableUnit(ctx context.Context, name string) error
	UnitStatus(ctx context.Context, name string) (activeState, subState string, err error)
	CreateUnitFile(name string, content []byte) error
	RemoveUnitFile(name string) error
	Reload(ctx context.Context) error
	Close() error

	// Logs returns a reader streaming journalctl output for the given unit.
	// lines: 0 = all past lines; follow: -f mode (blocks until cancelled).
	// Caller must Close the returned reader.
	Logs(ctx context.Context, name string, lines int, follow bool) (io.ReadCloser, error)

	// SetupLogging modifies the unit file to append stdout/stderr to logPath,
	// creates a logrotate config at /etc/logrotate.d/vigil-<name> with the
	// given maxSize (e.g. "10M") and rotate count, then reloads systemd.
	SetupLogging(ctx context.Context, name string, logPath string, maxSize string, rotate int) error

	// RemoveLogging reverts SetupLogging: removes log directives from the unit,
	// deletes the logrotate config, and reloads systemd.
	RemoveLogging(ctx context.Context, name string) error
}
