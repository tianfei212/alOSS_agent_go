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


