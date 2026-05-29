# PRD：OSS 标准获取 WebP 格式支持

> 文档版本：1.0  
> 日期：2026-05-29  
> 状态：已实现（feature/view-webp-format）  
> 参考：[阿里云 OSS 图片处理 — 格式转换](https://help.aliyun.com/zh/oss/user-guide/img-implementation-modes) · [格式转换参数](https://help.aliyun.com/zh/oss/user-guide/format-conversion)  
> 关联：[ARCHITECTURE.md](./ARCHITECTURE.md) · [DESIGN.md](./DESIGN.md) · [实施计划](./PLAN-VIEW-WEBP-FORMAT.md)

---

## 功能边界

本需求为 **F3 访问与预览** 的增强，在现有 `w` / `h` 缩略图能力之上，增加 **WebP 格式转换** 的签名 URL 申请能力。

| 维度 | 现状（V1.0.3） | 本需求（F3 增强） |
|------|----------------|-------------------|
| HTTP 入口 | `GET /view/{file_id}` | 同上，新增 query 参数 `format=webp` |
| 缩略图 | `?w=&h=` → `image/resize,w_X,h_Y,m_fill` | 可与 `format=webp` 组合 |
| 格式转换 | 不支持 | `?format=webp` → `image/format,webp` |
| CLI | `url` / `thumbnail` 无 format | 新增 `--format webp` |
| 鉴权 | `/view/*` 无鉴权，靠签名 URL 时效 | **不变** |
| 存储 | 不修改 OSS 原对象 | **不变**（转换在 OSS 图片处理边缘完成） |

**不在范围**：`/v1/files/:id/content` 307 重定向、`view_url` 上传响应字段、png/jpg 等其他输出格式、单边缩放（仅 `w` 或仅 `h`）。

---

## 背景与目标

### 问题陈述

前端通过 `GET /view/{file_id}` 获取 OSS 签名 URL 用于 `<img>` 展示。当前仅支持原图签名或固定宽高缩略图，**无法在签名阶段指定输出为 WebP**，导致：

- 列表页、缩略图场景传输体积偏大；
- 调用方需自行做格式转换或依赖 CDN 层能力；
- 私有 Bucket 无法在签名 URL 后拼接 `x-oss-process`，必须在 Agent 侧将 process 纳入签名。

### 为谁解决什么问题

| 用户 | 问题 | 价值 |
|------|------|------|
| H5 / 前端 | 需要更小的图片体积 | 一条 URL 同时获得 WebP 与可选缩放 |
| Agent / 自动化 | 需标准化图片预览参数 | 统一 `format=webp` 约定，无需直连 OSS SDK |
| 运维 / CLI 用户 | 命令行调试签名 URL | `oss-cli url --format webp` 快速验证 |

### 成功指标（可量化）

| 指标 | 目标 |
|------|------|
| 纯 WebP | `GET /view/photo.jpg?format=webp` 返回的 `url` 含 `image/format,webp`（URL 编码后），浏览器可正常展示 |
| 缩略图 + WebP | `?w=200&h=100&format=webp` 的 process 为 `image/resize,w_200,h_100,m_fill/format,webp` |
| 向后兼容 | 不传 `format` 时行为与现网 **100% 一致** |
| CLI | `oss-cli url key.jpg --format webp` 退出码 0 且 stdout 含有效签名 URL |
| 文档 | README / DESIGN 含参数说明与 Bucket 前置条件 |

---

## 用户与场景

### 用户故事

**US-1 标准获取 + WebP 转换**

- **As a** 前端开发者  
- **I want** 请求原图的 WebP 签名 URL  
- **So that** 在不改变 OSS 存储格式的前提下减小传输体积  

**验收（Given/When/Then）**

- Given OSS 上存在 `photo.jpg`，且 Bucket 已开通图片处理（IMG）  
- When `GET /view/photo.jpg?format=webp`  
- Then HTTP 200；响应 `url` 的 query 含图片处理参数；`media_type` 为 `image`；可选字段 `output_format` 为 `webp`  

**US-2 缩略图 + WebP**

- **As a** 前端开发者  
- **I want** 固定宽高缩略图且输出 WebP  
- **So that** 列表页加载更快  

**验收**

- Given 有效 `w=200&h=100`  
- When `GET /view/photo.jpg?w=200&h=100&format=webp`  
- Then 签名 process 等价于 `image/resize,w_200,h_100,m_fill/format,webp`  

**US-3 向后兼容**

- **As a** 现有集成方  
- **I want** 不传新参数时行为不变  
- **So that** 升级无破坏性  

**验收**

- When 请求不含 `format`，或与现网相同的 `w`/`h` 组合  
- Then 响应 URL 与升级前一致（process 字符串相同）  

**US-4 CLI 调试**

- **As a** 运维人员  
- **I want** CLI 生成 WebP 签名 URL  
- **So that** 无需 curl 即可验证 Bucket 图片处理能力  

**验收**

- When `oss-cli url photo.jpg --format webp`  
- Then 标准输出含带 `format,webp` 的签名 URL  

---

## 功能需求（含验收标准）

### F3-W1 Query 参数 `format`

| 项 | 说明 |
|----|------|
| 参数名 | `format` |
| 取值（MVP） | `webp`（大小写不敏感） |
| 位置 | `GET /view/{file_id}?format=webp` |
| 与其他参数 | 可与 `w`、`h`、`expire_seconds` 任意组合 |

### F3-W2 OSS process 组合规则

| w | h | format | 生成的 process |
|---|---|--------|----------------|
| 无/无效 | 无/无效 | 无 | （空，走原图 `GetSignedURL`） |
| 无/无效 | 无/无效 | webp | `image/format,webp` |
| >0 | >0 | 无 | `image/resize,w_X,h_Y,m_fill`（现有） |
| >0 | >0 | webp | `image/resize,w_X,h_Y,m_fill/format,webp` |

**说明**：`w` 与 `h` 须**同时**有效且 > 0 才进入 resize 分支，与现网逻辑一致。

### F3-W3 HTTP 响应（可选增强）

在现有字段基础上，当 `format=webp` 生效时，响应可增加：

```json
{
  "id": "photo.jpg",
  "media_type": "image",
  "url": "https://...",
  "output_format": "webp",
  "expires_in": 3600,
  "created_at": 1717027200
}
```

`output_format` 仅在请求显式指定且被接受时返回；未指定时不出现该字段。

### F3-W4 OSS Client 层

| 项 | 说明 |
|----|------|
| 新增 | `buildImageProcess(width, height int, format string) string` |
| 新增 | `GetProcessedSignedURL(objectKey, process string, expiredInSec int)` |
| 重构 | `GetThumbnailSignedURL` 改为调用上述函数，避免重复 |

签名必须使用 `Bucket.SignURL(..., oss.Process(process))`，禁止在已签名 URL 后拼接参数。

### F3-W5 CLI

| 命令 | 新增 flag | 行为 |
|------|-----------|------|
| `oss-cli url` | `--format webp` | 生成 `image/format,webp` 签名 URL |
| `oss-cli thumbnail` | `--format webp` | 生成 resize + webp 组合签名 URL |

### F3-W6 非图片与非法 format

| 场景 | MVP 策略 |
|------|----------|
| `format=webp` 作用于 `.mp4` 等非图片 | **忽略** `format`，按原逻辑返回签名 URL；日志 `WARN` |
| `format=jpg` 等未支持值 | **忽略**，走原逻辑 |
| SVG | 文档注明 OSS 可能不支持 SVG→WebP；不单独拦截 |

---

## 非功能需求

| 类别 | 要求 |
|------|------|
| 兼容性 | 纯增量 query 参数；默认行为不变 |
| 性能 | 转换在 OSS 完成，Agent 无额外 CPU/内存 |
| 安全 | 仍依赖签名 URL 时效（`expire_seconds` / `link_expire_seconds`） |
| 依赖 | Bucket 须开通「图片处理」；未开通时 OSS 返回 4xx，文档需说明 |
| 限制 | WebP 转换要求原图宽高均 ≤ 16383 px（阿里云限制） |

---

## API 示例

```http
# 纯 WebP
GET /view/2026/photo.jpg?format=webp

# 缩略图 + WebP
GET /view/2026/photo.jpg?w=400&h=300&format=webp

# 自定义过期时间
GET /view/2026/photo.jpg?w=400&h=300&format=webp&expire_seconds=7200
```

---

## 里程碑（摘要）

| 阶段 | 交付 |
|------|------|
| M1 | OSS Client process 构建与签名 |
| M2 | `viewMedia` + CLI |
| M3 | 测试、文档、联调 |

详见 [PLAN-VIEW-WEBP-FORMAT.md](./PLAN-VIEW-WEBP-FORMAT.md)。

---

## 开放问题（已决 / 待确认）

| # | 问题 | 建议决策 | 状态 |
|---|------|----------|------|
| 1 | 参数命名 `format=webp` vs `webp=1` | 使用 `format=webp`，便于后续扩展 | **已决** |
| 2 | 非图片 + format=webp | 忽略 + WARN，不返回 400 | **已决** |
| 3 | 响应是否含 `output_format` | MVP 增加该字段 | **已决** |
| 4 | 是否扩展 `/v1/files` 的 `view_url` | V1.1 迭代，本次不做 | **已决** |

---

## 不在范围

- 其他输出格式（`png`、`jpg`、`q_80` 质量参数）
- `/v1/files/:id/content` 307 重定向携带 format
- 上传响应 `view_url` 自动 WebP
- 仅 `w` 或仅 `h` 的单边缩放
- 修改 OSS 上存储的对象格式

---

## 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-05-29 | 初稿：WebP 格式签名 PRD |
