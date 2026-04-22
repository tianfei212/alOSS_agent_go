#!/bin/bash

set -euo pipefail

APP_NAME="oss-cli"
OUTPUT_DIR="dist"
BUILD_TARGETS=(
  "darwin-arm64"
  "linux-amd64"
)

PKG_DIR=".pkg_tmp"

echo "============================================"
echo "  ${APP_NAME} 打包（编译 + 归档）"
echo "============================================"

echo ""
echo "[1/3] 编译跨平台二进制..."
chmod +x build.sh
./build.sh

echo ""
echo "[2/3] 准备打包文件..."
rm -rf "${PKG_DIR}"
mkdir -p "${PKG_DIR}"

if [ ! -f config.yaml ]; then
  echo "错误: config.yaml 不存在，请先创建或复制配置"
  exit 1
fi

if [ ! -f .env.example ]; then
  echo "警告: .env.example 不存在，将跳过"
else
  cp .env.example "${PKG_DIR}/"
fi

cp config.yaml "${PKG_DIR}/"

cat > "${PKG_DIR}/start.sh" << 'STARTSCRIPT'
#!/bin/bash
set -euo pipefail

APP_NAME="oss-cli"
CONFIG_FILE="config.yaml"
PORT="${1:-8080}"

OS_TYPE=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH_TYPE=$(uname -m)

case "${ARCH_TYPE}" in
  x86_64)  ARCH_TYPE="amd64" ;;
  arm64|aarch64) ARCH_TYPE="arm64" ;;
esac

BIN_FILE="${APP_NAME}-${OS_TYPE}-${ARCH_TYPE}"

if [ ! -f "./${BIN_FILE}" ]; then
  echo "错误: 未找到 ${BIN_FILE}，请确认该系统对应的程序已正确解压"
  exit 1
fi

chmod +x "./${BIN_FILE}"

echo "=========================================="
echo "  ${APP_NAME} Server 启动"
echo "=========================================="
echo "  系统: ${OS_TYPE}/${ARCH_TYPE}"
echo "  二进制: ${BIN_FILE}"
echo "  端口: ${PORT}"
echo "=========================================="

exec "./${BIN_FILE}" server -p "${PORT}"
STARTSCRIPT

chmod +x "${PKG_DIR}/start.sh"

echo ""
echo "[3/3] 生成压缩包..."

for target in "${BUILD_TARGETS[@]}"; do
  BIN_FILE="${OUTPUT_DIR}/${APP_NAME}-${target}"

  if [ ! -f "${BIN_FILE}" ]; then
    echo "警告: 找不到 ${BIN_FILE}，跳过"
    continue
  fi

  PKG_NAME="${APP_NAME}-${target}.tar.gz"

  mkdir -p "${PKG_DIR}/${target}"
  cp "${BIN_FILE}" "${PKG_DIR}/${target}/${APP_NAME}"
  cp "${PKG_DIR}/config.yaml" "${PKG_DIR}/${target}/"
  [ -f "${PKG_DIR}/.env.example" ] && cp "${PKG_DIR}/.env.example" "${PKG_DIR}/${target}/"
  cp "${PKG_DIR}/start.sh" "${PKG_DIR}/${target}/"

  tar -czvf "${OUTPUT_DIR}/${PKG_NAME}" -C "${PKG_DIR}" "${target}"
  rm -rf "${PKG_DIR}/${target}"

  echo "  打包完成: ${OUTPUT_DIR}/${PKG_NAME} ($(du -h "${OUTPUT_DIR}/${PKG_NAME}" | cut -f1))"
done

rm -rf "${PKG_DIR}"

echo ""
echo "============================================"
echo "  打包完成！"
echo "============================================"
ls -lh "${OUTPUT_DIR}"/*.tar.gz 2>/dev/null || true
ls -lh "${OUTPUT_DIR}"/${APP_NAME}-*  | grep -v ".tar.gz" || true
