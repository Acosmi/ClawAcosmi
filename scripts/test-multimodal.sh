#!/usr/bin/env bash
# ============================================================================
# 多模态管线集成测试脚本
#
# 用法: ./scripts/test-multimodal.sh [gateway-url]
# 默认: ws://localhost:19001/ws
#
# 需要: websocat (brew install websocat) 或 wscat (npm i -g wscat)
# 测试项: stt.config.get / docconv.config.get / docconv.formats
# ============================================================================

set -euo pipefail

GATEWAY="${1:-ws://localhost:19001/ws}"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
PASS=0
FAIL=0

info()    { echo -e "${YELLOW}[TEST]${NC} $*"; }
pass()    { echo -e "${GREEN}  ✅ PASS${NC}: $*"; PASS=$((PASS+1)); }
fail()    { echo -e "${RED}  ❌ FAIL${NC}: $*"; FAIL=$((FAIL+1)); }

# 检查 websocat 或 wscat
WS_CMD=""
if command -v websocat &>/dev/null; then
    WS_CMD="websocat"
elif command -v wscat &>/dev/null; then
    WS_CMD="wscat"
else
    echo "需要 websocat 或 wscat"
    echo "  brew install websocat"
    echo "  npm i -g wscat"
    exit 1
fi

# 发送 JSON-RPC 请求（websocat 模式）
rpc_call() {
    local method="$1"
    local params="${2:-{}}"
    local id
    id="test-$(date +%s)-$$"

    local frame
    frame=$(cat <<EOF
{"type":"request","id":"${id}","method":"${method}","params":${params}}
EOF
)

    if [ "$WS_CMD" = "websocat" ]; then
        echo "$frame" | timeout 5 websocat -1 "${GATEWAY}" 2>/dev/null || echo '{"error":"timeout"}'
    else
        echo "$frame" | timeout 5 wscat -c "${GATEWAY}" --wait 3 2>/dev/null || echo '{"error":"timeout"}'
    fi
}

# ─── 测试连接 ───
info "连接 Gateway: ${GATEWAY}"
HELLO=$(echo '{"type":"connect","params":{"client":{"displayName":"test","mode":"webchat","version":"1.0","platform":"test"}}}' \
    | timeout 5 websocat -1 "${GATEWAY}" 2>/dev/null || echo "FAIL")

if echo "$HELLO" | grep -q '"type":"hello"' 2>/dev/null; then
    pass "Gateway 连接成功"
else
    fail "无法连接 Gateway (是否已启动?)"
    echo ""
    echo "启动方法: cd backend && make gateway-dev"
    echo "或:       ./scripts/start.sh"
    exit 1
fi

# ─── Test 1: stt.config.get ───
info "Test 1: stt.config.get"
RESULT=$(rpc_call "stt.config.get")
if echo "$RESULT" | grep -q '"providers"' 2>/dev/null; then
    pass "stt.config.get 返回 providers 列表"
else
    fail "stt.config.get 无 providers 字段: $(echo "$RESULT" | head -c 200)"
fi

# ─── Test 2: docconv.config.get ───
info "Test 2: docconv.config.get"
RESULT=$(rpc_call "docconv.config.get")
if echo "$RESULT" | grep -q '"mcpPresets"' 2>/dev/null; then
    pass "docconv.config.get 返回 MCP presets"
else
    fail "docconv.config.get 无 mcpPresets 字段: $(echo "$RESULT" | head -c 200)"
fi

# ─── Test 3: docconv.formats ───
info "Test 3: docconv.formats"
RESULT=$(rpc_call "docconv.formats")
if echo "$RESULT" | grep -q '"formats"' 2>/dev/null; then
    pass "docconv.formats 返回格式列表"
else
    fail "docconv.formats 无 formats 字段: $(echo "$RESULT" | head -c 200)"
fi

# ─── Test 4: stt.models ───
info "Test 4: stt.models (provider=openai)"
RESULT=$(rpc_call "stt.models" '{"provider":"openai"}')
if echo "$RESULT" | grep -q '"models"' 2>/dev/null; then
    pass "stt.models 返回模型列表"
else
    fail "stt.models 异常: $(echo "$RESULT" | head -c 200)"
fi

# ─── 结果汇总 ───
echo ""
echo "════════════════════════════════════════"
echo -e "  通过: ${GREEN}${PASS}${NC}  失败: ${RED}${FAIL}${NC}"
echo "════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
