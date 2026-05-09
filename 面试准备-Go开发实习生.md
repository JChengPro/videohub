# Go 开发实习生面试准备

---

## 3 分钟自我介绍

面试官你好，我叫程健，上海理工大学计算机技术研二在读，25 年入学，预计 27 年毕业。

我本科开始接触编程，最早写 Java，拿过两次蓝桥杯省赛奖。读研后转向 Go，觉得 Go 的并发模型和简洁语法很适合后端开发。现在我主要的技术栈是 Go + Gin + GORM，MySQL 的索引优化和事务隔离级别都比较熟，Redis 的缓存策略和消息队列的基本使用也都实践过。Linux 常用命令和 Docker 没什么问题，日常开发和部署都在用。

前段时间独立做了一个短视频 Feed 流系统，算是把学的东西完整串了一遍。从数据库表结构设计、30 个 API 的开发，到 Redis 缓存、RabbitMQ 异步处理、Docker Compose 编排上线，都是自己一点点搭的。踩了不少坑——比如一开始把点赞和评论全走消息队列异步落库，发现用户点赞完刷新页面状态没变，后来改成了同步写库、MQ 只做热度更新。这个项目让我对后端系统的整体设计有了更深的理解，代码在 GitHub 上开源。

我觉得自己目前属于基础扎实、能独立干活但缺实战经验的状态。希望能在实习中碰到真实的高并发、大数据量的场景，跟着团队把技术深度提上来。

---

## 20 道面试题

---

### 第 1 题：项目相关 — 游标分页 vs 偏移分页

**问：你做 Feed 流的时候用的是游标分页（cursor-based），为什么不用传统的 `LIMIT + OFFSET`？**

**答：**

OFFSET 在 Feed 流场景有两个致命问题：

**1. 数据变化导致重复/遗漏。** 假设用户正在刷 Feed，每页 10 条：

```
第一页: LIMIT 10 OFFSET 0   → 拿到视频 1-10
（此时有人发布了新视频，插到最前面）
第二页: LIMIT 10 OFFSET 10  → 本来该拿视频 11-20，结果因为插入了新数据，视频 10 被挤到第二页，用户重复看到了
```

**2. 性能随 offset 增大而恶化。** `LIMIT 10 OFFSET 10000` 不是只读 10 条——MySQL 会扫描前面 10010 条然后丢弃，翻页越深越慢。

**游标分页怎么做：** 前端把上一页最后一条的时间戳传过来，SQL 变成 `WHERE create_time < ? ORDER BY create_time DESC LIMIT 10`。不管翻到第几页，都只扫描 10 条。而且即使中间插入了新数据，游标是固定时间点，不会出现重复或遗漏。

**为什么 listLikesCount 要用复合游标 `(likes_count, id)` 而不用单独的时间戳：** 因为按点赞数排序时，多条视频点赞数相同，单字段排序不稳定。加了 id 做第二排序字段后，`(likes_count, id)` 全局唯一，分页彻底稳定。

---

### 第 2 题：项目相关 — 架构演进

**问：你项目里点赞最开始是走 RabbitMQ 异步落库的，后来改成了同步写库。为什么一开始会用异步？实际踩了什么坑？**

**答：**

一开始参考了原项目的设计，把点赞、评论、关注全部走 RabbitMQ，Worker 消费后再写 MySQL。想法是接口快速返回，削峰填谷。

实际踩的坑是**用户体验**：点赞后前端立刻刷新页面调 `isLiked` 接口，Worker 还没处理完这条消息，`isLiked` 返回 `false`。用户刚点的赞，刷新就没了。点赞和评论是"自己操作自己立刻看结果"的场景，和"明星发微博通知千万粉丝"不一样——后者的粉丝不需要立刻看到，前者的操作者需要。

改完后，DB 写入走同步，RabbitMQ 只做热度更新和缓存失效——用户不直接等这些结果，异步没问题。

**追问：如果现在让你重新设计，你会一开始就用同步吗？**

