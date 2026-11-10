package migration

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/cng1985/dbgo/internal/platform/response"
	"github.com/cng1985/dbgo/internal/platform/routing"
)

type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

func (c *Controller) Register(groups routing.Groups) {
	groups.Private.POST("/migration/create", c.create)
	groups.Private.POST("/migration/update", c.update)
	groups.Private.POST("/migration/one", c.one)
	groups.Private.POST("/migration/search", c.search)
	groups.Private.POST("/migration/delete", c.delete)
	groups.Long.POST("/migration/apply", c.apply)
	groups.Long.POST("/migration/rollback", c.rollback)
	groups.Private.POST("/migration/resolve", c.resolve)
}

func (c *Controller) create(ctx *gin.Context) {
	var request MigrationRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Create(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) update(ctx *gin.Context) {
	var request MigrationRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Update(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) one(ctx *gin.Context) {
	var request MigrationRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.One(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) search(ctx *gin.Context) {
	var search MigrationSearch
	if err := response.BindJSON(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Search(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func (c *Controller) delete(ctx *gin.Context) {
	var request MigrationRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Delete(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) apply(ctx *gin.Context) {
	c.action(ctx, c.resource.Apply)
}

func (c *Controller) rollback(ctx *gin.Context) {
	c.action(ctx, c.resource.Rollback)
}

func (c *Controller) resolve(ctx *gin.Context) {
	c.action(ctx, c.resource.Resolve)
}

func (c *Controller) action(ctx *gin.Context, run func(context.Context, *MigrationActionRequest) (response.Response[MigrationResponse], error)) {
	var request MigrationActionRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := run(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}
