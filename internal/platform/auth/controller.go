package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/response"
	"github.com/cng1985/dbgo/internal/platform/routing"
)

const sessionCookie = "dbgo_session"

type Controller struct {
	resource *Resource
	secure   bool
	ttl      time.Duration
}

func NewController(resource *Resource, cfg *config.Config) *Controller {
	return &Controller{resource: resource, secure: cfg.Auth.CookieSecure, ttl: cfg.Auth.SessionTTL}
}

func (c *Controller) Register(groups routing.Groups) {
	groups.Public.POST("/auth/login", c.login)
	groups.Public.POST("/auth/register", c.register)
	groups.Public.POST("/auth/options", c.options)
	groups.Private.POST("/auth/logout", c.logout)
	groups.Private.POST("/auth/profile", c.profile)
	groups.Private.POST("/auth/profile/update", c.updateProfile)
	groups.Private.POST("/auth/password", c.password)
}

func (c *Controller) login(ctx *gin.Context) {
	var request AuthLoginRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Login(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	c.writeCookie(ctx, result.Data.SessionToken, int(c.ttl.Seconds()))
	result.Data.SessionToken = ""
	response.SuccessResponse(ctx, result)
}

func (c *Controller) register(ctx *gin.Context) {
	var request AuthRegisterRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Register(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	c.writeCookie(ctx, result.Data.SessionToken, int(c.ttl.Seconds()))
	result.Data.SessionToken = ""
	response.SuccessResponse(ctx, result)
}

func (c *Controller) options(ctx *gin.Context) {
	result, err := c.resource.Options(ctx.Request.Context(), &AuthProfileRequest{})
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) logout(ctx *gin.Context) {
	result, err := c.resource.Logout(ctx.Request.Context(), &AuthLogoutRequest{})
	c.writeCookie(ctx, "", -1)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) profile(ctx *gin.Context) {
	result, err := c.resource.Profile(ctx.Request.Context(), &AuthProfileRequest{})
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) updateProfile(ctx *gin.Context) {
	var request AuthProfileUpdateRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.UpdateProfile(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) password(ctx *gin.Context) {
	var request AuthPasswordRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.ChangePassword(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	c.writeCookie(ctx, "", -1)
	response.SuccessResponse(ctx, result)
}

func (c *Controller) writeCookie(ctx *gin.Context, value string, maxAge int) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}
