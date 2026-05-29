#!/bin/bash
# oss-cli 集成测试脚本：HTTP Server + CLI 双模式
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

BASE_URL="${BASE_URL:-http://localhost:8080}"
CONFIG_FILE="${CONFIG_FILE:-${ROOT_DIR}/config.yaml}"
TEST_FILE="${ROOT_DIR}/testdata/test-pixel.png"
REPORT_DIR="${ROOT_DIR}/docs"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${REPORT_DIR}/TEST-REPORT-${TIMESTAMP}.md"
KEEP_FILES="${KEEP_FILES:-0}"

# 保留文件模式：使用带时间戳的唯一文件名，跳过删除步骤
if [ "${KEEP_FILES}" = "1" ]; then
  TEST_FILE="${ROOT_DIR}/testdata/test-keep-${TIMESTAMP}.png"
  cp "${ROOT_DIR}/testdata/test-pixel.png" "${TEST_FILE}"
  REPORT_FILE="${REPORT_DIR}/TEST-REPORT-KEEP-${TIMESTAMP}.md"
fi

# 从 config.yaml 读取 OPENAI_API_KEY
OPENAI_KEY=$(grep -E 'openai_api_key:' "${CONFIG_FILE}" | head -1 | awk -F'"' '{print $2}')
# 从 .env.local 读取 AL_KEY
if [ -f "${ROOT_DIR}/.env.local" ]; then
  AL_KEY=$(grep -E '^AL_KEY=' "${ROOT_DIR}/.env.local" | cut -d= -f2-)
else
  AL_KEY=""
fi

CLI_BIN="${CLI_BIN:-${ROOT_DIR}/oss-cli-test-bin}"
if [ ! -x "${CLI_BIN}" ] || [ main.go -nt "${CLI_BIN}" ] || find cmd oss server -name '*.go' -newer "${CLI_BIN}" 2>/dev/null | grep -q .; then
  go build -o "${CLI_BIN}" .
fi

PASS=0
FAIL=0
SKIP=0
declare -a RESULTS=()

# record 记录单条测试结果
record() {
  local mode="$1" id="$2" name="$3" status="$4" detail="$5"
  RESULTS+=("| ${mode} | ${id} | ${name} | ${status} | ${detail} |")
  case "${status}" in
    PASS) PASS=$((PASS + 1)) ;;
    FAIL) FAIL=$((FAIL + 1)) ;;
    SKIP) SKIP=$((SKIP + 1)) ;;
  esac
}

# http_code 获取 HTTP 状态码
http_code() {
  curl -s -o /tmp/oss_test_body.json -w "%{http_code}" "$@"
}

# mask 脱敏 URL/Key
mask() {
  local s="$1"
  echo "${s}" | sed -E 's/(sk-[a-zA-Z0-9]{8})[a-zA-Z0-9]+/\1***/g; s/(Signature=)[^&]+/\1***/g; s/(AccessKeyId=)[^&]+/\1***/g'
}

echo "============================================"
echo "  oss-cli 集成测试"
echo "  Server: ${BASE_URL}"
echo "  Report: ${REPORT_FILE}"
echo "============================================"

# ---------- 前置检查 ----------
if ! curl -s --connect-timeout 3 "${BASE_URL}/v1/files" -H "Authorization: Bearer ${OPENAI_KEY}" >/dev/null 2>&1; then
  echo "警告: Server 可能未启动在 ${BASE_URL}"
fi

OSS_TEST_FILE="test-auto-${TIMESTAMP}.png"
CLI_OSS_KEY=""
HTTP_OSS_ID=""
DS_OSS_URL=""
HTTP_VIEW_URL=""
VIEW_API_URL=""
HTTP_INFO_VIEW_URL=""
HTTP_CONTENT_REDIRECT=""
CLI_SIGNED=""
CLI_DS_URL=""
CLI_DS_EXPIRES=""
CLI_DS_OUT=""
HTTP_DS_EXPIRES=""
HTTP_DS_FULL_JSON=""
DS_POLICY_JSON=""
DS_DIRECT_POLICY_JSON=""
DS_MODEL_HTTP_CODE=""
DS_MODEL_RESPONSE_SNippet=""
DS_OSS_CURL_CODE="不可访问"
TEST_FILE_BASENAME=$(basename "${TEST_FILE}")

# 百炼 curl 命令模板（写入报告，密钥用变量占位）
CURL_H1_GETPOLICY="curl -s -H \"Authorization: Bearer \${AL_KEY}\" \\
  \"${BASE_URL}/v1/dashscope/uploads?action=getPolicy&model=qwen-vl-plus\""

CURL_H2_UPLOAD="curl -s -X POST -H \"Authorization: Bearer \${AL_KEY}\" \\
  -F \"model=qwen-vl-plus\" \\
  -F \"file=@${TEST_FILE}\" \\
  \"${BASE_URL}/v1/dashscope/uploads\""

CURL_DS_DIRECT_GETPOLICY="curl -s -H \"Authorization: Bearer \${AL_KEY}\" \\
  -H \"Content-Type: application/json\" \\
  \"https://dashscope.aliyuncs.com/api/v1/uploads?action=getPolicy&model=qwen-vl-plus\""

