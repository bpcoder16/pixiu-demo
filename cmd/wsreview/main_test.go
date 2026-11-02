package main

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pixiu-demo/internal/wshandler"
)

func TestReviewClient(t *testing.T) {
	server, err := wshandler.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Run() }()
	defer func() {
		_ = server.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Wait(ctx); err != nil {
			server.Cancel()
			t.Error(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-ctx.Done():
			t.Error("服务未退出")
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !server.Ready() {
		if time.Now().After(deadline) {
			t.Fatal("服务未就绪")
		}
		time.Sleep(time.Millisecond)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	for _, encoding := range []string{"json", "pb"} {
		t.Run(encoding, func(t *testing.T) {
			var output bytes.Buffer
			err := run("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/ws/app", encoding, "home", &output)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "connected: ") || !strings.Contains(output.String(), "user=demo-user device=demo-device page=home revision=1") {
				t.Fatalf("客户端未完成事件和响应解码: %s", &output)
			}
		})
	}
	var output bytes.Buffer
	if err := run("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/ws/app", "pb", "", &output); err == nil || !strings.Contains(err.Error(), "INVALID_ARGUMENT") {
		t.Fatalf("客户端应报告业务错误: %v", err)
	}
}
