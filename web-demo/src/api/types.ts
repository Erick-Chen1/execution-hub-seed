export type Role = 'ADMIN' | 'OPERATOR' | 'VIEWER';
export type UserType = 'HUMAN' | 'AGENT';

export interface User {
  userId: string;
  username: string;
  role: Role;
  type: UserType;
  ownerUserId?: string;
  status: 'ACTIVE' | 'DISABLED';
  createdAt?: string;
  updatedAt?: string;
}

export interface WorkflowDefinition {
  workflowId: string;
  name: string;
  version: number;
  description?: string;
  status: string;
  definition: WorkflowSpec;
  createdAt?: string;
  updatedAt?: string;
}

export interface WorkflowSpec {
  workflow_id?: string;
  version?: number;
  name: string;
  description?: string;
  steps: WorkflowStep[];
  edges?: WorkflowEdge[];
  conditions?: WorkflowCondition[];
  approvals?: Record<string, unknown>;
}

export interface WorkflowStep {
  step_key: string;
  name: string;
  executor_type: string;
  executor_ref: string;
  action_type: string;
  action_config: Record<string, unknown>;
  timeout_seconds?: number;
  max_retries?: number;
  depends_on?: string[];
  on_fail?: Record<string, unknown>;
}

export interface WorkflowEdge {
  from: string;
  to: string;
  condition?: string;
}

export interface WorkflowCondition {
  name: string;
  expression: string;
}

export interface Task {
  taskId: string;
  workflowId: string;
  workflowVersion: number;
  title: string;
  status: string;
  context?: Record<string, unknown>;
  traceId: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface Step {
  stepId: string;
  taskId: string;
  stepKey: string;
  name: string;
  status: string;
  executorType: string;
  executorRef: string;
  actionType: string;
  actionConfig?: Record<string, unknown>;
  dependsOn?: string[];
  actionId?: string;
  evidence?: Record<string, unknown>;
  traceId?: string;
  retryCount?: number;
  maxRetries?: number;
  timeoutSeconds?: number;
  createdAt?: string;
  dispatchedAt?: string;
  ackedAt?: string;
  resolvedAt?: string;
  failedAt?: string;
}

export interface Approval {
  approvalId: string;
  entityType: string;
  entityId: string;
  operation: string;
  status: string;
  requestedBy: string;
  requiredRoles?: string[];
  approvalsCount?: number;
  rejectionsCount?: number;
}

export interface Executor {
  executorId: string;
  executorType: string;
  displayName: string;
  capabilityTags?: string[];
  status: string;
  metadata?: Record<string, unknown>;
  createdAt?: string;
  updatedAt?: string;
}

export interface Notification {
  notificationId: string;
  actionId?: string;
  dedupeKey?: string;
  channel?: string;
  priority?: string;
  title?: string;
  body?: string;
  payload?: Record<string, unknown>;
  status?: string;
  targetUserId?: string;
  targetGroup?: string;
  retryCount?: number;
  maxRetries?: number;
  lastError?: string;
  expiresAt?: string;
  createdAt?: string;
  sentAt?: string;
  deliveredAt?: string;
  failedAt?: string;
  traceId?: string;
}

export interface ActionTransition {
  actionId: string;
  fromStatus: string;
  toStatus: string;
  transitionedAt: string;
  reason?: string;
}

export interface ActionEvidence {
  Action?: Record<string, unknown>;
  Evaluation?: Record<string, unknown>;
  Rule?: Record<string, unknown>;
}

export interface StepEvidence {
  step: Step;
  action_evidence?: ActionEvidence;
  transitions?: ActionTransition[];
}

export interface TaskEvidence {
  task_id: string;
  trace_id: string;
  steps: StepEvidence[];
}

export interface TrustEventIngestRequest {
  event_id?: string;
  client_record_id?: string;
  source_type: string;
  source_id: string;
  ts_device?: string;
  ts_gateway?: string;
  key?: string;
  event_type: string;
  payload: Record<string, unknown>;
  schema_version: string;
}

export interface EvidenceBundle {
  bundleId?: string;
  bundleType?: string;
  subjectId?: string;
  generatedAt?: string;
  events?: Record<string, unknown>[];
  hashChain?: Record<string, unknown>[];
  signatures?: Record<string, unknown>[];
  rules?: Record<string, unknown>[];
  actions?: Record<string, unknown>[];
  verification?: Record<string, unknown>;
  bundleHash?: string;
}

export interface AuditLog {
  auditId: string;
  entityType?: string;
  entityId?: string;
  action?: string;
  actor?: string;
  riskLevel?: string;
  traceId?: string;
  createdAt?: string;
}

export interface AuditQueryResult {
  logs: AuditLog[];
  pagination?: {
    cursor?: string;
    hasMore?: boolean;
    count?: number;
    total?: number;
  };
}
