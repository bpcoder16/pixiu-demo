package httphandler

import (
	"net/http"
	"strings"

	"pixiu-demo/errorcode"

	"github.com/gin-gonic/gin"
)

type Response struct {
	// 业务状态码，0 表示成功，非 0 表示业务错误
	Code int `json:"code" example:"0"`
	// 响应消息，成功时为"成功"，失败时为错误原因
	Msg string `json:"msg" example:"成功"`
	// 响应数据，具体结构以接口 @Success 中的 data 类型为准
	Data any `json:"data,omitempty"`
}

func returnErrJson(ctx *gin.Context, code int, extraMsg ...string) {
	extraMsgStr := strings.Join(extraMsg, ";")
	ctx.JSON(http.StatusOK, Response{
		Code: code,
		Msg: func() string {
			if len(extraMsg) > 0 {
				return extraMsgStr
			}
			return errorcode.CodeMsg(code)
		}(),
	})
}

func returnSuccessJson(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Response{
		Code: errorcode.Success,
		Msg:  errorcode.CodeMsg(errorcode.Success),
		Data: data,
	})
}

func paramsValidator(ctx *gin.Context, paramsPtr any) (err error) {
	if err = ctx.ShouldBind(paramsPtr); err != nil {
		returnErrJson(ctx, errorcode.ErrParams, err.Error())
	}
	return
}
