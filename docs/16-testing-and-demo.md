# 测试与 Demo 指引

建议测试范围
- Task 启动 -> Step 依赖推进
- Step 超时 -> 升级策略触发
- 人工 ACK -> 下游步骤继续
- EvidenceBundle 校验
- 审批策略触发与审批通过后自动执行
- Trust 事件写入与 EvidenceBundle 验证

Demo 演示流程
1. 创建 Workflow
2. 创建 Task
3. 启动 Task
4. 展示 AI 步骤输出
5. 人工复核 ACK
6. 展示 Task Evidence 回放
7. 审批策略与审批流演示（Approvals + 审批配置）
8. Trust 事件写入与 EvidenceBundle 验证
9. 执行器/通知/Audit 查询演示

验收标准
- 状态机正确流转
- Evidence 可校验
- 审计记录完整

本地运行提示
- 启动数据库: `podman compose -f compose.yaml up -d`
- 启动服务: `go run ./cmd/server`
- 初始化管理员: `POST /v1/auth/bootstrap` 创建首个 ADMIN 用户
- 登录获取会话: `POST /v1/auth/login`（Cookie 会话或 Bearer）
- 启动前端: `cd web && npm install && npm run dev`
- 前端页面覆盖: Dashboard / Workflows / Tasks / Executors / Actions / Notifications / Approvals / Trust / Audit / Users

可重复冒烟测试
- 运行 `scripts/smoke-test.ps1`（自动启动 Postgres、启动服务、执行 API 冒烟流程并清理环境）
- 脚本内置 `AUDIT_SIGNING_KEY` 与独立端口，避免影响本地开发
- 脚本会自动执行 bootstrap + login，后续请求携带会话 Cookie

集成测试（Webhook/SSE）
- 需先设置 `TEST_DATABASE_URL` 指向独立测试库
- 运行 `go test -tags=integration ./internal/integration`
