# VideoHub

VideoHub 是一个基于 Go + Gin + GORM + Redis + RabbitMQ + Vue3 的视频内容社区。项目包含账号、视频发布、点赞、评论、关注、最新视频流、关注视频流、点赞数视频流、热视频榜等功能，并提供 Docker Compose 一键启动。

后端采用 API + Worker 双进程模型：API 负责 HTTP 请求、参数校验、鉴权和同步写库；Worker 负责消费 RabbitMQ 消息，执行缓存失效、热度更新、Redis 时间线维护等异步任务。

详细设计见 [项目设计.md](项目设计.md)。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 后端 | Go, Gin, GORM, JWT |
| 前端 | Vue 3, Vite, TypeScript, Nginx |
| 数据库 | MySQL 8 |
| 缓存/限流/排行 | Redis, Redis ZSET, go-cache |
| 消息队列 | RabbitMQ |
| 异步任务 | Go Worker |
| 容器化 | Docker, Docker Compose |

## 一键启动

环境要求：

- Docker Desktop / Docker Engine
- Docker Compose

启动：

```bash
docker compose up -d --build
```

访问：

| 服务 | 地址 |
| --- | --- |
| 前端页面 | http://localhost:5173 |
| 后端 API | http://localhost:8080 |
| RabbitMQ 管理台 | http://localhost:15672 |
| MySQL | localhost:3307 |
| Redis | localhost:6379 |

RabbitMQ 默认账号：

```text
admin / password123
```

停止：

```bash
docker compose down
```

清空容器数据卷：

```bash
docker compose down -v
```

说明：仓库里的 `backend/configs/config.docker.yaml` 和 `docker-compose.yml` 使用本地演示默认密码，便于 clone 后直接运行。生产环境应改为环境变量或密钥管理，不应继续使用默认密码。

## 项目结构

```text
.
├── backend/              # Go API + Worker
├── frontend/             # Vue3 前端，Nginx 反向代理 /api 到 backend
├── picture/              # 架构图、流程图、表结构图
├── test/                 # Postman 测试集合
├── docker-compose.yml    # MySQL / Redis / RabbitMQ / API / Worker / Frontend
├── 项目设计.md            # 详细设计文档
└── README.md
```

`backend_bak/` 是本地备份目录，已通过 `.gitignore` 和 `.dockerignore` 排除，不属于对外提交内容。

## 系统架构

![整体架构](picture/整体架构.png)

核心链路：

- `frontend` 由 Nginx 托管，浏览器请求 `/api/*` 时反向代理到 `backend:8080`。
- `backend` 负责 HTTP API、JWT 鉴权、限流、同步写 MySQL 和写 Outbox。
- `worker` 同时运行 Outbox Poller 和 RabbitMQ Consumer。
- `mysql` 存储账号、视频、点赞、评论、关注、Outbox 消息。
- `redis` 承担 token 缓存、接口限流、视频详情缓存、视频时间线、热榜排行。
- `rabbitmq` 承担发布/删除视频、点赞、评论等后置事件投递。

## 核心功能

| 模块 | 功能 |
| --- | --- |
| 账号 | 注册、登录、退出、改密码、改用户名、查询用户 |
| 鉴权 | JWTAuth 强鉴权、SoftJWTAuth 软鉴权、Redis token 缓存 |
| 视频 | 上传封面、上传视频、发布、详情、作者视频列表、删除 |
| 视频流 | 最新视频流、关注视频流、点赞数视频流、热视频榜 |
| 点赞 | 点赞、取消点赞、是否点赞、我的点赞列表 |
| 评论 | 发表评论、删除评论、评论列表 |
| 关注 | 关注、取关、粉丝列表、关注列表 |
| 工程 | Docker Compose、Outbox、Redis 缓存、RabbitMQ Worker、限流、pprof |

## 核心设计

### Outbox Pattern

业务写库和 MQ 投递之间使用 Outbox Pattern：

```text
API 同步写业务表
-> 同一事务写 outbox_msgs
-> Worker 内的 Outbox Poller 扫描 pending 消息
-> 发布到 RabbitMQ
-> 成功后标记 published
-> 失败则退回 pending 并记录 retry_count/last_error
```

已接入 Outbox 的事件：

| 事件 | 业务表同步操作 | MQ 后置动作 |
| --- | --- | --- |
| `video_published` | 写 `videos` | 写入 Redis 视频时间线，删除旧视频流缓存 |
| `video_deleted` | 删除 `videos` | 从 Redis 视频时间线移除，删除旧视频流缓存 |
| `like_created` / `like_deleted` | 写/删 `likes`，事务更新 `videos.likes_count` | 更新 `popularity`、更新热榜、删除详情缓存 |
| `comment_published` / `comment_deleted` | 写/删 `comments` | 更新 `popularity`、更新热榜 |

