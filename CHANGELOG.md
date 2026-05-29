2026-05-29 23:45:00 +0800

修改模型：GPT-5.5（Cursor Agent 模式，005-go-backend-expert + 012-code-reviewer）

修改目的：**补救 V1.0.5 配置模板遗漏**——补全 `config.yaml.example` 与 `release-build.sh` 中的保存周期配置项，并在 CHANGELOG 记录过程性错误，防止同类问题再次发生。

**过程性错误（V1.0.5 首次提交 `b90e80f`）**：

| 项 | 应有做法 | 实际疏漏 |
|----|----------|----------|
| 需求约定 | `config.yaml` 增加 `default_retention_years` 等可选项 | 只实现了 Go 代码与 `config/config.go` 字段，**未在可提交的配置模板中写入显式示例值** |
| `config.yaml.example` | 与 PRD 一致，写出 `default_retention_years: 2` 等 | 仅加了**注释行**（`# default_retention_years: 2`），运维/用户复制后看不到完整配置结构 |
| `release-build.sh` 内嵌模板 | 与 `config.yaml.example` 保持同步 | 同样只有注释，release 包内 config 缺字段 |
| 本地 `config.yaml` | 部署前更新（gitignore，不提交） | 合并推送后**未同步更新**本地 config，用户发现「约点未落地」 |

**根因**：实现功能时以代码路径为验收标准，**未将「配置模板三件套」纳入同一 PR 检查清单**（`config.yaml.example` ↔ `release-build.sh` ↔ README 配置说明）。

**补救**：
- `config.yaml.example`：写入显式 `default_retention_years: 2`、`allowed_retention_years: [2, 3, 5, 10]`、`allowed_retention_days: [1]`
- `release-build.sh`：release 包内 config 模板与 example **字段对齐**
- 代码层向下兼容不变：缺字段仍 fallback 默认 2 年（见 `config/retention.go`）

**防再犯检查清单（新增配置项时必须逐项勾选）**：

1. [ ] `config/config.go` 结构体 + viper/mapstructure
2. [ ] `config.yaml.example` **显式示例值**（禁止仅注释占位）
3. [ ] `release-build.sh` 内嵌 config 与 example **同步**
4. [ ] README 配置段说明（若面向用户）
5. [ ] CHANGELOG 改动文件列表含上述路径
6. [ ] 提醒更新本地 `config.yaml`（gitignore，不提交）

改动文件：

- config.yaml.example
- release-build.sh
- CHANGELOG.md

---

2026-05-29 23:40:00 +0800

修改模型：GPT-5.5（Cursor Agent 模式，005-go-backend-expert + 012-code-reviewer）

修改目的：版本升级至 **V1.0.5**。为 OSS 对象引入保存周期（Tag + Lifecycle 自动删除）：配置默认 2 年、上传可指定 `retention_years` / `retention_days`、存量脚本批量打 3 年 tag；删除由 OSS Lifecycle Expiration 执行，应用侧无 cleanup。

**问题**：Bucket 未配置 Lifecycle，1000+ 对象永久保留；上传无法按文件指定保留年限。

**方案**：
- `config.yaml` 可选 `default_retention_years`（默认 2）、`allowed_retention_years`、`allowed_retention_days`（测试最短 1 天，OSS 不支持分钟/小时级）
- 上传时写入对象 Tag `retention-years` / `retention-days`，Bucket Lifecycle 按 Prefix+Tag 匹配 Expiration Days
- `POST /v1/files` 支持 `retention_years`、`retention_days`；响应含 `retention_until`
- CLI `upload --retention-years` / `--retention-days`
- 运维脚本：`sync-lifecycle-rules.sh`（Get→merge→Put 规则）、`set-default-retention.sh`（存量 PutObjectTagging，默认 3 年）
- `scripts/oss-inventory/` 输出含 tag 与预计删除时间的 Markdown 报告

**存量迁移（2026-05-29）**：1068 个对象已打 tag `retention-years=3`；Lifecycle 规则 2/3/5/10 年 + 1 天已同步。

**联调测试文件（retention_days=1）**：
- HTTP：`test-retention-http-20260529_233447.png`（`POST /v1/files` + `retention_days=1`）
- CLI：`file-4ac5a752-12c6-47b0-802f-035a8e05ed05-test-retention-cli-20260529_233447.png`（`oss-cli upload --retention-days 1`）

改动文件：

- VERSION
- cmd/root.go
- cmd/upload.go
- config/config.go
- config/retention.go（新建）
- config/retention_test.go（新建）
- config.yaml.example
- oss/client.go
- oss/retention.go（新建）
- oss/lifecycle.go（新建）
- oss/retention_test.go（新建）
- server/server.go
- scripts/sync-lifecycle/（新建）
- scripts/sync-lifecycle-rules.sh（新建）
- scripts/set-retention/（新建）
- scripts/set-default-retention.sh（新建）
- scripts/oss-inventory/（新建）
- docs/PRD-RETENTION-PERIOD.md（新建）
- docs/PLAN-RETENTION-PERIOD.md（新建）
- docs/ARCHITECT-DESIGN-RETENTION-PERIOD.md（新建）
- docs/OSS-RETENTION-REPORT-20260529.md（新建）
- release-build.sh
- README.md
- CHANGELOG.md

---

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


