package catalog

import (
	"go.uber.org/fx"

	"github.com/cng1985/dbgo/internal/platform/routing"
)

func asRegistrar(controller *Controller) routing.Registrar { return controller }

var Module = fx.Module(
	"catalog",
	fx.Provide(NewResource, NewController),
	fx.Provide(fx.Annotate(asRegistrar, fx.ResultTags(`group:"registrars"`))),
)
