package secret

import "go.uber.org/fx"

var Module = fx.Module("secret", fx.Provide(Load))
