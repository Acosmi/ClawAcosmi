# Argus-Compound Makefile — Go+Rust 混合架构统一构建
# Usage: make [target]
#   make build   — 构建 Rust + Go 全量
#   make test    — 运行全量测试
#   make bench   — 运行性能基准测试
#   make lint    — 静态分析 (clippy + vet)
#   make clean   — 清理构建产物

RUST_DIR := rust-core
GO_DIR := go-sensory
DYLIB := $(RUST_DIR)/target/release/libargus_core.dylib

.PHONY: all build rust go test bench lint clean app pkg

all: build

# ===== Build =====

build: rust go

rust:
	@echo "🦀 Building Rust core library..."
	cd $(RUST_DIR) && cargo build --release
	@echo "✅ Rust: $(DYLIB)"

go: rust
	@echo "🐹 Building Go sensory service..."
	cd $(GO_DIR) && go build ./...
	@echo "✅ Go: build complete"

# ===== Test =====

test: rust
	@echo "🧪 Running Go tests..."
	cd $(GO_DIR) && go test -v -count=1 -timeout 120s \
		./internal/imaging/ \
		./internal/pipeline/ \
		./internal/ipc/ \
		./internal/input/ \
		./internal/crypto/ \
		./internal/metrics/

bench: rust
	@echo "📊 Running benchmarks..."
	cd $(GO_DIR) && go test -bench=. -benchmem -count=1 -timeout 120s \
		./internal/imaging/ \
		./internal/pipeline/ \
		./internal/ipc/

# ===== Lint =====

lint: lint-rust lint-go

lint-rust:
	@echo "🔍 Rust clippy..."
	cd $(RUST_DIR) && cargo clippy -- -D warnings

lint-go: rust
	@echo "🔍 Go vet..."
	cd $(GO_DIR) && go vet ./...

# ===== Package =====

APP_NAME := Argus
APP_DIR  := build/$(APP_NAME).app
GO_BIN   := $(GO_DIR)/argus-sensory

app: build
	@echo "📦 Packaging $(APP_NAME).app..."
	mkdir -p $(APP_DIR)/Contents/MacOS
	mkdir -p $(APP_DIR)/Contents/Frameworks
	mkdir -p $(APP_DIR)/Contents/Resources
	cp scripts/package/Info.plist $(APP_DIR)/Contents/
	cd $(GO_DIR) && go build -o ../$(APP_DIR)/Contents/MacOS/argus-sensory ./cmd/server
	cp $(DYLIB) $(APP_DIR)/Contents/Frameworks/
	install_name_tool -change \
		$(DYLIB) \
		@executable_path/../Frameworks/libargus_core.dylib \
		$(APP_DIR)/Contents/MacOS/argus-sensory
	codesign --force --deep -s "Argus Dev" $(APP_DIR)
	@echo "✅ $(APP_DIR) created (signed with 'Argus Dev')"

pkg: app
	@echo "📦 Building .pkg installer..."
	./scripts/package/build-pkg.sh
	@echo "✅ Installer: build/Argus-Installer.pkg"

# ===== Console App =====

WAILS := $(shell go env GOPATH)/bin/wails

web:
	@echo "🌐 Building Next.js frontend..."
	cd web-console && npm install && npm run build

console: rust web
	@echo "🖥  Building Argus Console.app (Wails)..."
	cd wails-console && $(WAILS) build -platform darwin/arm64
	@echo "🔏 Signing Argus Console.app..."
	./scripts/package/sign-console.sh
	@echo "✅ Argus Console.app created (signed)"


# ===== Clean =====

clean:
	@echo "🗑  Cleaning..."
	cd $(RUST_DIR) && cargo clean
	cd $(GO_DIR) && go clean -cache
	@echo "✅ Clean complete"
