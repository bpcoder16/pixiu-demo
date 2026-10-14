package route

import "github.com/gin-gonic/gin"

// Register 为已构造的 Engine 注册路由；本函数不读取配置。
func Register(r *gin.Engine) {
	// 已登记路径的方法不匹配时返回 405。
	r.HandleMethodNotAllowed = true
	api := r.Group("/api")
	registerAppRoutesV2(api.Group("/app"))
	registerAdminRoutesV2(api.Group("/admin"))
}
