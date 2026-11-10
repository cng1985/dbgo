package catalog

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
	groups.Private.POST("/catalog/schemas", c.schemas)
	groups.Private.POST("/catalog/tables", c.tables)
	groups.Private.POST("/catalog/columns", c.columns)
	groups.Private.POST("/catalog/indexes", c.indexes)
	groups.Private.POST("/catalog/suggest", c.suggest)
}

func (c *Controller) schemas(ctx *gin.Context) {
	c.call(ctx, c.resource.Schemas)
}

func (c *Controller) tables(ctx *gin.Context) {
	c.call(ctx, c.resource.Tables)
}

func (c *Controller) columns(ctx *gin.Context) {
	c.call(ctx, c.resource.Columns)
}

func (c *Controller) indexes(ctx *gin.Context) {
	c.call(ctx, c.resource.Indexes)
}

func (c *Controller) suggest(ctx *gin.Context) {
	var search CatalogSearch
	if err := response.BindJSON(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Suggest(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) call(ctx *gin.Context, action func(context.Context, *CatalogSearch) (response.Response[CatalogResponse], error)) {
	var search CatalogSearch
	if err := response.BindJSON(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := action(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}
