#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
REPORT_DIR="$ROOT_DIR/docs/codex"
REPORT_DATE="$(date +%F)"
REPORT_PATH="$REPORT_DIR/${REPORT_DATE}-多媒体回归报告-自动生成.md"

mkdir -p "$REPORT_DIR"

run_and_capture() {
  local cmd="$1"
  local output_file="$2"
  (
    cd "$BACKEND_DIR"
    GOCACHE=/tmp/go-build GOTMPDIR=/tmp bash -lc "$cmd"
  ) >"$output_file" 2>&1
}

TMP_TEST="$(mktemp)"
TMP_BENCH="$(mktemp)"
trap 'rm -f "$TMP_TEST" "$TMP_BENCH"' EXIT

TEST_CMD="make test-multimedia"
BENCH_CMD="go test ./internal/gateway -run '^$' -bench 'BenchmarkChatAttachmentProviderCache_ProcessAttachments' -benchmem -benchtime=100ms -count=1"

TEST_STATUS="PASS"
if ! run_and_capture "$TEST_CMD" "$TMP_TEST"; then
  TEST_STATUS="FAIL"
fi

BENCH_STATUS="PASS"
if ! run_and_capture "$BENCH_CMD" "$TMP_BENCH"; then
  BENCH_STATUS="FAIL"
fi

cat >"$REPORT_PATH" <<EOF
# 多媒体回归报告（自动生成）

- 生成日期：$REPORT_DATE
- 生成脚本：\`scripts/generate_multimedia_regression_report.sh\`
- 测试状态：$TEST_STATUS
- 基准状态：$BENCH_STATUS

## 1. 多媒体关键回归

命令：
\`\`\`bash
$TEST_CMD
\`\`\`

输出：
\`\`\`text
$(cat "$TMP_TEST")
\`\`\`

## 2. Provider 缓存基准

命令：
\`\`\`bash
$BENCH_CMD
\`\`\`

输出：
\`\`\`text
$(cat "$TMP_BENCH")
\`\`\`
EOF

echo "report generated: $REPORT_PATH"
if [[ "$TEST_STATUS" != "PASS" || "$BENCH_STATUS" != "PASS" ]]; then
  exit 1
fi
