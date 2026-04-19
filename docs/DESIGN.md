# alOSS Agent Go — 设计文档

## 1. 系统概述

alOSS Agent Go 是一个兼容 OpenAI Files API 规范的阿里云 OSS 文件管理服务，同时提供 CLI 工具和 HTTP API 服务器两种使用方式。

### 1.1 核心功能

| 功能 | 说明 |
|------|------|
| **OpenAI Files API 兼容** | `/v1/files` 系列接口，上传、列表、详情、删除、下载 |
| **流式大文件上传** | 支持 HTTP 流式上传，适用于前端分片上传场景 |
| **OSS 签名 URL** | 生成带临时有效期的下载/播放链接 |
| **媒体预览** | `/view/*file_id` 返回 OSS 签名 URL，前端直接播放 |
| **CLI 工具** | `upload`/`list`/`delete`/`url`/`server` 子命令 |
| **IP 黑名单** | 内置 IP 黑名单中间件，防御恶意请求 |
| **CORS 支持** | 允许前端跨域调用 |

---

## 2. 系统架构

### 2.1 整体架构图

```
                                    ┌─────────────────────────────────────────┐
                                    │              用户 / 客户端               │
                                    └──────────┬──────────────────┬──────────┘
                                               │                  │
                                    ┌───────────▼──┐       ┌──────▼──────┐
                                    │  HTTP API    │       │   CLI Tool   │
                                    │ (Gin Server) │       │ (Cobra CLI)  │
                                    └──────┬───────┘       └──────┬───────┘
                                           │                      │
                                    ┌──────▼───────┐       ┌──────▼──────┐
                                    │   server/    │       │    cmd/     │
                                    │  server.go   │       │  (子命令)    │
                                    └──────┬───────┘       └──────┬───────┘
                                           │                      │
                                    ┌──────▼───────┐       ┌──────▼──────┐
                                    │     oss/     │◄─────►│    oss/    │
                                    │   client.go  │       │  client.go  │
                                    └──────┬───────┘       └──────┬───────┘
                                           │                      │
                                    ┌──────▼──────────────────────▼──────┐
                                    │             阿里云 OSS              │
                                    │  (上传/下载/列表/删除/签名URL生成)    │
                                    └─────────────────────────────────────┘
```

### 2.2 包结构

```
alOSS_go_probject/
├── main.go                 # 程序入口，调用 cmd.Execute()
├── config/
│   └── config.go          # 配置加载（Viper + .env.local）
├── oss/
│   └── client.go          # OSS SDK 封装，所有 OSS 操作入口
├── server/
│   ├── server.go          # HTTP API handlers（/v1/files 系列）
│   └── auth.go            # 鉴权中间件 + IP 黑名单
├── cmd/
│   ├── root.go            # CLI root 命令（初始化配置/OSS客户端）
│   ├── server.go          # oss-cli server（启动 HTTP 服务）
│   ├── upload.go          # oss-cli upload（上传本地文件）
│   ├── list.go            # oss-cli list（列出 OSS 文件）
│   ├── delete.go         # oss-cli delete（删除 OSS 文件）
│   └── url.go             # oss-cli url（生成签名链接）
├── test_openai_api.html   # 浏览器测试页面（带分片上传+媒体预览）
└── dist/                   # 编译产物（跨平台二进制）
```

---

## 3. 配置管理

### 3.1 配置来源（优先级从高到低）

1. **命令行 Flag**（如 `oss-cli server --port 9000`）
2. **环境变量**（`.env.local` 文件）
3. **`config.yaml`** 配置文件

### 3.2 配置项说明

