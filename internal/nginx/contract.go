package nginx

import "context"

type Client interface {
	EnableSite(name string, port int, domain, root string) error
	DisableSite(name string) error
	RemoveSiteConfig(name string) error
	SiteEnabled(name string) (bool, error)
	Reload(ctx context.Context) error
	Close() error
}
