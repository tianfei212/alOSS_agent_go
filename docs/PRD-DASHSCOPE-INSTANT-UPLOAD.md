# PRD：百炼临时文件上传与凭证获取

> 文档版本：1.1  
> 日期：2026-05-29  
> 状态：已评审（规划阶段，待开发）  
> 参考：[百炼官方文档 — 上传本地文件获取临时 URL](https://help.aliyun.com/zh/model-studio/get-temporary-file-url)  
> 关联：[ARCHITECTURE.md](./ARCHITECTURE.md) · [架构分析报告](./ARCHITECTURE-ANALYSIS-DASHSCOPE-INSTANT-UPLOAD.md) · [实施计划](./PLAN-DASHSCOPE-INSTANT-UPLOAD.md)

---

## 功能边界（与 OSS 无关）

本需求为 **全新独立能力域（F5）**，与现有 F1–F4（自有 OSS、`/v1/files`、`oss-cli upload` 等）**无功能重叠、无代码复用、无配置共用**：

| 维度 | F1–F4 自有 OSS（现有） | F5 百炼临时上传（本需求） |
|------|------------------------|---------------------------|
| 业务目的 | 长期文件托管、OpenAI Files 兼容 | 多模态模型调用前的 48h 临时 `oss://` URL |
| 存储后端 | 用户配置的 OSS Bucket | 百炼 `dashscope-instant` 临时空间 |
| 凭证 | `OSS_ACCESS_KEY_ID` / `OSS_ACCESS_KEY_SECRET` | **客户百炼 API Key → `.env.local` 中的 `AL_KEY`** |
| HTTP 鉴权 | `OPENAI_API_KEY` / `server.openai_api_key` | **不使用** OSS 的 `AuthMiddleware` 密钥；见下文 F5-4 |
| CLI | 依赖 `oss.Init` | **`dashscope` 子命令不依赖 OSS 配置** |

**禁止**：在 `oss/client.go` 中实现本功能；禁止要求用户配置 OSS 才能使用百炼临时上传。

---

## 背景与目标

### 问题陈述

调用阿里云百炼（Model Studio）多模态、图像、视频或音频模型时，通常需要传入以 `oss://` 为前缀的**临时文件 URL**（有效期 **48 小时**）。官方流程为：

1. 调用 `GET /api/v1/uploads?action=getPolicy&model={model}` 获取上传凭证；
2. 使用凭证以 `multipart/form-data` POST 到临时 OSS `upload_host`；
3. 拼接 `oss://{key}` 作为模型入参；HTTP 调用模型时 Header 须带 `X-DashScope-OssResourceResolve: enable`。

仓库内 **F1–F4** 已覆盖长期自有 OSS 与 OpenAI Files API；**本 PRD 仅定义 F5**，不改造、不扩展 F1–F4。百炼临时存储在存储后端、API Key、URL 形态、生命周期上与 OSS 完全不同，需单独交付。

### 为谁解决什么问题

| 用户 | 问题 | 价值 |
|------|------|------|
| 脚本/运维人员 | 需手写 Python/Java 示例或 dashscope CLI | 使用 `oss-cli dashscope upload` 一条命令完成 |
| Agent / 自动化平台 | 需嵌入百炼三步上传逻辑 | 调用本服务 HTTP 接口一次拿 `oss_url` |
| 集成开发者 | 仅需 policy 自行上传 | 可单独调用 `getPolicy` 接口 |

### 成功指标（可量化）

| 指标 | 目标 |
|------|------|
| 端到端上传成功率 | 合法 `model`、文件未超限时 ≥ 99%（非压测场景） |
| CLI 端到端 | `oss-cli dashscope upload` 退出码 0 且 stdout 含 `oss://` |
| API 字段对齐 | `GET getPolicy` 响应 `data.*` 与百炼官方一致 |
| 上传响应 | 含 `oss_url`、`expires_at`（上传时间 + 48h，RFC3339）、`model` |
| 文档可验收 | README/DESIGN 含模型调用 Header 说明 |

---

## 用户与场景

### 用户故事

**US-1 脚本上传**

- **As a** 运维工程师  
- **I want** 通过 CLI 指定模型与本地文件上传  
- **So that** 获得可直接用于百炼 API 的 `oss://` URL  

**验收（Given/When/Then）**

- Given `.env.local` 中已配置客户百炼 API Key：`AL_KEY=sk-xxx`  
- When 执行 `oss-cli dashscope upload --model qwen-vl-plus --file ./cat.png`  
- Then 标准输出包含 `oss_url` 与 48 小时内的 `expires_at`，进程退出码为 0；**无需** OSS 相关环境变量  

**US-2 Agent 一站式 HTTP 上传**

- **As a** Agent 平台  
- **I want** 向本服务 POST 文件与 model，由服务端用客户 `AL_KEY` 代调百炼  
- **So that** 集成方无需在业务代码中硬编码百炼 Key（Key 仅落在服务端 `.env.local`）  

**验收**

- Given 服务端已加载 `AL_KEY`，且请求 `Authorization: Bearer` 与 `AL_KEY` 一致（见 F5-4）  
- When `POST /v1/dashscope/uploads`（multipart：`file` + `model`）  
- Then 响应 200，body 含 `oss_url`、`expires_at`、`model`  

**US-3 仅获取上传凭证**

- **As a** 高级集成方  
- **I want** 仅拉取 getPolicy 响应  
- **So that** 由自有客户端 POST 到 `upload_host`  

**验收**

- When `GET /v1/dashscope/uploads?action=getPolicy&model=qwen-vl-plus`  
- Then 返回 `data.policy`、`data.signature`、`data.upload_host` 等官方字段  

---

## 功能需求（含验收标准）

### F5-1 获取上传凭证（getPolicy）

| ID | 需求 | 验收标准 |
|----|------|----------|
| F5-1.1 | 代理百炼 getPolicy | `GET /v1/dashscope/uploads?action=getPolicy&model={model}` 返回结构与百炼一致 |
| F5-1.2 | 参数校验 | 缺少 `model` 返回 400 + 明确 `error` |
| F5-1.3 | 鉴权失败映射 | 百炼 401/403 透传或映射为同等语义 HTTP 状态 |

### F5-2 上传至临时空间并生成 URL

| ID | 需求 | 验收标准 |
|----|------|----------|
| F5-2.1 | multipart 上传 | 按官方 form 字段 POST 至 `upload_host`，`file` 为最后一项 |
| F5-2.2 | URL 拼接 | 成功返回 `oss_url` = `oss://` + `upload_dir` + `/` + 文件名 |
| F5-2.3 | 过期时间 | `expires_at` = 上传完成时间 + 48 小时 |
| F5-2.4 | policy 过期重试 | 收到 Policy expired 时自动重新 getPolicy 并重传，最多 1 次 |

### F5-3 CLI 子命令

| 命令 | 说明 |
|------|------|
| `oss-cli dashscope upload --model M --file PATH` | 端到端上传并打印 `oss_url` |
| `oss-cli dashscope policy --model M` | 仅输出 getPolicy JSON |

### F5-4 安全与鉴权（API Key：`AL_KEY`）

百炼 getPolicy / 上传 **必须**使用客户阿里云百炼 API Key。本项目统一从 **`.env.local` 环境变量 `AL_KEY`** 读取（由 `godotenv` 加载，与现有配置加载方式一致）。

| 通道 | 规则 |
|------|------|
| **调百炼上游** | `Authorization: Bearer {AL_KEY}`；未配置 `AL_KEY` 时拒绝启动 F5 相关命令/路由（明确错误信息） |
| **HTTP `/v1/dashscope/*` 入站** | **不复用** F1–F4 的 `OPENAI_API_KEY` / `AuthMiddleware`；使用 **F5 专用中间件**，校验 `Authorization: Bearer` 与进程内 `AL_KEY` 一致 |
| **CLI `dashscope *`** | 仅读取 `AL_KEY`；**不**读取 `OPENAI_API_KEY`，**不**触发 `oss.Init` |
| **密钥隔离** | `AL_KEY` 与 `OSS_*`、`OPENAI_API_KEY` 三者互不替代；文档与示例不得混用 |

**验收**

- Given `.env.local` 无 `AL_KEY`，When 调用 `GET /v1/dashscope/uploads`，Then 503 或 401 且 message 提示配置 `AL_KEY`  
- Given `AL_KEY` 有效，When getPolicy，Then 百炼请求 Header 为 `Bearer {AL_KEY}`  

### F5-5 模型调用说明（文档交付）

- README 明确：通过 **HTTP** 调百炼模型时 Header **必须**包含 `X-DashScope-OssResourceResolve: enable`  
- 使用 DashScope SDK 时由 SDK 自动添加，本模块不代理推理  

---

## 非功能需求

| 类别 | 要求 |
|------|------|
| 限流认知 | 文档注明百炼 getPolicy **100 QPS（主账号 + 模型维度）**；MVP 不实现本地限流 |
| 生产适用性 | 文档标注临时 URL **48 小时**、官方建议**勿用于生产**；长期文件继续用 `/v1/files` |
| 依赖 | 标准库 `net/http` + `mime/multipart`；不引入 DashScope SDK |
| 配置 | **必选** `AL_KEY`（`.env.local`）；可选 `dashscope.base_url`（默认 `https://dashscope.aliyuncs.com`）、`default_model` |
| 可观测性 | 关键步骤 INFO 日志：getPolicy、POST upload_host、重试 |

---

## 里程碑

| 阶段 | 交付物 | MoSCoW |
|------|--------|--------|
| **M1 MVP** | `dashscope/` 包、`GET getPolicy`、CLI `dashscope upload/policy` | Must |
| **M2 HTTP** | `POST /v1/dashscope/uploads` 一站式上传 | Must |
| **M3 文档** | README、DESIGN、ARCHITECTURE F5 更新 | Should |
| **M4 可选** | `test_openai_api.html` 百炼上传 Tab、policy 内存缓存 | Could |

---

## 优先级（MoSCoW）

| 优先级 | 项 |
|--------|-----|
| **Must** | getPolicy、临时 OSS POST、`oss://` 返回、CLI upload、`AL_KEY` 配置与 F5 独立鉴权 |
| **Should** | policy 过期自动重试 1 次、DESIGN/ARCHITECTURE 更新 |
| **Could** | 前端测试页、getPolicy 短期缓存 |
| **Won't（本版本）** | 模型推理代理、临时文件 list/delete、QPS 网关 |

---

## 开放问题

| # | 问题 | 结论（v1.1） |
|---|------|----------------|
| Q1 | 百炼 API Key 来源？ | **已确认**：客户 Key，写在 `.env.local` 的 **`AL_KEY`** |
| Q2 | 是否与 OSS / `OPENAI_API_KEY` 共用鉴权？ | **已确认**：否，F5 完全独立 |
| Q3 | 是否在 config 中配置默认 `model`？ | 可选，CLI 仍要求显式 `--model` |
| Q4 | 是否在测试 HTML 增加百炼 Tab？ | M3 后按需 |

---

## 不在范围

- 百炼 `chat/completions` / 多模态推理代理  
- OpenAI SDK 对 `oss://` 的兼容扩展  
- 临时文件的查询、修改、HTTPS 签名下载（官方不支持）  
- 替代自有 OSS 的生产级持久存储  
- 分布式限流、凭证池、多租户百炼 Key 路由  

---

## 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-05-29 | 初稿：基于 ARCHITECTURE.md 与百炼官方文档 |
| 1.1 | 2026-05-29 | 明确 F5 与 OSS 无重叠；百炼凭证统一为 `.env.local` 的 `AL_KEY` |
