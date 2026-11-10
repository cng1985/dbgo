package app

import (
	"context"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/platform/auth"
	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/database"
	"github.com/cng1985/dbgo/internal/platform/logging"
)

func Bootstrap(cfg *config.Config, handle *database.Handle, accounts *auth.Resource, log *logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, handle, log); err != nil {
		return err
	}
	if _, err := accounts.EnsureAdmin(ctx, &auth.AuthEnsureRequest{
		Username: cfg.Auth.AdminUsername,
		Password: cfg.Auth.AdminPassword,
	}); err != nil {
		return err
	}
	handle.MarkReady()
	log.Info("应用已完成启动准备",
		"profile", cfg.App.Profile,
		"driver", cfg.Database.Driver,
		"data_dir", cfg.App.DataDir,
		"sqlite", cfg.SQLiteFile(),
		"sources", strings.Join(cfg.Sources, ","),
	)
	return nil
}
