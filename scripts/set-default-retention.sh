#!/bin/bash
# 为存量 OSS 对象批量设置保存周期标签（默认 3 年）。
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

CONFIG="config.yaml"
YEARS=3
DRY_RUN=0
SKIP_EXISTING=0

usage() {
  cat <<EOF
用法: $(basename "$0") [选项]

选项:
  --config PATH       配置文件（默认 config.yaml）
  --years N           保存周期年数（默认 3）
  --dry-run           仅打印，不写入 OSS
  --skip-existing     跳过已有相同 retention-years 标签的对象
  -h, --help          显示帮助

示例:
  $(basename "$0") --dry-run
  $(basename "$0") --years 3 --skip-existing
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --config)
      CONFIG="$2"
      shift 2
      ;;
    --years)
      YEARS="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --skip-existing)
      SKIP_EXISTING=1
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

ARGS=(--config "${CONFIG}" --years "${YEARS}")
if [ "${DRY_RUN}" -eq 1 ]; then
  ARGS+=(--dry-run)
fi
if [ "${SKIP_EXISTING}" -eq 1 ]; then
  ARGS+=(--skip-existing)
fi

exec go run ./scripts/set-retention/ "${ARGS[@]}"
