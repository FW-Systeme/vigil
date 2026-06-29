package nginx

import (
	"context"
	"io"
)

// Client defines the interface for nginx site management and log access.
type Client interface {
	EnableSite(name string, port int, domain, root string) error
	DisableSite(name string) error
	RemoveSiteConfig(name string) error
	SiteEnabled(name string) (bool, error)
	Reload(ctx context.Context) error
	Close() error

	// LogFile returns the filesystem path of the access log for the given site.
	LogFile(name string) string

	// Logs returns a reader streaming the access log for the given site.
	// lines: 0 = all; follow: tail -f mode (blocks until cancelled).
	Logs(ctx context.Context, name string, lines int, follow bool) (io.ReadCloser, error)

	// SetupLogging creates a logrotate config for the site log at logPath
	// with maxSize (e.g. "10M") and rotate count.
	SetupLogging(name string, logPath string, maxSize string, rotate int) error

	// RemoveLogging removes the logrotate config for the site.
	RemoveLogging(name string) error
}
