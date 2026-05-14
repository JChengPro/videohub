# feedsystem_go

`feedsystem_go` 是一个基于 Go 和 Vue 的短视频 Feed 流系统，支持账号登录、视频发布、点赞、评论、关注、最新流、关注流、点赞数流和热榜等功能。

后端使用 Gin + GORM + MySQL 实现核心业务，Redis 用于 token 缓存、Feed 缓存、视频详情缓存、限流和 ZSET 热榜，RabbitMQ + Worker 用于热度更新、排行榜刷新和缓存失效等后台任务。项目提供 Docker Compose 一键启动，包含 MySQL、Redis、RabbitMQ、后端 API、Worker 和前端服务。

详细设计与接口说明见 [项目设计.md](项目设计.md)。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 后端 | Go, Gin, GORM, JWT |
| 前端 | Vue 3, Vite, TypeScript, Nginx |
| 数据库 | MySQL |
| 缓存/限流/热榜 | Redis, Redis ZSET |
| 消息队列 | RabbitMQ |
| 异步任务 | Go Worker |
| 容器化 | Docker, Docker Compose |

## Docker Compose 一键启动

环境要求：

- Docker Desktop / Docker Engine
- Docker Compose

启动全部服务：

```bash
docker compose up -d --build
```

访问地址：

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

停止服务：

```bash
docker compose down
```

Compose 会启动：

```text
mysql + redis + rabbitmq + backend(API) + worker + frontend
```

容器内后端配置使用 [backend/configs/config.docker.yaml](backend/configs/config.docker.yaml)，该文件会挂载到容器内的 `/app/configs/config.yaml`。

## 系统架构

![整体架构](picture/整体架构.png)

整体链路：

- `frontend` 通过 Nginx 将 `/api/*` 请求反向代理到 `backend:8080`。
- `backend` 负责 HTTP API、参数校验、鉴权、同步写库和事件投递。
- `worker` 消费 RabbitMQ 消息，处理热度更新、Redis ZSET 热榜更新和缓存失效。
- `mysql` 存储账号、视频、点赞、评论、关注等核心数据。
- `redis` 承担 token 缓存、Feed 缓存、视频详情缓存、限流计数和热榜排行。

## 核心功能

| 模块 | 功能 |
| --- | --- |
| 账号 | 注册、登录、退出、改密码、改用户名、查询用户 |
| 鉴权 | JWTAuth 强鉴权、SoftJWTAuth 软鉴权、Redis token 缓存 |
| 视频 | 上传封面、上传视频、发布、详情、作者视频列表、删除 |
| Feed | 最新流、关注流、点赞数流、热门流 |
| 点赞 | 点赞、取消点赞、是否点赞、我的点赞列表 |
| 评论 | 发表评论、删除评论、评论列表 |
| 关注 | 关注、取关、粉丝列表、关注列表 |
| 工程能力 | Docker Compose、Redis 缓存、RabbitMQ Worker、限流、pprof |

## 核心设计

### 账号与鉴权

![登录_鉴权_撤销token](picture/登录_鉴权_撤销token.png)

- 密码使用 bcrypt 哈希后入库。
- 登录成功后生成 JWT，并将 token 同时写入 MySQL 和 Redis。
- 鉴权中间件优先读取 Redis token，未命中时回源 MySQL，通过后回填 Redis。
- 退出登录会清空 MySQL token 并删除 Redis token，使旧 token 立即失效。
- `SoftJWTAuth` 用于 Feed 等公开接口：未登录可访问，带合法 token 时返回当前用户相关状态，例如 `is_liked`。

### Feed 流

![Feed 软鉴权_缓存热榜_分页游标](picture/Feed%20软鉴权_缓存热榜_分页游标.png)

Feed 返回结构统一为 `FeedVideoItem`，包含作者信息、视频信息、毫秒时间戳和当前用户是否点赞。

![Feed返回表](picture/Feed返回表.png)