| 配置项 | 环境变量 | 说明 |
|--------|----------|------|
| `oss.endpoint` | `OSS_ENDPOINT` | OSS Endpoint，如 `oss-cn-beijing.aliyuncs.com` |
| `oss.access_key_id` | `OSS_ACCESS_KEY_ID` | 阿里云 AccessKey ID |
| `oss.access_key_secret` | `OSS_ACCESS_KEY_SECRET` | 阿里云 AccessKey Secret |
| `oss.bucket_name` | `OSS_BUCKET` | OSS Bucket 名称 |
| `oss.bucket_prefix` | `OSS_BUCKET_PREFIX` | 存储路径前缀，如 `images_data/`（可选） |
| `server.port` | — | HTTP 服务监听端口，默认 `8080` |
| `server.link_expire_seconds` | — | 签名链接有效期（秒），默认 `3600` |
| `server.openai_api_key` | `OPENAI_API_KEY` | API 鉴权密钥 |

### 3.3 配置加载流程

```
main.go
  └─> cmd.Execute()
        └─> rootCmd.Execute()
              └─> initConfig()          [cmd/root.go]
                    ├─> config.LoadConfig()
                    │     ├─> godotenv.Load(".env.local")
                    │     ├─> viper.SetConfigFile("config.yaml")
                    │     ├─> viper.BindEnv()          ← 绑定环境变量
                    │     └─> viper.Unmarshal(&AppConfig)
                    └─> oss.Init(AppConfig.OSS)
                          └─> oss.New() / client.Bucket()
```

---

## 4. OSS 路径管理策略

### 4.1 前缀的双重角色

```
OSS 存储路径（内部）          对外 API 返回（外部）
─────────────────            ────────────────────
images_data/photo.jpg    →    photo.jpg
images_data/video.mp4   →    video.mp4
```

- **存储时**：传入 `objectKey`（无前缀），由 `resolveKey()` 拼接前缀后存入 OSS
- **返回时**：统一使用 `stripPrefix()` 去掉前缀再返回给前端

### 4.2 resolveKey 与 stripPrefix

```go
// oss/client.go — 上传到 OSS 时自动拼接前缀
func (c *Client) resolveKey(key string) string {
    if c.Prefix != "" {
        return path.Join(c.Prefix, key)  // user_key → images_data/user_key
    }
    return key
}

// server/server.go — 对外返回时去掉前缀
func stripPrefix(key string) string {
    prefix := config.AppConfig.OSS.BucketPrefix
    if prefix != "" {
        key = strings.TrimPrefix(key, prefix)
    }
    return key
}
```

### 4.3 为何不直接用前缀作为 objectKey

如果上传时直接将 `images_data/photo.jpg` 作为 objectKey，则：
- `resolveKey()` 会再次拼接前缀，导致 **双重前缀**：`images_data/images_data/photo.jpg`
- 因此上传时传入无前缀的 key，由 `resolveKey()` 统一管理前缀

---

## 5. HTTP API 详解

### 5.1 路由总览

```
/view/*file_id                      [无鉴权]  获取媒体预览（返回签名URL）
/v1/files                           [需鉴权]  上传文件
/v1/files                           [需鉴权]  获取文件列表
/v1/files/:file_id                  [需鉴权]  获取文件详情
/v1/files/:file_id                  [需鉴权]  删除文件
/v1/files/:file_id/content          [需鉴权]  获取下载链接（重定向）
```

### 5.2 鉴权中间件

```
请求 Header: Authorization: Bearer <openai_api_key>
                    │
                    ▼
           AuthMiddleware()
                    │
          ┌────────┴────────┐
          │  key 匹配？      │
          └────────┬────────┘
              是   │   否
                  ▼
           c.Next()      →  401 Unauthorized {"error": "Invalid API key"}
```

IP 黑名单在 `AuthMiddleware` 之前执行，命中直接 403。

### 5.3 文件上传流程

```
客户端                              服务器                         OSS
  │                                  │                            │
  │──── POST /v1/files (multipart) ──▶                           │
  │       Content-Type: multipart/form-data                        │
  │                                  │                            │
  │                                  │  FormFile("file")           │
  │                                  │──── PutObject (流式) ──────▶│
  │                                  │◀─────── 200 OK ────────────│
  │                                  │                            │
  │                                  │  SignURL(key)               │
  │                                  │──── SignURL ───────────────▶│
  │                                  │◀─── 签名URL ───────────────│
  │                                  │                            │
  │◀─── JSON { id, view_url, ... } ─│                            │
```

