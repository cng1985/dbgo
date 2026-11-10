package connection

import (
	"go.uber.org/fx"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/routing"
)

func asRegistrar(controller *Controller) routing.Registrar { return controller }

func asOpener(opener *TargetOpener) shared.Opener { return opener }

var Module = fx.Module(
	"connection",
	fx.Provide(NewRepository, NewResource, NewController, NewTargetOpener),
	fx.Provide(asOpener),
	fx.Provide(fx.Annotate(asRegistrar, fx.ResultTags(`group:"registrars"`))),
)
