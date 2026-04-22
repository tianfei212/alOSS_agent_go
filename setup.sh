#!/bin/bash

set -euo pipefail

REPO="tianfei212/alOSS_agent_go"
APP_NAME="oss-cli"
OUTPUT_DIR="dist"
GITHUB_API="https://api.github.com/repos/${REPO}/releases/latest"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "${ARCH}" in
  x86_64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
esac

PLATFORM="${OS}-${ARCH}"

SUPPORTED_PLATFORMS="darwin-arm64 linux-amd64"

echo "============================================"
echo "  ${APP_NAME} 安装脚本"
echo "============================================"
echo "  当前平台: ${PLATFORM}"
echo ""

if ! echo "${SUPPORTED_PLATFORMS}" | grep -qw "${PLATFORM}"; then
  echo "错误: 当前仅支持以下平台:"
  echo "  - darwin-arm64  (macOS Apple Silicon)"
  echo "  - linux-amd64   (Ubuntu / Linux x86)"
  echo "当前系统: ${PLATFORM}"
  exit 1
fi

mkdir -p "${OUTPUT_DIR}"

ASSET_NAME="${APP_NAME}-${PLATFORM}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"

echo "[1/2] 下载 ${ASSET_NAME}..."
if command -v curl &>/dev/null; then
  curl -fL -o "${OUTPUT_DIR}/${ASSET_NAME}" "${DOWNLOAD_URL}"
elif command -v wget &>/dev/null; then
  wget -O "${OUTPUT_DIR}/${ASSET_NAME}" "${DOWNLOAD_URL}"
else
  echo "错误: 需要 curl 或 wget"
  exit 1
fi

echo ""
echo "[2/2] 解压到 ${OUTPUT_DIR}/..."
tar -xzvf "${OUTPUT_DIR}/${ASSET_NAME}" -C "${OUTPUT_DIR}"

echo ""
echo "============================================"
echo "  安装完成！"
echo "============================================"
echo "  二进制: ${OUTPUT_DIR}/${APP_NAME}-${PLATFORM}"
echo ""
echo "启动方式:"
echo "  ./start.sh"
echo ""
echo "或直接运行:"
echo "  cd ${OUTPUT_DIR} && ./${APP_NAME}-${PLATFORM} server"
