import type {
  ActionEvidence,
  ActionTransition,
  Approval,
  AuditLog,
  AuditQueryResult,
  EvidenceBundle,
  Executor,
  Notification,
  Step,
  Task,
  TaskEvidence,
  TrustEventIngestRequest,
  User,
  WorkflowDefinition,
  WorkflowSpec
} from './types';

type RequestOptions = RequestInit & { actor?: string };

const now = Date.now();
const iso = (minsAgo: number) => new Date(now - minsAgo * 60_000).toISOString();

let seq = 1;
const makeId = (prefix: string) => `${prefix}-${String(seq++).padStart(4, '0')}`;

const adminUser: User = {
  userId: 'user-admin',
  username: 'admin',
  role: 'ADMIN',
  type: 'HUMAN',
  status: 'ACTIVE',
  createdAt: iso(720)
};

const users: User[] = [
  adminUser,
  {
    userId: 'user-ops',
    username: 'ops',
    role: 'OPERATOR',
    type: 'HUMAN',
    status: 'ACTIVE',
    createdAt: iso(600)
  },
  {
    userId: 'user-view',
    username: 'viewer',
    role: 'VIEWER',
    type: 'HUMAN',
    status: 'ACTIVE',
    createdAt: iso(420)
  }
];

const agents: User[] = [
  {
    userId: 'agent-flow',
    username: 'agent-flow',
    role: 'OPERATOR',
    type: 'AGENT',
    ownerUserId: 'user-ops',
    status: 'ACTIVE',
    createdAt: iso(360)
  },
  {
    userId: 'agent-risk',
    username: 'agent-risk',
    role: 'OPERATOR',
    type: 'AGENT',
    ownerUserId: 'user-admin',
    status: 'ACTIVE',
    createdAt: iso(300)
  }
];

let currentUserId = adminUser.userId;

const workflowSpecs: WorkflowSpec[] = [
  {
    name: 'Data Intake',
    description: 'High-throughput ingestion flow',
    steps: [
      {
        step_key: 'extract',
        name: 'Extract',
        executor_type: 'AGENT',
        executor_ref: 'agent-flow',
        action_type: 'FETCH',
        action_config: { source: 's3://landing' }
      },
      {
        step_key: 'validate',
        name: 'Validate',
        executor_type: 'AGENT',
        executor_ref: 'agent-risk',
        action_type: 'CHECK',
        action_config: { policy: 'schema-v2' }
      },
      {
        step_key: 'enrich',
        name: 'Enrich',
        executor_type: 'AGENT',
        executor_ref: 'agent-flow',
        action_type: 'ENRICH',
        action_config: { model: 'feature-pack-v4' }
      },
      {
        step_key: 'load',
        name: 'Load',
        executor_type: 'HUMAN',
        executor_ref: 'ops',
        action_type: 'APPROVE',
        action_config: { target: 'warehouse' }
      }
    ],
    edges: [
      { from: 'extract', to: 'validate' },
      { from: 'validate', to: 'enrich' },
      { from: 'enrich', to: 'load' }
    ]
  },
  {
    name: 'Incident Response',
    description: 'Coordinated triage + remediation',
    steps: [
      {
        step_key: 'triage',
        name: 'Triage',
        executor_type: 'HUMAN',
        executor_ref: 'ops',
        action_type: 'ACK',
        action_config: { channel: 'pager' }
      },
      {
        step_key: 'contain',
        name: 'Contain',
        executor_type: 'AGENT',
        executor_ref: 'agent-risk',
        action_type: 'EXEC',
        action_config: { runbook: 'containment-v3' }
      },
      {
        step_key: 'recover',
        name: 'Recover',
        executor_type: 'HUMAN',
        executor_ref: 'admin',
        action_type: 'VERIFY',
        action_config: { checklist: 'rto-30m' }
      }
    ],
    edges: [
      { from: 'triage', to: 'contain' },
      { from: 'contain', to: 'recover' }
    ]
  }
];