不一定。像关注（Follow）这种操作，"关注完立刻看粉丝列表"的场景其实没那么高频，异步其实是可以接受的。关键不是"同步好还是异步好"，而是**判断这个操作的结果是不是操作者自己立刻要消费的**——是就同步，不是就可以异步。

---

### 第 3 题：项目相关 — 同步写库 vs 异步写库

**问：你项目里点赞、评论、关注是同步写库还是异步写库？为什么这样选？**

**答：**

同步写库。因为用户点赞后立刻刷新页面，`isLiked` 应该立刻返回 `true`。如果走 RabbitMQ 异步，Worker 还没处理到这条消息，用户看到的是"点赞没成功"，体验很差。

RabbitMQ 只做热度更新（popularity +/-1）、排行榜刷新（Redis ZINCRBY）、缓存失效这些用户不直接等待的后台任务。这样既保证了用户体验，又保持了架构解耦。

**追问：那之前为什么有人用异步写库？**

异步写库适合"写扩散"场景——比如一个明星发微博，要给 1000 万粉丝的 Feed 流各插入一条数据。如果同步做，发微博的请求要等 1000 万次写入完成才返回，接口直接超时。这种场景走 MQ 异步，粉丝的 Feed 流"最终一致"是可以接受的。但点赞这种"自己点完自己看"的操作，必须同步。

---

### 第 4 题：Go 基础 — slice 和数组的区别

**问：Go 里 `[3]int` 和 `[]int` 有什么区别？**

**答：**

`[3]int` 是数组，长度是类型的一部分，`[3]int` 和 `[5]int` 是不同类型。数组是值类型，赋值和传参会**整体拷贝**。

`[]int` 是 slice，底层是指向数组的指针 + len + cap。slice 是引用类型，赋值和传参只拷贝 header（24 字节），共享底层数组。

```go
arr := [3]int{1, 2, 3}
mutate(arr)   // arr 不变，因为传的是拷贝
fmt.Println(arr) // [1, 2, 3]

s := []int{1, 2, 3}
mutateSlice(s) // s 可能变，因为共享底层数组
```

实际开发中几乎都用 slice，数组只在极少数场景用（比如 `[32]byte` 作为 hash 结果）。

---

### 第 5 题：Go 基础 — map 的并发安全

**问：Go 的 map 是并发安全的吗？多个 goroutine 同时读写会怎样？怎么解决？**

**答：**

不是并发安全的。多个 goroutine 同时读写会触发 **fatal error: concurrent map writes**，程序直接 crash，recover 都抓不住。

```go
m := make(map[string]int)
go func() { for { m["a"] = 1 } }()
go func() { for { m["a"] = 2 } }()
// fatal error: concurrent map writes
```

三种解决方案：

1. **`sync.Mutex`**：读写前加锁
2. **`sync.RWMutex`**：读多写少场景，读用 RLock，写用 Lock，比 Mutex 并发度高
3. **`sync.Map`**：官方提供的并发安全 map，适合"key 写一次读多次"的场景，不适合频繁更新同一 key

你项目里 `social/service.go` 不需要并发控制因为每个请求是独立的 goroutine，不共享 map。

---

### 第 6 题：Go 基础 — goroutine 和 channel

**问：goroutine 和线程有什么区别？channel 的缓冲区满了会怎样？**

**答：**

**goroutine vs 线程：**
- 线程由 OS 调度，栈默认 1MB；goroutine 由 Go runtime 调度，栈初始 2KB 且可动态扩缩
- 线程切换需要内核态/用户态切换；goroutine 切换在用户态完成，代价极小
- 一个程序可以轻松跑数万个 goroutine，线程一般几百个就到头了

**channel 缓冲区满了：**
- 向满的无缓冲 channel（或者满的有缓冲 channel）发送数据会**阻塞**当前 goroutine，直到有接收方取走数据
- 从空 channel 接收也会阻塞，直到有发送方写入

这是 Go 的 CSP（Communicating Sequential Processes）模型：**不要通过共享内存来通信，而要通过通信来共享内存**。

