package update

import (
	"context"
	"errors"
)

var (
	ErrLocked       = errors.New("update lock held")
	ErrNotScript    = errors.New("update_script not configured")
	ErrNoPackage    = errors.New("no package found in incoming dir")
	ErrIntegrity    = errors.New("package integrity check failed")
	ErrScriptFailed = errors.New("update script failed")
	ErrHealthCheck  = errors.New("health check failed")
	ErrRolledBack   = errors.New("update failed, rolled back")
)

type Service interface {
	Update(ctx context.Context, name string, version string) error
}

type RestartFunc func(ctx context.Context, name string) error
