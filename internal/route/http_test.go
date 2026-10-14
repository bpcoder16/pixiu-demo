package route_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pixiu-demo/internal/route"

	"github.com/bpcoder16/pixiu/infra/ginx"
)

func TestHTTPRouteStatus(t *testing.T) {
	handler, err := ginx.New(ginx.Config{DisableAccessLog: true})
	if err != nil {
		t.Fatal(err)
	}
	route.Register(handler)
	for _, tc := range []struct {
		method string
		path   string
		status int
		allow  string
	}{
		{http.MethodPost, "/api/app/ping", http.StatusMethodNotAllowed, "GET"},
		{http.MethodGet, "/api/app/oss/upload", http.StatusMethodNotAllowed, "POST"},
		{http.MethodGet, "/missing", http.StatusNotFound, ""},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			writer := httptest.NewRecorder()
			handler.ServeHTTP(writer, httptest.NewRequest(tc.method, tc.path, nil))
			if writer.Code != tc.status || writer.Header().Get("Allow") != tc.allow {
				t.Fatalf("路由响应错误: status=%d, Allow=%q", writer.Code, writer.Header().Get("Allow"))
			}
			if writer.Header().Get(ginx.TraceHeader) == "" {
				t.Fatal("未匹配请求均应经过 ginx 请求作用域")
			}
		})
	}
}
