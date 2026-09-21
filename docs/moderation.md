# 视频审核模块

## 业务边界

`admin-frontend` 是独立网站，复用 Go API、MySQL、Redis、RabbitMQ 和存储。社区端仅增加投稿状态和通知反馈。

状态流转：

```text
上传 -> processing -> pending_review -> published
                   |                 -> rejected
                   -> failed
processing / pending_review / rejected / failed / published -> deleted
```

Worker 媒体处理成功后不再发布；只有管理员审核事务可以将待审内容发布。提交后禁止修改封面，避免更换已审核内容。修改投稿采用重新上传的新投稿，原有审核记录保留。

## 数据与一致性

- `staffs`：独立后台账号、bcrypt 密码、`owner/reviewer` 角色、`pending/active/disabled/deleted` 状态、添加人、创建时间、最近登录和 `deleted_at` 删除时间。新后台成员不会生成社区账号。
- `staff_states`：单行迁移标记和成员变更事务锁。首次将旧 `accounts.role=admin` 账号迁入 `staffs`，保留 ID、账号、昵称和密码摘要，赋予 owner；后续启动不会再次导入或覆盖成员权限。
- `staff_links`：每位成员最多一个邀请或重置链接，只存随机令牌的 SHA-256 摘要、用途与过期时间；有效期 24 小时。
- `staff_audits`：成员操作日志，记录操作人、目标、动作、角色／状态变更和时间，不记录凭据。
- `accounts.role`：仅保留为旧版迁移来源，新后台授权不再读取该字段。
- `admin_sessions`：只保存随机会话令牌的 SHA-256、后台成员 ID、密码指纹和过期时间（8 小时）。为兼容既有列，成员 ID 仍写入 `account_id` 列，语义为 `staffs.id`。
- `reviews`：每个投稿一条审核决定，包含审核人、决定、理由、时间。后续支持多轮复审时需引入版本并修改该唯一约束。
- `videos.status`：默认 `processing`，增加 `pending_review`、`rejected`。
- `videos.published_at`：审批通过时间；迁移时历史已发布视频使用原 `create_time` 回填。
- `videos.review_reason`：当前审核意见。

审核在 MySQL 行锁事务中校验待审状态，写审核记录、更新状态、写 Outbox。通过时写 `video_published`，通过和驳回都写 `notification_review`。已有 Worker 负责发布、幂等通知和实时提醒。并发审批、重复请求及撤回后审批返回 409，不产生重复事件。

人员变更、链接消费和首次初始化使用同一个状态行锁，事务内再次校验操作者会话和权限，避免在排队期间被撤权后仍写入。变更权限和停用前在锁内检查正常 owner 数量，防止并发移除最后一个管理员。审核写入也在该锁内重新校验工作人员权限，当前优先保证权限撤销与审批的顺序一致性；大规模审核吞吐场景可再细化锁粒度。

邀请接受和密码重置将设置密码、更新状态、删除一次性链接、撤销旧会话、写日志放在同一事务。后台身份与社区身份独立，审核通知使用系统 actor（新事件 `actor_id=0`），对外显示“审核中心”，避免工作人员 ID 被误当作社区用户 ID。

成员删除也使用同一事务锁，在事务内复核超级管理员权限、目标 ID 与确认账号、禁止自删和最后一个正常 owner 保护。删除将状态置为 `deleted`、记录时间、清空密码摘要、清除后台会话和链接，并追加一条 `delete` 日志。重复／并发删除中只有第一次成功，后续返回 404；确认账号不匹配返回 400，自删或最后管理员保护返回 409，审核员调用返回 403。

采用显式状态软删除，保留原始人员行、唯一登录账号和历史关联，成员列表与数量均排除 deleted。已删除人员不能通过修改角色、启用、邀请、重置、CLI 恢复或重复迁移重新获得权限；不提供恢复／回收站。保留的记录仍计入初始化检查，不会重新开放安装入口。旧审核记录的人员 ID 和姓名不改变，删除日志记录执行人和目标账号。

最新流和关注流按发布时间排序，Redis 使用 `feed:published_timeline:v2`，避免与旧上传时间索引混用。数据库迁移保留旧作品，不自动补造审核记录。发布新版本时必须同时更新 API 和 Worker，旧 Worker 不应继续消费媒体队列。

## 访问控制

公开详情、作品列表和信息流只展示 `published`。详情及信息流缓存读取仍核验数据库可见状态。

`/static/*filepath` 由媒体网关校验资源属于已发布视频的播放文件、选定封面或用户当前头像，禁止整个上传目录直接暴露，包括原始视频、分片和待审封面。

作者和管理员通过私有详情获得 5 分钟预览票据。票据限定视频与资源类型，每次请求重新检查数据库状态及关联会话；退出、改密码、角色撤销或撤回投稿会使其失效。票据属于短期 bearer 凭据，有效期内需避免分享链接。预览响应禁止缓存，支持 HTTP Range。本地文件通过 ServeContent，OSS 通过后端代理播放，不向浏览器提供原始 OSS 预览签名。

新上传 OSS 对象显式设置 private ACL，Bucket 也应保持私有并避免公开读取的 Bucket Policy。已经发布的 OSS 视频仍沿用现有签名播放地址。上传接口返回的文件定位路径并不是公开授权，提交和浏览器本地预览都不依赖这些路径可直接播放。

