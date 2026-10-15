package route

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bpcoder16/pixiu/infra/elasticsearchx"
	v8 "github.com/bpcoder16/pixiu/infra/elasticsearchx/v8"
	"github.com/bpcoder16/pixiu/logit"
)

func TestAppElasticsearchValidateSynonyms(t *testing.T) {
	// 默认客户端注册表关闭不可逆，使用独立进程隔离完整路由测试。
	const marker = "PIXIU_DEMO_ES_VALIDATE_TEST"
	if os.Getenv(marker) != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAppElasticsearchValidateSynonyms$")
		cmd.Env = append(os.Environ(), marker+"="+t.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ES 同义词验证独立进程测试失败: %v\n%s", err, output)
		}
		return
	}

	var logs bytes.Buffer
	logger := logit.MustNew(
		logit.OptEncoder(logit.DefaultJSONEncoder),
		logit.OptWriter(logit.NewWriter(&logs)),
	)
	previous := logit.Default()
	logit.SetDefault(logger)
	t.Cleanup(func() {
		logit.SetDefault(previous)
		_ = logit.Close(logger)
	})

	const synonymsPath = " analysis/同义词.txt "
	var mode atomic.Int32
	var mu sync.Mutex
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/" {
			_, _ = io.WriteString(w, `{"version":{"number":"8.17.0","build_flavor":"default"}}`)
			return
		}
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if !strings.HasPrefix(r.URL.Path, "/pixiu-synonym-validation-") {
			t.Errorf("错误的临时索引路径: %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodPut:
			var definition struct {
				Settings struct {
					Analysis struct {
						Filter map[string]struct {
							SynonymsPath string `json:"synonyms_path"`
						} `json:"filter"`
					} `json:"analysis"`
				} `json:"settings"`
			}
			if err := json.NewDecoder(r.Body).Decode(&definition); err != nil {
				t.Error(err)
			}
			if got := definition.Settings.Analysis.Filter["pixiu_validation_synonyms"].SynonymsPath; got != synonymsPath {
				t.Errorf("节点词库路径未按原值传递: %q", got)
			}
			if mode.Load() == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"type":"illegal_argument_exception","reason":"private-synonym-error"},"status":400}`)
				return
			}
			fmt.Fprintf(w, `{"acknowledged":true,"shards_acknowledged":true,"index":%q}`, strings.TrimPrefix(r.URL.Path, "/"))
		case http.MethodDelete:
			if mode.Load() == 2 {
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `{"error":{"type":"security_exception","reason":"private-cleanup-error"},"status":403}`)
				return
			}
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		default:
			t.Errorf("意外的 ES 调用: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	if _, err := v8.NewDefault(elasticsearchx.Config{
		Name:      "demo",
		Addresses: []string{server.URL},
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := elasticsearchx.CloseAll(); err != nil {
			t.Error(err)
		}
	})
	router := newAPITestRouter(t)
	validBody, err := json.Marshal(map[string]string{"synonymsPath": synonymsPath})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		body   string
		mode   int32
		cancel bool
		code   int
		calls  int
	}{
		{name: "节点词库路径原样透传并清理索引", body: string(validBody), calls: 2},
		{name: "空白路径不调用 ES", body: `{"synonymsPath":" \t "}`, code: 10001},
		{name: "含空字符路径不调用 ES", body: `{"synonymsPath":"analysis/\u0000.txt"}`, code: 10001},
		{name: "创建失败不重试或自动删除", body: string(validBody), mode: 1, code: 400, calls: 1},
		{name: "验证通过但清理失败仍返回错误", body: string(validBody), mode: 2, code: 400, calls: 2},
		{name: "请求取消传递到 ES", body: string(validBody), cancel: true, code: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode.Store(tc.mode)
			logs.Reset()
			mu.Lock()
			calls = nil
			mu.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			req := httptest.NewRequest(http.MethodPost, "/api/app/elasticsearch/synonyms-validate", strings.NewReader(tc.body)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			writer := httptest.NewRecorder()
			router.ServeHTTP(writer, req)
			var got struct {
				Code int             `json:"code"`
				Msg  string          `json:"msg"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(writer.Body.Bytes(), &got); err != nil {
				t.Fatalf("响应不是 JSON: %v, body=%s", err, writer.Body.String())
			}
			if writer.Code != http.StatusOK || got.Code != tc.code {
				t.Fatalf("响应协议错误: status=%d, body=%s", writer.Code, writer.Body.String())
			}
			mu.Lock()
			observed := append([]string(nil), calls...)
			mu.Unlock()
			if len(observed) != tc.calls {
				t.Fatalf("下游调用次数错误: got=%v, want=%d", observed, tc.calls)
			}
			if tc.calls == 2 && observed[1] != "DELETE "+strings.TrimPrefix(observed[0], "PUT ") {
				t.Fatalf("未清理本次创建的索引: %v", observed)
			}
			if tc.code != 0 {
				if got.Data != nil {
					t.Fatalf("失败应省略 data: %s", writer.Body.String())
				}
				if tc.code == 400 && got.Msg != "服务异常" {
					t.Fatalf("失败不应泄露底层错误: %s", writer.Body.String())
				}
			} else {
				index := strings.TrimPrefix(observed[0], "PUT /")
				want := fmt.Sprintf(`{"index":%q,"valid":true,"stage":"complete","cleanup":"succeeded","mayExist":false}`, index)
				var data, expected any
				if err := json.Unmarshal(got.Data, &data); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(want), &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(data, expected) {
					t.Fatalf("验证结果错误: got=%s, want=%s", got.Data, want)
				}
			}
			if tc.mode != 0 {
				var record map[string]any
				if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &record); err != nil {
					t.Fatalf("失败应记录验证与清理结果: %v, logs=%s", err, logs.String())
				}
				stage, cleanup := "create", "not_attempted"
				if tc.mode == 2 {
					stage, cleanup = "complete", "failed"
				}
				if record["level"] != "ERROR" || record["index"] != strings.TrimPrefix(observed[0], "PUT /") || record["stage"] != stage || record["cleanup"] != cleanup || record["mayExist"] != true || record["valid"] != (tc.mode == 2) || record["err"] == nil {
					t.Fatalf("失败日志缺少遗留索引排查信息: %s", logs.String())
				}
			}
		})
	}
}
