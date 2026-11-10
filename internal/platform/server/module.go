package server

import "go.uber.org/fx"

var Module = fx.Module(
	"server",
	fx.Provide(NewEngine, NewHTTPServer),
	fx.Invoke(Mount, func(*HTTPServer) {}),
)
