#!/bin/bash

set -euo pipefail

APP_NAME="oss-cli"
CONFIG_FILE="config.yaml"
OUTPUT_DIR="dist"
PORT="${1:-8080}"

if [ ! -f "${CONFIG_FILE}" ]; then
  echo "错误: 未找到 ${CONFIG_FILE}"
  echo "提示: 首次使用请先解压发布包，或复制 config.yaml 和 .env.example 到当前目录"
  exit 1
fi

if [ ! -f .env.example ] && [ -f config.yaml ]; then
  echo "提示: 建议创建 .env.example（参考配置）以了解环境变量用法"
fi

OS_TYPE=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH_TYPE=$(uname -m)

case "${ARCH_TYPE}" in
  x86_64)  ARCH_TYPE="amd64" ;;
  arm64|aarch64) ARCH_TYPE="arm64" ;;
esac

BIN_FILE="${OUTPUT_DIR}/${APP_NAME}-${OS_TYPE}-${ARCH_TYPE}"

if [ ! -f "${BIN_FILE}" ]; then
  echo "错误: 未找到当前系统对应的二进制: ${BIN_FILE}"
  echo "支持的构建目标:"
  echo "  - macOS ARM64 (Apple Silicon): ${APP_NAME}-darwin-arm64"
  echo "  - Linux AMD64 (Ubuntu):        ${APP_NAME}-linux-amd64"
  echo ""
  echo "请从 release 包中解压对应平台的二进制到 ${OUTPUT_DIR}/"
  exit 1
fi

chmod +x "${BIN_FILE}"

echo "=========================================="
echo "  ${APP_NAME} Server 启动"
echo "=========================================="
echo "  系统:   ${OS_TYPE}/${ARCH_TYPE}"
echo "  二进制: ${BIN_FILE}"
echo "  端口:   ${PORT}"
echo "  配置:   ${CONFIG_FILE}"
echo "=========================================="

exec "./${BIN_FILE}" server -p "${PORT}"
