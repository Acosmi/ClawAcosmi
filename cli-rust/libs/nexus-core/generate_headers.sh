#!/bin/bash
# generate_headers.sh — 使用 cbindgen 为所有 crate 生成 C 头文件
# 用法: cd libs/nexus-core && ./generate_headers.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT_DIR="$SCRIPT_DIR/include"

mkdir -p "$OUT_DIR"

CRATES=("nexus-tokenizer" "nexus-vector" "nexus-crypto" "nexus-decay" "nexus-graph")

for crate in "${CRATES[@]}"; do
    header_name="${crate//-/_}.h"
    echo "生成 $header_name ..."
    cbindgen --crate "$crate" --output "$OUT_DIR/$header_name"
done

echo "✅ 所有头文件已生成到 $OUT_DIR/"
ls -la "$OUT_DIR/"