const workflows: WorkflowDefinition[] = workflowSpecs.map((spec, idx) => ({
  workflowId: `wf-${idx + 1}-${spec.name.toLowerCase().replace(/\\s+/g, '-')}`,
  name: spec.name,
  version: 1,
  description: spec.description,
  status: 'ACTIVE',
  definition: spec,
  createdAt: iso(900 - idx * 40),
  updatedAt: iso(700 - idx * 30)
}));

const tasks: Task[] = [
  {
    taskId: 'task-0001',
    workflowId: workflows[0].workflowId,
    workflowVersion: 1,
    title: 'Onboard supplier feed',
    status: 'RUNNING',
    context: { priority: 'high', region: 'apac' },
    traceId: makeId('trace'),
    createdAt: iso(120),
    updatedAt: iso(10)
  },
  {
    taskId: 'task-0002',
    workflowId: workflows[1].workflowId,
    workflowVersion: 1,
    title: 'Edge outage response',
    status: 'CREATED',
    context: { priority: 'critical', zone: 'us-east' },
    traceId: makeId('trace'),
    createdAt: iso(200),
    updatedAt: iso(180)
  },
  {
    taskId: 'task-0003',
    workflowId: workflows[0].workflowId,
    workflowVersion: 1,
    title: 'Daily telemetry rollup',
    status: 'SUCCEEDED',
    context: { priority: 'normal' },
    traceId: makeId('trace'),
    createdAt: iso(480),
    updatedAt: iso(300)
  }
];

const approvals: Approval[] = [
  {
    approvalId: 'appr-1001',
    entityType: 'TASK',
    entityId: 'task-0002',
    operation: 'TASK_START',
    status: 'PENDING',
    requestedBy: 'ops',
    requiredRoles: ['ADMIN', 'OPERATOR'],
    approvalsCount: 0,
    rejectionsCount: 0
  },
  {
    approvalId: 'appr-1002',
    entityType: 'EXECUTOR',
    entityId: 'exec-ops',
    operation: 'DEACTIVATE',
    status: 'APPROVED',
    requestedBy: 'admin',
    requiredRoles: ['ADMIN'],
    approvalsCount: 2,
    rejectionsCount: 0
  }
];

const executors: Executor[] = [
  {
    executorId: 'exec-ops',
    executorType: 'HUMAN',
    displayName: 'Ops Desk',
    capabilityTags: ['triage', 'approval'],
    status: 'ACTIVE',
    metadata: { region: 'apac', timezone: 'UTC+8' },
    createdAt: iso(700)
  },
  {
    executorId: 'exec-agent-01',
    executorType: 'AGENT',
    displayName: 'Ingest Agent',
    capabilityTags: ['fetch', 'validate', 'enrich'],
    status: 'ACTIVE',
    metadata: { version: '1.4.2' },
    createdAt: iso(640)
  },
  {
    executorId: 'exec-agent-02',
    executorType: 'AGENT',
    displayName: 'Risk Agent',
    capabilityTags: ['policy', 'containment'],
    status: 'INACTIVE',
    metadata: { version: '2.1.0' },
    createdAt: iso(520)
  }
];

const notifications: Notification[] = [
  {
    notificationId: 'notif-0001',
    channel: 'sse',
    status: 'QUEUED',
    title: 'Approval needed',
    body: 'Task start requires approval.',
    traceId: makeId('trace'),
    createdAt: iso(30)
  },
  {
    notificationId: 'notif-0002',
    channel: 'email',
    status: 'SENT',
    title: 'Daily rollup complete',
    body: 'Telemetry rollup finished.',
    traceId: makeId('trace'),
    createdAt: iso(90),
    sentAt: iso(85)
  }
];

