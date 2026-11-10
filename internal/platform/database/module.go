package database

import (
	"context"

	"go.uber.org/fx"

	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/logging"
)

func NewHandle(lc fx.Lifecycle, cfg *config.Config, log *logging.Logger) (*Handle, error) {
	handle, err := Open(cfg, log)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			handle.MarkNotReady()
			return handle.Close()
		},
	})
	return handle, nil
}

var Module = fx.Module("database", fx.Provide(NewHandle))
