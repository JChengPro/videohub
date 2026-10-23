# VideoHub

VideoHub 是一个基于 Go 的视频内容社区，覆盖视频投稿、异步媒体处理、人工审核、视频流浏览、互动通知和 WebSocket 私信。项目提供独立构建的桌面端、手机端和审核管理网站，可通过 Docker Compose 在本地一键启动。

后端采用 **API + Worker 双进程模型**：API 负责鉴权、限流和核心业务写入；Worker 负责 Outbox 消息投递、RabbitMQ 消费、视频转码、缓存维护和文件清理。视频默认存储在 Docker 数据卷中，也可切换至阿里云 OSS 私有 Bucket。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 后端 | Go、Gin、GORM、JWT |
| 前端 | Vue 3、TypeScript、Vite、Pinia、Nginx |
| 数据库 | MySQL 8 |
| 缓存与排行 | Redis、Redis ZSET、go-cache、singleflight |
| 消息与实时通信 | RabbitMQ、WebSocket、Redis Pub/Sub |
| 文件与媒体 | Local Storage、阿里云 OSS、ffprobe、FFmpeg |
| 部署 | Docker、Docker Compose |

## 核心功能

| 模块 | 功能 |
| --- | --- |
| 账号 | 注册登录、JWT 鉴权、退出失效、头像与资料管理 |
| 视频投稿 | 普通上传、3 路并发分片、断点续传、异步转码、处理进度 |
| 审核发布 | 待审队列、私有预览、通过／驳回、审核记录与结果通知 |
| 视频浏览 | 推荐流、关注流、点赞排行、热榜和游标分页 |
| 社区互动 | 点赞、评论、关注、粉丝关系与个人作品 |
| 通知私信 | 互动通知、审核通知、WebSocket 私信、未读与拉黑 |
| 后台管理 | 独立后台账号、角色授权、成员邀请、停用与操作日志 |

桌面端与手机端共享同一套 API，但分别针对宽屏交互和移动端竖屏浏览设计。用户可以在“我的投稿与审核结果”中查看处理状态、驳回原因和未公开作品的私有预览。

## 系统架构

```text
桌面端 / 手机端                    审核管理端
       |                               |
       +--------- Nginx / API ----------+
                       |
                       v
                Go API + WebSocket
                 |       |       |
                 |       |       +--> Redis
                 |       +----------> Local / OSS
                 +------------------> MySQL
                                           |
                                      Outbox Poller
                                           |
                                           v
                                       RabbitMQ
                                           |
                                           v
                                         Worker
                              转码 / 通知 / 热榜 / 缓存清理
```

三个前端独立构建，共享 Go API 和基础设施。新投稿先进入 `processing`，Worker 完成媒体校验、转码和默认封面生成后进入 `pending_review`；只有审核通过并变为 `published` 后，视频才会公开出现在视频流和作者主页。

## 技术亮点

### 可靠消息与异步处理

- 业务数据和待发送事件在同一个 MySQL 事务中写入 Outbox，避免“数据库成功但 MQ 发送失败”的双写不一致。
- Poller 抢占待投递事件后发布到 RabbitMQ，异常中断的 `publishing` 消息会恢复为 `pending`。
- Worker 使用 `(event_id, consumer_name)` 联合唯一索引保证消费幂等，不同消费者仍可分别处理同一业务事件。
- RabbitMQ Connection 在进程内复用，发布者和各消费者使用独立 Channel，隔离并发职责和故障影响。

### 视频流与缓存

- Redis ZSET 保存最近 1000 条已发布视频，以发布时间作为 score；历史数据回退 MySQL，形成冷热分离。
- 视频实体采用 L1 本地缓存、L2 Redis、L3 MySQL 的三级查询链路。
- 缓存未命中时使用 singleflight 合并同一进程内的并发回源，降低热点数据失效时的数据库压力。
- 点赞和评论事件异步更新热度，并使用 Redis ZSET 维护热视频榜；Redis 不可用时回退 MySQL。

### 上传与媒体处理

- 大于 10 MB 的视频按 5 MB 分片，通过固定 3 路工作池并发上传。
- 上传任务使用文件名、文件大小和最后修改时间生成稳定 ID；重新选择同一文件时只补传缺失分片。
- 分片遇到网络错误、超时、限流或服务端错误时最多重试 3 次，服务端在合并前校验片号和完整文件大小。
- Worker 使用 ffprobe 校验媒体内容，再由 FFmpeg 输出最高 1080P 的 `MP4 + H.264 + AAC` 标准播放文件。
- 自定义封面为选填项；未提供时从视频开头附近生成一张默认封面。

### 业务正确性与安全

- 点赞、评论、审核和删除等关键写入使用事务、唯一索引、条件更新或行锁保证一致性与幂等。
- 公开查询只返回 `published` 视频，待审媒体通过绑定会话的短期票据进行私有预览。
- 社区使用 JWTAuth 和 SoftJWTAuth，退出登录时删除服务端 token，使旧 JWT 立即失效。
- 后台使用独立账号和 8 小时随机会话；登录注册按 IP 限流，互动接口按账号限流。
- WebSocket 使用 30 秒一次性 ticket 建立连接，正式消息持久化到 MySQL，实时事件丢失后可通过 REST 补拉。

完整实现说明见 [项目设计](项目设计.md)，审核模块见 [审核设计文档](docs/moderation.md)。

## 一键启动

### 环境要求

- Docker Desktop，或 Docker Engine + Docker Compose
- 建议至少预留 4 GB 可用内存

