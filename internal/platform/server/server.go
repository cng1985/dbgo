package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/database"
	"github.com/cng1985/dbgo/internal/platform/logging"
	"github.com/cng1985/dbgo/internal/platform/middleware"
	"github.com/cng1985/dbgo/internal/platform/response"
	"github.com/cng1985/dbgo/internal/platform/routing"
)

type HTTPServer struct {
	server *http.Server
	addr   string
	handle *database.Handle
	log    *logging.Logger
	stop   fx.Shutdowner
}

type routeIn struct {
	fx.In
	Engine     *gin.Engine
	Config     *config.Config
	Chain      *middleware.Chain
	Handle     *database.Handle
	Log        *logging.Logger
	Shutdown   fx.Shutdowner
	Registrars []routing.Registrar `group:"registrars"`
}

func NewEngine(cfg *config.Config) (*gin.Engine, error) {
	if cfg.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	proxies := cfg.Server.TrustedProxies
	if proxies == nil {
		proxies = []string{}
	}
	if err := engine.SetTrustedProxies(proxies); err != nil {
		return nil, err
	}
	engine.HandleMethodNotAllowed = true
	return engine, nil
}

func Mount(in routeIn) {
	in.Chain.Install(in.Engine)
	in.Engine.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "live"})
	})
	in.Engine.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		if err := in.Handle.Ready(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	guard := in.Chain.GuardAPI()
	public := in.Engine.Group("/api", guard, in.Chain.Timeout(false))
	private := in.Engine.Group("/api", guard, in.Chain.Timeout(false), in.Chain.Authenticate)
	long := in.Engine.Group("/api", guard, in.Chain.Timeout(true), in.Chain.Authenticate)
	groups := routing.Groups{Public: public, Private: private, Long: long}
	for _, registrar := range in.Registrars {
		registrar.Register(groups)
	}
	in.Engine.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			response.JSONStatus(c, http.StatusNotFound, 404, "接口不存在")
			return
		}
		response.JSONStatus(c, http.StatusNotFound, 404, "页面不存在")
	})
	in.Engine.NoMethod(func(c *gin.Context) {
		response.JSONStatus(c, http.StatusMethodNotAllowed, 405, "方法不被允许")
	})
}

func NewHTTPServer(lc fx.Lifecycle, engine *gin.Engine, cfg *config.Config, handle *database.Handle, log *logging.Logger, stop fx.Shutdowner) *HTTPServer {
	httpServer := &http.Server{
		Handler:           engine,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}
	srv := &HTTPServer{server: httpServer, handle: handle, log: log, stop: stop}
	lc.Append(fx.Hook{
		OnStart: srv.start(cfg),
		OnStop:  srv.stopServer(cfg),
	})
	return srv
}

func (s *HTTPServer) Addr() string { return s.addr }

func (s *HTTPServer) start(cfg *config.Config) func(context.Context) error {
	return func(context.Context) error {
		address := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return err
		}
		s.addr = listener.Addr().String()
		s.log.Info("http 开始监听", "addr", s.addr, "profile", cfg.App.Profile, "data_dir", cfg.App.DataDir)
		go func() {
			err := s.server.Serve(listener)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.log.Error("http 服务异常退出", "error", err.Error())
				if s.stop != nil {
					_ = s.stop.Shutdown()
				}
			}
		}()
		return nil
	}
}

func (s *HTTPServer) stopServer(cfg *config.Config) func(context.Context) error {
	return func(ctx context.Context) error {
		if s.handle != nil {
			s.handle.MarkNotReady()
		}
		shutdownCtx, cancel := context.WithTimeout(ctx, cfg.Server.ShutdownTimeout)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	}
}
