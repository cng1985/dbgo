package routing

import "github.com/gin-gonic/gin"

// Groups 区分公开路由、普通认证路由和允许更长超时的认证路由。
type Groups struct {
	Public  *gin.RouterGroup
	Private *gin.RouterGroup
	Long    *gin.RouterGroup
}

type Registrar interface {
	Register(groups Groups)
}
