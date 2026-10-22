package httphandler

import (
	"pixiu-demo/errorcode"
	natsservice "pixiu-demo/internal/service/nats"

	"github.com/gin-gonic/gin"
)

// AppNATSPublish 通过默认 NATS 客户端发布一组固定示例消息。
//
// @Summary      发布 NATS 示例消息
// @Description  无需请求参数，Service 每次生成 UUID 批次前缀，向 Core 广播、Queue 及 JetStream 同步、异步各发布一条。等待两条 JetStream PubAck，不等待消费完成；重复调用会重新发布，Core 不去重。失败可能已有部分消息发出，不回滚或自动重试
// @x-module-app true
// @Tags         NATS
// @Produce      json
// @Success      200 {object} Response "code=0 表示发布成功，code=400 表示发布失败；响应不含 data"
// @Router       /api/app/nats/publish [post]
func AppNATSPublish(ctx *gin.Context) {
	if err := natsservice.NewService().PublishExamples(ctx.Request.Context()); err != nil {
		returnErrJson(ctx, errorcode.ErrServiceException)
		return
	}
	returnSuccessJson(ctx, nil)
}
