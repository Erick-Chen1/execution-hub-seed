# Leaderless Multi-Agent + Multi-Human Collaboration Design

## 1. Goal
- Rebuild this project as a leaderless collaboration runtime.
- No legacy API compatibility requirement.
- Keep only the capabilities that matter: workflow, approval, audit, evidence, realtime notifications.
- Make first-run usage simple: join session, claim step, submit artifact.

## 2. Non-Goals
- Do not preserve old step APIs or old action semantics.
- Do not keep the `AgentRunner` execution path.
- Do not keep dual mode (`ASSIGNED` + `LEADERLESS`) in production design.

## 3. Target Architecture
Rule: no "main agent" as decision authority.  
A deterministic coordination kernel manages state, concurrency, policy, and security.

### 3.1 Core Services
- `session-service`: task-level collaboration room and lifecycle.
- `participant-service`: human/agent registry with capabilities and trust score.
- `claim-service`: lease-based step claiming with conflict control.
- `artifact-service`: versioned outputs and evidence links.
- `decision-service`: quorum/vote policy evaluation.
- `event-service`: append-only event log + SSE fanout.
- `coordination-kernel`: dependency evaluation and state transitions.

### 3.2 Execution Flow
1. Create session from workflow template.
2. Kernel opens claimable steps.
3. Participants claim with lease TTL.
4. Claimer submits artifact or handoff.
5. If policy requires, decision/vote is opened.
6. Step resolves after policy pass.
7. All changes are persisted as events + audit records.

## 4. Domain Model (Breaking Change)
Keep `task` and `workflow` as top-level entities, replace step execution model with collaboration entities:

- `session`
  - `session_id`, `task_id`, `status`, `created_at`
- `participant`
  - `participant_id`, `type(HUMAN|AGENT)`, `ref`, `capabilities[]`, `trust_score`
- `step_instance`
  - `step_id`, `session_id`, `step_key`, `status(OPEN|CLAIMED|IN_REVIEW|RESOLVED|FAILED)`
  - `required_capabilities[]`, `lease_ttl_sec`, `consensus_policy(jsonb)`
- `step_claim`
  - `claim_id`, `step_id`, `participant_id`, `lease_until`, `status(ACTIVE|EXPIRED|RELEASED)`
- `artifact`
  - `artifact_id`, `step_id`, `producer_id`, `kind`, `content`, `version`, `created_at`
- `decision`
  - `decision_id`, `step_id`, `policy`, `deadline`, `result(PENDING|PASSED|REJECTED)`
- `vote`
  - `vote_id`, `decision_id`, `voter_id`, `choice(APPROVE|REJECT)`, `comment`
- `collab_event`
  - `event_id`, `session_id`, `step_id`, `type`, `actor`, `payload`, `created_at`

## 5. API Contract (New Only)
Use a clean versioned API set: `/v2/collab/*`.

- `POST /v2/collab/sessions`
  - Create a session from a template/workflow.
- `POST /v2/collab/sessions/{sessionId}/join`
  - Join as human/agent.
- `GET /v2/collab/sessions/{sessionId}/steps/open`
  - List currently claimable steps.
- `POST /v2/collab/steps/{stepId}/claim`
  - Claim with lease request.
- `POST /v2/collab/steps/{stepId}/release`
  - Voluntary release.
- `POST /v2/collab/steps/{stepId}/handoff`
  - Transfer to another participant.
- `POST /v2/collab/steps/{stepId}/artifacts`
  - Submit artifact version.
- `POST /v2/collab/steps/{stepId}/decisions`
  - Open decision process.
- `POST /v2/collab/decisions/{decisionId}/votes`
  - Vote.
- `POST /v2/collab/steps/{stepId}/resolve`
  - Resolve step when policy is satisfied.
- `GET /v2/collab/sessions/{sessionId}/events`
  - Event replay.
- `GET /v2/collab/sessions/{sessionId}/stream`
  - SSE stream.

## 6. State Machine
`OPEN -> CLAIMED -> IN_REVIEW -> RESOLVED`  
`OPEN -> CLAIMED -> FAILED`  
`CLAIMED -> OPEN` (lease expired or released)  
`IN_REVIEW -> FAILED` (decision reject or timeout)

Concurrency rules:
- One active claim per step.
- Claim is transaction-protected by unique active constraint.
- Lease heartbeat extends `lease_until`.

## 7. Security and Governance
- AuthN: existing session/JWT infra can be reused.
- AuthZ: role + capability checks for claim/handoff/resolve/vote.
- Audit: every mutation emits both `audit_log` and `collab_event`.
- Evidence: artifacts and decisions are bound into evidence bundles.

## 8. First-Run Simplification
Keep `start-win11.cmd`, but shift default UX to template bootstrap:

- Auto seed template: `coauthor_review_publish`.
- Auto seed demo participants:
  - `agent:writer`
  - `agent:researcher`
  - `user:reviewer_a`
  - `user:reviewer_b`
- CLI/Web guided quickstart:
  1. Join session
  2. Claim step
  3. Submit artifact
  4. Vote
  5. Resolve

## 9. Implementation Plan (No Compatibility Track)

### Phase A (Week 1)
- Remove `AgentRunner` from orchestration path.
- Add new migrations for collaboration entities.
- Implement repositories and domain services.

### Phase B (Week 2)
- Implement `/v2/collab/*` handlers and SSE events.
- Add lease/claim conflict tests and timeout workers.

### Phase C (Week 3)
- Add `Collab` web page with open-steps, claims, artifacts, votes.
- Seed default template and demo participants in startup script.

### Phase D (Week 4)
- Delete obsolete step execution endpoints and old orchestrator branches.
- Clean docs and OpenAPI to v2-only model.

## 10. Done Criteria
- At least 2 agents + 2 humans can collaborate in one session without any central agent.
- End-to-end flow supports claim, handoff, artifact, vote, resolve.
- Every action is replayable from events and traceable in audit.
- New user can complete demo in 5 minutes after `start-win11.cmd`.

## 11. Immediate Code Changes (Recommended Order)
- `internal/application/orchestrator/orchestrator.go`
  - Replace current execution path with coordination-kernel transitions.
- `internal/domain/*` and `internal/infrastructure/postgres/*`
  - Add collab domain and persistence layers.
- `internal/api/http/server.go` + new `internal/api/http/collab_handlers.go`
  - Register and serve `/v2/collab/*`.
- `docs/openapi.yaml`
  - Rewrite to v2 collaboration contract.
- `scripts/start-win11.ps1`
  - Add template/demo seeding on first run.
- `web/src/pages/Collab.tsx`
  - Build collaboration-first UI.

## References
- LangGraph multi-agent concepts:
  - https://langchain-ai.github.io/langgraph/concepts/multi_agent/
- AutoGen Selector Group Chat:
  - https://microsoft.github.io/autogen/dev/user-guide/agentchat-user-guide/selector-group-chat.html
- AutoGen Swarm:
  - https://microsoft.github.io/autogen/dev/user-guide/agentchat-user-guide/swarm.html
- OpenAI Agents SDK handoffs:
  - https://openai.github.io/openai-agents-python/handoffs/
- Semantic Kernel Agent Group Chat:
  - https://learn.microsoft.com/semantic-kernel/frameworks/agent/agent-group-chat
