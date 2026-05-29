#!/bin/bash
# 同步 OSS Bucket 保存周期 Lifecycle 规则。
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

CONFIG="config.yaml"
DRY_RUN=0

usage() {
  cat <<EOF
用法: $(basename "$0") [选项]

选项:
  --config PATH   配置文件（默认 config.yaml）
  --dry-run       仅打印将写入的规则，不调用 OSS
  -h, --help      显示帮助

示例:
  $(basename "$0") --dry-run
  $(basename "$0") --config config.yaml
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --config)
      CONFIG="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      usage
      exit 1
      ;;
  esac
done

ARGS=(--config "${CONFIG}")
if [ "${DRY_RUN}" -eq 1 ]; then
  ARGS+=(--dry-run)
fi

exec go run ./scripts/sync-lifecycle/ "${ARGS[@]}"
