.DEFAULT_GOAL := all

# 路径与构建选项；在项目根目录执行。
BINARY_NAME ?= pixiu-demo
HOMEDIR := $(CURDIR)
OUTDIR := $(HOMEDIR)/output
GO ?= go
GENERATE_SWAGGER ?= 0

.PHONY: all set-env prepare gomod compile build swagger
.PHONY: test test-case test-script package package-bin

all: package

# 设置当前用户的 Go 默认环境，必须在下载依赖前完成。
set-env:
	$(GO) env -w GO111MODULE=on
	$(GO) env -w GOPROXY=https://goproxy.cn,direct

# 下载依赖，保留 go.mod 中的 ../pixiu 本地依赖。
prepare: gomod

gomod: set-env
	$(GO) mod download

compile: build

# 下载完成后编译；需要文档时，先生成再编译。
build: prepare

ifeq ($(GENERATE_SWAGGER),1)
build: swagger
endif

build:
	$(GO) version
	$(GO) build -o "$(HOMEDIR)/$(BINARY_NAME)" .

# 复用源码中的版本和 App/Admin 生成参数。
swagger: prepare
	$(GO) generate ./openapi

test: test-case test-script

test-case: prepare
	CGO_ENABLED=1 $(GO) test -v -cover ./...

test-script:
	bash test/control_test.sh

package: package-bin

package-bin: build
	mkdir -p "$(OUTDIR)/bin" "$(OUTDIR)/conf.example" "$(OUTDIR)/log" "$(OUTDIR)/status/$(BINARY_NAME)"
	cp -R "$(HOMEDIR)/package/bin/." "$(OUTDIR)/bin/"
	printf '%s\n' "$(BINARY_NAME)" > "$(OUTDIR)/bin/.binary-name"
	cp "$(HOMEDIR)/$(BINARY_NAME)" "$(OUTDIR)/bin/$(BINARY_NAME)"
	cp "$(HOMEDIR)"/conf.example/*.yaml "$(OUTDIR)/conf.example/"
