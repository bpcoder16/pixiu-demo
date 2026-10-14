package route

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pixiu-demo/errorcode"

	"github.com/bpcoder16/pixiu/infra/ginx"
	"github.com/gin-gonic/gin"
)

func TestPublicPingRoutes(t *testing.T) {
	router := newAPITestRouter(t)
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/api/app/ping", "hello world"},
		{"/api/app/ping?name=demo", "hello demo"},
		{"/api/admin/ping", "hello admin"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			writer := httptest.NewRecorder()
			// 公开样例无需任何身份或外部服务初始化。
			router.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, tc.path, nil))
			var body struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
				Data struct {
					Message string `json:"message"`
				} `json:"data"`
			}
			if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if writer.Code != http.StatusOK || body.Code != errorcode.Success || body.Msg != "成功" || body.Data.Message != tc.want {
				t.Fatalf("Ping 响应错误: %d %s", writer.Code, writer.Body.String())
			}
			if writer.Header().Get(ginx.TraceHeader) == "" {
				t.Fatal("请求应经过 ginx 日志作用域")
			}
		})
	}
}

func TestAppPingNameValidation(t *testing.T) {
	router := newAPITestRouter(t)
	for _, tc := range []struct {
		name string
		code int
	}{
		{strings.Repeat("a", 32), errorcode.Success},
		{strings.Repeat("a", 33), errorcode.ErrParams},
	} {
		writer := httptest.NewRecorder()
		router.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/api/app/ping?name="+tc.name, nil))
		var body map[string]json.RawMessage
		if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		var code int
		if err := json.Unmarshal(body["code"], &code); err != nil {
			t.Fatal(err)
		}
		if writer.Code != http.StatusOK || code != tc.code {
			t.Fatalf("长度 %d 的参数响应错误: %d %s", len(tc.name), writer.Code, writer.Body.String())
		}
		if code != errorcode.Success {
			if _, exists := body["data"]; exists {
				t.Fatal("错误响应不应包含 data")
			}
		}
	}
}

func newAPITestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router, err := ginx.New(ginx.Config{DisableAccessLog: true})
	if err != nil {
		t.Fatal(err)
	}
	Register(router)
	return router
}
