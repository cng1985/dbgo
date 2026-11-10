package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cng1985/dbgo/internal/platform/buildinfo"
	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/domain"
	"github.com/cng1985/dbgo/internal/platform/logging"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type Authenticator interface {
	Authenticate(ctx context.Context, token string) (domain.Scope, error)
}

type Chain struct {
	cfg        *config.Config
	log        *logging.Logger
	authn      Authenticator
	apiLimit   *Limiter
	loginLimit *Limiter
}

func New(cfg *config.Config, log *logging.Logger, authn Authenticator) *Chain {
	return &Chain{
		cfg:        cfg,
		log:        log,
		authn:      authn,
		apiLimit:   NewLimiter(cfg.Server.RatePerMinute),
		loginLimit: NewLimiter(cfg.Auth.LoginRatePerMinute),
	}
}

func (m *Chain) Install(engine *gin.Engine) {
	engine.Use(m.requestID, m.recover, m.security, m.access)
}

func (m *Chain) GuardAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, m.cfg.Server.MaxBodyBytes)
		if !m.sameOrigin(c) {
			response.HandleError(c, responseForbidden())
			return
		}
		if !strings.EqualFold(c.GetHeader("X-DBGo-Request"), "1") {
			response.HandleError(c, responseForbiddenHeader())
			return
		}
		if err := m.apiLimit.Allow("api:" + c.ClientIP()); err != nil {
			response.HandleError(c, err)
			return
		}
		if c.Request.URL.Path == "/api/auth/login" {
			if err := m.loginLimit.Allow("login:" + c.ClientIP()); err != nil {
				response.HandleError(c, err)
				return
			}
		}
		c.Next()
	}
}

func (m *Chain) Authenticate(c *gin.Context) {
	token, err := c.Cookie("dbgo_session")
	if err != nil || token == "" {
		response.HandleError(c, unauth())
		return
	}
	scope, err := m.authn.Authenticate(c.Request.Context(), token)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	scope.RequestID = domain.RequestID(c.Request.Context())
	ctx := domain.WithScope(c.Request.Context(), scope)
	ctx = domain.WithSessionToken(ctx, token)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func (m *Chain) Timeout(long bool) gin.HandlerFunc {
	limit := m.cfg.Server.RequestTimeout
	if long {
		limit = m.cfg.Server.LongRequestTimeout
	}
	return func(c *gin.Context) {
		ctx, cancel := contextWithTimeout(c, limit)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (m *Chain) requestID(c *gin.Context) {
	id := newID()
	ctx := domain.WithRequestID(c.Request.Context(), id)
	c.Request = c.Request.WithContext(ctx)
	c.Set("logger", m.log.Logger)
	c.Header("X-Request-Id", id)
	c.Next()
}

func (m *Chain) recover(c *gin.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			m.log.Error("请求处理发生 panic",
				"request_id", domain.RequestID(c.Request.Context()),
				"route", c.FullPath(),
				"panic", rec,
				"stack", string(debug.Stack()),
			)
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				response.JSONStatus(c, http.StatusInternalServerError, 500, "内部错误")
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}()
	c.Next()
}

func (m *Chain) security(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	c.Header("X-DBGo-Version", buildinfo.Version)
	c.Next()
}

func (m *Chain) access(c *gin.Context) {
	start := time.Now()
	c.Next()
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	scope, _ := domain.ScopeFrom(c.Request.Context())
	m.log.Info("request",
		"request_id", domain.RequestID(c.Request.Context()),
		"actor_id", scope.ActorID,
		"method", c.Request.Method,
		"route", route,
		"status", c.Writer.Status(),
		"latency_ms", time.Since(start).Milliseconds(),
	)
}

func (m *Chain) sameOrigin(c *gin.Context) bool {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, c.Request.Host)
}

func newID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(raw)
}
