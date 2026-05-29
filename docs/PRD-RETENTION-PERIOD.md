# PRD：OSS 文件保存周期（Retention Period）

> 文档版本：**2.0**（Lifecycle 修订）  
> 日期：2026-05-29  
> 状态：设计阶段  
> 关联：[ARCHITECT-DESIGN-RETENTION-PERIOD.md](./ARCHITECT-DESIGN-RETENTION-PERIOD.md) · [实施计划](./PLAN-RETENTION-PERIOD.md) · [OSS 文件清单](./OSS-FILE-INVENTORY.md)  
> 参考：[阿里云 OSS 生命周期概述](https://help.aliyun.com/zh/oss/user-guide/overview-54/) · [生命周期配置元素](https://help.aliyun.com/zh/oss/user-guide/configuration-elements)

---

## 背景与目标

### 问题陈述

当前 Bucket **未配置 Lifecycle**，1068+ 对象永久保留。业务需要：

- 为每个文件声明保存周期；
- **到期后由 OSS 自动删除**（不实现应用侧 cleanup）；
- 新上传可通过前端参数指定周期（缺省用配置默认 **2 年**）；
- 存量对象通过脚本一次性标记为 **3 年**。

本需求与 **`link_expire_seconds`（签名 URL 有效期）** 无关。

### 为谁解决什么问题

| 用户 | 问题 | 价值 |
|------|------|------|
| 前端 / Agent | 上传时无法指定保留年限 | 按场景打 tag，OSS 到期自动删 |
| 运维 | 存量无策略、Bucket 无 Lifecycle | 脚本打 tag + 同步 Lifecycle 规则 |
| 平台 | 无统一默认值 | `config.yaml` 默认 2 年 |

### 成功指标（可量化）

| 指标 | 目标 |
|------|------|
| 配置默认 | 无 `default_retention_years` 时解析为 **2** |
| 新上传 | 未传 `retention_years` 打 tag `retention-years=2`；传入合法值打对应 tag |
| Lifecycle | Bucket 存在与 tag 值匹配的 Expiration 规则（Days = years × 365） |
| 自动删除 | 到期后 OSS Lifecycle 执行 Expiration（**无需应用删除代码**） |
| 存量迁移 | 全部对象 tag `retention-years=3`，且存在 Days=1095 的规则 |
| 清单 | `oss-inventory` 展示 Lifecycle 匹配与预计删除时间 |
| 兼容 | 旧 config 无新字段时服务正常，等同默认 2 年 |

---

## 用户与场景

### US-1 全局默认 2 年

- **As a** 运维  
- **I want** 配置默认保存周期并同步 Lifecycle  
- **So that** 新上传自动 2 年后被 OSS 删除  

**验收**

- Given 无 `default_retention_years`，且已执行 `sync-lifecycle-rules`  
- When 上传不传 `retention_years`  
- Then 对象 tag 为 `retention-years=2`；命中 Days=730 的 Expiration 规则  

### US-2 前端指定保存周期

- **As a** 前端开发者  
- **I want** 上传时传 `retention_years`  
- **So that** 归档文件保留更久  

**验收**

- Given 已存在 `retention-years=5` 对应 Lifecycle 规则（Days=1825）  
- When `POST /v1/files` 且 `retention_years=5`  
- Then tag 为 5；响应含 `retention_years`、`retention_until`（预计删除时间）  

### US-3 存量 3 年

- **As a** 运维  
- **I want** 一次性为现有对象设置 3 年  
- **So that** 按 **各对象 LastModified** 起算 3 年后由 OSS 删除  

**验收**

- When `set-default-retention.sh --years 3`  
- Then 每对象 tag `retention-years=3`；Lifecycle 规则 Days=1095 已启用；inventory 显示预计删除时间  

> **官方语义**：[生命周期概述 FAQ](https://help.aliyun.com/zh/oss/user-guide/overview-54/) — 规则对配置**前**的存量 Object 同样生效，按 **Last-Modified-Time + Days** 删除。

---

## 功能需求

### F1 配置项

| ID | 需求 | 验收 |
|----|------|------|
| F1-1 | `oss.default_retention_years` 可选，默认 **2** | 缺失/0 时 fallback 2 |
| F1-2 | `oss.allowed_retention_years` 可选列表，如 `[2,3,5,10]` | 未配置时至少含 default 与迁移值 3 |
| F1-3 | `config.yaml.example` 注释说明 | 含 lifecycle 需 sync 脚本配合 |
| F1-4 | 环境变量 `OSS_DEFAULT_RETENTION_YEARS`（可选） | viper BindEnv |

```yaml
oss:
  bucket_prefix: "video_T/"
  default_retention_years: 2      # 可选，默认 2
  allowed_retention_years: [2, 3, 5, 10]   # 可选，上传允许的值
```

### F2 对象标签 + Lifecycle 规则（替代原 metadata 方案）

| ID | 需求 | 验收 |
|----|------|------|
| F2-1 | 对象 Tag 键 `retention-years`，值为年数字符串 | 与 Lifecycle Tag 筛选一致 |
| F2-2 | 每条允许的年份 N 对应一条 Lifecycle Rule | ID=`retention-years-N`，Prefix=bucket_prefix，Tag=N，Expiration Days=N×365 |
| F2-3 | `sync-lifecycle-rules` 合并写入规则 | GetBucketLifecycle → merge → PutBucketLifecycle，**不覆盖**非本系统规则 |
| F2-4 | 规则 Status=Enabled | 24h 内加载（官方） |

**删除执行方**：OSS Lifecycle Expiration，应用 **零** 删除逻辑。

### F3 上传 API

| ID | 需求 | 验收 |
|----|------|------|
| F3-1 | `POST /v1/files` 支持 `retention_years` | multipart 字段 |
| F3-2 | 未传 → `default_retention_years` | tag 与默认一致 |
| F3-3 | 值须在 `allowed_retention_years` 且已有 Lifecycle 规则 | 否则 400 |
| F3-4 | PutObject 时写入 Tagging | SDK `Tagging` option |
| F3-5 | 响应增加 `retention_years`、`retention_until` | until = LastModified.AddDate(years,0,0) 预计值 |

```json
{
  "id": "photo.jpg",
  "retention_years": 5,
  "retention_until": "2031-05-29T12:00:00Z",
  "view_url": "https://..."
}
```

> `retention_until` 为 **展示用预计时间**；实际删除以 OSS Lifecycle 批次为准（UTC 0 扫描，规则加载后 24h 起生效）。

### F4 运维脚本

| 脚本 | 用途 | 默认 |
|------|------|------|
| `scripts/sync-lifecycle-rules.sh` | 按 allowed 列表同步 Lifecycle 规则 | 必跑于上线前 |
| `scripts/set-default-retention.sh` | 存量 `PutObjectTagging` | `--years 3` |

**set-default-retention.sh**

| ID | 需求 |
|----|------|
| F4-1 | Bash + `go run ./scripts/set-retention/` |
| F4-2 | `--dry-run` / `--skip-existing` |
| F4-3 | 使用 **PutObjectTagging**，不用 CopyObject（避免 LastModified 漂移） |
| F4-4 | 执行前检查 Days=1095 规则存在，否则报错提示先 sync |

### F5 CLI 上传（Should）

| ID | 需求 |
|----|------|
| F5-1 | `oss-cli upload --retention-years` |

---

## 非功能需求

| 类别 | 要求 |
|------|------|
| 删除 | **OSS Lifecycle 负责**；应用不实现 cleanup |
| 生效时延 | 规则创建后 ≤24h 加载；删除在 UTC 0 批次（官方） |
| 向后兼容 | 旧 config / 旧客户端不传参 → 默认 2 年 tag |
| 安全 | sync/迁移脚本需 AK/SK；Precise Prefix 防误删 |
| 幂等 | sync 按 Rule ID  upsert；迁移 skip-existing |
| 可观测 | OSS 日志 `ExpireObject`；inventory 报表 |

---

## 优先级（MoSCoW）

| 优先级 | 项 |
|--------|-----|
| **Must** | F1、F2（tag+lifecycle）、F3、F4、sync-lifecycle-rules |
| **Should** | F5 CLI；inventory 增强；list 返回 retention |
| **Could** | 无 tag 对象的 Prefix 兜底 Expiration 规则 |
| **Won't** | 应用 cleanup-expired；Transition 转低频/归档；Last-Access-Time 规则；DashScope F5 |

---

## 里程碑

| 阶段 | 交付 |
|------|------|
| M1 | 架构/PRD 定稿（v2） |
| M2 | sync-lifecycle-rules + config |
| M3 | 上传打 tag + API 字段 |
| M4 | 存量 set-default-retention（3y） |
| M5 | 测试、inventory、文档 |

---

## 开放问题

| # | 问题 | 状态 |
|---|------|------|
| OQ-1 | Bucket 是否开启版本控制 | UNCONFIRMED；若开启需补充 NoncurrentVersion 规则 |
| OQ-2 | allowed 列表默认值 | 建议 `[2,3,5,10]`，可配置扩展 |
| OQ-3 | Days 用 365×N 还是日历 AddDate | 规则用 365×N；展示用 AddDate；接受 ±1～2 天偏差 |

---

## 不在范围

- 应用侧到期扫描与 DeleteObject  
- `link_expire_seconds` 变更  
- F5 DashScope 临时上传  
- 存储类型 Transition（低频/归档）  
- 数据库索引  

---

## 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-05-29 | 初版：x-oss-meta + 应用 cleanup |
| 2.0 | 2026-05-29 | 改为 Tag + OSS Lifecycle 自动删除；移除 cleanup |