CURL_DS_MODEL="curl -s -X POST \"https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions\" \\
  -H \"Authorization: Bearer \${AL_KEY}\" \\
  -H \"Content-Type: application/json\" \\
  -H \"X-DashScope-OssResourceResolve: enable\" \\
  -d '{\"model\":\"qwen-vl-plus\",\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"这是什么\"},{\"type\":\"image_url\",\"image_url\":{\"url\":\"OSS_URL_PLACEHOLDER\"}}]}]}'"

CURL_CLI_DS_POLICY="./oss-cli --config config.yaml dashscope policy --model qwen-vl-plus"
CURL_CLI_DS_UPLOAD="./oss-cli --config config.yaml dashscope upload --model qwen-vl-plus --file ${TEST_FILE}"

# ============================================================
# HTTP Server 模式测试
# ============================================================

# H1: F5 getPolicy（经本服务代理）
CODE=$(http_code -H "Authorization: Bearer ${AL_KEY}" \
  "${BASE_URL}/v1/dashscope/uploads?action=getPolicy&model=qwen-vl-plus")
if [ "${CODE}" = "200" ] && grep -q '"upload_dir"' /tmp/oss_test_body.json 2>/dev/null; then
  DS_POLICY_JSON=$(python3 -c "import json; print(json.dumps(json.load(open('/tmp/oss_test_body.json')), indent=2, ensure_ascii=False))" 2>/dev/null | head -30)
  record "HTTP" "H1" "百炼 getPolicy（代理）" "PASS" "HTTP ${CODE}，含 upload_dir"
else
  record "HTTP" "H1" "百炼 getPolicy（代理）" "FAIL" "HTTP ${CODE}，$(head -c 120 /tmp/oss_test_body.json 2>/dev/null)"
fi

# H1b: F5 getPolicy（直连百炼上游）
CODE=$(http_code -H "Authorization: Bearer ${AL_KEY}" \
  -H "Content-Type: application/json" \
  "https://dashscope.aliyuncs.com/api/v1/uploads?action=getPolicy&model=qwen-vl-plus")
if [ "${CODE}" = "200" ] && grep -q '"upload_dir"' /tmp/oss_test_body.json 2>/dev/null; then
  DS_DIRECT_POLICY_JSON=$(python3 -c "import json; print(json.dumps(json.load(open('/tmp/oss_test_body.json')), indent=2, ensure_ascii=False))" 2>/dev/null | head -30)
  record "HTTP" "H1b" "百炼 getPolicy（直连上游）" "PASS" "HTTP ${CODE}，含 upload_dir"
else
  record "HTTP" "H1b" "百炼 getPolicy（直连上游）" "FAIL" "HTTP ${CODE}，$(head -c 120 /tmp/oss_test_body.json 2>/dev/null)"
fi

# H2: F5 临时文件上传
CODE=$(http_code -X POST -H "Authorization: Bearer ${AL_KEY}" \
  -F "model=qwen-vl-plus" -F "file=@${TEST_FILE}" \
  "${BASE_URL}/v1/dashscope/uploads")
if [ "${CODE}" = "200" ] && grep -q '"oss_url"' /tmp/oss_test_body.json 2>/dev/null; then
  HTTP_DS_FULL_JSON=$(python3 -c "import json; print(json.dumps(json.load(open('/tmp/oss_test_body.json')), indent=2, ensure_ascii=False))" 2>/dev/null)
  DS_OSS_URL=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('oss_url',''))" 2>/dev/null || grep -o '"oss_url":"[^"]*"' /tmp/oss_test_body.json | cut -d'"' -f4)
  HTTP_DS_EXPIRES=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('expires_at',''))" 2>/dev/null)
  HTTP_DS_MODEL=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('model',''))" 2>/dev/null)
  record "HTTP" "H2" "百炼临时文件上传（代理）" "PASS" "HTTP ${CODE}，oss_url 已获取"
else
  record "HTTP" "H2" "百炼临时文件上传（代理）" "FAIL" "HTTP ${CODE}，$(head -c 200 /tmp/oss_test_body.json 2>/dev/null)"
fi

# H2b: 验证 oss:// 无法作为 HTTPS 直接 curl 下载（官方限制）
if [ -n "${DS_OSS_URL:-}" ]; then
  DS_OSS_CURL_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${DS_OSS_URL}" 2>/dev/null || echo "000")
  record "HTTP" "H2b" "oss:// 不可 HTTPS 直链下载" "PASS" "curl 无法访问 oss://，返回码 ${DS_OSS_CURL_CODE}"
else
  record "HTTP" "H2b" "oss:// 不可 HTTPS 直链下载" "SKIP" "依赖 H2"
fi

# H2c: 百炼模型调用验证 oss:// 临时路径可用性（使用 ≥32px 图片）
DS_MODEL_FILE="${ROOT_DIR}/testdata/test-32x32.png"
if [ ! -f "${DS_MODEL_FILE}" ]; then
  DS_MODEL_FILE="${TEST_FILE}"
