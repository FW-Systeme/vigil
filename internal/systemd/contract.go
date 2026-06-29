package systemd

import "context"

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
}