### 5.4 文件列表流程

```
客户端                    服务器                         OSS
  │                        │                             │
  │── GET /v1/files ──────▶│                             │
  │                        │  ListObjectsV2()             │
  │                        │────────────────────────────▶│
  │                        │◀──────── objects[] ─────────│
  │                        │                             │
  │                        │  遍历每个 object:            │
  │                        │    stripPrefix(key)          │
  │                        │    SignURL(publicKey)        │
  │                        │────────────────────────────▶│
  │                        │◀─── signedURL ─────────────│
  │                        │                             │
  │◀── JSON { data: [...] }│                             │
```

### 5.5 媒体预览流程

```
客户端                      服务器                      OSS
  │                          │                           │
  │── GET /view/photo.jpg ──▶│                           │
  │  (无需 Authorization)   │                           │
  │                          │  GetFileMeta(key)          │
  │                          │──────────────────────────▶│
  │                          │◀─── 200 OK ──────────────│
  │                          │                           │
  │                          │  SignURL(key, expireSec)   │
  │                          │──────────────────────────▶│
  │                          │◀─── 签名URL ──────────────│
  │                          │                           │
  │◀── JSON {                │                           │
  │    media_type: "image",  │                           │
  │    url: "https://oss...?"│                           │
  │  }                      │                           │
```

### 5.6 API 响应格式

**上传文件成功**
```json
{
  "id": "photo.jpg",
  "object": "file",
  "bytes": 1024000,
  "created_at": 1745123456,
  "filename": "photo.jpg",
  "purpose": "fine-tune",
  "view_url": "https://oss-cn-beijing.aliyuncs.com/images_data/photo.jpg?OSSAccessKeyId=...&Signature=...&Expires=..."
}
```

**文件列表**
```json
{
  "object": "list",
  "data": [
    {
      "id": "photo.jpg",
      "object": "file",
      "bytes": 1024000,
      "created_at": 1745123456,
      "filename": "photo.jpg",
      "purpose": "assistants",
      "view_url": "https://oss-cn-beijing.aliyuncs.com/images_data/photo.jpg?..."
    }
  ]
}
```

**媒体预览**
```json
{
  "id": "photo.jpg",
  "media_type": "image",
  "url": "https://oss-cn-beijing.aliyuncs.com/images_data/photo.jpg?...",
  "expires_in": 3600,
  "created_at": 1745123456
}
```

---

## 6. CLI 命令详解

### 6.1 命令树

```
oss-cli
├── server              启动 HTTP API 服务
│   ├── --port, -p      指定端口（覆盖配置）
├── upload [file_path]  上传本地文件到 OSS
├── list                列出 OSS 文件
│   ├── --prefix, -p    按前缀过滤
│   └── --limit, -l     数量限制（默认100）
├── delete [key]        删除 OSS 文件
└── url [key]           生成签名下载链接
    └── --expires, -e   有效期（秒）
```

### 6.2 上传流程（CLI）

```
oss-cli upload /path/to/file.jpg
          │
          ▼
   initConfig()
     ├─> LoadConfig()         加载配置
     └─> oss.Init()           初始化 OSS 客户端
          │
          ▼
   objectKey = "file-" + uuid + "-" + basename
          │
          ▼
   ossClient.UploadFile(absPath, objectKey)
          │
          ├─> resolveKey(objectKey)  → "images_data/file-uuid-basename"
          ├─> os.Stat(localFile)    获取文件大小
          ├─> Bucket.UploadFile()   分片并发上传（3并发，100KB分片）
          └─> ProgressListener      实时打印上传进度
          │
          ▼
   打印: Upload successful. Object Key: file-uuid-basename
```

---

## 7. 前端分片上传方案

### 7.1 何时需要分片

