package route

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestAppElasticsearchReloadSynonyms(t *testing.T) {
	// 默认客户端注册表关闭不可逆，使用独立进程隔离完整路由测试。
	const marker = "PIXIU_DEMO_ES_RELOAD_TEST"
	if os.Getenv(marker) != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAppElasticsearchReloadSynonyms$")
		cmd.Env = append(os.Environ(), marker+"="+t.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ES 同义词重载独立进程测试失败: %v\n%s", err, output)
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

	const reloadCall = "POST /products/_reload_search_analyzers"
	const cacheCall = "POST /products/_cache/clear?request=true"
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
		call := r.Method + " " + r.URL.RequestURI()
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) != 0 {
			t.Errorf("重载与清缓存请求应无正文: body=%s err=%v", body, err)
		}
		switch call {
		case reloadCall:
			if mode.Load() == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"type":"illegal_argument_exception","reason":"private-reload-error"}}`)
				return
			}
			details := `[{"index":"products","reloaded_analyzers":["search_synonyms"],"reloaded_node_ids":["node-a"]}]`
			if mode.Load() == 3 {
				details = `[]`
			}
			_, _ = io.WriteString(w, `{"_shards":{"total":1,"successful":1,"failed":0},"reload_details":`+details+`}`)
		case cacheCall:
			if mode.Load() == 2 {
				_, _ = io.WriteString(w, `{"_shards":{"total":2,"successful":1,"failed":1,"failures":[{"reason":"private-cache-error"}]}}`)
				return
			}
			_, _ = io.WriteString(w, `{"_shards":{"total":2,"successful":2,"failed":0}}`)
		default:
			t.Errorf("意外的 ES 调用: %s", call)
			w.WriteHeader(http.StatusNotFound)
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

	for _, tc := range []struct {
		name   string
		body   string
		mode   int32
		cancel bool
		code   int
		calls  []string
	}{
		{name: "先重载再仅清理目标索引的请求缓存", body: `{"index":"products"}`, calls: []string{reloadCall, cacheCall}},
		{name: "未重载分析器时如实返回空详情", body: `{"index":"products"}`, mode: 3, calls: []string{reloadCall, cacheCall}},
		{name: "重载失败不清缓存或重试", body: `{"index":"products"}`, mode: 1, code: 400, calls: []string{reloadCall}},
		{name: "清缓存分片失败保留重载诊断", body: `{"index":"products"}`, mode: 2, code: 400, calls: []string{reloadCall, cacheCall}},
		{name: "取消传递到下游", body: `{"index":"products"}`, cancel: true, code: 400},
		{name: "缺少索引", body: `{}`, code: 10001},
		{name: "空白索引", body: `{"index":" \t "}`, code: 10001},
		{name: "通配符不得扩大操作范围", body: `{"index":"products*"}`, code: 10001},
		{name: "列表不得扩大操作范围", body: `{"index":"products,orders"}`, code: 10001},
		{name: "全部索引选择器", body: `{"index":"_all"}`, code: 10001},
		{name: "索引名按字节限制长度", body: `{"index":"` + strings.Repeat("商", 86) + `"}`, code: 10001},
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
			req := httptest.NewRequest(http.MethodPost, "/api/app/elasticsearch/synonyms-reload", strings.NewReader(tc.body)).WithContext(ctx)
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
			if !reflect.DeepEqual(observed, tc.calls) {
				t.Fatalf("下游调用顺序或范围错误: got=%v, want=%v", observed, tc.calls)
			}
			if tc.code != 0 {
				if got.Data != nil || tc.code == 400 && got.Msg != "服务异常" {
					t.Fatalf("失败应省略 data 且不泄露下游详情: %s", writer.Body.String())
				}
			} else {
				details := `[{"index":"products","reloadedAnalyzers":["search_synonyms"],"reloadedNodeIds":["node-a"]}]`
				if tc.mode == 3 {
					details = `[]`
				}
				want := `{"shards":{"total":1,"successful":1,"failed":0},"reloadDetails":` + details + `,"cacheShards":{"total":2,"successful":2,"failed":0}}`
				var data, expected any
				if err := json.Unmarshal(got.Data, &data); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(want), &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(data, expected) {
					t.Fatalf("重载结果错误: got=%s, want=%s", got.Data, want)
				}
			}
			if tc.mode == 2 {
				var record struct {
					Index         string                        `json:"index"`
					ReloadDetails []elasticsearchx.ReloadDetail `json:"reloadDetails"`
					CacheShards   *elasticsearchx.ShardsInfo    `json:"cacheShards"`
				}
				if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &record); err != nil {
					t.Fatalf("失败应记录已完成阶段: %v, logs=%s", err, logs.String())
				}
				if record.Index != "products" || len(record.ReloadDetails) != 1 || record.ReloadDetails[0].ReloadedAnalyzers[0] != "search_synonyms" || record.CacheShards == nil || record.CacheShards.Failed != 1 {
					t.Fatalf("清缓存失败丢失重载诊断: %s", logs.String())
				}
			}
		})
	}
}
