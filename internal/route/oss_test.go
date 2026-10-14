package route

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"pixiu-demo/errorcode"

	"github.com/bpcoder16/pixiu/infra/aliyun/ossx"
)

func TestAppOSSUpload(t *testing.T) {
	// 默认 OSS 注册表的关闭不可逆，使用独立进程隔离真实客户端测试。
	const marker = "PIXIU_DEMO_OSS_TEST"
	if os.Getenv(marker) != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAppOSSUpload$")
		cmd.Env = append(os.Environ(), marker+"="+t.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("OSS 独立进程测试失败: %v\n%s", err, output)
		}
		return
	}

	const largeFileSize = 12 << 20
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPut {
			t.Errorf("上传应使用 PUT: %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("读取上传内容失败: %v", err)
		}
		switch r.URL.Path {
		case "/upload-bucket/uploads/demo.txt":
			if string(body) != "demo oss data\n" || r.Header.Get("Content-Type") != "text/plain" {
				t.Errorf("文件内容或类型未传递: size=%d, type=%q", len(body), r.Header.Get("Content-Type"))
			}
		case "/upload-bucket/uploads/large.bin":
			if len(body) != largeFileSize || bytes.Count(body, []byte{'x'}) != largeFileSize || r.Header.Get("Content-Type") != "application/octet-stream" {
				t.Errorf("较大文件内容或默认类型错误: size=%d, type=%q", len(body), r.Header.Get("Content-Type"))
			}
		case "/upload-bucket/uploads/failure.txt":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, "<Error><Code>AccessDenied</Code><Message>private-oss-error</Message><RequestId>failed-request</RequestId></Error>")
			return
		default:
			t.Errorf("上传路径错误: %s", r.URL.Path)
		}
		w.Header().Set("x-oss-request-id", "upload-request")
		w.Header().Set("ETag", `"upload-etag"`)
		w.Header().Set("x-oss-version-id", "upload-version")
	}))
	t.Cleanup(server.Close)
	if _, err := ossx.NewDefault(ossx.Config{
		Name:            "demo",
		Region:          "cn-hangzhou",
		Bucket:          "upload-bucket",
		AccessKeyID:     "test-access-key",
		AccessKeySecret: "test-secret",
		Endpoint:        server.URL,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ossx.CloseAll(); err != nil {
			t.Error(err)
		}
	})
	router := newAPITestRouter(t)

	for _, tc := range []struct {
		name        string
		key         string
		content     string
		contentType string
	}{
		{
			name:        "普通文件",
			key:         "uploads/demo.txt",
			content:     "demo oss data\n",
			contentType: "text/plain",
		},
		{
			name:    "较大文件及默认类型",
			key:     "uploads/large.bin",
			content: strings.Repeat("x", largeFileSize),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			t.Setenv("TMPDIR", tempDir)
			before := calls.Load()
			req := newOSSUploadRequest(t, tc.key, tc.content, tc.contentType, 1)
			writer := httptest.NewRecorder()
			router.ServeHTTP(writer, req)
			var body struct {
				Code int `json:"code"`
				Data struct {
					Key       string `json:"key"`
					ETag      string `json:"etag"`
					RequestID string `json:"requestId"`
					VersionID string `json:"versionId"`
				} `json:"data"`
			}
			if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if writer.Code != http.StatusOK || body.Code != errorcode.Success || body.Data.Key != tc.key || body.Data.ETag != `"upload-etag"` || body.Data.RequestID != "upload-request" || body.Data.VersionID != "upload-version" || calls.Load() != before+1 {
				t.Fatalf("上传结果错误: status=%d, body=%s, calls=%d", writer.Code, writer.Body.String(), calls.Load()-before)
			}
			assertUploadTempFilesRemoved(t, tempDir)
		})
	}

	for _, tc := range []struct {
		name    string
		key     string
		content string
		files   int
	}{
		{
			name: "缺少文件",
			key:  "uploads/demo.txt",
		},
		{
			name:    "缺少 key",
			content: "data",
			files:   1,
		},
		{
			name:    "空白 key",
			key:     " \t",
			content: "data",
			files:   1,
		},
		{
			name:    "多个文件",
			key:     "uploads/demo.txt",
			content: "data",
			files:   2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			t.Setenv("TMPDIR", tempDir)
			before := calls.Load()
			writer := httptest.NewRecorder()
			router.ServeHTTP(writer, newOSSUploadRequest(t, tc.key, tc.content, "", tc.files))
			assertOSSUploadError(t, writer, errorcode.ErrParams)
			if calls.Load() != before {
				t.Fatal("无效请求不应调用 OSS")
			}
			assertUploadTempFilesRemoved(t, tempDir)
		})
	}

	t.Run("非 multipart 请求", func(t *testing.T) {
		before := calls.Load()
		req := httptest.NewRequest(http.MethodPost, "/api/app/oss/upload", strings.NewReader(`{"key":"uploads/demo.txt"}`))
		req.Header.Set("Content-Type", "application/json")
		writer := httptest.NewRecorder()
		router.ServeHTTP(writer, req)
		assertOSSUploadError(t, writer, errorcode.ErrParams)
		if calls.Load() != before {
			t.Fatal("无效请求不应调用 OSS")
		}
	})

	t.Run("请求取消", func(t *testing.T) {
		before := calls.Load()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		req := newOSSUploadRequest(t, "uploads/demo.txt", "data", "", 1).WithContext(ctx)
		writer := httptest.NewRecorder()
		router.ServeHTTP(writer, req)
		assertOSSUploadError(t, writer, errorcode.ErrServiceException)
		if calls.Load() != before {
			t.Fatal("已取消的请求不应发起 OSS 网络调用")
		}
	})

	t.Run("OSS 失败", func(t *testing.T) {
		before := calls.Load()
		writer := httptest.NewRecorder()
		router.ServeHTTP(writer, newOSSUploadRequest(t, "uploads/failure.txt", "data", "", 1))
		assertOSSUploadError(t, writer, errorcode.ErrServiceException)
		if calls.Load() != before+1 {
			t.Fatal("OSS 失败不应自动重试")
		}
		if strings.Contains(writer.Body.String(), "private-oss-error") {
			t.Fatal("响应不应暴露底层 OSS 错误")
		}
	})
}

func newOSSUploadRequest(t *testing.T, key, content, contentType string, files int) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if key != "" {
		if err := form.WriteField("key", key); err != nil {
			t.Fatal(err)
		}
	}
	for range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="file"; filename="demo.txt"`)
		if contentType != "" {
			header.Set("Content-Type", contentType)
		}
		part, err := form.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(part, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/app/oss/upload", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	return req
}

func assertOSSUploadError(t *testing.T, writer *httptest.ResponseRecorder, wantCode int) {
	t.Helper()
	var body struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if writer.Code != http.StatusOK || body.Code != wantCode || body.Data != nil {
		t.Fatalf("错误响应不符合协议: status=%d, body=%s", writer.Code, writer.Body.String())
	}
}

func assertUploadTempFilesRemoved(t *testing.T, dir string) {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("上传临时文件未清理: %v", files)
	}
}
