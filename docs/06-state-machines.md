# 状态机与执行语义

## Step 状态机（复用 Action）
- CREATED -> DISPATCHING -> DISPATCHED -> ACKED/RESOLVED/FAILED
- FAILED 可根据重试策略回到 CREATED
- RESOLVED 为终态

## Task 状态机
- DRAFT -> RUNNING -> COMPLETED
- RUNNING -> FAILED (关键步骤失败且无补救)
- RUNNING -> CANCELLED
- RUNNING -> BLOCKED (等待人工/依赖)

## 升级策略
- 超时触发: step 超时 -> 自动改派或升级通知
- 失败触发: step FAILED -> 执行补救步骤或升级到人工
- 证据不足: step evidence 缺失 -> 回滚或阻塞