| 文件大小 | 建议方案 |
|----------|----------|
| < 100 MB | 普通单次上传（`PUT /v1/files`） |
| 100 MB ~ 10 GB | 前端分片（`File.slice()` + 并发请求） |
| > 10 GB | 后端代理下载（`file_url` 传 URL，服务端下载后上传） |

### 7.2 分片上传流程

```
浏览器                              服务器                         OSS
  │                                  │                            │
  │  用户选择文件 (10GB)             │                            │
  │                                  │                            │
  │  File.slice(0, 10MB)             │                            │
  │  File.slice(10MB, 20MB)         │                            │
  │  ...                             │                            │
  │                                  │                            │
  │─── POST /v1/files (chunk 1) ────▶│                            │
  │       POST /v1/files (chunk 2) ──▶│  ← 并发10个分片           │
  │       POST /v1/files (chunk 3) ──▶│                            │
  │       ...                        │                            │
  │                                  │  各自独立 PutObject         │
  │                                  │────────────────────────────▶│
  │◀── 各自返回 { id, view_url } ────│                            │
  │                                  │                            │
  │  等待所有分片完成                 │                            │
  │                                  │                            │
  └── 显示上传结果                   │                            │
```

### 7.3 test_openai_api.html 分片参数

| 参数 | 值 | 说明 |
|------|-----|------|
| `CHUNK_SIZE` | 10 MB | 每个分片大小 |
| `CHUNK_CONCURRENCY` | 10 | 最大并发分片数 |
| 重试次数 | 3 次 | 每个分片失败自动重试 |

---

## 8. 安全机制

### 8.1 鉴权流程

```
请求到达
    │
    ▼
NoRoute handler (处理 OPTIONS 预检请求)
    │
    ▼
AuthMiddleware()
    │
    ├─> 检查 Authorization Header
    │
    ├─> 遍历 blackList IPs
    │     └─> 命中 → 403 Forbidden
    │
    ├─> Bearer token 与 config.openai_api_key 匹配
    │     └─> 不匹配 → 401 Unauthorized
    │
    └─> 验证通过 → c.Next()
```

### 8.2 CORS 策略

- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: POST, GET, OPTIONS, PUT, DELETE`
- `Access-Control-Allow-Headers: Content-Type, Authorization`
- 所有 `OPTIONS` 请求直接返回 `204 No Content`

### 8.3 签名 URL 安全

- 签名 URL 包含时间戳和签名，防止链接被劫持
- 默认有效期 3600 秒（1小时），可按需调整
- `/view/*file_id` 不需要鉴权，但返回的是签名 URL，有时效性

---

## 9. 关键设计决策

### 9.1 为何 `/view/*file_id` 不需要鉴权

媒体预览（图片/视频/音频）需要在浏览器中直接访问：
- `<img src="签名URL">` — 浏览器直接请求 OSS，不带 Authorization Header
- 签名 URL 本身具有时效性（默认1小时），无需额外鉴权

### 9.2 前端分片 vs 后端分片

| 维度 | 前端分片 | 后端分片 |
|------|----------|----------|
| 服务端压力 | 低（透传） | 高（需缓冲） |
| 断点续传 | 易实现 | 需记录已上传分片 |
| 进度展示 | 精确到每个分片 | 需轮询 |
| 超大文件（>10GB） | 内存压力 | 推荐 |

本项目采用 **前端分片**，通过 OSS SDK 的 `UploadFile`（分片并发）实现大文件可靠上传。

### 9.3 双重前缀问题解决

`resolveKey()` 会拼接前缀，但如果 objectKey 本身已带前缀，则会产生双重前缀：

```
错误路径：images_data/images_data/photo.jpg
正确路径：images_data/photo.jpg
```

**解决方案**：
- API 层上传时使用**无前缀**的 objectKey
- `resolveKey()` 统一管理前缀拼接
- 对外返回时用 `stripPrefix()` 去掉前缀

---

## 10. 编译与发布

### 10.1 跨平台编译

```bash
# 编译 macOS ARM64（M系列 Mac）
GOOS=darwin GOARCH=arm64 go build -o dist/oss-cli-darwin-arm64 .

# 编译 Linux AMD64（Ubuntu/Debian）
GOOS=linux GOARCH=amd64 go build -o dist/oss-cli-linux-amd64 .
```

### 10.2 版本管理

- `VERSION` 文件存储当前版本号（如 `V1.0.1`）
- CLI `--version` flag 输出版本信息
- Git Tag 与 VERSION 文件保持同步

### 10.3 启动脚本自动检测

`start.sh` 会自动检测当前系统的 `GOOS` 和 `GOARCH`，选择对应二进制：
```bash
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
# darwin + arm64  → oss-cli-darwin-arm64
# linux  + x86_64 → oss-cli-linux-amd64
```

---

## 11. 函数/方法清单

### config/config.go

| 函数 | 说明 |
|------|------|
| `LoadConfig(cfgFile string)` | 加载配置：`.env.local` → `config.yaml` → 环境变量 |

### oss/client.go

| 方法 | 说明 |
|------|------|
| `Init(cfg OSSConfig)` | 初始化 OSS 客户端单例 |
| `GetInstance() *Client` | 获取 OSS 客户端单例 |
| `(c *Client) resolveKey(key)` | 拼接 BucketPrefix 前缀 |
| `(c *Client) UploadFile(path, key)` | 本地文件分片并发上传 |
| `(c *Client) UploadStream(reader, key)` | 流式上传（HTTP 请求体） |
| `(c *Client) DeleteFile(key)` | 删除 OSS 文件 |
| `(c *Client) ListFiles(prefix, maxKeys)` | 列出 OSS 文件 |
| `(c *Client) GetSignedURL(key, expireSec)` | 生成签名下载/播放 URL |
| `(c *Client) GetFileInfo(key)` | 获取文件元数据 |

### server/server.go

| 函数 | 说明 |
|------|------|
| `RunServer()` | 启动 Gin HTTP 服务 |
| `stripPrefix(key)` | 去掉 BucketPrefix 前缀（对外暴露） |
| `ossKey(key)` | 添加 BucketPrefix 前缀（未使用） |
| `uploadFile(c)` | 处理 `POST /v1/files` |
| `listFiles(c)` | 处理 `GET /v1/files` |
| `getFileInfo(c)` | 处理 `GET /v1/files/:file_id` |
| `deleteFile(c)` | 处理 `DELETE /v1/files/:file_id` |
| `getFileContent(c)` | 处理 `GET /v1/files/:file_id/content`（重定向到签名URL） |
| `viewMedia(c)` | 处理 `GET /view/*file_id`（无需鉴权） |

### server/auth.go

| 函数 | 说明 |
|------|------|
| `InitBlacklist()` | 从文件加载 IP 黑名单 |
| `AuthMiddleware()` | 返回 Gin 中间件：鉴权 + IP 检查 |

### cmd/root.go

| 函数 | 说明 |
|------|------|
| `Execute()` | CLI 程序入口 |
| `initConfig()` | 初始化配置和 OSS 客户端 |

### cmd 子命令

| 命令 | 说明 |
|------|------|
| `oss-cli server` | 启动 HTTP API 服务 |
| `oss-cli upload [path]` | 上传本地文件 |
| `oss-cli list` | 列出 OSS 文件 |
| `oss-cli delete [key]` | 删除文件 |
| `oss-cli url [key]` | 生成签名 URL |

---

## 12. 常见问题

### Q: 上传后 `view_url` 无法访问？

检查：
1. 签名 URL 是否过期（`expires_in`）
2. OSS Bucket 是否允许跨域访问
3. 文件是否成功上传到 OSS

### Q: `images_data/` 前缀暴露给前端？

正常。API 层的 `stripPrefix()` 会自动去除前缀，对外不暴露内部路径。

### Q: 大文件上传超时？

前端分片上传方案（`test_openai_api.html`）支持断点续传和并发上传，建议使用该方案。

### Q: 如何修改签名 URL 有效期？

修改 `config.yaml` 中的 `server.link_expire_seconds`，或设置环境变量。