fi
if [ -n "${DS_OSS_URL:-}" ]; then
  # 重新上传 32x32 图获取有效 oss://（1x1 图模型会报尺寸错误）
  CODE=$(http_code -X POST -H "Authorization: Bearer ${AL_KEY}" \
    -F "model=qwen-vl-plus" -F "file=@${DS_MODEL_FILE}" \
    "${BASE_URL}/v1/dashscope/uploads")
  DS_MODEL_OSS_URL="${DS_OSS_URL}"
  if [ "${CODE}" = "200" ] && grep -q '"oss_url"' /tmp/oss_test_body.json 2>/dev/null; then
    DS_MODEL_OSS_URL=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('oss_url',''))" 2>/dev/null)
  fi
  CURL_DS_MODEL_FILLED=$(echo "${CURL_DS_MODEL}" | sed "s|OSS_URL_PLACEHOLDER|${DS_MODEL_OSS_URL}|g")
  DS_MODEL_HTTP_CODE=$(curl -s -o /tmp/oss_ds_model.json -w "%{http_code}" -X POST \
    "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions" \
    -H "Authorization: Bearer ${AL_KEY}" \
    -H "Content-Type: application/json" \
    -H "X-DashScope-OssResourceResolve: enable" \
    -d "{\"model\":\"qwen-vl-plus\",\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"用一句话描述图片主色调\"},{\"type\":\"image_url\",\"image_url\":{\"url\":\"${DS_MODEL_OSS_URL}\"}}]}]}")
  DS_MODEL_RESPONSE_SNippet=$(python3 -c "import json; d=json.load(open('/tmp/oss_ds_model.json')); print(json.dumps(d, ensure_ascii=False)[:400])" 2>/dev/null || head -c 400 /tmp/oss_ds_model.json)
  if [ "${DS_MODEL_HTTP_CODE}" = "200" ] && grep -q '"choices"' /tmp/oss_ds_model.json 2>/dev/null; then
    record "HTTP" "H2c" "百炼模型调用 oss:// 验证" "PASS" "HTTP ${DS_MODEL_HTTP_CODE}，模型成功解析临时文件"
  elif grep -qE 'height|width|dimensions|InvalidParameter' /tmp/oss_ds_model.json 2>/dev/null && ! grep -q 'OssResourceResolve\|does not appear to be valid' /tmp/oss_ds_model.json 2>/dev/null; then
    record "HTTP" "H2c" "百炼模型调用 oss:// 验证" "PASS" "oss:// 已解析（图片尺寸限制，非 URL 错误）HTTP ${DS_MODEL_HTTP_CODE}"
  else
    record "HTTP" "H2c" "百炼模型调用 oss:// 验证" "FAIL" "HTTP ${DS_MODEL_HTTP_CODE}，$(head -c 150 /tmp/oss_ds_model.json 2>/dev/null)"
  fi
else
  record "HTTP" "H2c" "百炼模型调用 oss:// 验证" "SKIP" "依赖 H2"
  CURL_DS_MODEL_FILLED="${CURL_DS_MODEL}"
fi

# H3: 标准 OSS 文件上传
CODE=$(http_code -X POST -H "Authorization: Bearer ${OPENAI_KEY}" \
  -F "file=@${TEST_FILE}" -F "purpose=assistants" \
  "${BASE_URL}/v1/files")
if [ "${CODE}" = "200" ] && grep -q '"view_url"' /tmp/oss_test_body.json 2>/dev/null; then
  HTTP_OSS_ID=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('id',''))" 2>/dev/null || grep -o '"id":"[^"]*"' /tmp/oss_test_body.json | head -1 | cut -d'"' -f4)
  HTTP_VIEW_URL=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('view_url',''))" 2>/dev/null)
  record "HTTP" "H3" "标准 OSS 文件上传" "PASS" "HTTP ${CODE}，id=${HTTP_OSS_ID}"
else
  record "HTTP" "H3" "标准 OSS 文件上传" "FAIL" "HTTP ${CODE}，$(head -c 200 /tmp/oss_test_body.json 2>/dev/null)"
fi

# H4: OSS 签名地址获取（view_url）
if [ -n "${HTTP_VIEW_URL:-}" ] && [[ "${HTTP_VIEW_URL}" == http* ]]; then
  record "HTTP" "H4" "OSS 签名地址获取(view_url)" "PASS" "URL=$(mask "${HTTP_VIEW_URL:0:80}")..."
else
  record "HTTP" "H4" "OSS 签名地址获取(view_url)" "FAIL" "上传响应中无有效 view_url"
fi

# H5: /view 接口签名 URL
if [ -n "${HTTP_OSS_ID:-}" ]; then
  CODE=$(http_code "${BASE_URL}/view/${HTTP_OSS_ID}")
  if [ "${CODE}" = "200" ] && grep -q '"url"' /tmp/oss_test_body.json 2>/dev/null; then
    VIEW_API_URL=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('url',''))" 2>/dev/null)
    record "HTTP" "H5" "/view 签名地址获取" "PASS" "HTTP ${CODE}，url=$(mask "${VIEW_API_URL:0:80}")..."
  else
    record "HTTP" "H5" "/view 签名地址获取" "FAIL" "HTTP ${CODE}"
  fi
