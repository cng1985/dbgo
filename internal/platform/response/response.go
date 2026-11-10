package response

import "github.com/gin-gonic/gin"

type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data,omitempty"`
}

type PageResponse[T any] struct {
	Data     []T   `json:"data"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func OK[T any](data T) Response[T] {
	return Response[T]{Code: 200, Msg: "success", Data: data}
}

func SuccessResponse[T any](c *gin.Context, data Response[T]) {
	c.JSON(200, data)
}

func SuccessPage[T any](c *gin.Context, data PageResponse[T]) {
	if data.Data == nil {
		data.Data = []T{}
	}
	c.JSON(200, data)
}
