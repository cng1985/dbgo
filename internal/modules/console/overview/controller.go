package overview

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
	groups.Private.POST("/overview/one", c.one)
}

func (c *Controller) one(ctx *gin.Context) {
	result, err := c.resource.One(ctx.Request.Context(), &OverviewSearch{})
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}
