2026-05-29 23:30:00 +0800

修改模型：GPT-5.5（Cursor Agent 模式，012-code-reviewer）

修改目的：安全加固——从远程仓库移除 `config.yaml`（曾含或可含真实凭证），确保 `.env.local` 不被跟踪；新增 `config.yaml.example` 占位模板供本地复制使用。

**问题**：`config.yaml` 被 git 跟踪并推送至远程，存在凭证泄露风险；`.env.local` 虽已在 .gitignore，需与 config 一并明确禁止提交。

**方案**：
- `git rm --cached config.yaml`，本地文件保留
- `.gitignore` 增加 `config.yaml`
- 新增 `config.yaml.example`（占位符，可提交）
- README 说明 `cp config.yaml.example config.yaml`

改动文件：

- .gitignore
- config.yaml（从 git 索引删除，远程不再存在）
- config.yaml.example（新建）
- README.md
- CHANGELOG.md

---

2026-05-29 23:10:00 +0800

修改模型：GPT-5.5（Cursor Agent 模式，012-code-reviewer）

修改目的：版本升级至 **V1.0.4**。解决 OSS 标准获取（F3）无法在签名阶段申请 WebP 格式的问题——私有 Bucket 不能在签名 URL 后拼接 `x-oss-process`，必须在 Agent 侧将图片处理参数纳入签名。

**问题**：`GET /view/{file_id}` 仅支持原图签名或 `w`/`h` 缩略图，列表页等场景无法同时获得更小体积的 WebP 输出。

**方案**：
- 新增 query 参数 `format=webp`，可与 `w`/`h` 组合（process 示例：`image/format,webp`、`image/resize,w_200,h_100,m_fill/format,webp`）
- OSS 层统一 `BuildImageProcess` + `GetViewSignedURL`，`SignURL(..., oss.Process(...))` 纳入签名
- 非图片传 `format=webp` 时忽略并 WARN，向后兼容（不传 format 行为不变）
- 响应在 WebP 生效时增加 `output_format: webp`
- CLI `url` / `thumbnail` 增加 `--format webp`
- 集成测试 H5b/H5c/H5d、C4b；批量验证脚本下载 100 张 WebP（100/100 成功）

改动文件：

- VERSION
- cmd/root.go
- cmd/url.go
- oss/image_process.go（新建）
- oss/image_process_test.go（新建）
- oss/client.go
- server/server.go
- scripts/webp-download-test/main.go（新建）
- scripts/run-integration-tests.sh
- docs/PRD-VIEW-WEBP-FORMAT.md（新建）
- docs/PLAN-VIEW-WEBP-FORMAT.md（新建）
- docs/TEST-REPORT-WEBP-20260529.md（新建）
- docs/ARCHITECTURE.md
- README.md
- .gitignore
- CHANGELOG.md

---

2026-05-29 22:30:00 +0800





修改模型：GPT-5.5（Cursor Agent 模式，005-go-backend-expert + 012-code-reviewer）



修改目的：在 feature/dashscope-instant-upload 分支完成 F5 百炼临时文件上传 MVP：新增独立 dashscope 领域包（getPolicy、multipart 上传、oss:// URL 生成、policy 过期重试）；扩展 config 支持 AL_KEY；新增 HTTP 路由 /v1/dashscope/uploads 与 F5 专用鉴权中间件；新增 CLI dashscope upload/policy（跳过 OSS Init）；新增 oss-cli.sh 服务管理脚本（start/stop/status/logs）；新增 release-build.sh 与集成测试脚本；版本升级至 V1.0.3。



改动文件：





.env.example



.gitignore



VERSION



cmd/dashscope.go



cmd/root.go



config.yaml



config/config.go



dashscope/client.go



dashscope/client_test.go



dashscope/logger.go



dashscope/types.go



docs/ARCHITECTURE-ANALYSIS-DASHSCOPE-INSTANT-UPLOAD.md



docs/ARCHITECTURE.md



docs/PLAN-DASHSCOPE-INSTANT-UPLOAD.md



docs/PRD-DASHSCOPE-INSTANT-UPLOAD.md



oss-cli.sh



release-build.sh



scripts/run-integration-tests.sh



server/dashscope.go



server/dashscope_auth.go



server/server.go



testdata/test-pixel.png



testdata/test-32x32.png



CHANGELOG.md



git 分支：feature/dashscope-instant-upload



提交 hash：a027522



测试与验证结果：





已运行 go test ./... 通过（含 dashscope 包 5 项单测）。



已运行 go vet ./... 通过。



已运行 KEEP_FILES=1 ./scripts/run-integration-tests.sh，HTTP Server + CLI 双模式 17/17 通过；百炼临时文件专项测试含完整 curl 与 oss:// 路径验证（报告 docs/TEST-REPORT-KEEP-20260529_222728.md，未入库）。



合并前代码审查：config.yaml 已恢复占位符，未提交 .env.local / release 产物 / 真实密钥。



🔴 Critical（合并前已处理）：工作区 config.yaml 曾含真实 OSS 凭证，提交前已改回占位符。



🟡 Suggestion：GetUploadPolicy 中 model 参数建议 url.QueryEscape；isDashscopeCommand() 依赖 os.Args 可改为 Cobra PreRun 更稳健。



🟢 Nice to have：server 上传 handler 可改为流式处理减少临时文件；F5 路由可补充 IP 限流。



LGTM（正确性 / 设计边界 / 鉴权隔离 / 测试覆盖）— 与 main 无冲突，已 fast-forward 合并。


