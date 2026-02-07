# Execution Hub Seed

Execution Hub 种子工程，包含后端 API 与 Web 前端，用于工作流编排、审批、审计与证据链管理。

请从文档索引开始阅读:
- `docs/INDEX.md`

OpenAPI 规范:
- `docs/openapi.yaml`

## 目录结构
- `docs/` 开发目标、需求、架构、模型、接口与计划
- `cmd/` 服务入口
- `internal/` 服务实现
- `web/` 前端项目
- `modules/ids/` 从原仓库复制的参考模块（语义对齐用）

## 本地运行
依赖:
- Go 1.24+
- Podman（支持 `podman compose`）
- Node 18+

启动 Postgres:
```
podman compose -f compose.yaml up -d
```

启动服务:
```
go run ./cmd/server
```

启动前端:
```
cd web
npm install
npm run dev
```

初始化管理员（首次）:
```
curl -X POST http://127.0.0.1:8080/v1/auth/bootstrap \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Strong!Passw0rd"}'
```

登录获取会话:
```
curl -X POST http://127.0.0.1:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Strong!Passw0rd"}'
```

## 测试
- 冒烟测试: `powershell -File scripts/smoke-test.ps1`
- 集成测试（Webhook/SSE）: 设置 `TEST_DATABASE_URL` 后运行 `go test -tags=integration ./internal/integration`

## 默认配置
- 服务地址: `SERVER_ADDR`（默认 `0.0.0.0:8080`）
- 数据库连接: `DATABASE_URL` 或 `POSTGRES_*` 环境变量（参见 `.env.example`）
- 会话配置: `SESSION_TTL` / `SESSION_COOKIE_NAME` / `SESSION_COOKIE_SECURE`
- 审计签名: `AUDIT_SIGNING_KEY`（可选，十六进制密钥）