Outbox Poller 使用 `pending -> publishing -> published` 状态流转，通过条件更新抢占消息，避免多 worker 同时处理同一条 Outbox 记录。

### 最新视频流

最新视频流已升级为 Redis 视频时间线 + 冷热分离 + singleflight + 三级缓存。

Redis 时间线：

```text
key = Redis 视频时间线
member = video_id
score = 视频发布时间毫秒时间戳
```

流程：

```text
最新视频流请求
-> 取 Redis 视频时间线最老 score 作为 watermark
-> 请求时间 > watermark：热数据，从 Redis ZSET 取 video_id
-> 请求时间 <= watermark：冷数据，查 MySQL
-> Redis 热数据不够一页：从 MySQL 补齐冷数据
-> GetVideoByIDs 根据 video_id 查询完整视频信息
```

三级缓存：

```text
L1：go-cache 本地内存缓存，视频实体缓存约 5 秒
L2：Redis，key = video:entity:{id}，TTL 1 小时
L3：MySQL videos 表
```

singleflight 用途：

| Key | 场景 |
| --- | --- |
| `sf:fallback:global_timeline_rebuild` | Redis 时间线为空时，只允许一个请求重建最近 1000 条 |
| `sf:cold:listLatest:{limit}:{reqTime}` | 冷数据分页查询去重 |
| `sf:stitch:listLatest:{limit}:{cursor}` | 冷热边界补数据查询去重 |
| `sf:entity:{videoID}` | 同一视频实体缓存 miss 时只查一次 MySQL |

### Redis 热榜

热榜使用 Redis ZSET：

```text
key = 热榜 ZSET
member = video_id
score = popularity
```

点赞/评论事件由 Worker 异步更新 `videos.popularity` 和 Redis 热榜 ZSET，热门流优先从 Redis 取 video_id，再回表查询视频详情并按 ZSET 顺序重排；Redis 无数据时回退 MySQL 按 `popularity desc, create_time desc, id desc` 查询。

### 鉴权与限流

![登录_鉴权_撤销token](picture/登录_鉴权_撤销token.png)

- 密码使用 bcrypt 哈希后入库。
- 登录成功后生成 JWT，并将 token 写入 MySQL 和 Redis。
- JWTAuth 用于必须登录的接口。
- SoftJWTAuth 用于视频流等公开接口：未登录可访问，登录时补充 `is_liked` 等用户态字段。
- 退出登录会清空 MySQL token 并删除 Redis token，使旧 token 立即失效。
- 登录/注册按 IP 限流，点赞/评论/关注按账号限流，计数存 Redis。

## 数据库设计

核心表：

- `accounts`
- `videos`
- `likes`
- `comments`
- `socials`
- `outbox_msgs`

![表关系](picture/表关系.png)

表结构图：

| 表 | 图 |
| --- | --- |
| 用户表 | ![用户表](picture/用户表.png) |
| 视频表 | ![视频表](picture/视频表.png) |
| 点赞表 | ![点赞表](picture/点赞表.png) |
| 评论表 | ![评论表](picture/评论表.png) |
| 关注表 | ![关注表](picture/关注表.png) |

## 接口概览

| 模块 | 接口 |
| --- | --- |
| 账号 | `/account/register`, `/account/login`, `/account/changePassword`, `/account/rename`, `/account/me`, `/account/logout` |
| 视频 | `/video/uploadCover`, `/video/uploadVideo`, `/video/publish`, `/video/getDetail`, `/video/listByAuthorID`, `/video/delete` |
| 视频流 | 最新视频流、关注视频流、点赞数视频流、热视频榜 |
| 点赞 | `/like/like`, `/like/unlike`, `/like/isLiked`, `/like/listMyLikedVideos` |
| 评论 | `/comment/publish`, `/comment/delete`, `/comment/listAll` |
| 关注 | `/social/follow`, `/social/unfollow`, `/social/getAllFollowers`, `/social/getAllVloggers` |

## 本地开发

只启动依赖：

```bash
docker compose up -d mysql redis rabbitmq
```

启动后端 API：

```bash
cd backend
go run ./cmd
```

启动 Worker：

```bash
cd backend
go run ./cmd/worker
```

启动前端：

```bash
cd frontend
npm install
npm run dev
```

## 验证

```bash
cd backend
GOCACHE=/tmp/videohub-go-build-cache go test ./...
```

Docker 配置校验：

```bash
docker compose config
```

## 后续优化方向

- 视频文件接入对象存储，例如 MinIO、OSS、S3，减少本地磁盘依赖。
- 热榜从当前全局 ZSET 继续升级为分钟窗口滑动热榜。
- 增加 Outbox 管理接口或告警，用于观察长期 pending 的消息。
- 为核心 service/repository 增加单元测试和集成测试。
