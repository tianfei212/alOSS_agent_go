#!/bin/bash

set -euo pipefail

APP_NAME="oss-cli"
CONFIG_FILE="config.yaml"
OUTPUT_DIR="dist"
PORT="${1:-8080}"

OS_TYPE=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH_TYPE=$(uname -m)

case "${ARCH_TYPE}" in
  x86_64) ARCH_TYPE="amd64" ;;
  arm64|aarch64) ARCH_TYPE="arm64" ;;
esac

BIN_FILE="${OUTPUT_DIR}/${APP_NAME}-${OS_TYPE}-${ARCH_TYPE}"

if [ ! -f "${BIN_FILE}" ]; then
  echo "未找到当前系统对应的二进制: ${BIN_FILE}"
  echo "开始自动编译..."
  chmod +x build.sh
  ./build.sh
fi

if [ ! -f "${BIN_FILE}" ]; then
  echo "错误：仍未找到可执行文件 ${BIN_FILE}"
  exit 1
fi

if [ ! -f "${CONFIG_FILE}" ]; then
  echo "警告：未找到 ${CONFIG_FILE}，程序仍会尝试读取 .env.local"
fi

chmod +x "${BIN_FILE}"

echo "=========================================================="
echo "准备启动 ${APP_NAME} Server"
echo "系统类型: ${OS_TYPE}"
echo "CPU 架构: ${ARCH_TYPE}"
echo "二进制: ${BIN_FILE}"
echo "端口: ${PORT}"
echo "=========================================================="

exec "./${BIN_FILE}" server -p "${PORT}"
