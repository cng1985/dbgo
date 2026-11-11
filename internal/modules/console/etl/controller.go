package etl

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
	groups.Private.POST("/etl/create", c.create)
	groups.Private.POST("/etl/update", c.update)
	groups.Private.POST("/etl/one", c.one)
	groups.Private.POST("/etl/search", c.search)
	groups.Private.POST("/etl/delete", c.delete)
	groups.Private.POST("/etl/runs", c.runs)
	groups.Private.POST("/etl/abort", c.abort)
	groups.Long.POST("/etl/preview", c.preview)
	groups.Long.POST("/etl/run", c.run)
}

func (c *Controller) create(ctx *gin.Context) {
	var request TaskRequest
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
	var request TaskRequest
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
	var request TaskActionRequest
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
	var search TaskSearch
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
	var request TaskActionRequest
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

func (c *Controller) runs(ctx *gin.Context) {
	var search RunSearch
	if err := response.BindJSON(ctx, &search); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Runs(ctx.Request.Context(), &search)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessPage(ctx, result)
}

func (c *Controller) preview(ctx *gin.Context) {
	var request PreviewRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Preview(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) run(ctx *gin.Context) {
	var request TaskActionRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Run(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) abort(ctx *gin.Context) {
	var request TaskActionRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Abort(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}
