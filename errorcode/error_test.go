package errorcode_test

import (
	"testing"

	"pixiu-demo/errorcode"
)

func TestCodeContract(t *testing.T) {
	// 期望值使用协议字面量，避免常量变化时生产响应和测试期望一起变化。
	tests := []struct {
		name     string
		code     int
		wantCode int
		wantMsg  string
	}{
		{"Success", errorcode.Success, 0, "成功"},
		{"ErrServiceException", errorcode.ErrServiceException, 400, "服务异常"},
		{"ErrParams", errorcode.ErrParams, 10001, "参数错误"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.wantCode {
				t.Errorf("code = %d, want %d", tt.code, tt.wantCode)
			}
			if got := errorcode.CodeMsg(tt.wantCode); got != tt.wantMsg {
				t.Errorf("CodeMsg(%d) = %q, want %q", tt.wantCode, got, tt.wantMsg)
			}
		})
	}
}

func TestCodeMsgUnknown(t *testing.T) {
	if got := errorcode.CodeMsg(-1); got != "未知错误" {
		t.Errorf("CodeMsg(-1) = %q, want %q", got, "未知错误")
	}
}