## 接口

所有下表业务接口均为 POST；预览媒体为 GET/HEAD。

| 接口 | 权限 | 功能 |
| --- | --- | --- |
| `/admin/login` | 管理员账号密码 | 获取独立审核会话 |
| `/admin/me`、`/admin/logout` | 审核会话 | 身份及退出 |
| `/admin/setup/status` | 公开 | 当前是否开放首位管理员初始化 |
| `/admin/setup` | 有效安装凭证且无成员 | 创建首位超级管理员 |
| `/admin/link/info`、`/admin/link/accept` | 有效一次性链接 | 查看邀请身份、设置密码激活或重置 |
| `/admin/password` | 审核会话及原密码 | 修改本人密码并清除后台会话 |
| `/admin/members` | 超级管理员 | query、status、offset 筛选及分页 |
| `/admin/members/invite` | 超级管理员 | 姓名、账号、角色，返回邀请链接 |
| `/admin/members/update` | 超级管理员 | 修改角色和状态，撤销会话及链接 |
| `/admin/members/delete` | 超级管理员 | id、account_name 二次确认，软删除成员并撤销所有凭据 |
| `/admin/members/link` | 超级管理员 | invite/reset/revoke，重新邀请、重置或撤销 |
| `/admin/audit` | 超级管理员 | 成员操作日志分页 |
| `/admin/videos` | 审核会话 | 状态、query、offset、limit 筛选 |
| `/admin/detail` | 审核会话 | 投稿、预览及审核记录 |
| `/admin/decide` | 审核会话 | id、approve/reject、reason |
| `/video/mine` | 社区 JWT | 当前作者全部未删除投稿分页 |
| `/video/submission` | 作者社区 JWT | 私有投稿详情及原因 |
| `/media/preview?ticket=...` | 有效预览票据及关联会话 | 播放文件或封面 |

超级管理员才能访问成员与日志接口，审核员被拒绝时返回 403，但不清除仍有效的审核登录；会话失效返回 401。公开登录、安装和链接接口有限流。链接格式为 `/activate#随机令牌`，由页面通过 POST 传给 API，避免凭据进入 URL 查询日志；响应禁止缓存，页面不使用第三方资源。当前邀请不验证邮箱所有权，由超级管理员通过受控渠道交付链接，持有有效链接的人可以激活指定账号。

## 验证

1. 对独立 MySQL 数据库运行 Go 集成测试，验证角色、会话隔离、并发审批、驳回、撤回与媒体访问控制。
2. 构建三个前端，检查 TypeScript 和生产打包；容器内执行 `nginx -t`。
3. 执行 API 完整验证，创建专用账号，生成 FFmpeg 样片，实际上传至当前存储并走 RabbitMQ 媒体处理和审核链路。
4. 使用 Playwright 验证独立审核站、桌面和手机端，保存截图并检查实际视频帧、布局、通知跳转和上传审批。

```bash
# 数据库必须是独立测试库，测试会创建表和测试数据。
cd backend
MODERATION_TEST_DSN='USER:PASSWORD@tcp(127.0.0.1:3307)/videohub_moderation_test?charset=utf8mb4&parseTime=True&loc=Local' go test -p 1 ./...

# 必须新建名称以 _staff_test 结尾的专用库；这组测试清空其中的人员测试表。
STAFF_TEST_DSN='USER:PASSWORD@tcp(127.0.0.1:3307)/videohub_staff_test?charset=utf8mb4&parseTime=True&loc=Local' go test ./internal/moderation -run TestStaffIntegration -v

# 回到仓库根目录后运行真实服务验证。
node scripts/verify-moderation.mjs
PLAYWRIGHT_MODULE=/path/to/node_modules/playwright node scripts/verify-moderation-browser.mjs
PLAYWRIGHT_MODULE=/path/to/node_modules/playwright node scripts/verify-staff-browser.mjs
```

浏览器脚本需要已安装 Playwright Chromium 及运行库，可通过 `PLAYWRIGHT_BROWSERS_PATH` 指定。API 脚本默认连接本机 8080，浏览器默认连接 5173、5174、5176；可通过 `VIDEOHUB_API` 和 `REVIEW_URL` 修改相关入口。先运行 API 脚本生成本轮数据，再运行浏览器脚本；浏览器脚本会审核本轮待审样片，重跑完整流程时应重新执行 API 脚本。

全新环境先在审核站完成初始化或邀请一个测试超级管理员，再以 `STAFF_ACCOUNT`、`STAFF_PASSWORD` 环境变量传给 API 脚本；脚本将其写入本地测试凭据文件的 `staff` 项，后续浏览器脚本复用。已有旧版验证账号迁移的环境兼容原 `admin` 项。脚本不会自行提升社区用户权限。成员浏览器脚本创建独立测试成员和新的投稿，最终验证软删除这些成员并保留日志；中途失败时停用尚未删除的测试成员。连续运行触发登录限流时，脚本等待窗口结束后重试，不关闭或绕过限流。

测试账号及凭据存于忽略提交的 `.run/moderation-verification/accounts.json`，文件权限 0600。测试输出也保存在该目录，生成的样片使用“审核验证”或“浏览器投稿验证”标题，便于识别。