const auditLogs: AuditLog[] = [
  {
    auditId: 'audit-0001',
    entityType: 'TASK',
    entityId: 'task-0001',
    action: 'TASK_START',
    actor: 'ops',
    riskLevel: 'MEDIUM',
    traceId: makeId('trace'),
    createdAt: iso(60)
  },
  {
    auditId: 'audit-0002',
    entityType: 'APPROVAL',
    entityId: 'appr-1002',
    action: 'APPROVAL_DECIDE',
    actor: 'admin',
    riskLevel: 'LOW',
    traceId: makeId('trace'),
    createdAt: iso(140)
  },
  {
    auditId: 'audit-0003',
    entityType: 'EXECUTOR',
    entityId: 'exec-agent-02',
    action: 'EXECUTOR_DEACTIVATE',
    actor: 'admin',
    riskLevel: 'HIGH',
    traceId: makeId('trace'),
    createdAt: iso(200)
  }
];

const stepsByTaskId: Record<string, Step[]> = {};

function dependsMap(spec: WorkflowSpec): Record<string, string[]> {
  const map: Record<string, string[]> = {};
  (spec.edges || []).forEach((edge) => {
    if (!map[edge.to]) {
      map[edge.to] = [];
    }
    map[edge.to].push(edge.from);
  });
  return map;
}

function buildSteps(task: Task, spec: WorkflowSpec): Step[] {
  const deps = dependsMap(spec);
  return spec.steps.map((s, idx) => {
    let status = 'PENDING';
    if (task.status === 'SUCCEEDED') status = 'RESOLVED';
    if (task.status === 'RUNNING') status = idx === 0 ? 'RESOLVED' : idx === 1 ? 'RUNNING' : 'PENDING';
    if (task.status === 'FAILED') status = idx === 1 ? 'FAILED' : 'PENDING';
    return {
      stepId: `${task.taskId}-${s.step_key}`,
      taskId: task.taskId,
      stepKey: s.step_key,
      name: s.name,
      status,
      executorType: s.executor_type,
      executorRef: s.executor_ref,
      actionType: s.action_type,
      actionConfig: s.action_config,
      dependsOn: deps[s.step_key],
      actionId: `${task.taskId}-act-${idx + 1}`,
      traceId: task.traceId,
      createdAt: task.createdAt
    };
  });
}

function seedSteps() {
  tasks.forEach((task) => {
    const spec = workflows.find((w) => w.workflowId === task.workflowId)?.definition as WorkflowSpec;
    stepsByTaskId[task.taskId] = spec ? buildSteps(task, spec) : [];
  });
}

seedSteps();

function listWithLimit<T>(items: T[], limitParam: string | null) {
  const limit = limitParam ? Number(limitParam) : items.length;
  if (!Number.isFinite(limit)) return items;
  return items.slice(0, Math.max(0, limit));
}

function parseBody(body?: BodyInit | null): any {
  if (!body) return undefined;
  if (typeof body === 'string') {
    try {
      return JSON.parse(body);
    } catch {
      return undefined;
    }
  }
  return undefined;
}

function getCurrentUser() {
  return users.find((u) => u.userId === currentUserId) || adminUser;
}

function updateTaskStatus(taskId: string, status: string) {
  const task = tasks.find((t) => t.taskId === taskId);
  if (task) {
    task.status = status;
    task.updatedAt = iso(0);
  }
  return task;
}

function updateStepStatus(taskId: string, stepId: string, status: string) {
  const steps = stepsByTaskId[taskId];
  if (!steps) return;
  const step = steps.find((s) => s.stepId === stepId);
  if (step) {
    step.status = status;
  }
}

function buildTaskEvidence(taskId: string): TaskEvidence {
  const steps = stepsByTaskId[taskId] || [];
  return {
    task_id: taskId,
    trace_id: tasks.find((t) => t.taskId === taskId)?.traceId || makeId('trace'),
    steps: steps.map((s) => ({
      step: s,
      transitions: [
        {
          actionId: s.actionId || makeId('act'),
          fromStatus: 'PENDING',
          toStatus: s.status,
          transitionedAt: iso(5),
          reason: 'demo'
        }
      ]
    }))
  };
}

function buildActionEvidence(actionId: string): ActionEvidence {
  return {
    Action: { actionId, status: 'RESOLVED', detail: 'mock evidence' },
    Evaluation: { score: 0.92, model: 'v4' },
    Rule: { id: 'policy-17', outcome: 'PASS' }
  };
}

