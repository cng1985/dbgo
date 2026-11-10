package backup

import (
	"net/http"
	"os"
	"strconv"

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
	groups.Long.POST("/backup/create", c.create)
	groups.Private.POST("/backup/one", c.one)
	groups.Private.POST("/backup/search", c.search)
	groups.Private.POST("/backup/delete", c.delete)
	groups.Long.POST("/backup/restore", c.restore)
	groups.Private.GET("/backup/download", c.download)
}

func (c *Controller) create(ctx *gin.Context) {
	var request BackupRequest
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

func (c *Controller) one(ctx *gin.Context) {
	var request BackupRequest
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
	var search BackupSearch
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
	var request BackupRequest
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

func (c *Controller) restore(ctx *gin.Context) {
	var request BackupRequest
	if err := response.BindJSON(ctx, &request); err != nil {
		response.HandleError(ctx, err)
		return
	}
	result, err := c.resource.Restore(ctx.Request.Context(), &request)
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	response.SuccessResponse(ctx, result)
}

func (c *Controller) download(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Query("id"), 10, 64)
	if err != nil || id <= 0 {
		response.HandleError(ctx, errInvalidID())
		return
	}
	result, err := c.resource.Download(ctx.Request.Context(), &BackupRequest{ID: id})
	if err != nil {
		response.HandleError(ctx, err)
		return
	}
	file, err := os.Open(result.Data.LocalPath)
	if err != nil {
		response.HandleError(ctx, errMissingFile())
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		response.HandleError(ctx, errMissingFile())
		return
	}
	ctx.Header("Content-Disposition", "attachment; filename=\""+sanitize(result.Data.FileName)+"\"")
	ctx.DataFromReader(http.StatusOK, info.Size(), "application/octet-stream", file, nil)
}

func errInvalidID() error { return invalid("备份主键无效") }

func errMissingFile() error { return missing("备份文件不存在") }

func sanitize(name string) string {
	out := make([]rune, 0, len(name))
	for _, ch := range name {
		if ch == '"' || ch == '\\' || ch == '\r' || ch == '\n' {
			continue
		}
		out = append(out, ch)
	}
	if len(out) == 0 {
		return "backup.sql"
	}
	return string(out)
}