- `/feed/listLatest`：最新流，基于 `latest_time` 游标分页，首页可缓存。
- `/feed/listByFollowing`：关注流，需要登录。
- `/feed/listLikesCount`：按点赞数排序，使用 `(likes_count, id)` 复合游标，避免排序不稳定。
- `/feed/listByPopularity`：热门流，优先读取 Redis ZSET，Redis 不可用时回退 MySQL。

### 点赞和评论

点赞、评论采用“同步写库 + 异步后置任务”的方式：

- 点赞/取消点赞同步写 `likes` 表，并在事务内更新 `videos.likes_count`。
- 评论发布/删除同步写入或删除评论表。
- RabbitMQ 只负责异步更新 `popularity`、刷新 Redis ZSET 热榜、删除视频详情缓存等后置任务。
- 主链路不依赖 MQ 成功，MQ 失败不会导致核心写库失败。

### Redis 缓存和热榜

| 场景 | 类型 | Key | 说明 |
| --- | --- | --- | --- |
| token 缓存 | String | `account:<id>` | 鉴权优先查 Redis，miss 后回源 MySQL |
| Feed 首页缓存 | String | `feed:latest:*` | 最新流首页短 TTL 缓存 |
| 视频详情缓存 | String | `video:detail:id=<id>` | 详情页缓存，点赞/删除等操作后主动失效 |
| 限流计数 | String | `feedsystem:ratelimit:*` | 登录/注册按 IP，写操作按账号 |
| 热榜 | ZSET | `feed:hot:zset` | member 为视频 ID，score 为热度分 |

Redis ZSET 热榜的查询流程：

```text
/feed/listByPopularity
-> ZREVRANGE feed:hot:zset 取 topN videoID
-> MySQL 回表查询视频详情
-> 按 ZSET 返回顺序重排
-> Redis 无数据时 fallback 到 MySQL popularity 排序
```

### RabbitMQ Worker

![点赞评论_同步写库_异步热度缓存](picture/点赞评论_同步写库_异步热度缓存.png)

| 队列 | 触发时机 | Worker | 后置动作 |
| --- | --- | --- | --- |
| `feedsystem.video.published.queue` | 发布/删除视频 | VideoWorker | 删除 `feed:latest:*` 缓存 |
| `feedsystem.like.queue` | 点赞/取消点赞 | LikeWorker | 热度 `+1/-1`、更新 ZSET、删除详情缓存 |
| `feedsystem.comment.queue` | 发评论/删评论 | CommentWorker | 热度 `+2/-2`、更新 ZSET |
| `feedsystem.social.queue` | 关注/取关 | SocialWorker | 关注关系异步处理 |

## 数据库设计

核心表包括：

- `accounts`
- `videos`
- `likes`
- `comments`
- `socials`

![表关系](picture/表关系.png)

表结构：

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
| Feed | `/feed/listLatest`, `/feed/listByFollowing`, `/feed/listLikesCount`, `/feed/listByPopularity` |
| 点赞 | `/like/like`, `/like/unlike`, `/like/isLiked`, `/like/listMyLikedVideos` |
| 评论 | `/comment/publish`, `/comment/delete`, `/comment/listAll` |
| 关注 | `/social/follow`, `/social/unfollow`, `/social/getAllFollowers`, `/social/getAllVloggers` |

更完整的请求和响应字段见 [项目设计.md](项目设计.md)。

## 本地开发启动

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

前端开发模式下，Vite 会将 `/api` 代理到：

```text
http://127.0.0.1:8080
```

## 项目目录

```text
.
├── backend/              # Go API + Worker
├── frontend/             # Vue3 前端
├── picture/              # 架构图、流程图、表结构图
├── test/                 # Postman 测试集合
├── docker-compose.yml    # 一键启动编排
├── 项目设计.md            # 详细设计与接口说明
└── README.md
```

## 后续优化方向

- 引入 Outbox Pattern，提高“业务写库 + 消息投递”的可靠性。
- 热榜支持时间窗口衰减，避免老视频长期占据榜单。
- 视频文件接入对象存储，例如 MinIO 或云存储。
- Feed 缓存进一步拆分为“公共视频列表缓存 + 用户态字段补全”。
- 为核心 service 和 repository 增加单元测试与集成测试。
