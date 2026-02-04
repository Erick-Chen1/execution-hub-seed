# 测试与 Demo 指引

建议测试范围
- Task 启动 -> Step 依赖推进
- Step 超时 -> 升级策略触发
- 人工 ACK -> 下游步骤继续
- EvidenceBundle 校验

Demo 演示流程
1. 创建 Workflow
2. 创建 Task
3. 启动 Task
4. 展示 AI 步骤输出
5. 人工复核 ACK
6. 展示 Task Evidence 回放

验收标准
- 状态机正确流转
- Evidence 可校验
- 审计记录完整
