package app

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/cng1985/dbgo/internal/platform/buildinfo"
	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/logging"
)

func Run() {
	locator, err := config.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}
	if locator.ShowVersion {
		fmt.Printf("dbgo %s commit %s built %s\n", buildinfo.Version, buildinfo.Commit, buildinfo.Time)
		return
	}
	cfg, err := config.Load(locator)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	application := fx.New(
		fx.Supply(cfg),
		fx.StartTimeout(60*time.Second),
		fx.StopTimeout(cfg.Server.ShutdownTimeout),
		fx.WithLogger(func(log *logging.Logger) fxevent.Logger {
			return &fxevent.SlogLogger{Logger: log.Logger}
		}),
		Module,
	)
	application.Run()
}
