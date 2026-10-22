# 木鱼进销存 构建脚本
#
# 常用命令：
#   make            # 构建前端 + 编译出单文件可执行程序 ./mufish
#   make run        # 直接运行（自动打开浏览器）
#   make web        # 只构建前端
#   make cross      # 交叉编译 Linux / Windows / macOS 三个平台
#   make clean      # 清理构建产物

APP      := mufish
VERSION  ?= 0.11.0
LDFLAGS  := -s -w -X main.version=$(VERSION)
GOFLAGS  :=
GO       ?= go

.PHONY: all build web run dev test test-web test-components test-e2e test-all cover vet fmt tidy clean cross install help

all: build

## build       构建前端并编译单文件可执行程序
build: web
	$(GO) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o $(APP) .
	@echo "✅ 构建完成：./$(APP)"

## web         仅构建前端（产物输出到 web/dist，会被 go:embed 打进程序）
web:
	@cd web && (test -d node_modules || npm install --no-audit --no-fund) && npm run build

## run         直接运行（开发用）
run: build
	./$(APP) -open

## dev         后端热运行（前端资源需先 make web）
dev:
	$(GO) run . -addr 127.0.0.1:8899

## watch       前端开发监听模式（另开一个终端执行 make dev）
watch:
	cd web && npm run dev

## test        后端单元测试
test:
	$(GO) test ./...

## test-web    前端纯函数测试（Node 内置 test runner）
test-web:
	@cd web && (test -d node_modules || npm install --no-audit --no-fund) && npm test

## test-components  前端组件测试（Vitest + jsdom）
test-components:
	@cd web && (test -d node_modules || npm install --no-audit --no-fund) && npm run test:components

## test-e2e    浏览器端到端测试（Playwright + 系统 Chrome）
test-e2e:
	@cd web && (test -d node_modules || npm install --no-audit --no-fund) && npm run test:e2e

## test-all    后端 + 前端单元/组件测试
test-all: test test-web test-components

## cover       后端测试覆盖率
cover:
	$(GO) test -cover ./...

## vet         静态检查
vet:
	$(GO) vet ./...

## fmt         格式化
fmt:
	$(GO) fmt ./...

## tidy        整理依赖
tidy:
	$(GO) mod tidy

## cross       交叉编译（SQLite 驱动为纯 Go 实现，无需 CGO）
cross:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(APP)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(APP)-linux-arm64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(APP)-windows-amd64.exe .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(APP)-darwin-arm64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(APP)-darwin-amd64 .
	@echo "✅ 交叉编译完成，产物位于 dist/"

## install     安装到 GOPATH/bin
install:
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" .

clean:
	rm -rf $(APP) dist web/dist/app.js web/dist/app.js.map web/dist/app.css
	@echo "🧹 已清理"

help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
