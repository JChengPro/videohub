# VideoHub Mobile

独立的手机端 Vue 3 前端，复用项目现有 Gin API，不修改桌面端 `frontend/`。

## 本地运行

先启动后端，再执行：

```bash
cd mobile-frontend
npm install
npm run dev
```

手机与开发电脑处于同一局域网时，可访问：

```text
http://电脑局域网IP:5174
```

开发服务器会将 `/api` 和 `/static` 请求代理到 `127.0.0.1:8080`。

## 已接入功能

- 推荐、关注、热门竖屏视频流
- 自动播放、暂停、点赞、评论、关注和分享
- 视频上传与发布，大于 10MB 时自动使用分片上传
- 点赞、评论、关注通知
- 登录、注册、改名、修改密码和退出登录
- 个人作品、喜欢列表、关注列表、粉丝列表和删除视频

## 构建

```bash
npm run build
```

Dockerfile 和 Nginx 配置已放在本目录，后续可以作为独立移动站点容器部署。
