#!/bin/bash

set -euo pipefail

REPO="tianfei212/alOSS_agent_go"
APP_NAME="oss-cli"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "${ARCH}" in
  x86_64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
esac

BIN_NAME="${APP_NAME}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/latest/download/${BIN_NAME}"

case "${BIN_NAME}" in
  "oss-cli-darwin-arm64"|"oss-cli-linux-amd64") ;;
  *)
    echo "当前仅提供以下二进制："
    echo "  - oss-cli-darwin-arm64"
    echo "  - oss-cli-linux-amd64"
    echo "当前系统不在支持范围内：${OS}/${ARCH}"
    exit 1
    ;;
esac

echo "下载 ${BIN_NAME} ..."
curl -fL -o "${APP_NAME}" "${URL}"
chmod +x "${APP_NAME}"
echo "安装完成，运行方式：./${APP_NAME} server"
