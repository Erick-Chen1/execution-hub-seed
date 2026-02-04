# 风险登记

- 风险: Orchestrator 幂等处理不足导致重复执行
  缓解: 对 Step 加唯一约束或分布式锁

- 风险: Evidence 聚合不一致
  缓解: EvidenceBundle 以 Action 为最小单位, Task 聚合只做引用

- 风险: 人工步骤长时间阻塞
  缓解: 超时 + 升级策略

- 风险: 任务 DAG 设计过复杂
  缓解: MVP 先支持简单条件与并行
