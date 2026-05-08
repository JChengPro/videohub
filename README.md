# feedsystem_go

基于 Go 的短视频 Feed 系统（后端 + 前端），包含账号、视频、点赞、评论、关注与 Feed 流；支持 Redis 缓存与 RabbitMQ 异步 Worker。

详细设计与接口说明请阅读：`项目设计.md`

## Docker Compose 一键启动

要求：已安装 Docker Desktop / Docker Engine + Docker Compose。

```bash
docker compose up -d --build
```

访问：
- 前端：`http://localhost:5173`
- 后端 API：`http://localhost:8080`
- RabbitMQ 管理台：`http://localhost:15672`（默认账号 `admin` / `password123`）

Compose 会启动 `mysql`、`redis`、`rabbitmq`、`backend`（API）、`worker`、`frontend`。

## 本地开发启动

1) 先启动依赖：
```bash
docker compose up -d mysql redis rabbitmq
```

2) 启动后端 API：
```bash
cd backend
go run ./cmd
```

3) 启动 Worker（消费 MQ、更新热度/缓存）：
```bash
cd backend
go run ./cmd/worker
```

4) 启动前端（开发模式）：
```bash
cd frontend
npm install
npm run dev
```
