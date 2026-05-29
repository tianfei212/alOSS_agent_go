#!/bin/bash
# 构建 release 包：release/{时间}_{版本}/ 目录下包含二进制、config.yaml、.env.local

set -euo pipefail

APP_NAME="oss-cli"
VERSION=$(cat VERSION 2>/dev/null || echo "V1.0.0")
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RELEASE_DIR="release/${TIMESTAMP}_${VERSION}"

echo "============================================"
echo "  ${APP_NAME} Release 构建"
echo "  版本: ${VERSION}"
echo "  输出: ${RELEASE_DIR}/"
echo "============================================"

mkdir -p "${RELEASE_DIR}"

echo ""
echo "[1/4] 编译 Linux amd64 二进制..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "${RELEASE_DIR}/${APP_NAME}" .
chmod +x "${RELEASE_DIR}/${APP_NAME}"
echo "    大小: $(du -h "${RELEASE_DIR}/${APP_NAME}" | cut -f1)"

echo ""
echo "[2/4] 复制 config.yaml..."
cp config.yaml "${RELEASE_DIR}/config.yaml"

echo ""
echo "[3/4] 生成 .env.local 模板..."
cat > "${RELEASE_DIR}/.env.local" << 'ENVEOF'
# F1-F4 自有 OSS / OpenAI Files API
OPENAI_API_KEY=
OSS_ENDPOINT=
OSS_BUCKET=
OSS_BUCKET_PREFIX=
OSS_ACCESS_KEY_ID=
OSS_ACCESS_KEY_SECRET=

# F5 百炼临时文件上传（与 OSS、OPENAI_API_KEY 无关，必填才能使用 dashscope 功能）
AL_KEY=
ENVEOF

echo ""
echo "[4/4] 生成 oss-cli.sh 服务管理脚本..."
cp oss-cli.sh "${RELEASE_DIR}/oss-cli.sh"
chmod +x "${RELEASE_DIR}/oss-cli.sh"

echo ""
echo "============================================"
echo "  Release 构建完成！"
echo "============================================"
ls -lh "${RELEASE_DIR}/"