---

### 第 7 题：Go 基础 — defer 的执行顺序和坑

**问：defer 的执行顺序是什么？下面代码输出什么？**

```go
func test() int {
    i := 0
    defer func() { i++ }()
    return i
}
```

**答：**

defer 是**后进先出**（栈顺序），注册顺序和执行顺序相反。

上面的代码输出 **0**，不是 1。因为 `return i` 的执行分三步：
1. 确定返回值 = i 的当前值（0）
2. 执行 defer（i 变成 1）
3. 函数返回（返回 0——已经确定的值）

defer 改的是局部变量 i，不影响已经确定的返回值。如果返回值是命名返回值 `func test() (i int)`，defer 里改 i 会影响返回值，因为返回值就是 i 本身。

---

### 第 8 题：MySQL — 索引

**问：你项目里 likes 表有 `UNIQUE KEY (video_id, account_id)`，为什么建这个索引？联合索引的最左前缀原则是什么？**

**答：**

这个唯一索引有两个作用：
1. **业务约束**：同一用户不能重复点赞同一视频（INSERT 重复会报错）
2. **查询加速**：`isLiked` 方法查的就是 `WHERE video_id = ? AND account_id = ?`，联合索引直接命中

**最左前缀原则**：联合索引 `(A, B, C)` 的索引树按 A 排序、A 相同按 B 排序、AB 相同按 C 排序。所以这个索引能加速：
- `WHERE A = ?` ✅（用到 A）
- `WHERE A = ? AND B = ?` ✅（用到 A+B）
- `WHERE A = ? AND B = ? AND C = ?` ✅（用到全部）

但不能加速：
- `WHERE B = ?` ❌（跳过了 A）
- `WHERE A = ? AND C = ?` ⚠️（只有 A 走索引，C 不走）

---

### 第 9 题：MySQL — 事务隔离级别

**问：MySQL 默认的事务隔离级别是什么？它能解决什么问题和不能解决什么问题？**

**答：**

MySQL InnoDB 默认隔离级别是 **REPEATABLE READ（可重复读）**。

**能解决的问题：**
- **脏读**：不会读到其他事务未提交的数据（MVCC + undo log 保证）
- **不可重复读**：同一事务内两次读同一行结果一致

**不能完全解决的问题：**
- **幻读**：同一事务内两次范围查询结果集不一致（别的事务 INSERT 了新行）。InnoDB 的间隙锁（Gap Lock）在一定程度上缓解了幻读，但不是 100% 消除

你项目的点赞事务 `LikeWithTx` 在 REPEATABLE READ 下是安全的，因为它操作的是具体行（`WHERE video_id = ?`），不是范围查询。

---

### 第 10 题：Redis — 缓存穿透/击穿/雪崩

**问：讲讲缓存穿透、缓存击穿、缓存雪崩的区别和解决方案。你项目里有遇到吗？**

**答：**

| 问题 | 现象 | 原因 | 解决 |
|------|------|------|------|
| **穿透** | 查一个不存在的数据，缓存没有，DB 也没有，每次请求都穿透到 DB | 恶意攻击或业务 bug | 布隆过滤器 / 缓存空值 |
| **击穿** | 一个热点 key 过期瞬间，大量并发请求同时打到 DB | 热点过期 | 互斥锁 / 永不过期 + 异步刷新 |
| **雪崩** | 大量 key 同时过期，DB 瞬间压力飙升 | 设置了相同的过期时间 | TTL 加随机值 / 多级缓存 |

**我项目里的处理：**

- **击穿**：Feed 首屏缓存 TTL 只有 30s，但在原版设计里有 `SETNX` 分布式锁的防击穿方案（未在当前版本实现，可以作为后续优化点）
- **token 鉴权自愈**：Redis 挂了 → 回退 MySQL → 通过后回填 Redis，这是一种容灾策略，不是防击穿，但也是 Redis 不可用时的降级保护

---

### 第 11 题：Redis — ZSET 实现热榜

