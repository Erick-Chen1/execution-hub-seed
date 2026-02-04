# Orchestrator 设计与执行逻辑

核心职责
- 依据 workflow/step 依赖关系选取可执行步骤
- 创建 Action 并触发分发
- 监控超时, 触发重试或升级
- 汇总任务状态

执行模型
- 建议使用事件驱动 + 定时扫描的混合模式
- 事件驱动: Step 状态变更时触发依赖检查
- 定时扫描: 兜底超时/卡死步骤

关键流程
1. Task 启动 -> 找到所有无依赖步骤
2. 为每个步骤创建 Action (step_id = action_id)
3. Action 分发 -> AI 执行或人类确认
4. Step 完成 -> 检查下游依赖 -> 触发下一步
5. Step 超时 -> 执行 on_fail 策略

幂等与并发
- Orchestrator 必须幂等, 避免重复创建 Action
- 依赖检查要有锁或乐观版本控制

失败策略
- 重试: FAILED -> CREATED
- 升级: 发通知到高级执行者
- 阻塞: 标记 BLOCKED 直到人工介入

建议技术实现
- 用数据库事务保证 Step 与 Action 一致
- 事件发布可复用现有消息队列
