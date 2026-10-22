package httphandler

import (
	"strings"

	"pixiu-demo/errorcode"
	"pixiu-demo/internal/model/request"
	natsservice "pixiu-demo/internal/service/nats"

	"github.com/gin-gonic/gin"
)

// AppNATSPublish 通过默认 NATS 客户端发布一组固定示例消息。
//
// @Summary      发布 NATS 示例消息
// @Description  向 Core 广播、Queue 及 JetStream 同步、异步各发布一条。等待两条 JetStream PubAck，不等待消费完成；同一批重试复用 messageIDPrefix，Core 不去重。失败可能已有部分消息发出，不回滚或自动重试
// @x-module-app true
// @Tags         NATS
// @Accept       json
// @Produce      json
// @Param        request body request.AppNATSPublishRequest true "本批消息 ID 前缀"
// @Success      200 {object} Response "code=0 表示发布成功；code=10001 表示参数错误，code=400 表示发布失败；响应不含 data"
// @Router       /api/app/nats/publish [post]
func AppNATSPublish(ctx *gin.Context) {
	var req request.AppNATSPublishRequest
	if err := paramsValidator(ctx, &req); err != nil {
		return
	}
	if strings.TrimSpace(req.MessageIDPrefix) == "" {
		returnErrJson(ctx, errorcode.ErrParams, "messageIDPrefix 不能为空白")
		return
	}
	if err := natsservice.NewDefaultService().PublishExamples(ctx.Request.Context(), req.MessageIDPrefix); err != nil {
		returnErrJson(ctx, errorcode.ErrServiceException)
		return
	}
	returnSuccessJson(ctx, nil)
}