**问：你项目里的热榜是怎么用 Redis 实现的？为什么用 ZSET 而不是 MySQL ORDER BY？**

**答：**

Redis 的 ZSET（有序集合），member 存视频 ID，score 存热度分。点赞 `ZINCRBY feed:hot:zset 1 video:123`，查询 `ZREVRANGE feed:hot:zset 0 19` 拿 top 20。

**为什么不用 MySQL：**
1. **性能**：ZSET 基于跳表（skiplist），ZINCRBY 是 O(log N)。MySQL 的 `ORDER BY popularity DESC` 每次全表扫描或走索引，100 万条数据差几十倍
2. **高频写入**：点赞是高频操作，每次都 UPDATE MySQL 的 popularity 字段会对同一行加锁，高并发下严重争用。ZSET 是内存操作，不存在行锁问题
3. **实时性**：热榜需要秒级更新，MySQL 扛不住

但 Redis 数据可能丢，所以 popularity 字段在 MySQL 里也有一份（`videos.popularity`），Worker 同步更新，Redis 挂了可以从 MySQL 重建。

---

### 第 12 题：网络 — HTTP 状态码

**问：你项目里接口返回了 200、400、401、404、500 这些状态码，分别什么场景用？401 和 403 有什么区别？**

**答：**

**项目里的实际使用：**
| 状态码 | 场景 |
|--------|------|
| 200 | 正常返回 |
| 400 | 参数校验失败（`ShouldBindJSON` 报错）、业务规则不满足（自己关注自己） |
| 401 | 没登录或 token 过期（JWT 中间件拦截） |
| 404 | 资源不存在（查用户查视频没找到） |
| 429 | 触发限流（登录接口短时间内请求过多） |
| 500 | 服务端内部错误（类型断言失败、DB 连接异常） |

**401 vs 403：**
很多人搞混。401 是"我不知道你是谁"——没认证、没 token、token 过期。403 是"我知道你是谁但你没权限"——比如普通用户尝试删别人的视频。我项目里评论删除的权限判断（`comment.AuthorID != accountID`）返回的是 400，其实用 403 更合适。

**追问：为什么不用 200 统一返回，body 里放 error 字段？**

有些公司的确这么做（比如微信支付 API）。好处是客户端解析简单，坏处是违背 HTTP 语义，中间代理/CDN 看不懂你的业务错误。现在 RESTful 风格更主流。

---

### 第 13 题：网络 — TCP 三次握手

**问：TCP 为什么是三次握手而不是两次？**

**答：**

核心原因是**防止已失效的连接请求到达服务端导致错误**。

假设只有两次握手：
1. 客户端发 SYN
2. 服务端回 SYN-ACK，连接就建立

如果客户端发的一个旧 SYN 因为网络延迟，在连接关闭后才到达服务端，服务端回 SYN-ACK 就建立了一个无效连接，浪费资源。

三次握手：
1. 客户端发 SYN
2. 服务端回 SYN-ACK
3. 客户端发 ACK 确认——**这次确认让服务端知道客户端确实想建立连接**，旧 SYN 的 ACK 客户端不会回应，服务端超时释放

还有另一个原因：三次握手让双方确认自己的发送和接收能力都正常，同时协商初始序列号。

---

### 第 14 题：操作系统 — 进程和线程

**问：进程和线程的区别？Go 的 goroutine 和它们又有什么不同？**

**答：**

| | 进程 | 线程 | goroutine |
|------|------|------|------|
| 调度者 | OS | OS | Go runtime |
| 内存 | 独立地址空间 | 共享进程内存 | 共享进程内存 |
| 切换代价 | 大（切换页表） | 中（内核态切换） | 小（用户态切换） |
| 栈大小 | 固定 MB 级 | 固定 1MB | 初始 2KB，动态扩缩 |
| 通信 | IPC（管道/共享内存） | 共享内存+锁 | channel |

goroutine 的核心优势是**用户态调度**——Go runtime 的 GMP 模型（G=goroutine, M=OS 线程, P=逻辑处理器）在用户态完成 goroutine 的调度和切换，不需要陷入内核，所以可以大量创建（几万个）且切换极快。

