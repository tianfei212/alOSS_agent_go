# alOSS_agent_go

当前版本：`V1.0.5`

基于 Go 实现的阿里云 OSS 工具集，提供两种使用方式：

- `CLI`：适合脚本、终端、批处理任务
- `OpenAI Files 兼容 HTTP API`：适合其他系统、前端页面、Agent、自动化平台调用

当前版本已经统一了对外返回格式：

- 对前端返回的 `id` / `filename` 不再暴露内部 OSS 前缀
- `view_url` 直接返回 OSS 签名 URL，可直接用于图片显示、视频播放、文件下载
- `/view/*file_id` 返回 JSON，其中 `url` 也是 OSS 签名 URL

## 主要能力

- 直接对接阿里云 OSS
- 支持本地文件上传
- 支持远程 URL 离线上传
- 支持列表、详情、删除、签名下载
- 兼容 OpenAI `v1/files` 风格接口
- 默认开启 CORS，前端可直接调用
- 支持按系统与 CPU 架构选择启动二进制
- 支持打包 macOS 与 Ubuntu 二进制

## 目录说明

- `cmd/`: CLI 命令定义
- `server/`: HTTP API 与鉴权逻辑
- `oss/`: 阿里云 OSS 客户端封装
- `config/`: 配置读取
- `build.sh`: 跨平台编译脚本
- `start.sh`: 按系统/CPU 自动选择可执行文件并启动
- `setup.sh`: 从 GitHub Release 下载对应平台二进制
- `test_openai_api.html`: 本地测试页面

## 配置方式

程序默认会先读取 `.env.local`，再读取 `config.yaml`。

**这两个文件均含敏感信息，不得提交到 git。** 首次使用请复制模板：

```bash
cp config.yaml.example config.yaml
cp .env.example .env.local
# 编辑 config.yaml 与 .env.local 填入真实凭证
```

推荐保留 `.env.local` 作为敏感配置文件，不要提交到 git。

### `.env.local` 示例

```bash
OPENAI_API_KEY=your_api_key
OSS_ENDPOINT=https://oss-cn-beijing.aliyuncs.com
OSS_ACCESS_KEY_ID=your_access_key_id
OSS_ACCESS_KEY_SECRET=your_access_key_secret
OSS_BUCKET=your_bucket
OSS_BUCKET_PREFIX=images_data/
```

### `config.yaml` 示例

```yaml
oss:
  endpoint: "https://oss-cn-beijing.aliyuncs.com"
  access_key_id: "your_access_key_id"
  access_key_secret: "your_access_key_secret"
  bucket_name: "your_bucket"
  bucket_prefix: "images_data/"

server:
  port: 8080
  link_expire_seconds: 3600
  openai_api_key: "your_api_key"
```

## CLI 使用

先本地编译：

```bash
go build -o oss-cli .
```

### 启动服务

```bash
./oss-cli server
./oss-cli server -p 9090
./oss-cli server --config config.yaml
```

### 上传文件

```bash
./oss-cli upload /path/to/file.jpg
./oss-cli upload /path/to/file.mp4 --config config.yaml
```

### 获取文件列表

```bash
./oss-cli list
./oss-cli list --prefix="2026/" --limit=50
```

### 获取签名链接

```bash
./oss-cli url your-file.jpg
./oss-cli url your-file.jpg -e 7200
./oss-cli url your-file.jpg --format webp
./oss-cli thumbnail your-file.jpg --format webp
```

### 删除文件

```bash
./oss-cli delete your-file.jpg
```

## 给其他系统调用

### 服务启动

```bash
./start.sh
./start.sh 8080
```

`start.sh` 会根据当前系统和 CPU 自动选择：

- macOS Apple Silicon: `dist/oss-cli-darwin-arm64`
- Ubuntu amd64: `dist/oss-cli-linux-amd64`

### 鉴权

所有 `/v1/*` 接口都需要：

```http
Authorization: Bearer <OPENAI_API_KEY>
```

`/view/*file_id` 不需要鉴权，适合浏览器直接请求。

## HTTP API

### 1. 上传文件

`POST /v1/files`

支持两种方式：

- 表单上传：`file` + `purpose`
- URL 上传：`file_url` + `purpose`

响应示例：

```json
{
  "id": "example.jpg",
  "object": "file",
  "bytes": 102400,
  "created_at": 1713430588,
  "filename": "example.jpg",
  "purpose": "fine-tune",
  "view_url": "https://oss-xxx.aliyuncs.com/images_data%2Fexample.jpg?Expires=..."
}
```

### 2. 获取文件列表

`GET /v1/files`

响应中的每一项：

- `id`: 对外文件 ID，不带内部前缀
- `filename`: 对外文件名，不带内部前缀
- `view_url`: 直接可用的 OSS 签名 URL

### 3. 获取单个文件详情

`GET /v1/files/{file_id}`

当前返回中：

- `id`: 不带 `images_data/`
- `filename`: 不带 `images_data/`
- `view_url`: 直接可显示/播放/下载的 OSS 签名 URL

示例：

```json
{
  "bytes": 8597020,
  "created_at": 1773573203,
  "filename": "bd7b4a02de743111_20260315182120_sr_4x.jpg",
  "id": "bd7b4a02de743111_20260315182120_sr_4x.jpg",
  "object": "file",
  "purpose": "assistants",
  "view_url": "https://oss-416-talon.oss-cn-beijing.aliyuncs.com/images_data%2Fbd7b4a02de743111_20260315182120_sr_4x.jpg?Expires=..."
}
```

### 4. 删除文件

`DELETE /v1/files/{file_id}`

### 5. 获取文件内容

`GET /v1/files/{file_id}/content`

返回 307 跳转到 OSS 签名 URL。

### 6. 获取媒体预览 JSON

`GET /view/{file_id}`

Query 参数：

| 参数 | 说明 |
|------|------|
| `w` / `h` | 缩略图宽高（须同时有效且 > 0） |
| `format=webp` | 输出 WebP 格式（可与 w/h 组合） |
| `expire_seconds` | 签名 URL 有效期（秒） |

示例：

```http
GET /view/photo.jpg?format=webp
GET /view/photo.jpg?w=400&h=300&format=webp
```

返回：

- `id`
- `media_type`
- `url`
- `expires_in`
- `created_at`

其中 `url` 是 OSS 签名 URL，可直接给前端 `<img>`、`<video>`、`<audio>`、`iframe` 使用。

## 测试页面

打开：

```bash
open test_openai_api.html
```

页面支持：

- 文件上传
- URL 上传
- 文件列表
- 预览图片 / 视频 / 音频 / PDF
- 查看原始 JSON

## 跨平台编译

执行：

```bash
chmod +x build.sh
./build.sh
```

输出目录为 `dist/`，包含：

- `oss-cli-darwin-arm64`
- `oss-cli-linux-amd64`

说明：

- Mac M1 / M2 / M3 / M4 / M5 都使用 `darwin-arm64`
- Ubuntu 常见服务器使用 `linux-amd64`

## GitHub Release 安装

执行：

```bash
chmod +x setup.sh
./setup.sh
```

脚本会自动根据系统与架构下载对应的二进制。

## Git 发布建议

建议发布时只提交源码、脚本、README，不提交：

- `.env.local`
- `dist/`
- `build/`
- 本地测试大文件
- 本地调试产生的二进制

## 常用命令

```bash
go test ./...
go build -o oss-cli .
./build.sh
./start.sh
```