克隆并启动全部服务：

```bash
git clone https://github.com/JChengPro/videohub.git
cd videohub
docker compose up -d --build
```

第一次构建需要下载基础镜像和依赖，耗时取决于网络环境。容器启动后可执行以下命令检查状态：

```bash
docker compose ps
```

### 访问地址

| 服务 | 地址 |
| --- | --- |
| 桌面端 | http://localhost:5173 |
| 手机端 | http://localhost:5174 |
| 审核管理站 | http://localhost:5176 |
| 后端 API | http://localhost:8080 |
| RabbitMQ 管理台 | http://localhost:15672 |
| MySQL | `localhost:3307` |
| Redis | `localhost:6379` |

本地 RabbitMQ 演示账号为 `admin / password123`。首次播放遵循浏览器自动播放策略，以静音状态开始；点击“开启声音”后才会输出声音。

### 创建首位后台管理员

1. 执行 `openssl rand -hex 32` 生成安装凭证。
2. 在仓库根目录创建 `.env`，写入 `ADMIN_SETUP_TOKEN=生成的凭证`。
3. 执行 `docker compose up -d --force-recreate backend`，让 API 重新加载配置。
4. 打开 `http://localhost:5176/setup` 创建首位超级管理员。

初始化成功后入口会自动关闭。后续审核人员由超级管理员在审核站内邀请，详细规则见 [审核站说明](admin-frontend/README.md)。

### 常用容器命令

```bash
# 查看服务状态
docker compose ps

# 查看 API 和 Worker 日志
docker compose logs -f backend worker

# 停止服务但保留容器和数据
docker compose stop

# 停止并删除容器，保留数据卷
docker compose down

# 重新构建并启动
docker compose up -d --build
```

不要在需要保留数据时执行 `docker compose down -v`，该命令会同时删除 MySQL、Redis、RabbitMQ 和本地上传文件使用的数据卷。

## 可选：启用阿里云 OSS

默认配置使用本地存储。如需使用私有 OSS Bucket，在根目录 `.env` 中填写：

```dotenv
STORAGE_TYPE=oss
OSS_ENDPOINT=https://oss-cn-shanghai.aliyuncs.com
OSS_REGION=oss-cn-shanghai
OSS_BUCKET_NAME=your-bucket-name
OSS_ACCESS_KEY_ID=your-ram-access-key-id
OSS_ACCESS_KEY_SECRET=your-ram-access-key-secret
```

建议使用仅拥有目标 Bucket 必要权限的 RAM 用户 AccessKey。`.env` 已被 Git 忽略，不要提交真实凭据。修改配置后重新创建 API 和 Worker：

```bash
docker compose up -d --build --force-recreate backend worker
```

生产环境变量模板见 [`.env.production.example`](.env.production.example)，服务编排见 [`docker-compose.prod.yml`](docker-compose.prod.yml)。生产部署还需要根据服务器环境准备网关配置、密码、域名和 TLS，不应直接使用本地演示凭据。

## 项目结构

```text
.
├── backend/
│   ├── cmd/                    # API、Worker 和管理工具入口
│   └── internal/               # 账号、视频流、媒体、审核、MQ 等业务模块
├── frontend/                   # 桌面端 Vue 应用
├── mobile-frontend/            # 手机端 Vue 应用
├── admin-frontend/             # 独立审核管理应用
├── docs/                       # 模块设计与验证文档
├── scripts/                    # 审核功能验证脚本
├── test/                       # Postman 接口集合
├── docker-compose.yml          # 本地完整服务编排
├── docker-compose.prod.yml     # 生产环境编排模板
├── 项目设计.md                  # 详细架构与实现说明
└── README.md
```

## 本地开发

后端使用 Go 1.24 或更新版本；前端建议使用 Node.js 22.12 或更新版本。本机直接运行 Worker 时还需安装 FFmpeg 和 ffprobe。

```bash
# 只启动基础依赖
docker compose up -d mysql redis rabbitmq

# 启动 API
cd backend
go run ./cmd

# 启动 Worker（另一个终端）
cd backend
go run ./cmd/worker
```

三个前端分别进入 `frontend/`、`mobile-frontend/` 或 `admin-frontend/`，执行：

```bash
npm ci
npm run dev
```

## 测试

```bash
# 后端单元测试和静态检查
cd backend
go test ./...
go vet ./...

# 前端生产构建（在对应前端目录执行）
npm run build
```

数据库集成测试必须连接专用测试库，不能使用业务数据库。审核 API、浏览器和成员管理验证方式见 [审核模块文档](docs/moderation.md)。

## 相关文档

| 文档 | 内容 |
| --- | --- |
| [项目设计](项目设计.md) | Outbox、缓存、视频流、媒体处理和安全设计 |
| [审核站使用说明](admin-frontend/README.md) | 初始化、人员邀请、角色和密码管理 |
| [审核模块设计](docs/moderation.md) | 视频状态、后台身份、接口和一致性设计 |
| [审核验证记录](docs/moderation-verification.md) | 审核与成员管理验证方式和结果 |
| [生产环境变量模板](.env.production.example) | 生产部署需要配置的环境变量 |

## 后续计划

- 完善 RabbitMQ 自动重连、Publisher Confirm、QoS、死信队列和消息重放。
- 增加 HLS、多码率转码、CDN 和 OSS 客户端直传。
- 补充 Prometheus、Grafana、结构化日志和链路追踪。
- 扩展并发与故障场景的自动化测试。