---

### 第 15 题：操作系统 — 死锁

**问：死锁的四个必要条件是什么？Go 里怎么避免死锁？**

**答：**

四个必要条件（缺一不可）：
1. **互斥**：资源同一时间只能被一个持有
2. **持有并等待**：持有一个资源的同时等待另一个
3. **不可剥夺**：资源不能被强制释放
4. **循环等待**：A 等 B，B 等 C，C 等 A——形成环

**Go 里避免死锁：**
- 加锁顺序一致：所有 goroutine 按相同顺序获取锁
- 用 `sync.Mutex` 的 `TryLock`（Go 1.18+）检测是否可获取
- 用 channel 替代 mutex：channel 天然避免了"持有并等待"
- `go run -race` 检测竞态条件

你项目里出现死锁风险的地方是 `social/service.go` 的 cache context——`context.WithTimeout` 加 `defer cancel()` 保证 context 一定释放，不会泄漏。

---

### 第 16 题：Go 基础 — interface 和空 interface

**问：Go 的 interface 是怎么实现的？`interface{}`（空接口）和 `any` 有什么区别？**

**答：**

interface 底层是一个二元组 `(type, value)`，type 指向具体类型的元信息，value 指向实际数据。所以一个 `var s interface{} = "hello"` 占 16 字节（两个指针），不是字符串的长度。

**`interface{}` 和 `any` 没有区别**——`any` 是 Go 1.18 引入的类型别名，完全是同一个东西。官方推荐用 `any`，更直观。

**空接口的使用场景（项目里就有）：**
```go
value, ok := c.Get("accountID")  // Get 返回 interface{}
accountID, ok := value.(uint)     // 类型断言转回具体类型
```

Gin 的 context 存储用 `map[string]interface{}`，因为中间件不知道 handler 需要什么类型，只能用空接口存，handler 拿的时候做类型断言。代价是类型安全从编译期推迟到了运行期——断言失败程序 panic。

**追问：怎么避免类型断言失败？**

用 comma-ok 模式：`accountID, ok := value.(uint); if !ok { return }`。不要直接用 `value.(uint)` 不带 ok，失败会 panic。

---

### 第 17 题：项目相关 — JWT vs Session

**问：你项目用 JWT 做登录鉴权，和传统的 Session 方案有什么区别？各有什么优缺点？**

**答：**

| | JWT | Session |
|------|-----|--------|
| 状态存储 | 客户端存 token（无状态） | 服务端存 session（有状态） |
| 扩展性 | 好，任何服务器都能验证 | 差，需要共享 session 存储 |
| 注销 | 麻烦，需要黑名单机制 | 简单，删 session 就行 |
| 数据量 | token 可能较大（header+payload+sig） | 只传 session_id，很小 |

**我项目里 JWT 的注销方案：**
`accounts` 表里有个 `token` 字段存当前有效 token。注销时清空这个字段。中间件验证时先查 Redis 缓存，miss 了查 DB 的 `token` 字段。这样即使 JWT 本身没过期，只要不匹配数据库里的 token 就会被拒绝，实现了"退出登录、改名后旧 token 立即失效"。

---

### 第 18 题：设计题 — 三层分层架构

**问：你项目用了 handler → service → repository 三层分层，每层具体负责什么？假设要加"视频播放量 +1"功能，各层怎么写？**

**答：**

**三层职责：**
- **handler**：和 HTTP 打交道——解析请求参数、调用 service、返回 JSON。不碰业务逻辑和 SQL
- **service**：业务逻辑——校验、判断、编排调用。不碰 HTTP 和 SQL
- **repository**：和数据库打交道——执行 SQL、返回模型对象。不碰业务逻辑

**"播放量 +1"各层的实现：**

