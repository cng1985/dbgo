package app

import (
	"go.uber.org/fx"

	"github.com/cng1985/dbgo/internal/modules/console"
	"github.com/cng1985/dbgo/internal/platform/auth"
	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/database"
	"github.com/cng1985/dbgo/internal/platform/logging"
	"github.com/cng1985/dbgo/internal/platform/middleware"
	"github.com/cng1985/dbgo/internal/platform/secret"
	"github.com/cng1985/dbgo/internal/platform/server"
	"github.com/cng1985/dbgo/web"
)

var Module = fx.Options(
	config.Module,
	logging.Module,
	secret.Module,
	database.Module,
	auth.Module,
	middleware.Module,
	server.Module,
	console.Module,
	web.Module,
	fx.Invoke(Bootstrap),
)
