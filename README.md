# contest-go

国防知识竞赛的精简 Go 重写版：只保留 CAS 登录、答题、自动交卷和历史分数。

## 决策

- 题目只存 Redis，并在进程内存缓存；启动时从 `data/problems.yaml` 导入一次。
- PostgreSQL 只保存最终成绩，一张 `scores` 表。
- 不保留 Django Admin、回顾、姓名、单位、本地密码、Celery。
- 多选全对得分。
- 断线续答与自动交卷依赖 Redis 草稿和 deadline ZSET。

## 本地开发

不依赖 Redis/PostgreSQL：

```bash
USE_MEMORY_STORE=1 DEV_LOGIN_ENABLED=true BANK_PATH=data/problems.yaml go run ./cmd/server
```

打开 <http://localhost:8080>，使用页面上的“开发登录”。

测试：

```bash
go test ./...
```

## 生产部署

镜像由 GitHub Actions 构建并推送到：

```text
ghcr.io/bitnp/contest-go:latest
```

`docker-compose.yml` 是单文件生产栈：

```text
app + PostgreSQL + Redis + Traefik labels
```

部署步骤：

```bash
cp .env.example .env
# 编辑 .env，填写 POSTGRES_PASSWORD、POSTGRES_URL、SESSION_SECRET、CAS_SERVICE_URL 等

# 题库不放进镜像，部署时放到 ./data/problems.yaml
cp /path/to/problems.yaml ./data/problems.yaml

podman-compose up -d
```

生产环境必须保持：

```text
DEV_LOGIN_ENABLED=false
COOKIE_SECURE=true
```

停止但保留数据：

```bash
podman-compose down
```

Redis 必须开启 AOF，除非使用 compose 中的默认配置：

```conf
appendonly yes
```

## 当前接口

```text
GET  /api/me
GET  /api/exam
POST /api/answer
POST /api/submit
GET  /api/scores
GET  /auth/cas/login
GET  /auth/cas/callback
GET  /auth/dev
POST /auth/logout
GET  /healthz
```

`/api/answer` 示例：

```json
{
  "question_id": 205,
  "choice_ids": [20500, 20502, 20503]
}
```
