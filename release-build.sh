#!/bin/bash
# 构建 release 包：release/{时间}_{版本}/ 下含 linux-amd64 / darwin-arm64 二进制与压缩包
# 不含真实 config.yaml、.env.local，仅附带 *.example 模板

set -euo pipefail

APP_NAME="oss-cli"
VERSION=$(cat VERSION 2>/dev/null || echo "V1.0.0")
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RELEASE_DIR="release/${TIMESTAMP}_${VERSION}"
BUILD_TARGETS=(
  "linux-amd64"
  "darwin-arm64"
)

echo "============================================"
echo "  ${APP_NAME} Release 构建"
echo "  版本: ${VERSION}"
echo "  输出: ${RELEASE_DIR}/"
echo "============================================"

mkdir -p "${RELEASE_DIR}"

for target in "${BUILD_TARGETS[@]}"; do
  GOOS="${target%-*}"
  GOARCH="${target#*-}"
  PLATFORM_DIR="${RELEASE_DIR}/${target}"

  echo ""
  echo ">>> [${target}] 编译 ${GOOS}/${GOARCH}..."
  mkdir -p "${PLATFORM_DIR}"
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" go build -o "${PLATFORM_DIR}/${APP_NAME}" .
  chmod +x "${PLATFORM_DIR}/${APP_NAME}"
  echo "    大小: $(du -h "${PLATFORM_DIR}/${APP_NAME}" | cut -f1)"

  echo ">>> [${target}] 复制模板与脚本（不含 config.yaml / .env.local）..."
  cp config.yaml.example "${PLATFORM_DIR}/config.yaml.example"
  cp .env.example "${PLATFORM_DIR}/.env.example"
  cp oss-cli.sh "${PLATFORM_DIR}/oss-cli.sh"
  chmod +x "${PLATFORM_DIR}/oss-cli.sh"

  cat > "${PLATFORM_DIR}/README-RELEASE.txt" << 'READMEEOF'
部署说明
========

1. 复制配置模板并填写真实凭证：
   cp config.yaml.example config.yaml
   cp .env.example .env.local

2. 启动服务：
   ./oss-cli.sh start

本 release 包 intentionally 不包含 config.yaml 与 .env.local，避免泄露密钥。
READMEEOF

  if [ -f "${PLATFORM_DIR}/config.yaml" ] || [ -f "${PLATFORM_DIR}/.env.local" ]; then
    echo "错误: release 包中不得包含 config.yaml 或 .env.local"
    exit 1
  fi

  PKG_NAME="${APP_NAME}-${target}-${VERSION}.tar.gz"
  echo ">>> [${target}] 打包 ${PKG_NAME}..."
  tar -czf "${RELEASE_DIR}/${PKG_NAME}" -C "${RELEASE_DIR}" "${target}"
  echo "    大小: $(du -h "${RELEASE_DIR}/${PKG_NAME}" | cut -f1)"
done

echo ""
echo "============================================"
echo "  Release 构建完成！"
echo "============================================"
ls -lh "${RELEASE_DIR}/"
