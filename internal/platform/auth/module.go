package auth

import (
	"go.uber.org/fx"

	"github.com/cng1985/dbgo/internal/platform/middleware"
	"github.com/cng1985/dbgo/internal/platform/routing"
)

func asRegistrar(controller *Controller) routing.Registrar { return controller }

func asAuthenticator(resource *Resource) middleware.Authenticator { return resource }

var Module = fx.Module(
	"auth",
	fx.Provide(NewRepository, NewResource, NewController),
	fx.Provide(fx.Annotate(asRegistrar, fx.ResultTags(`group:"registrars"`))),
	fx.Provide(asAuthenticator),
)
