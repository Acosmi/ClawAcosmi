.PHONY: run build build-pure build-ffi tidy test test-e2e test-all clean

RUST_TARGET ?= release
NEXUS_DIR   := ../../libs/nexus-core
OPENVIKING_DIR := ../../libs/openviking-rs

# ── 开发模式运行（纯 Go，无 CGO）──────────────────────────────
run:
	CGO_ENABLED=0 go run cmd/server/main.go

# ── 纯 Go 构建（CI / 快速验证，无 Rust FFI）────────────────────
build-pure:
	CGO_ENABLED=0 go build -o bin/uhms-api cmd/server/main.go

# ── 完整 CGO 构建（需先编译 Rust FFI 库）──────────────────────
build-ffi:
	@echo "Step 1/3: Building Rust nexus-core..."
	cd $(NEXUS_DIR) && cargo build --$(RUST_TARGET)
	@echo "Step 2/3: Building Rust openviking-rs..."
	cd $(OPENVIKING_DIR) && cargo build --$(RUST_TARGET)
	@echo "Step 3/3: Building Go binary with CGO..."
	go build -o bin/uhms-api cmd/server/main.go
	@echo "✓ Build complete: bin/uhms-api"

# ── 默认 build 别名（纯 Go）──────────────────────────────────
build: build-pure

# ── 整理依赖 ──────────────────────────────────────────────────
tidy:
	go mod tidy

# ── 运行单元测试（纯 Go，快速）────────────────────────────────
test:
	CGO_ENABLED=0 go test ./... -v

# ── 运行 E2E 集成测试 ──────────────────────────────────────────
test-e2e:
	go test -tags e2e -race -v ./internal/services/e2e_test/...

# ── 全量测试 (单元 + E2E) ──────────────────────────────────────
test-all:
	CGO_ENABLED=0 go test ./... && go test -tags e2e -race -v ./internal/services/e2e_test/...

# ── 清理构建产物 ──────────────────────────────────────────────
clean:
	rm -rf bin/
