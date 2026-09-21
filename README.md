# VideoHub

VideoHub 是一个基于 Go 开发的视频内容社区，支持账号登录、视频上传与人工审核发布、点赞评论、关注关系、互动通知、WebSocket 私信、视频流浏览和热视频排行，提供独立构建的桌面社区、手机社区和审核管理网站。

项目采用 **API + Worker 双进程模型**：API 负责鉴权、限流和同步写入核心业务数据；Worker 负责 Outbox 消息投递、RabbitMQ 消费、热度更新、缓存维护、视频媒体处理和文件清理。视频文件默认保存在本地，也可以通过环境变量切换至阿里云 OSS 私有 Bucket。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 后端 | Go、Gin、GORM、JWT |
| 前端 | Vue 3、TypeScript、Vite、Vue Router、Pinia、Nginx，桌面社区、手机社区和审核站独立构建 |
| 数据库 | MySQL 8 |
| 缓存与排行 | Redis、Redis ZSET、go-cache |
| 消息队列 | RabbitMQ |
| 实时通信 | WebSocket、Redis Pub/Sub |
| 文件存储 | 本地文件系统、阿里云 OSS 私有 Bucket |
| 媒体处理 | ffprobe、FFmpeg、H.264、AAC、MP4 |
| 并发控制 | MySQL 事务、行锁、singleflight |
| 容器化 | Docker、Docker Compose |

## 核心功能

| 模块 | 功能 |
| --- | --- |
| 社区账号 | 数字账号注册与登录、中文昵称、头像、个人资料、密码修改和退出 |
| 视频投稿 | 普通与分片上传、异步转码、处理进度、人工审核、私有预览、审核结果及撤回 |
| 视频浏览 | 推荐与关注视频流、点赞排行、热视频榜、桌面播放和手机竖屏切换 |
| 社区互动 | 点赞、评论、关注、粉丝列表及个人作品列表 |
| 通知与私信 | 互动与审核通知、WebSocket 私信、消息请求、未读数、已读和拉黑 |
| 独立审核站 | 待审队列、视频与封面预览、通过／驳回、审核记录、搜索和分页 |
| 后台人员 | 独立账号、角色授权、网页邀请激活、停用／启用、软删除、密码重置和操作日志 |
| 工程能力 | API 与 Worker 分离、Outbox、消息消费幂等、三级缓存、冷热分离和接口限流 |

### 独立审核网站