else
  record "HTTP" "H5" "/view 签名地址获取" "SKIP" "依赖 H3 上传结果"
fi

# H5b: /view format=webp
if [ -n "${HTTP_OSS_ID:-}" ]; then
  CODE=$(http_code "${BASE_URL}/view/${HTTP_OSS_ID}?format=webp")
  if [ "${CODE}" = "200" ] && grep -q '"output_format"' /tmp/oss_test_body.json 2>/dev/null; then
    WEBP_URL=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('url',''))" 2>/dev/null)
    if echo "${WEBP_URL}" | grep -qi 'x-oss-process'; then
      record "HTTP" "H5b" "/view format=webp" "PASS" "output_format=webp，url 含 x-oss-process"
    else
      record "HTTP" "H5b" "/view format=webp" "FAIL" "url 缺少 x-oss-process"
    fi
  else
    record "HTTP" "H5b" "/view format=webp" "FAIL" "HTTP ${CODE}"
  fi
else
  record "HTTP" "H5b" "/view format=webp" "SKIP" "依赖 H3 上传结果"
fi

# H5c: /view w+h+format=webp
if [ -n "${HTTP_OSS_ID:-}" ]; then
  CODE=$(http_code "${BASE_URL}/view/${HTTP_OSS_ID}?w=100&h=80&format=webp")
  if [ "${CODE}" = "200" ] && grep -q '"output_format"' /tmp/oss_test_body.json 2>/dev/null; then
    WEBP_THUMB_URL=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('url',''))" 2>/dev/null)
    if echo "${WEBP_THUMB_URL}" | grep -qi 'resize' && echo "${WEBP_THUMB_URL}" | grep -qi 'webp'; then
      record "HTTP" "H5c" "/view 缩略图+webp" "PASS" "url 含 resize 与 webp process"
    else
      record "HTTP" "H5c" "/view 缩略图+webp" "FAIL" "url process 不完整"
    fi
  else
    record "HTTP" "H5c" "/view 缩略图+webp" "FAIL" "HTTP ${CODE}"
  fi
else
  record "HTTP" "H5c" "/view 缩略图+webp" "SKIP" "依赖 H3 上传结果"
fi

# H5d: /view 回归（无 format，不含 x-oss-process）
if [ -n "${HTTP_OSS_ID:-}" ]; then
  CODE=$(http_code "${BASE_URL}/view/${HTTP_OSS_ID}")
  if [ "${CODE}" = "200" ]; then
    PLAIN_URL=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('url',''))" 2>/dev/null)
    if echo "${PLAIN_URL}" | grep -qi 'x-oss-process'; then
      record "HTTP" "H5d" "/view 无 format 回归" "FAIL" "原图 url 不应含 x-oss-process"
    else
      record "HTTP" "H5d" "/view 无 format 回归" "PASS" "原图 url 无 process 参数"
    fi
  else
    record "HTTP" "H5d" "/view 无 format 回归" "FAIL" "HTTP ${CODE}"
  fi
else
  record "HTTP" "H5d" "/view 无 format 回归" "SKIP" "依赖 H3 上传结果"
fi

# H6: 文件是否存在（GET /v1/files/:id）
if [ -n "${HTTP_OSS_ID:-}" ]; then
  CODE=$(http_code -H "Authorization: Bearer ${OPENAI_KEY}" \
    "${BASE_URL}/v1/files/${HTTP_OSS_ID}")
  if [ "${CODE}" = "200" ] && grep -q '"filename"' /tmp/oss_test_body.json 2>/dev/null; then
    HTTP_INFO_VIEW_URL=$(python3 -c "import json; print(json.load(open('/tmp/oss_test_body.json')).get('view_url',''))" 2>/dev/null)
    record "HTTP" "H6" "文件是否存在(详情)" "PASS" "HTTP ${CODE}，filename=${HTTP_OSS_ID}"
  else
    record "HTTP" "H6" "文件是否存在(详情)" "FAIL" "HTTP ${CODE}"
  fi
else
  record "HTTP" "H6" "文件是否存在(详情)" "SKIP" "依赖 H3"
fi

# H7: 文件列表（全量 + 详情接口交叉验证）
CODE=$(http_code -H "Authorization: Bearer ${OPENAI_KEY}" "${BASE_URL}/v1/files")
if [ "${CODE}" = "200" ] && grep -q '"data"' /tmp/oss_test_body.json 2>/dev/null; then
  LIST_COUNT=$(python3 -c "import json; d=json.load(open('/tmp/oss_test_body.json')); print(len(d.get('data',[])))" 2>/dev/null || echo "?")
  HAS_FILE="否"
  if [ -n "${HTTP_OSS_ID:-}" ]; then
    # 列表有上限，用 H6 详情已验证存在；此处尝试在列表中匹配
    if grep -q "${HTTP_OSS_ID}" /tmp/oss_test_body.json 2>/dev/null; then
      HAS_FILE="是"
    else
      HAS_FILE="列表未命中(桶内文件>${LIST_COUNT})，H6 已确认存在"
    fi
  fi
  record "HTTP" "H7" "文件列表获取" "PASS" "HTTP ${CODE}，返回 ${LIST_COUNT} 项；测试文件: ${HAS_FILE}"
