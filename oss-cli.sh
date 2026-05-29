#!/bin/bash
# oss-cli 服务管理脚本：start | stop | status | logs [-f]

set -euo pipefail

APP_NAME="oss-cli"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

CONFIG_FILE="config.yaml"
PID_FILE="${SCRIPT_DIR}/.oss-cli.pid"
LOG_DIR="${SCRIPT_DIR}/logs"
LOG_FILE="${LOG_DIR}/oss-cli.log"
DEFAULT_PORT="8080"

# usage 打印帮助信息。
usage() {
  cat <<EOF
用法: $(basename "$0") <命令> [选项]

命令:
  start [port]    启动 HTTP 服务（默认端口 ${DEFAULT_PORT}，或 config.yaml 中的 server.port）
  stop            停止服务
  status          查看运行状态
  logs [-f]       查看日志；-f 持续跟踪（类似 tail -f）

示例:
  $(basename "$0") start
  $(basename "$0") start 9090
  $(basename "$0") stop
  $(basename "$0") status
  $(basename "$0") logs
  $(basename "$0") logs -f

环境变量:
  PORT            覆盖默认端口（start 未指定 port 时生效）
EOF
}

# read_port_from_config 从 config.yaml 读取 server.port。
read_port_from_config() {
  if [ ! -f "${CONFIG_FILE}" ]; then
    echo "${DEFAULT_PORT}"
    return
  fi
  local port
  port=$(grep -E '^[[:space:]]*port:[[:space:]]*[0-9]+' "${CONFIG_FILE}" | head -1 | awk '{print $2}' | tr -d '"' || true)
  if [ -n "${port}" ]; then
    echo "${port}"
  else
    echo "${DEFAULT_PORT}"
  fi
}

# is_running 判断服务进程是否存活。
is_running() {
  if [ ! -f "${PID_FILE}" ]; then
    return 1
  fi
  local pid
  pid=$(cat "${PID_FILE}")
  if [ -z "${pid}" ]; then
    return 1
  fi
  kill -0 "${pid}" 2>/dev/null
}

# cmd_start 后台启动 oss-cli server。
cmd_start() {
  if is_running; then
    echo "错误: 服务已在运行 (PID: $(cat "${PID_FILE}"))"
    exit 1
  fi

  if [ ! -f "${CONFIG_FILE}" ]; then
    echo "错误: 未找到 ${CONFIG_FILE}"
    exit 1
  fi

  if [ ! -f "./${APP_NAME}" ]; then
    echo "错误: 未找到二进制 ./${APP_NAME}"
    exit 1
  fi

  if [ ! -f .env.local ]; then
    echo "警告: 未找到 .env.local，请配置 AL_KEY 及 OSS 相关变量"
  fi

  local port
  if [ -n "${1:-}" ]; then
    port="$1"
  elif [ -n "${PORT:-}" ]; then
    port="${PORT}"
  else
    port="$(read_port_from_config)"
  fi

  mkdir -p "${LOG_DIR}"
  chmod +x "./${APP_NAME}"

  echo "=========================================="
  echo "  ${APP_NAME} Server 启动"
  echo "=========================================="
  echo "  端口:    ${port}"
  echo "  配置:    ${CONFIG_FILE}"
  echo "  日志:    ${LOG_FILE}"
  echo "  F5 路由: /v1/dashscope/uploads (Bearer AL_KEY)"
  echo "=========================================="

  nohup "./${APP_NAME}" server -p "${port}" >> "${LOG_FILE}" 2>&1 &
  echo $! > "${PID_FILE}"

  sleep 1
  if is_running; then
    echo "服务已启动 (PID: $(cat "${PID_FILE}"))"
  else
    echo "错误: 服务启动失败，请查看日志: ${LOG_FILE}"
    rm -f "${PID_FILE}"
    exit 1
  fi
}

# cmd_stop 停止 oss-cli server 进程。
cmd_stop() {
  if ! is_running; then
    echo "服务未运行"
    rm -f "${PID_FILE}"
    exit 0
  fi

  local pid
  pid=$(cat "${PID_FILE}")
  echo "正在停止服务 (PID: ${pid})..."

  kill "${pid}" 2>/dev/null || true

  local i
  for i in $(seq 1 10); do
    if ! kill -0 "${pid}" 2>/dev/null; then
      rm -f "${PID_FILE}"
      echo "服务已停止"
      return
    fi
    sleep 1
  done

  echo "进程未响应 SIGTERM，发送 SIGKILL..."
  kill -9 "${pid}" 2>/dev/null || true
  rm -f "${PID_FILE}"
  echo "服务已强制停止"
}

# cmd_status 输出当前服务运行状态。
cmd_status() {
  if is_running; then
    local pid
    pid=$(cat "${PID_FILE}")
    echo "状态: 运行中"
    echo "PID:  ${pid}"
    if [ -f "${LOG_FILE}" ]; then
      echo "日志: ${LOG_FILE} ($(wc -l < "${LOG_FILE}" | tr -d ' ') 行)"
    fi
    exit 0
  fi

  echo "状态: 已停止"
  if [ -f "${PID_FILE}" ]; then
    echo "提示: 发现过期 PID 文件，已清理"
    rm -f "${PID_FILE}"
  fi
  exit 3
}

# cmd_logs 查看或跟踪服务日志。
cmd_logs() {
  local follow=false
  if [ "${1:-}" = "-f" ]; then
    follow=true
  fi

  if [ ! -f "${LOG_FILE}" ]; then
    echo "日志文件不存在: ${LOG_FILE}"
    echo "提示: 服务启动后会自动创建 logs/oss-cli.log"
    exit 1
  fi

  if [ "${follow}" = true ]; then
    echo "跟踪日志: ${LOG_FILE} (Ctrl+C 退出)"
    tail -f "${LOG_FILE}"
  else
    echo "最近日志: ${LOG_FILE}"
    tail -n 100 "${LOG_FILE}"
  fi
}

main() {
  local cmd="${1:-}"
  shift || true

  case "${cmd}" in
    start)
      cmd_start "${1:-}"
      ;;
    stop)
      cmd_stop
      ;;
    status)
      cmd_status
      ;;
    logs)
      cmd_logs "${1:-}"
      ;;
    -h|--help|help|"")
      usage
      exit 0
      ;;
    *)
      echo "错误: 未知命令 '${cmd}'"
      echo ""
      usage
      exit 1
      ;;
  esac
}

main "$@"
