# 内核、终端版和窗口版一律 CGO_ENABLED=0：纯 Go，可交叉编译（窗口版的 MyGo 用 purego 调系统 API）。
# 只有竞态检测要 cgo（-race 需要 C 工具链）。
GO      ?= go
# go may select a newer toolchain than the gofmt installed on PATH.
GOFMT   ?= $(shell $(GO) env GOROOT)/bin/gofmt
BIN     := bin/yan
APP     := bin/yan-app
LDFLAGS := -s -w
MAXSIZE := 10485760  # 10 MB：终端版的体积门禁（DECISIONS ADR-016：Chroma 高亮约占 4.7 MB）
# 竞态检测下 Chroma 的 regexp2 慢二十倍上下，highlight 包要跑约 5 分钟（M3 Max），留足余量（见 docs/macos-preview-release-report.md）。
RACE_TIMEOUT ?= 45m
NOTICES := resources/THIRD_PARTY_LICENSES.txt
export CGO_ENABLED := 0

.PHONY: build app notices bundle run run-app dev test race check size clean release

build:
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/yan

# 窗口版（开发用的单个可执行文件）。终端面板第一次打开时 MyGo 会把 libghostty-vt 下载到缓存。
app:
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(APP) ./cmd/yan-app

# 随二进制分发的第三方声明：仓库里的声明、libghostty-vt 及其静态库的许可、两个二进制实际链接的 Go 模块的许可原文。
notices: build app
	$(GO) run ./internal/tools/notices -o $(NOTICES) -native packaging/licenses $(BIN) $(APP)

# 打包成 build/darwin-arm64/yan.app 和 .dmg（带上 libghostty-vt，见 mygo.json；resources/ 里的东西进 Contents/Resources）。
bundle: notices
	env -u MYGO_UPDATER_PRIVATE_KEY $(GO) tool mygo build

run: build
	./$(BIN) $(or $(FILE),.)

run-app: app
	./$(APP) $(or $(FILE),.)

# 试新构建用：配置、会话、最近的文件夹、keybindings.json 都放在 build/dev-config，不碰 ~/.config/yan。
# 想从干净的状态开始就删掉 build/dev-config。
DEVCONFIG := $(CURDIR)/build/dev-config
dev: app
	mkdir -p $(DEVCONFIG)
	XDG_CONFIG_HOME=$(DEVCONFIG) ./$(APP) $(or $(FILE),.)

test:
	$(GO) test -count=1 ./...

# 竞态检测失败就是失败。这台机器编不了 -race（没有 C 工具链）时单独报「没跑」，同样让 check 失败，绝不当作通过。
race:
	@CGO_ENABLED=1 $(GO) build -race runtime/cgo >/dev/null 2>&1 || { \
		echo "竞态检测：没跑（BLOCKED）——这台机器的 Go 编不了 -race：需要 cgo 和 C 工具链，"; \
		echo "如 DEVELOPER_DIR=/Library/Developer/CommandLineTools make race；或者在装好 Xcode 的 macOS 上跑。"; \
		exit 2; }
	CGO_ENABLED=1 $(GO) test -race -count=1 -timeout $(RACE_TIMEOUT) ./...

# 提交前跑：格式、vet、测试、竞态检测、架构边界（在测试里）、体积门禁
check: build app
	@test -z "$$("$(GOFMT)" -l .)" || ("$(GOFMT)" -l .; echo "请先 gofmt"; exit 1)
	$(GO) vet ./...
	$(GO) test -count=1 ./...
	$(MAKE) race
	@size=$$(stat -f%z $(BIN)); echo "yan: $$size bytes"; test $$size -le $(MAXSIZE)
	@echo "yan-app: $$(stat -f%z $(APP)) bytes"

# 从打了标签、没有未提交修改的提交做发布包：make release VERSION=v0.1.0-alpha.1
release:
	scripts/release-macos.sh $(VERSION)

size: build app
	@ls -lh $(BIN) $(APP)

clean:
	rm -rf bin build dist $(NOTICES)
