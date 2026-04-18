#!/bin/bash

set -euo pipefail

APP_NAME="oss-cli"
OUTPUT_DIR="dist"
TARGETS=(
  "darwin arm64"
  "darwin amd64"
  "linux amd64"
  "linux arm64"
)

echo "开始编译 ${APP_NAME} 跨平台版本..."
mkdir -p "${OUTPUT_DIR}"
rm -f "${OUTPUT_DIR}/${APP_NAME}-"*

for target in "${TARGETS[@]}"; do
  GOOS=$(echo "${target}" | awk '{print $1}')
  GOARCH=$(echo "${target}" | awk '{print $2}')
  BIN_FILE="${OUTPUT_DIR}/${APP_NAME}-${GOOS}-${GOARCH}"

  echo "编译 ${GOOS}/${GOARCH} -> ${BIN_FILE}"
  GOOS="${GOOS}" GOARCH="${GOARCH}" CGO_ENABLED=0 go build -o "${BIN_FILE}" .
  chmod +x "${BIN_FILE}"
done

echo
echo "编译完成，输出目录: ${OUTPUT_DIR}"
ls -lh "${OUTPUT_DIR}"
