package cli

import (
	"context"

	"github.com/FW-Systeme/Virgil/internal/cron"
	"github.com/FW-Systeme/Virgil/internal/process"
)

type ctxKey string

const (
	pmKey   ctxKey = "pm"
	cronKey ctxKey = "cron"
)

func pmCtx(ctx context.Context, pm *process.Manager) context.Context {
	return context.WithValue(ctx, pmKey, pm)
}

func pmFromCtx(ctx context.Context) (*process.Manager, bool) {
	pm, ok := ctx.Value(pmKey).(*process.Manager)
	return pm, ok
}

type cronContext struct {
	store  cron.Store
	client cron.Client
}

func cronCtx(ctx context.Context, s cron.Store, c cron.Client) context.Context {
	return context.WithValue(ctx, cronKey, cronContext{store: s, client: c})
}

func cronFromCtx(ctx context.Context) (cron.Store, cron.Client, bool) {
	cc, ok := ctx.Value(cronKey).(cronContext)
	if !ok {
		return nil, nil, false
	}
	return cc.store, cc.client, true
}