function buildActionTransitions(actionId: string): ActionTransition[] {
  return [
    { actionId, fromStatus: 'PENDING', toStatus: 'ACKED', transitionedAt: iso(20), reason: 'operator ack' },
    { actionId, fromStatus: 'ACKED', toStatus: 'RESOLVED', transitionedAt: iso(5), reason: 'auto resolve' }
  ];
}

function buildEvidenceBundle(kind: string, subject: string): EvidenceBundle {
  return {
    bundleId: makeId('bundle'),
    bundleType: kind,
    subjectId: subject,
    generatedAt: iso(0),
    events: [{ event: 'demo', subject, ts: iso(2) }],
    hashChain: [{ hash: '0xabc', prev: '0x000' }],
    signatures: [{ signer: 'demo-signer', sig: '0xdeadbeef' }],
    verification: { status: 'VERIFIED', policy: 'demo' }
  };
}

function buildAuditResult(): AuditQueryResult {
  return {
    logs: auditLogs,
    pagination: { hasMore: false, count: auditLogs.length, total: auditLogs.length }
  };
}

function createApprovalForTask(taskId: string) {
  const approval: Approval = {
    approvalId: makeId('appr'),
    entityType: 'TASK',
    entityId: taskId,
    operation: 'TASK_START',
    status: 'PENDING',
    requestedBy: getCurrentUser().username,
    requiredRoles: ['ADMIN', 'OPERATOR'],
    approvalsCount: 0,
    rejectionsCount: 0
  };
  approvals.unshift(approval);
  return approval;
}

