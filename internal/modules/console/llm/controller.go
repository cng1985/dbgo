package llm

import (
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
	groups.Private.POST("/llm/create", c.create)
	groups.Private.POST("/llm/update", c.update)
	groups.Private.POST("/llm/one", c.one)
	groups.Private.POST("/llm/search", c.search)
	groups.Private.POST("/llm/delete", c.delete)
	groups.Long.POST("/llm/test", c.test)
	groups.Long.POST("/llm/optimize", c.optimize)
	groups.Long.POST("/llm/design-table", c.designTable)
}

func (c *Controller) create(ctx *gin.Context) {
	var request ProfileRequest
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
	var request ProfileRequest
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
	var request ProfileRequest
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
	var search ProfileSearch
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
	var request ProfileDeleteRequest
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

func (c *Controller) test(ctx *gin.Context) {
	var request ProfileRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Test(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) optimize(ctx *gin.Context) {
	var request OptimizeRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Optimize(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) designTable(ctx *gin.Context) {
	var request DesignTableRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.DesignTable(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}