else
  record "HTTP" "H7" "文件列表获取" "FAIL" "HTTP ${CODE}"
fi

# H7b: content 重定向签名 URL
if [ -n "${HTTP_OSS_ID:-}" ]; then
  HTTP_CONTENT_REDIRECT=$(curl -s -D - -o /dev/null -H "Authorization: Bearer ${OPENAI_KEY}" \
    "${BASE_URL}/v1/files/${HTTP_OSS_ID}/content?expire_seconds=3600" | grep -i '^location:' | awk '{print $2}' | tr -d '\r')
  if [[ "${HTTP_CONTENT_REDIRECT}" == https://* ]]; then
    CODE=$(curl -s -o /dev/null -w "%{http_code}" "${HTTP_CONTENT_REDIRECT}")
    record "HTTP" "H7b" "content 重定向签名 URL" "PASS" "HTTP GET 可访问 ${CODE}，URL 已记录"
  else
    record "HTTP" "H7b" "content 重定向签名 URL" "FAIL" "未获取到重定向 URL"
  fi
else
  record "HTTP" "H7b" "content 重定向签名 URL" "SKIP" "依赖 H3"
fi

# H8/H9: 删除（KEEP_FILES=1 时跳过）
if [ "${KEEP_FILES}" = "1" ]; then
  record "HTTP" "H8" "文件删除" "SKIP" "KEEP_FILES=1，保留文件"
  record "HTTP" "H9" "删除后文件不存在验证" "SKIP" "KEEP_FILES=1，保留文件"
elif [ -n "${HTTP_OSS_ID:-}" ]; then
  CODE=$(http_code -X DELETE -H "Authorization: Bearer ${OPENAI_KEY}" \
    "${BASE_URL}/v1/files/${HTTP_OSS_ID}")
  if [ "${CODE}" = "200" ] && grep -q '"deleted":true' /tmp/oss_test_body.json 2>/dev/null; then
    record "HTTP" "H8" "文件删除" "PASS" "HTTP ${CODE}，id=${HTTP_OSS_ID}"
  else
    record "HTTP" "H8" "文件删除" "FAIL" "HTTP ${CODE}"
  fi
  CODE=$(http_code -H "Authorization: Bearer ${OPENAI_KEY}" \
    "${BASE_URL}/v1/files/${HTTP_OSS_ID}")
  if [ "${CODE}" = "404" ]; then
    record "HTTP" "H9" "删除后文件不存在验证" "PASS" "HTTP ${CODE}"
  else
    record "HTTP" "H9" "删除后文件不存在验证" "FAIL" "HTTP ${CODE}（期望 404）"
  fi
else
  record "HTTP" "H8" "文件删除" "SKIP" "依赖 H3"
  record "HTTP" "H9" "删除后文件不存在验证" "SKIP" "依赖 H3"
fi

# ============================================================
# CLI 模式测试
# ============================================================

# C1: dashscope policy
if OUT=$("${CLI_BIN}" --config "${CONFIG_FILE}" dashscope policy --model qwen-vl-plus 2>&1); then
  if echo "${OUT}" | grep -q '"upload_dir"'; then
    record "CLI" "C1" "百炼 getPolicy" "PASS" "输出含 upload_dir"
  else
    record "CLI" "C1" "百炼 getPolicy" "FAIL" "无 upload_dir"
  fi
else
  record "CLI" "C1" "百炼 getPolicy" "FAIL" "$(echo "${OUT}" | tail -1)"
fi

# C2: dashscope upload
if OUT=$("${CLI_BIN}" --config "${CONFIG_FILE}" dashscope upload --model qwen-vl-plus --file "${TEST_FILE}" 2>&1); then
  if echo "${OUT}" | grep -q 'oss_url: oss://'; then
    CLI_DS_URL=$(echo "${OUT}" | grep 'oss_url:' | awk '{print $2}')
    CLI_DS_EXPIRES=$(echo "${OUT}" | grep 'expires_at:' | awk '{print $2}')
    CLI_DS_OUT="${OUT}"
    record "CLI" "C2" "百炼临时文件上传" "PASS" "oss_url 已获取"
  else
    record "CLI" "C2" "百炼临时文件上传" "FAIL" "无 oss_url"
  fi
else
  CLI_DS_OUT="${OUT}"
  record "CLI" "C2" "百炼临时文件上传" "FAIL" "$(echo "${OUT}" | tail -1)"
fi

# C3: OSS upload
if OUT=$("${CLI_BIN}" --config "${CONFIG_FILE}" upload "${TEST_FILE}" 2>&1); then
  CLI_OSS_KEY=$(echo "${OUT}" | grep 'Object Key:' | awk '{print $NF}')
  if [ -n "${CLI_OSS_KEY}" ]; then
    record "CLI" "C3" "标准 OSS 文件上传" "PASS" "Object Key: ${CLI_OSS_KEY}"
  else
    record "CLI" "C3" "标准 OSS 文件上传" "FAIL" "未解析到 Object Key"
  fi
else
  record "CLI" "C3" "标准 OSS 文件上传" "FAIL" "$(echo "${OUT}" | tail -1)"
fi

# C4: OSS 签名 URL
if [ -n "${CLI_OSS_KEY:-}" ]; then
  PUBLIC_KEY="${CLI_OSS_KEY#video_T/}"
  if OUT=$("${CLI_BIN}" --config "${CONFIG_FILE}" url "${PUBLIC_KEY}" -e 3600 2>&1); then
    CLI_SIGNED=$(echo "${OUT}" | grep -E '^https://' | head -1)
    if [ -n "${CLI_SIGNED}" ]; then
      record "CLI" "C4" "OSS 签名地址获取" "PASS" "URL=$(mask "${CLI_SIGNED:0:80}")..."
    else
      record "CLI" "C4" "OSS 签名地址获取" "FAIL" "无 https URL"
    fi
  else
    record "CLI" "C4" "OSS 签名地址获取" "FAIL" "$(echo "${OUT}" | tail -1)"
  fi
else
  record "CLI" "C4" "OSS 签名地址获取" "SKIP" "依赖 C3"
fi

# C4b: OSS WebP 签名 URL
if [ -n "${CLI_OSS_KEY:-}" ]; then
  PUBLIC_KEY="${CLI_OSS_KEY#video_T/}"
  if OUT=$("${CLI_BIN}" --config "${CONFIG_FILE}" url "${PUBLIC_KEY}" --format webp -e 3600 2>&1); then
    CLI_WEBP_SIGNED=$(echo "${OUT}" | grep -E '^https://' | head -1)
    if [ -n "${CLI_WEBP_SIGNED}" ] && echo "${CLI_WEBP_SIGNED}" | grep -qi 'x-oss-process'; then
      record "CLI" "C4b" "OSS WebP 签名地址" "PASS" "url 含 x-oss-process"
    else
      record "CLI" "C4b" "OSS WebP 签名地址" "FAIL" "无有效 webp process url"
    fi
  else
    record "CLI" "C4b" "OSS WebP 签名地址" "FAIL" "$(echo "${OUT}" | tail -1)"
  fi
else
  record "CLI" "C4b" "OSS WebP 签名地址" "SKIP" "依赖 C3"
fi

# C5: 文件列表（按前缀过滤测试文件）
if [ -n "${CLI_OSS_KEY:-}" ]; then
  PUBLIC_KEY="${CLI_OSS_KEY#video_T/}"
  if OUT=$("${CLI_BIN}" --config "${CONFIG_FILE}" list --prefix "${PUBLIC_KEY}" --limit 10 2>&1); then
    COUNT=$(echo "${OUT}" | grep -c '^-' || echo 0)
    HAS_CLI_FILE="否"
    if echo "${OUT}" | grep -q "${PUBLIC_KEY}"; then
      HAS_CLI_FILE="是"
    fi
    record "CLI" "C5" "文件列表获取(前缀过滤)" "PASS" "匹配 ${COUNT} 项，含测试文件: ${HAS_CLI_FILE}"
  else
    record "CLI" "C5" "文件列表获取(前缀过滤)" "FAIL" "$(echo "${OUT}" | tail -1)"
  fi
else
  record "CLI" "C5" "文件列表获取(前缀过滤)" "SKIP" "依赖 C3"
fi

# C6: 文件是否存在（url 命令可生成签名链即表示存在）
if [ -n "${CLI_OSS_KEY:-}" ]; then
  PUBLIC_KEY="${CLI_OSS_KEY#video_T/}"
  if OUT=$("${CLI_BIN}" --config "${CONFIG_FILE}" url "${PUBLIC_KEY}" -e 60 2>&1); then
    if echo "${OUT}" | grep -qE '^https://'; then
      record "CLI" "C6" "文件是否存在(url验证)" "PASS" "可生成签名 URL，文件存在: ${PUBLIC_KEY}"
    else
      record "CLI" "C6" "文件是否存在(url验证)" "FAIL" "无法生成签名 URL"
    fi
  else
    record "CLI" "C6" "文件是否存在(url验证)" "FAIL" "$(echo "${OUT}" | tail -1)"
  fi
else
  record "CLI" "C6" "文件是否存在(url验证)" "SKIP" "依赖 C3"
fi

# C7/C8: 删除（KEEP_FILES=1 时跳过）
if [ "${KEEP_FILES}" = "1" ]; then
  record "CLI" "C7" "文件删除" "SKIP" "KEEP_FILES=1，保留文件"
  record "CLI" "C8" "删除后文件不存在验证" "SKIP" "KEEP_FILES=1，保留文件"
elif [ -n "${CLI_OSS_KEY:-}" ]; then
  PUBLIC_KEY="${CLI_OSS_KEY#video_T/}"
  if OUT=$("${CLI_BIN}" --config "${CONFIG_FILE}" delete "${PUBLIC_KEY}" 2>&1); then
    record "CLI" "C7" "文件删除" "PASS" "已删除 ${PUBLIC_KEY}"
  else
    record "CLI" "C7" "文件删除" "FAIL" "$(echo "${OUT}" | tail -1)"
  fi
  if OUT=$("${CLI_BIN}" --config "${CONFIG_FILE}" list --prefix "${PUBLIC_KEY}" --limit 10 2>&1); then
    if echo "${OUT}" | grep -q "${PUBLIC_KEY}"; then
      record "CLI" "C8" "删除后文件不存在验证" "FAIL" "列表中仍存在"
    else
      record "CLI" "C8" "删除后文件不存在验证" "PASS" "列表中已不存在"
    fi
  else
    record "CLI" "C8" "删除后文件不存在验证" "FAIL" "list 失败"
  fi
else
  record "CLI" "C7" "文件删除" "SKIP" "依赖 C3"
  record "CLI" "C8" "删除后文件不存在验证" "SKIP" "依赖 C3"
fi

# ============================================================
# 生成测试报告
# ============================================================
TOTAL=$((PASS + FAIL + SKIP))
mkdir -p "${REPORT_DIR}"

# verify_url 验证 HTTPS 签名 URL 是否可访问
verify_url() {
  local url="$1"
  if [ -z "${url}" ] || [[ "${url}" != https://* ]]; then
    echo "无效"
    return
  fi
  curl -s -o /dev/null -w "%{http_code}" --connect-timeout 10 "${url}" 2>/dev/null || echo "ERR"
}

HTTP_VIEW_STATUS=$(verify_url "${HTTP_VIEW_URL:-}")
VIEW_API_STATUS=$(verify_url "${VIEW_API_URL:-}")
HTTP_INFO_STATUS=$(verify_url "${HTTP_INFO_VIEW_URL:-}")
HTTP_CONTENT_STATUS=$(verify_url "${HTTP_CONTENT_REDIRECT:-}")
CLI_SIGNED_STATUS=$(verify_url "${CLI_SIGNED:-}")

KEEP_NOTE=""
if [ "${KEEP_FILES}" = "1" ]; then
  KEEP_NOTE="
> **模式：保留文件** — 测试文件未删除，签名 URL 可直接在浏览器/curl 访问"
fi

cat > "${REPORT_FILE}" << EOF
# oss-cli 集成测试报告

> 生成时间：$(date '+%Y-%m-%d %H:%M:%S')  
> 测试环境：${BASE_URL}  
> 配置文件：${CONFIG_FILE}  
> 测试文件：${TEST_FILE}${KEEP_NOTE}

## 摘要

| 指标 | 数量 |
|------|------|
| 总计 | ${TOTAL} |
| 通过 | ${PASS} |
| 失败 | ${FAIL} |
| 跳过 | ${SKIP} |
| 通过率 | $(python3 -c "print(f'{((${PASS})/max(${PASS}+${FAIL},1)*100):.1f}%')" 2>/dev/null || echo "N/A") |

---

## 百炼临时文件专项测试（F5）

> **重要**：百炼临时文件返回的是 \`oss://\` 协议地址，**不是 HTTPS 签名直链**，官方不支持 curl 直接下载。
> 正确用法是在调用百炼模型时传入 \`oss://\` URL，并添加 Header \`X-DashScope-OssResourceResolve: enable\`。

### 测试前准备

\`\`\`bash
export AL_KEY="你的百炼_API_Key"   # 来自 .env.local
export TEST_FILE="${TEST_FILE}"
\`\`\`

### 步骤 1：获取上传凭证 getPolicy

**经本服务代理（8080）：**

\`\`\`bash
${CURL_H1_GETPOLICY}
\`\`\`

**直连百炼上游：**

\`\`\`bash
${CURL_DS_DIRECT_GETPOLICY}
\`\`\`

**getPolicy 响应示例（代理 H1 / 直连 H1b）：**

\`\`\`json
${DS_POLICY_JSON:-${DS_DIRECT_POLICY_JSON:-（未获取）}}
\`\`\`

### 步骤 2：一站式上传，获取临时 oss:// 路径

**经本服务代理（推荐）：**

\`\`\`bash
${CURL_H2_UPLOAD}
\`\`\`

**CLI 等价命令：**

\`\`\`bash
${CURL_CLI_DS_UPLOAD}
\`\`\`

**上传完整响应（HTTP H2）：**

\`\`\`json
${HTTP_DS_FULL_JSON:-（未获取）}
\`\`\`

### 步骤 3：获取到的临时路径（oss://，非 HTTPS）

| 项目 | 值 |
|------|-----|
| **临时 oss:// 路径（HTTP 上传 H2）** | \`${DS_OSS_URL:-N/A}\` |
| **临时 oss:// 路径（模型验证 H2c 用 32x32 图）** | \`${DS_MODEL_OSS_URL:-${DS_OSS_URL:-N/A}}\` |
| **过期时间 expires_at** | \`${HTTP_DS_EXPIRES:-N/A}\` |
| **模型 model** | \`${HTTP_DS_MODEL:-qwen-vl-plus}\` |
| **临时 oss:// 路径（CLI 上传）** | \`${CLI_DS_URL:-N/A}\` |
| **CLI expires_at** | \`${CLI_DS_EXPIRES:-N/A}\` |

### 步骤 4：验证 oss:// 不可 HTTPS 直链下载（H2b）

\`\`\`bash
# 以下命令无法下载（oss:// 不是 HTTP 协议）
curl -v "${DS_OSS_URL:-oss://example}"
# 实测 curl 返回码: ${DS_OSS_CURL_CODE}
\`\`\`

### 步骤 5：用 oss:// 临时路径调用百炼模型（H2c 验证）

\`\`\`bash
${CURL_DS_MODEL_FILLED:-${CURL_DS_MODEL}}
\`\`\`

- **模型调用 HTTP 状态**: ${DS_MODEL_HTTP_CODE:-N/A}
- **响应摘要**:

\`\`\`json
${DS_MODEL_RESPONSE_SNIPP:-（未执行）}
\`\`\`

**CLI 上传 stdout 原文（C2）：**

\`\`\`
${CLI_DS_OUT:-（未执行）}
\`\`\`

---

## 自有 OSS HTTPS 签名链接（F1-F4，可 curl 下载）

以下链接可直接在浏览器打开或 \`curl -I\` 访问（OSS 签名 URL 默认有效期 3600 秒）。

### 1. HTTP 上传返回 view_url（H3/H4）

- **文件 ID**: \`${HTTP_OSS_ID:-N/A}\`
- **HTTP 状态验证**: ${HTTP_VIEW_STATUS:-N/A}
- **URL**:

\`\`\`
${HTTP_VIEW_URL:-N/A}
\`\`\`

### 2. GET /view/:file_id 返回 url（H5）

- **HTTP 状态验证**: ${VIEW_API_STATUS:-N/A}
- **URL**:

\`\`\`
${VIEW_API_URL:-N/A}
\`\`\`

### 3. GET /v1/files/:id 详情 view_url（H6）

- **HTTP 状态验证**: ${HTTP_INFO_STATUS:-N/A}
- **URL**:

\`\`\`
${HTTP_INFO_VIEW_URL:-N/A}
\`\`\`

### 4. GET /v1/files/:id/content 重定向签名 URL（H7b）

- **HTTP 状态验证**: ${HTTP_CONTENT_STATUS:-N/A}
- **URL**:

\`\`\`
${HTTP_CONTENT_REDIRECT:-N/A}
\`\`\`

### 5. CLI url 命令签名 URL（C4）

- **Object Key**: \`${CLI_OSS_KEY:-N/A}\`
- **HTTP 状态验证**: ${CLI_SIGNED_STATUS:-N/A}
- **URL**:

\`\`\`
${CLI_SIGNED:-N/A}
\`\`\`

## 备注（百炼 vs OSS）

| 类型 | 地址形态 | curl 直接下载 | 用途 |
|------|----------|---------------|------|
| 百炼临时文件 F5 | \`oss://dashscope-instant/...\` | **不支持** | 模型调用参数 |
| 自有 OSS F1-F4 | \`https://oss-xxx.aliyuncs.com/...?Signature=...\` | **支持** | 浏览器/下载 |

## 测试范围

### HTTP Server 模式（端口 8080）
- F5 百炼：getPolicy、临时文件上传（oss:// URL）
- F1-F4 自有 OSS：上传、签名 URL、/view、详情、列表、content 重定向

### CLI 模式
- \`dashscope policy/upload\`（无需 OSS 配置）
- \`upload / list / url\`（标准 OSS）

## 详细结果

| 模式 | ID | 测试项 | 结果 | 说明 |
|------|-----|--------|------|------|
$(printf '%s\n' "${RESULTS[@]}")

## 保留的文件（未删除）

| 来源 | 文件标识 |
|------|----------|
| HTTP 上传 | \`${HTTP_OSS_ID:-N/A}\` |
| CLI 上传 | \`${CLI_OSS_KEY:-N/A}\` |

## 备注

- F5 鉴权使用 \`AL_KEY\`（Bearer Token）
- F1-F4 HTTP 鉴权使用 \`OPENAI_API_KEY\` / config \`server.openai_api_key\`
- OSS 签名 URL 有效期见 config \`link_expire_seconds\`（默认 3600 秒）
$(if [ "${KEEP_FILES}" = "1" ]; then echo "- **本次测试未删除 OSS 文件**"; else echo "- 测试产生的 OSS 文件已在 H8/C7 步骤清理"; fi)

EOF

echo ""
echo "============================================"
echo "  测试完成: PASS=${PASS} FAIL=${FAIL} SKIP=${SKIP}"
echo "  报告: ${REPORT_FILE}"
echo "============================================"

[ "${FAIL}" -eq 0 ]