```go
// repository: 纯数据库操作
func (r *Repository) IncrPlayCount(ctx context.Context, videoID uint) error {
    return r.db.WithContext(ctx).
        Model(&Video{}).Where("id = ?", videoID).
        UpdateColumn("play_count", gorm.Expr("play_count + 1")).Error
}

// service: 业务逻辑
func (s *Service) RecordPlay(ctx context.Context, videoID uint, ip string) error {
    // 业务判断：同一个 IP 30 秒内重复播放不计数（防刷）
    if s.recentlyPlayed(ctx, videoID, ip) {
        return nil
    }
    return s.repo.IncrPlayCount(ctx, videoID)
}

// handler: HTTP 层
func (h *Handler) Play(c *gin.Context) {
    var req struct{ VideoID uint `json:"video_id"` }
    c.ShouldBindJSON(&req)
    h.service.RecordPlay(c.Request.Context(), req.VideoID, c.ClientIP())
    c.JSON(200, gin.H{"message": "ok"})
}
```

**分层的好处：**
1. 改存储只改 repo（比如播放量从 MySQL 移到 Redis）
2. 改规则只改 service（比如防刷策略从"同 IP 30s"改成"同用户 10s"）
3. handler 永远不变——只要入参出参一样

这是后端开发最核心的设计习惯，面试官大概率会问到。

---

### 第 19 题：设计题 — 接口限流

**问：你项目里有登录接口的限流，怎么实现的？如果让你设计一个全局限流器防刷，你会怎么做？**

**答：**

项目里的限流用的 Redis：每次请求 `INCR login:attempts:<ip>`，设 TTL（比如 1 分钟），超过阈值（10 次）返回 `429 Too Many Requests`。这是一个简单的**固定窗口计数器**。

**固定窗口的问题**：窗口边界会被打爆。比如 1 分钟限制 10 次，有人在第 59 秒和第 61 秒各发 10 次（跨窗口），实际 2 秒发了 20 次。

**改进方案——滑动窗口**：
用 Redis 的 ZSET，member 存请求 ID/随机值，score 存时间戳。每次请求：
1. `ZREMRANGEBYSCORE` 删除窗口外的记录
2. `ZCARD` 统计窗口内请求数
3. 超过阈值则拒绝，否则 `ZADD` 记录本次

如果不想依赖 Redis，Go 可以用 `golang.org/x/time/rate` 包，基于令牌桶算法，本地限流。

---

### 第 20 题：开放题 — 如果用户量增长 100 倍

**问：你的 Feed 流项目如果日活从 100 人涨到 10000 人，你觉得哪些地方会先出问题？怎么优化？**

**答：**

**1. Feed 流查询（第一个瓶颈）**
当前 `/feed/listLatest` 每次请求都 `SELECT * FROM videos ORDER BY create_time DESC LIMIT 20`。1 万人同时刷，每秒几百次同样的 SQL。优化：
- Redis 缓存首页，TTL 设短（30s），新视频发布时主动失效
- 读写分离：读走从库，写走主库

**2. 点赞/评论的 popularity 写争用**
当前每次点赞 UPDATE 一次 videos 表。如果 1000 人同时点赞同一个视频，`videos` 的那一行被反复加锁。优化：
- popularity 字段完全交给 Redis ZSET，定时异步批量落库
- 或者用 `UPDATE videos SET popularity = popularity + ?`（原子操作，不需要 SELECT + UPDATE 两次）

**3. RabbitMQ 单点**
当前单 Worker 串行消费。峰值时消息堆积。优化：
- 增加 Worker 实例数，队列分片
- 或者用 Redis Stream 替代，支持消费者组

**4. 文件存储**
封面和视频存在本地磁盘。优化：
- 上 OSS/S3 + CDN，上传直接走对象存储，后端只存 URL

**5. 数据库连接池**
当前 GORM 默认连接池可能不够。配置 `SetMaxOpenConns(100)`、`SetMaxIdleConns(20)`。

**面试技巧**：这种题考察的是"能不能想到实际瓶颈"，不需要多完美的方案，关键是有分析思路——从流量入口一层层往下推：API → 缓存 → DB → 存储。
