#!/bin/bash

set -euo pipefail

APP_NAME="oss-cli"
OUTPUT_DIR="dist"
BUILD_TARGETS=(
  "darwin-arm64"
  "linux-amd64"
)

echo "============================================"
echo "  ${APP_NAME} 跨平台编译"
echo "============================================"
mkdir -p "${OUTPUT_DIR}"
rm -f "${OUTPUT_DIR}/${APP_NAME}-"*

for target in "${BUILD_TARGETS[@]}"; do
  BIN_FILE="${OUTPUT_DIR}/${APP_NAME}-${target}"
  echo ""
  echo ">>> 编译 ${target} -> ${BIN_FILE}"
  GOOS="${target%-*}" GOARCH="${target#*-}" CGO_ENABLED=0 go build -o "${BIN_FILE}" .
  chmod +x "${BIN_FILE}"
  echo "    大小: $(du -h "${BIN_FILE}" | cut -f1)"
done

echo ""
echo "============================================"
echo "  编译完成，输出目录: ${OUTPUT_DIR}/"
echo "============================================"
ls -lh "${OUTPUT_DIR}"