export async function mockApiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const method = (options.method || 'GET').toUpperCase();
  const url = new URL(path, 'http://demo.local');
  const pathname = url.pathname;
  const body = parseBody(options.body);

  if (pathname === '/v1/auth/me' && method === 'GET') {
    return getCurrentUser() as T;
  }
  if (pathname === '/v1/auth/login' && method === 'POST') {
    const username = body?.username || 'admin';
    const existing = users.find((u) => u.username === username);
    if (existing) {
      currentUserId = existing.userId;
    } else {
      const newUser: User = {
        userId: makeId('user'),
        username,
        role: 'OPERATOR',
        type: 'HUMAN',
        status: 'ACTIVE',
        createdAt: iso(0)
      };
      users.push(newUser);
      currentUserId = newUser.userId;
    }
    return { ok: true } as T;
  }
  if (pathname === '/v1/auth/logout' && method === 'POST') {
    currentUserId = adminUser.userId;
    return { ok: true } as T;
  }
  if (pathname === '/v1/auth/bootstrap' && method === 'POST') {
    const username = body?.username || 'admin';
    adminUser.username = username;
    currentUserId = adminUser.userId;
    return { ok: true } as T;
  }

  if (pathname === '/v1/users' && method === 'GET') {
    return { users: listWithLimit(users, url.searchParams.get('limit')) } as T;
  }
  if (pathname === '/v1/users' && method === 'POST') {
    const newUser: User = {
      userId: makeId('user'),
      username: body?.username || `user-${seq}`,
      role: body?.role || 'OPERATOR',
      type: 'HUMAN',
      status: 'ACTIVE',
      createdAt: iso(0)
    };
    users.push(newUser);
    return newUser as T;
  }
  if (pathname === '/v1/agents' && method === 'GET') {
    return { agents: listWithLimit(agents, url.searchParams.get('limit')) } as T;
  }
  if (pathname === '/v1/agents' && method === 'POST') {
    const newAgent: User = {
      userId: makeId('agent'),
      username: body?.username || `agent-${seq}`,
      role: body?.role || 'OPERATOR',
      type: 'AGENT',
      ownerUserId: body?.owner_user_id || users[0].userId,
      status: 'ACTIVE',
      createdAt: iso(0)
    };
    agents.push(newAgent);
    return newAgent as T;
  }

  if (pathname === '/v1/workflows' && method === 'GET') {
    return { workflows: listWithLimit(workflows, url.searchParams.get('limit')) } as T;
  }
  if (pathname === '/v1/workflows' && method === 'POST') {
    const spec = body as WorkflowSpec;
    const newWorkflow: WorkflowDefinition = {
      workflowId: makeId('wf'),
      name: spec?.name || `Workflow ${seq}`,
      version: 1,
      description: spec?.description,
      status: 'ACTIVE',
      definition: spec,
      createdAt: iso(0)
    };
    workflows.unshift(newWorkflow);
    return newWorkflow as T;
  }

  if (pathname === '/v1/tasks' && method === 'GET') {
    return { tasks: listWithLimit(tasks, url.searchParams.get('limit')) } as T;
  }
  if (pathname === '/v1/tasks' && method === 'POST') {
    const workflow = workflows.find((w) => w.workflowId === body?.workflow_id) || workflows[0];
    const newTask: Task = {
      taskId: makeId('task'),
      workflowId: workflow.workflowId,
      workflowVersion: workflow.version,
      title: body?.title || `Task ${seq}`,
      status: 'CREATED',
      context: body?.context || {},
      traceId: makeId('trace'),
      createdAt: iso(0),
      updatedAt: iso(0)
    };
    tasks.unshift(newTask);
    stepsByTaskId[newTask.taskId] = buildSteps(newTask, workflow.definition as WorkflowSpec);
    return newTask as T;
  }

  const taskStepsMatch = pathname.match(/^\/v1\/tasks\/([^/]+)\/steps$/);
  if (taskStepsMatch && method === 'GET') {
    const taskId = taskStepsMatch[1];
    return { steps: stepsByTaskId[taskId] || [] } as T;
  }

  const taskEvidenceMatch = pathname.match(/^\/v1\/tasks\/([^/]+)\/evidence$/);
  if (taskEvidenceMatch && method === 'GET') {
    const taskId = taskEvidenceMatch[1];
    return buildTaskEvidence(taskId) as T;
  }

  const taskStartMatch = pathname.match(/^\/v1\/tasks\/([^/]+)\/start$/);
  if (taskStartMatch && method === 'POST') {
    const taskId = taskStartMatch[1];
    const approval = createApprovalForTask(taskId);
    updateTaskStatus(taskId, 'PENDING_APPROVAL');
    return { status: 'PENDING_APPROVAL', approval } as T;
  }

  const taskCancelMatch = pathname.match(/^\/v1\/tasks\/([^/]+)\/cancel$/);
  if (taskCancelMatch && method === 'POST') {
    updateTaskStatus(taskCancelMatch[1], 'CANCELLED');
    return { status: 'CANCELLED' } as T;
  }

  const stepActionMatch = pathname.match(/^\/v1\/tasks\/([^/]+)\/steps\/([^/]+)\/(ack|resolve)$/);
  if (stepActionMatch && method === 'POST') {
    const [, taskId, stepId, action] = stepActionMatch;
    updateStepStatus(taskId, stepId, action === 'ack' ? 'ACKED' : 'RESOLVED');
    return { status: action === 'ack' ? 'ACKED' : 'RESOLVED' } as T;
  }

  if (pathname === '/v1/approvals' && method === 'GET') {
    let list = [...approvals];
    const status = url.searchParams.get('status');
    const entityType = url.searchParams.get('entity_type');
    const entityId = url.searchParams.get('entity_id');
    const operation = url.searchParams.get('operation');
    const requestedBy = url.searchParams.get('requested_by');
    if (status) list = list.filter((a) => a.status === status);
    if (entityType) list = list.filter((a) => a.entityType === entityType);
    if (entityId) list = list.filter((a) => a.entityId?.includes(entityId));
    if (operation) list = list.filter((a) => a.operation?.includes(operation));
    if (requestedBy) list = list.filter((a) => a.requestedBy?.includes(requestedBy));
    return { approvals: listWithLimit(list, url.searchParams.get('limit')) } as T;
  }

  const decideMatch = pathname.match(/^\/v1\/approvals\/([^/]+)\/decide$/);
  if (decideMatch && method === 'POST') {
    const approval = approvals.find((a) => a.approvalId === decideMatch[1]);
    if (approval) {
      if (body?.decision === 'APPROVE') {
        approval.status = 'APPROVED';
        approval.approvalsCount = (approval.approvalsCount || 0) + 1;
        if (approval.entityType === 'TASK') {
          updateTaskStatus(approval.entityId, 'RUNNING');
        }
      } else {
        approval.status = 'REJECTED';
        approval.rejectionsCount = (approval.rejectionsCount || 0) + 1;
        if (approval.entityType === 'TASK') {
          updateTaskStatus(approval.entityId, 'FAILED');
        }
      }
    }
    return { ok: true } as T;
  }

  if (pathname === '/v1/executors' && method === 'GET') {
    return { executors: listWithLimit(executors, url.searchParams.get('limit')) } as T;
  }
  if (pathname === '/v1/executors' && method === 'POST') {
    const payload = body || {};
    const newExecutor: Executor = {
      executorId: payload.executor_id || makeId('exec'),
      executorType: payload.executor_type || 'HUMAN',
      displayName: payload.display_name || `Executor ${seq}`,
      capabilityTags: payload.capability_tags || [],
      status: 'ACTIVE',
      metadata: payload.metadata || {},
      createdAt: iso(0)
    };
    executors.unshift(newExecutor);
    return newExecutor as T;
  }
  const execActivateMatch = pathname.match(/^\/v1\/executors\/([^/]+)\/(activate|deactivate)$/);
  if (execActivateMatch && method === 'POST') {
    const [, execId, action] = execActivateMatch;
    const exec = executors.find((e) => e.executorId === execId);
    if (exec) {
      exec.status = action === 'activate' ? 'ACTIVE' : 'INACTIVE';
    }
    return { ok: true } as T;
  }

  if (pathname === '/v1/notifications' && method === 'GET') {
    return { notifications: listWithLimit(notifications, url.searchParams.get('limit')) } as T;
  }
  const notifSendMatch = pathname.match(/^\/v1\/notifications\/([^/]+)\/send$/);
  if (notifSendMatch && method === 'POST') {
    const note = notifications.find((n) => n.notificationId === notifSendMatch[1]);
    if (note) {
      note.status = 'SENT';
      note.sentAt = iso(0);
    }
    return { ok: true } as T;
  }

  if (pathname === '/v1/admin/audit' && method === 'GET') {
    return buildAuditResult() as T;
  }

  const actionEvidenceMatch = pathname.match(/^\/v1\/actions\/([^/]+)\/evidence$/);
  if (actionEvidenceMatch && method === 'GET') {
    return buildActionEvidence(actionEvidenceMatch[1]) as T;
  }
  const actionTransitionMatch = pathname.match(/^\/v1\/actions\/([^/]+)\/transitions$/);
  if (actionTransitionMatch && method === 'GET') {
    return { transitions: buildActionTransitions(actionTransitionMatch[1]) } as T;
  }
  const actionDecisionMatch = pathname.match(/^\/v1\/actions\/([^/]+)\/(ack|resolve)$/);
  if (actionDecisionMatch && method === 'POST') {
    return { ok: true } as T;
  }

  if (pathname === '/v1/trust/events' && method === 'POST') {
    const req = body as TrustEventIngestRequest;
    return {
      event_id: makeId('evt'),
      status: 'INGESTED',
      source_id: req?.source_id,
      event_type: req?.event_type,
      received_at: iso(0)
    } as T;
  }

  const trustEvidenceMatch = pathname.match(/^\/v1\/trust\/evidence\/([^/]+)\/([^/]+)$/);
  if (trustEvidenceMatch && method === 'GET') {
    return buildEvidenceBundle(trustEvidenceMatch[1], trustEvidenceMatch[2]) as T;
  }

  return {} as T;
}
