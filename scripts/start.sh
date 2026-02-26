#!/usr/bin/env bash
# ============================================================================
# OpenAcosmi 一键启动脚本
#
# 功能：环境检查 → 依赖安装 → 编译 Gateway → 启动服务 → 打开浏览器
# 适用：macOS / Linux
# ============================================================================

set -euo pipefail

# ===== 常量 =====
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
GATEWAY_PORT=19001
UI_PORT=26222
UI_URL="http://localhost:${UI_PORT}"
GATEWAY_URL="http://localhost:${GATEWAY_PORT}"
MAX_WAIT=60  # 最长等待秒数

# 子进程 PID 跟踪
GATEWAY_PID=""
UI_PID=""

# ===== 颜色 =====
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ===== 工具函数 =====
info()    { echo -e "${BLUE}ℹ ${NC}$*"; }
success() { echo -e "${GREEN}✅ ${NC}$*"; }
warn()    { echo -e "${YELLOW}⚠️  ${NC}$*"; }
error()   { echo -e "${RED}❌ ${NC}$*"; }

banner() {
    echo -e "${CYAN}${BOLD}"
    echo "╔══════════════════════════════════════════╗"
    echo "║          OpenAcosmi  快速启动            ║"
    echo "╚══════════════════════════════════════════╝"
    echo -e "${NC}"
}

# ===== 清理函数 =====
cleanup() {
    echo ""
    info "正在停止所有服务..."

    if [ -n "$UI_PID" ] && kill -0 "$UI_PID" 2>/dev/null; then
        kill "$UI_PID" 2>/dev/null || true
        wait "$UI_PID" 2>/dev/null || true
        success "前端 UI 已停止"
    fi

    if [ -n "$GATEWAY_PID" ] && kill -0 "$GATEWAY_PID" 2>/dev/null; then
        kill "$GATEWAY_PID" 2>/dev/null || true
        wait "$GATEWAY_PID" 2>/dev/null || true
        success "Gateway 已停止"
    fi

    success "所有服务已关闭，再见！"
    exit 0
}

trap cleanup SIGINT SIGTERM EXIT

# ===== 打开浏览器（跨平台） =====
open_browser() {
    local url="$1"
    if command -v open &>/dev/null; then
        open "$url"                     # macOS
    elif command -v xdg-open &>/dev/null; then
        xdg-open "$url"                 # Linux
    elif command -v wslview &>/dev/null; then
        wslview "$url"                  # WSL
    else
        warn "无法自动打开浏览器，请手动访问: $url"
    fi
}

# ===== 1. 环境检查 =====
check_env() {
    info "检查开发环境..."
    local missing=0

    # Go
    if command -v go &>/dev/null; then
        local go_ver
        go_ver=$(go version | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?' | head -1)
        success "Go ${go_ver}"
    else
        error "Go 未安装 — 请访问 https://go.dev/dl/ 安装"
        missing=1
    fi

    # Node.js
    if command -v node &>/dev/null; then
        local node_ver
        node_ver=$(node --version)
        success "Node.js ${node_ver}"
    else
        error "Node.js 未安装 — 请访问 https://nodejs.org/ 安装"
        missing=1
    fi

    # npm
    if command -v npm &>/dev/null; then
        local npm_ver
        npm_ver=$(npm --version)
        success "npm ${npm_ver}"
    else
        error "npm 未安装 — 通常随 Node.js 一起安装"
        missing=1
    fi

    # Rust（可选 — 仅 Argus 需要）
    if command -v rustc &>/dev/null; then
        local rust_ver
        rust_ver=$(rustc --version | awk '{print $2}')
        success "Rust ${rust_ver} (Argus 编译需要)"
    else
        warn "Rust 未安装 — Argus 视觉子智能体不可用（可选）"
    fi

    echo ""

    if [ "$missing" -ne 0 ]; then
        error "缺少必要依赖，请先安装后重试。"
        exit 1
    fi
}

# ===== 2. 安装前端依赖 =====
install_ui_deps() {
    if [ ! -d "$PROJECT_DIR/ui/node_modules" ]; then
        info "首次运行，安装前端依赖 (npm install)..."
        cd "$PROJECT_DIR/ui" && npm install
        echo ""
    fi
}

# ===== 3. 编译 Gateway =====
build_gateway() {
    info "编译 Gateway..."
    cd "$PROJECT_DIR/backend" && make gateway 2>&1
    if [ ! -f "$PROJECT_DIR/backend/build/acosmi" ]; then
        error "Gateway 编译失败"
        exit 1
    fi
    success "Gateway 编译完成"
    echo ""
}

# ===== 4. 检查 Argus 状态 =====
check_argus() {
    local argus_app="$PROJECT_DIR/Argus/build/Argus.app/Contents/MacOS/argus-sensory"
    if [ -f "$argus_app" ]; then
        success "Argus 已构建 — 视觉子智能体可用"
    else
        warn "Argus 未构建 — 视觉子智能体不可用"
        warn "如需 Argus，请在另一终端运行: cd Argus && make app"
    fi
    echo ""
}

# ===== 5. 启动 Gateway =====
start_gateway() {
    info "启动 Gateway (port ${GATEWAY_PORT})..."
    cd "$PROJECT_DIR/backend"
    ./build/acosmi -dev -port "$GATEWAY_PORT" &
    GATEWAY_PID=$!
    success "Gateway PID: ${GATEWAY_PID}"
}

# ===== 6. 启动前端 UI =====
start_ui() {
    info "启动前端 UI (port ${UI_PORT})..."
    cd "$PROJECT_DIR/ui"
    npm run dev &
    UI_PID=$!
    success "前端 UI PID: ${UI_PID}"
    echo ""
}

# ===== 7. 等待服务就绪 =====
wait_for_ready() {
    info "等待服务就绪..."
    local elapsed=0

    while [ "$elapsed" -lt "$MAX_WAIT" ]; do
        # 检查子进程是否意外退出
        if [ -n "$GATEWAY_PID" ] && ! kill -0 "$GATEWAY_PID" 2>/dev/null; then
            error "Gateway 进程意外退出"
            exit 1
        fi

        if curl -s -o /dev/null -w '' "$UI_URL" 2>/dev/null; then
            echo ""
            success "所有服务已就绪！"
            return 0
        fi

        printf "."
        sleep 1
        elapsed=$((elapsed + 1))
    done

    echo ""
    warn "等待超时 (${MAX_WAIT}s)，服务可能还未完全就绪"
    warn "请手动检查: Gateway ${GATEWAY_URL}  |  UI ${UI_URL}"
}

# ===== 主流程 =====
main() {
    banner
    check_env
    install_ui_deps
    build_gateway
    check_argus
    start_gateway
    start_ui
    wait_for_ready

    # 打开浏览器
    info "正在打开浏览器..."
    open_browser "$UI_URL"

    echo ""
    echo -e "${CYAN}${BOLD}════════════════════════════════════════════${NC}"
    echo -e "  ${GREEN}Gateway${NC}:  ${GATEWAY_URL}"
    echo -e "  ${GREEN}前端 UI${NC}:  ${UI_URL}"
    echo -e "  ${YELLOW}按 Ctrl+C 停止所有服务${NC}"
    echo -e "${CYAN}${BOLD}════════════════════════════════════════════${NC}"
    echo ""

    # 前台等待（不退出）
    wait
}

main "$@"
