package request

type AppPingRequest struct {
	// 打招呼名称，不传或为空时使用 world
	Name string `form:"name" binding:"omitempty,max=32" default:"world" example:"world"`
}