`admin-frontend/` 是独立审核管理网站，本地 Docker 入口为 [http://localhost:5176](http://localhost:5176)。视频上传后转码进入待审核，由审核人员批准后才会出现在社区。两端个人中心的“我的投稿与审核结果”支持查看处理状态、审核原因和私有预览，审核结果通过通知模块反馈。

后台人员使用独立账号和会话，无需先注册社区账号。日常添加人员全部通过网页完成：

```text
超级管理员登录 -> 成员管理 -> 添加成员 -> 填写姓名、登录账号和角色
-> 生成并复制邀请链接 -> 成员打开链接自行设置密码 -> 登录审核站
```

| 后台角色 | 视频审核与审核记录 | 成员邀请、角色变更、停用、删除、密码重置 | 成员操作日志 |
| --- | --- | --- | --- |
| 超级管理员 | 支持 | 支持 | 支持 |
| 审核员 | 支持 | 不允许 | 不允许 |

- 成员列表支持搜索、状态筛选、分页，以及查看添加人、最近登录和邀请状态。
- 邀请和密码重置链接 24 小时有效、只能使用一次，可撤销或重新生成；当前采用复制链接交付，尚未接入自动邮件发送。
- 角色变更、停用、发起密码重置、完成密码重置和修改密码会撤销已有后台会话。私有预览同样检查会话有效性。
- 成员管理操作记录操作人、目标、时间及角色／状态变更；并发操作也不能停用或降级最后一位正常的超级管理员。
- 超级管理员可在成员行点击删除图标，输入该成员的登录账号后确认删除。正常、停用和待激活成员均支持删除；禁止删除当前登录者和最后一位正常的超级管理员。
- 删除采用软删除：从成员列表移除，清除密码摘要、所有后台会话及邀请／重置链接，保留身份、历史审核和操作日志。删除不可通过启用或运维恢复撤销，登录账号保留占用；重新添加人员需使用新账号。停用适合暂时禁用，删除适合永久移除。
- 全新部署由部署负责人配置 `ADMIN_SETUP_TOKEN`，通过审核站 `/setup` 页面创建首位超级管理员，完成后自动关闭初始化入口。没有默认管理员密码。

初始化、人员管理和部署说明见 [审核站说明](admin-frontend/README.md)；数据结构、接口和验证步骤见 [审核模块文档](docs/moderation.md)。

## 双端客户端体验

桌面端和手机端共享同一套后端接口，但保持独立的页面结构和构建产物。两端使用统一的深色视觉语言、状态反馈和账号数据，不强行将桌面页面缩放成手机页面。

### 桌面端

- 使用侧边导航、顶部搜索和居中视频舞台，兼容宽屏、普通桌面和小屏桌面。
- 推荐流和关注流支持自动播放、上下切换、播放暂停、静音切换、点赞、关注、评论和分享。
- 播放器底部使用轻量进度线；鼠标交互时显示时间、滑块和倍速菜单，拖动时隐藏视频上的干扰信息。
- 支持键盘操作：`↑` / `↓` 切换视频、`Space` 播放或暂停、`M` 切换声音、`C` 打开评论、`Esc` 关闭弹层。
- 评论抽屉打开时暂停当前视频，关闭后只在视频原本处于播放状态时恢复。
- 发布页面支持视频预览、封面预览、真实上传进度和大文件分片上传，并在上传期间阻止误离开页面。
- 消息筛选、用户主页、个人中心和视频详情均提供加载、空数据、错误与重试状态。
- 私信支持实时收发、断线重连、历史消息分页、已读状态、请求接受/拒绝和拉黑。

### 手机端

- 使用 `100dvh`、`safe-area-inset-top` 和 `safe-area-inset-bottom` 适配移动浏览器地址栏、刘海和底部安全区。
- 推荐、关注和热门视频流采用一屏一个视频的纵向滚动吸附，只播放当前可见视频，快速滑动时自动暂停其他视频。
- 页面进入后台时暂停视频，恢复页面后仅按之前的播放状态恢复当前视频。
- 支持单击播放或暂停、双击点赞、长描述展开、静音切换、拖动播放进度、倍速播放、关注、评论、分享和游标分页。
- 评论 Bottom Sheet 支持遮罩关闭、`Esc` 关闭、焦点管理、背景滚动锁定、评论发布和删除。
- 消息未读数由 Pinia Store 统一维护；个人中心支持作品、喜欢、关注、粉丝、改名、改密码和删除作品。
- 私信页面按移动端单列交互实现，支持实时消息、会话列表、已读回执和消息请求处理。
- 发布页面支持手机常见视频格式、上传前预览、封面选择、分片上传进度和上传期间路由保护。

### 播放与格式说明

- 为满足浏览器自动播放策略，视频默认静音播放。页面显示“开启声音”表示当前处于静音状态，点击后才会播放声音；有声时按钮显示“关闭声音”。
- 当前支持上传 `MP4`、`MOV`、`M4V`、`MKV`、`WebM`、`AVI`、`3GP`、`3GPP`、`FLV`、`WMV`、`MPEG` 和 `MPG`，视频最大 200 MB，封面支持 JPG、PNG、WebP，最大 10 MB。
- 大于 10 MB 的视频自动按 5 MB 分片上传，小文件使用单次上传并展示真实网络进度。
- 上传完成后不会直接公开原文件。Worker 使用 ffprobe 验证真实媒体内容，再由 FFmpeg 统一输出 `MP4 + H.264 + AAC`，最高 1080P、保持宽高比且不放大低分辨率视频。
- 当前每个视频只生成一个标准播放文件，不提供多清晰度切换；播放器中的画质文字显示实际输出尺寸，不代表存在多个码率版本。

### 前端可靠性

- Feed、消息、用户主页和详情请求使用请求序号隔离，快速切换页面或筛选时，旧响应不会覆盖新状态。
- 点赞、关注、评论、删除、登录和发布操作具备请求中禁用或防重复提交处理。
- 视频 DOM 引用、`IntersectionObserver`、事件监听器、`requestAnimationFrame` 和预览 Object URL 会在切换或卸载时清理。
- 登录状态支持同源浏览器标签页同步；退出登录或切换账号时清理旧的关注、点赞、消息和个人资料状态。
- API 层兼容空响应、非 JSON 错误、401 登录失效、413 文件过大和上传网络异常。
- 主要图标按钮具备 `aria-label`，评论弹层支持焦点管理，并适配 `prefers-reduced-motion`。

## 系统架构

三个前端独立构建和部署，共享 Go API 与基础设施。社区网关按设备分流桌面端和手机端；审核站使用独立入口和身份体系，不参与社区设备分流。`picture/` 中保留的是早期设计资料。

```text
桌面 / 手机浏览器                    审核人员浏览器
        |                                 |
社区 Gateway（按设备分流）           独立审核入口
        |                                 |
Desktop Vue / Mobile Vue + Nginx     Admin Vue + Nginx
        |                                 |
        +---------- /api 反向代理 ---------+
                          |
                          v
Go API + WebSocket Hub
  |-- 参数校验 / 社区 JWT / 独立后台会话 / Redis 限流
  |-- 视频审核 / 后台角色与成员管理 / 私有媒体访问控制
  |-- 同步写 MySQL 业务表
  |-- 同事务写 outbox_msgs
  |-- 上传原始文件至 Local Storage / OSS
  |
  +----> Redis：token、缓存、时间线、热榜、实时事件 Pub/Sub
  |
  +----> MySQL：社区数据、后台人员、审核与操作日志、Outbox、消费记录
              |
              | Outbox Poller
              v
          RabbitMQ
              |
              v
            Worker
              |-- 消费幂等
              |-- ffprobe 校验、FFmpeg 转码和候选封面生成
              |-- 更新热度和热榜
              |-- 维护视频时间线
              |-- 清理缓存和实际文件
```

## 核心设计

### Outbox Pattern

业务写库和 MQ 投递之间存在双写一致性问题：MySQL 写入成功后，RabbitMQ 可能发送失败。

项目将业务数据和待发送事件放在同一个 MySQL 事务中提交：

```text
同步修改业务表
-> 同事务写 outbox_msgs
-> Poller 扫描 pending 消息
-> 条件更新 pending -> publishing，抢占消息
-> 发布 RabbitMQ
-> 成功后标记 published
-> 失败后恢复 pending 并记录 retry_count / last_error
```

多个 Worker 同时扫描到一条 Outbox 时，通过条件更新和 `RowsAffected` 判断谁抢占成功。卡在 `publishing` 超过一分钟的消息会被恢复为 `pending`。

当前通过 Outbox 投递的事件：

| 事件 | 同步主链路 | Worker 后置任务 |
| --- | --- | --- |
| `video_processing_requested` | 创建 processing 视频和 Outbox | 校验原文件、转码、生成封面、上传产物并更新状态 |
| `video_published` | 管理员审核通过事务写 Outbox | 写 Redis 发布时间线、清理旧视频流缓存 |
| `notification_review` | 审核通过或驳回事务写 Outbox | 幂等写审核通知并实时推送 |
| `video_deleted` | 视频状态改为 deleted、写 Outbox | 清理 Redis、删除本地或 OSS 文件 |
| `like_created` / `like_deleted` | 修改点赞关系和点赞数、写 Outbox | 更新热度、同步热榜、删除详情缓存 |
| `comment_published` / `comment_deleted` | 修改评论、写 Outbox | 更新热度、同步热榜 |
| 通知事件 | 点赞、评论、关注事务写通知 Outbox | 幂等写入 notifications 表 |

### RabbitMQ Connection 与 Channel 隔离

API 和 Worker 进程分别复用自己的 RabbitMQ TCP Connection，但不同并发职责不共享 AMQP Channel：

```text
API RabbitMQ Connection
└── HTTP MQ 测试请求临时 Channel（请求结束立即关闭）

Worker RabbitMQ Connection
├── Outbox Publisher Channel
├── Media Consumer Channel
├── Video Consumer Channel
├── Like Consumer Channel
├── Comment Consumer Channel
└── Notification Consumer Channel
```

- `RabbitMQ` 只持有进程级 Connection，`AMQPChannel` 包装单一职责使用的 Channel。
- Outbox Poller 在自己的 goroutine 内创建并长期持有 Publisher Channel。
- 五个已启动的 Consumer 分别创建、消费和关闭自己的 Channel；单个 Channel 初始化失败只结束对应消费者，不会直接终止整个 Worker。
- `/mq` 测试接口按 HTTP 请求临时创建 Channel，并通过 `defer` 在请求结束后关闭；正常业务写入通过 Outbox 投递，不长期占用 API Channel。

当前隔离解决的是多个并发角色共享同一 Channel 的生命周期和故障影响问题；自动重连、Publisher Confirm、QoS/prefetch、死信队列仍属于后续可靠性增强。

### MQ 消费幂等

RabbitMQ 消息可能因为 ACK 丢失或消费失败而被重复投递。项目使用 `consumed_events` 表记录已处理事件，并通过 `(event_id, consumer_name)` 联合唯一索引保证：

- 同一个消费者只能处理一次相同事件。
- 同一个事件未来可以被热度、通知、统计等不同消费者分别处理。

消费记录和 MySQL 热度更新放在同一个事务中。Redis 热榜不盲目重复执行 `ZINCRBY`，而是读取 MySQL 最终热度后使用 `ZADD` 覆盖 score，使 Redis 最终结果与 MySQL 一致。

### Redis 时间线与冷热分离

最新视频流使用 Redis ZSET 保存最近 1000 条视频：

```text
key    = feed:published_timeline:v2
member = video_id
score  = 发布时间毫秒时间戳
```

Redis 时间线最老数据的时间作为冷热边界：

```text
请求时间 > 冷热边界：从 Redis 读取热数据 ID
请求时间 <= 冷热边界：从 MySQL 读取历史冷数据
Redis 数据不足一页：继续从 MySQL 补齐
```

Redis 时间线为空时，从 MySQL 重建最近 1000 条 published 视频，并使用 singleflight 避免并发重复重建。

### 三级缓存与 singleflight

根据视频 ID 查询完整实体时使用：

```text
L1：进程内 go-cache，约 5 秒
-> L2：Redis video:entity:{id}，1 小时
-> L3：MySQL videos 表
```

同一视频缓存失效时，singleflight 合并当前 API 进程内的并发回源请求，减少重复查询 MySQL。视频详情缓存使用互斥锁和双重检查控制缓存重建。

### Redis 热榜

热视频榜使用 Redis ZSET：

```text
key    = feed:hot:zset
member = video_id
score  = popularity
```

点赞和评论事件由 Worker 异步更新 MySQL 热度，再用最终热度覆盖 Redis score。热榜查询优先从 Redis 获取排序后的 ID，再批量查询 published 视频；Redis 无数据时回退 MySQL。

### 业务正确性与并发控制

- 点赞、点赞数和 Outbox 在同一个 MySQL 事务中提交。
- 重复点赞使用唯一索引、`OnConflict DoNothing` 和 `RowsAffected` 实现请求幂等。
- 点赞、取消点赞和发表评论时使用 `SELECT ... FOR UPDATE` 锁定 published 视频，避免删除过程中继续产生互动。
- 作者可撤回未删除的投稿；视频删除通过状态条件更新避免重复写入删除 Outbox。
- 审核事务在行锁内检查待审状态、写入决定和发布／通知 Outbox，避免重复审批或批准已撤回的视频。
- 所有公开查询只返回 published 视频。
- API 同步清理关键缓存，Worker 消费删除事件后再次兜底清理。

### 文件存储与私有 OSS

业务层通过统一的 `Storage` 接口访问文件：

```go
type Storage interface {
    Upload(ctx context.Context, objectKey string, reader io.Reader) error
    Open(ctx context.Context, objectKey string) (io.ReadCloser, error)
    Delete(ctx context.Context, objectKey string) error
    URL(ctx context.Context, objectKey string, expires time.Duration) (string, error)
}
```

默认使用本地存储；配置 OSS 环境变量后切换为阿里云 OSS。

数据库使用稳定的 `object_key` 定位文件，已发布 OSS 视频的访问地址按需签名。Media Worker 通过同一接口读取原始文件、上传标准播放文件和候选封面；处理成功后清理原始对象，视频删除后再异步删除长期媒体产物。

未公开的视频只允许作者或审核人员通过绑定会话的短期票据预览。预览由后端代理并支持 HTTP Range，每次请求检查访问权限，不直接暴露待审 OSS 对象。本地静态入口也校验资源归属和发布状态，不开放原始文件、分片或待审媒体目录。

### 异步媒体处理

发布视频时，API 只创建 `processing` 记录并在同一事务写入 `video_processing_requested` Outbox，不在 HTTP 请求内执行耗时转码：

```text
上传/合并原始文件
-> API 创建 processing 视频 + Outbox
-> Outbox Poller 发布 feedsystem.video.processing.queue
-> Media Worker 下载原文件并运行 ffprobe
-> FFmpeg 输出 MP4 + H.264 + AAC（最高 1080P，不放大）
-> 在视频 25% / 50% / 75% 位置生成三张候选封面
-> 上传标准播放文件和封面
-> 数据库 processing -> pending_review，等待管理员审核
-> Redis 进度 completed=100
-> 清理原始对象和本地临时目录
-> 管理员通过后更新 published，并写 video_published 和审核通知 Outbox
```

- ffprobe 读取视频/音频编码、容器、时长、宽高和旋转信息，并拒绝仅音频或损坏文件。
- Redis 保存 `queued`、`downloading`、`probing`、`transcoding`、`generating_covers`、`uploading`、`completed`、`failed` 等阶段及百分比。
- 临时错误最多执行 3 次有限重试；媒体本身无效时直接失败，不做无意义重试。
- 输出 ObjectKey 按用户和视频 ID 固定生成，重复投递不会产生无限份媒体文件；所有产物完成后进入待审核，通过审核才公开。
- 状态接口返回处理阶段、进度、尝试次数和错误；投稿页提供作者私有预览及审核结果。封面在提交时固定，提交后不允许更换。

### 身份与访问控制

- 登录身份与公开昵称分离：新注册的 `account_name` 必须为 6–12 位唯一数字账号，`username` 是支持中文且可修改的公开昵称；历史字母账号继续兼容登录。
- 老数据库启动时会自动补齐 `account_name`；迁移账号暂时保留旧昵称登录兼容，新注册账号只使用 `account_name + password` 登录。
- 用户可以在网页端和手机端账号设置中上传 JPG、PNG 或 WebP 头像（最大 5MB）；未上传头像时由后端生成稳定的彩色 SVG 默认头像。
- JWTAuth 用于必须登录的发布、点赞、评论和关注接口。
- SoftJWTAuth 用于公开视频流：未登录可以访问，登录用户额外返回用户态信息。
- token 同时保存在 MySQL 和 Redis；退出登录时删除服务端 token，使旧 JWT 立即失效。
- 后台人员账号与社区账号分别保存，审核站使用独立的 8 小时随机会话；服务器仅保存令牌摘要，逐次核验成员状态、角色、密码指纹和有效期。
- 后台成员管理仅对超级管理员开放。权限变更与删除使用事务锁并复核会话，防止并发移除最后一个有效超级管理员。
- 登录和注册按 IP 限流；点赞、评论和关注按账号限流。
- Redis 不可用时限流采用 fail-open，优先保证核心业务可用。

### WebSocket 私信与实时通知

- 浏览器先通过 JWT 保护的接口获取 30 秒一次性 WebSocket ticket，再连接 `/ws`；ticket 在 Redis 中原子读取并删除，不能重复使用。
- WebSocket Hub 支持同一账号多设备连接、心跳保活、慢连接清理和发送队列隔离。
- API 和 Worker 通过 Redis Pub/Sub 发布实时事件，因此多 API 实例之间也能把私信、互动通知和审核结果推送到正确连接。
- 私信正文、会话状态、未读数和已读游标以 MySQL 为准；实时事件丢失时客户端可通过 REST 接口补拉，不会丢失正式消息。
- 非互关用户由一方发起消息请求，接收者回复或主动接受后双方可正常聊天；在此之前发起者最多发送三条。互关用户可直接聊天，任意一方拉黑后双方都不能继续发送。
- 每条消息带客户端幂等 ID，会话发送使用事务和行锁串行修改三条额度及未读数，避免并发绕过规则。

## 一键启动

环境要求：

- Docker Desktop 或 Docker Engine
- Docker Compose

默认使用本地文件存储，无需配置 OSS：

```bash
docker compose up -d --build
```

访问地址：

| 服务 | 地址 |
| --- | --- |
| 桌面端页面 | http://localhost:5173 |
| 手机端页面 | http://localhost:5174 |
| 独立审核网站 | http://localhost:5176 |
| 后端 API | http://localhost:8080 |
| RabbitMQ 管理台 | http://localhost:15672 |
| MySQL | localhost:3307 |
| Redis | localhost:6379 |

首次打开视频流时浏览器会以静音方式自动播放，点击视频可以播放或暂停，点击“开启声音”后才会输出声音。手机端也可以直接访问 `http://localhost:5174`，无需依赖自动设备分流。

审核站 Docker 端口可通过 `ADMIN_PORT` 修改。新投稿必须通过审核才会出现在社区，首次使用需先创建后台超级管理员。

### 创建首位超级管理员

1. 部署负责人使用 `openssl rand -hex 32` 生成安装凭证，在根目录 `.env` 中设置 `ADMIN_SETUP_TOKEN=生成的凭证`。
2. 执行 `docker compose up -d backend`，让 API 容器加载配置。
3. 打开审核站 `http://localhost:5176/setup`，填写安装凭证、姓名、登录账号和密码。
4. 创建完成后，初始化入口自动关闭；可移除环境中的安装凭证并重新创建 API 容器。

后续人员由超级管理员通过“成员管理”邀请，无需终端操作。已有后台成员时不能再次初始化。登录账号为 3 至 64 位字母、数字或 `. _ @ + -`；新密码至少 12 字符、最多 72 字节，首尾不能包含空格。

### 容器管理

RabbitMQ 本地演示账号：

```text
admin / password123
```

停止容器但保留数据：

```bash
docker compose stop
```

停止并删除容器：

```bash
docker compose down
```

停止并删除容器与数据卷：

```bash
docker compose down -v
```

### 升级已有环境

升级时重新构建 API、Worker 和三个前端：

```bash
git pull
docker compose up -d --build
```

API 启动时自动迁移数据库字段和审核相关表。历史已发布视频保持公开，发布时间使用原创建时间回填；新投稿转码后进入待审核。API 和 Worker 应同步更新，避免旧 Worker 继续执行直接发布逻辑。Worker 镜像内包含 ffmpeg/ffprobe。

旧 `accounts.role=admin` 账号首次升级时迁入独立后台账号，保留原登录名和密码并成为超级管理员，旧审核会话清除后需要重新登录。迁移只执行一次，之后社区和后台密码互相独立，成员停用或删除也不会因重新启动而恢复。

## 可选：启用阿里云 OSS

使用私有 OSS Bucket 时，在仓库根目录创建 `.env`：

```dotenv
STORAGE_TYPE=oss
OSS_ENDPOINT=https://oss-cn-shanghai.aliyuncs.com
OSS_REGION=oss-cn-shanghai
OSS_BUCKET_NAME=your-bucket-name
OSS_ACCESS_KEY_ID=your-ram-access-key-id
OSS_ACCESS_KEY_SECRET=your-ram-access-key-secret
```

建议使用仅拥有目标 Bucket 必要权限的 RAM 用户 AccessKey，不要使用阿里云主账号 AccessKey。

`.env` 已被 `.gitignore` 排除，不应提交到 Git。

配置后重新构建并启动：

```bash
docker compose up -d --build
```

## 服务器部署

仓库提供生产环境 Compose 示例：

```bash
git clone https://github.com/JChengPro/videohub.git
cd videohub
cp .env.production.example .env
```

修改 `.env` 中所有 `CHANGE_ME` 密码和 `JWT_SECRET`。启动前需准备 Compose 引用的 `deploy/nginx/videohub-gateway-http.conf` 网关配置；`deploy/` 属于本地部署材料，不随当前仓库提交。完成配置后执行：

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

生产 Compose 的社区网关对外开放 `80` 端口，按设备类型分流桌面社区和手机社区。审核站默认仅绑定服务器 `127.0.0.1:5176`，通过独立域名的 HTTPS 反向代理提供访问，不加入社区设备分流。

公网部署需自行配置域名、TLS 和实际环境的凭据。首次创建管理员使用上述安装凭证流程，重新创建服务时使用 `-f docker-compose.prod.yml`。

生产部署参数分别见 [`docker-compose.prod.yml`](docker-compose.prod.yml) 和 [`.env.production.example`](.env.production.example)。

## 项目结构

```text
.
├── backend/
│   ├── cmd/main.go                    # API 入口
│   ├── cmd/worker/main.go             # Worker 入口
│   ├── cmd/admin/main.go              # 运维恢复工具，日常人员管理使用审核网站
│   ├── internal/account/              # 账号模块
│   ├── internal/config/               # YAML 加载与环境变量覆盖
│   ├── internal/feed/                 # 视频流、冷热分离、三级缓存
│   ├── internal/media/                # ffprobe、FFmpeg 转码、进度解析和候选封面
│   ├── internal/mediagate/            # 作者投稿、私有预览和静态资源访问控制
│   ├── internal/message/              # 私信会话、消息策略和已读状态
│   ├── internal/middleware/           # JWTAuth / SoftJWTAuth
│   ├── internal/moderation/           # 视频审核、独立后台账号、邀请、角色、日志和迁移
│   ├── internal/mq/                   # RabbitMQ 和事件结构
│   ├── internal/notification/         # 点赞、评论、关注通知及实时推送
│   ├── internal/ratelimit/            # Redis 接口限流
│   ├── internal/realtime/             # WebSocket Hub、ticket 和 Redis Pub/Sub
│   ├── internal/social/               # 关注模块
│   ├── internal/storage/              # Local / OSS 存储实现
│   ├── internal/video/                # 视频、点赞、评论、Outbox
│   ├── internal/worker/               # Poller、Media Worker 和其他 MQ Consumer
│   └── Dockerfile                     # API / Worker 多阶段构建
├── frontend/                          # Vue 3 桌面端前端
│   └── src/                           # 框架、视频流、发布、消息、账号和详情页面
├── mobile-frontend/                   # Vue 3 手机端前端
│   └── src/                           # 竖屏视频流、评论弹层、底部导航和业务页面
├── admin-frontend/                    # 独立审核站、成员管理、激活和个人设置
├── docs/moderation.md                 # 最新审核及成员管理设计
├── docs/moderation-verification.md    # 分功能与完整链路验证记录
├── scripts/                          # 审核 API 与浏览器、成员管理浏览器验证
├── picture/                           # 早期架构与表结构资料（当前设计以本文为准）
├── test/                              # Postman 测试集合
├── docker-compose.yml                 # 服务编排
├── docker-compose.prod.yml            # 生产环境服务编排
├── 项目设计.md                         # 项目设计说明
└── README.md
```

## 接口概览

| 模块 | 接口 |
| --- | --- |
| 账号 | `/account/register`、`/account/login`、`/account/checkAccountName`、`/account/findByID`、`/account/findByUsername`、`/account/search`、`/account/changePassword`、`/account/rename`、`/account/avatar`、`/account/avatar/:id`、`/account/me`、`/account/logout` |
| 视频 | `/video/uploadCover`、`/video/uploadVideo`、`/video/uploadChunk`、`/video/chunkStatus`、`/video/mergeChunks`、`/video/publish`、`/video/processingStatus`、`/video/selectCover`、`/video/getDetail`、`/video/listByAuthorID`、`/video/delete` |
| 作者投稿 | `/video/mine`、`/video/submission`、`GET/HEAD /media/preview` |
| 后台登录 | `/admin/setup/status`、`/admin/setup`、`/admin/login`、`/admin/me`、`/admin/logout`、`/admin/password` |
| 视频审核 | `/admin/videos`、`/admin/detail`、`/admin/decide` |
| 成员管理 | `/admin/members`、`/admin/members/invite`、`/admin/members/update`、`/admin/members/delete`、`/admin/members/link`、`/admin/audit` |
| 成员激活 | `/admin/link/info`、`/admin/link/accept` |
| 视频流 | `/feed/listLatest`、`/feed/listByFollowing`、`/feed/listLikesCount`、`/feed/listByPopularity` |
| 点赞 | `/like/like`、`/like/unlike`、`/like/isLiked`、`/like/listMyLikedVideos` |
| 评论 | `/comment/publish`、`/comment/delete`、`/comment/listAll` |
| 关注 | `/social/follow`、`/social/unfollow`、`/social/getAllFollowers`、`/social/getAllVloggers` |
| 通知 | `/notification/list`、`/notification/unreadCount`、`/notification/markRead`、`/notification/markAllRead` |
| 私信 | `/message/listConversations`、`/message/listMessages`、`/message/send`、`/message/markRead`、`/message/accept`、`/message/reject`、`/message/block`、`/message/unblock`、`/message/unreadCount` |
| 实时通信 | `/realtime/wsTicket`、`GET /ws` |

## 本地开发

后端使用 Go 1.24 或更新版本；前端建议使用 Node.js 22.12+。本机运行 Worker 时需安装 FFmpeg 和 ffprobe。各服务和前端开发服务器分别在独立终端启动，以下启动命令均从仓库根目录执行。

只启动依赖：

```bash
docker compose up -d mysql redis rabbitmq
```

启动 API：

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

启动手机端前端：

```bash
cd mobile-frontend
npm install
npm run dev
```

启动独立审核前端：

```bash
cd admin-frontend
npm ci
npm run dev
```

审核前端 Vite 默认使用 5175，端口占用时自动递增，终端输出实际访问地址；Docker 部署默认使用 5176。前端通过 `/api` 代理访问后端，直接调用 API 时不带此前缀。未特别标注的业务接口均为 POST；`/video/selectCover` 保留兼容路由，但不允许修改已提交投稿的封面。

执行生产构建检查：

```bash
cd frontend
npm run build

cd ../mobile-frontend
npm run build

cd ../admin-frontend
npm run build
```

### 测试与检查

后端基础检查：

```bash
cd backend
go test ./...
go vet ./...
```

数据库集成测试需要独立 MySQL 测试库，通过 `MODERATION_TEST_DSN` 和 `STAFF_TEST_DSN` 指定；未配置时相应测试跳过。人员管理测试会清理专用测试库中的人员表，不能连接业务数据库。

`scripts/` 提供真实服务的审核 API、社区与审核站浏览器、成员管理浏览器验证。执行顺序、Playwright 依赖、测试账号配置和结果位置见 [审核模块文档](docs/moderation.md)。具体执行结果单独保存在 [验证记录](docs/moderation-verification.md)。

## 相关文档

| 文档 | 内容 |
| --- | --- |
| [审核站使用说明](admin-frontend/README.md) | 人员邀请、角色、密码、成员删除与初始化 |
| [审核模块设计](docs/moderation.md) | 视频状态、后台身份、接口、数据一致性和测试方法 |
| [部署参数示例](.env.production.example) | 生产环境变量和首次安装配置 |

## 后续优化方向

- 增加 Outbox 失败消息告警、指数退避、死信队列和重放接口。
- 增加 RabbitMQ 自动重连、Publisher Confirm 和 Consumer QoS/prefetch。
- 增加 OSS 孤儿对象定时清理、客户端直传和 CDN。
- 在现有单标准播放文件基础上按需要增加 HLS、多码率产物和 ABR 自适应播放。
- 将媒体任务的即时有限重试升级为延迟重试、死信队列、人工重放和孤儿产物巡检。
- 扩展现有审核与成员管理 Playwright 验证，覆盖更多社区功能和真实移动设备。
- 按实际需求接入邀请邮件、后台多因素认证和企业单点登录。
- 补充 Prometheus、Grafana、结构化日志和链路追踪。
- 补充并发、故障场景的自动化单元测试与集成测试。
- 将固定窗口限流升级为滑动窗口或令牌桶。
