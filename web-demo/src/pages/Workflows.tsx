import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { WorkflowDefinition, WorkflowSpec } from '../api/types';
import DagView from '../components/DagView';
import ErrorBanner from '../components/ErrorBanner';

function parseSpec(definition: unknown): WorkflowSpec | null {
  if (!definition) return null;
  if (typeof definition === 'object') return definition as WorkflowSpec;
  try {
    return JSON.parse(String(definition));
  } catch {
    return null;
  }
}

const defaultWorkflowSpec = `{
  "name": "Sample Workflow",
  "description": "Data-dense flow",
  "steps": [
    {
      "step_key": "review",
      "name": "Review",
      "executor_type": "HUMAN",
      "executor_ref": "reviewer",
      "action_type": "NOTIFY",
      "action_config": { "channel": "sse", "template": "review" }
    }
  ],
  "edges": []
}`;

type ApprovalForm = {
  enabled: boolean;
  applies_to: string;
  roles: string;
  min_approvals: string;
};

const approvalDefaults: Record<string, ApprovalForm> = {
  task_start: { enabled: false, applies_to: 'BOTH', roles: 'ADMIN', min_approvals: '1' },
  task_cancel: { enabled: false, applies_to: 'BOTH', roles: 'ADMIN', min_approvals: '1' },
  step_ack: { enabled: false, applies_to: 'HUMAN', roles: 'OPERATOR', min_approvals: '1' },
  step_resolve: { enabled: false, applies_to: 'HUMAN', roles: 'OPERATOR', min_approvals: '1' },
  action_ack: { enabled: false, applies_to: 'AGENT', roles: 'OPERATOR', min_approvals: '1' },
  action_resolve: { enabled: false, applies_to: 'AGENT', roles: 'OPERATOR', min_approvals: '1' }
};

export default function WorkflowsPage() {
  const queryClient = useQueryClient();
  const { data } = useQuery({
    queryKey: ['workflows'],
    queryFn: () => api.get<{ workflows: WorkflowDefinition[] }>('/v1/workflows?limit=50')
  });
  const workflows = data?.workflows || [];
  const [selected, setSelected] = useState<WorkflowDefinition | null>(null);
  const [specText, setSpecText] = useState(defaultWorkflowSpec);
  const [approvalForm, setApprovalForm] = useState<Record<string, ApprovalForm>>(approvalDefaults);
  const [error, setError] = useState<string | undefined>();

  useEffect(() => {
    if (!selected && workflows.length) {
      setSelected(workflows[0]);
    }
  }, [workflows, selected]);

  const spec = useMemo(() => parseSpec(selected?.definition), [selected]);
  const steps = (spec?.steps || []).map((s) => ({ id: s.step_key, name: s.name }));
  const edges = (spec?.edges || []).map((e) => ({ from: e.from, to: e.to }));

  const createWorkflow = useMutation({
    mutationFn: async () => {
      setError(undefined);
      const parsed = JSON.parse(specText);
      const approvals: Record<string, unknown> = {};
      Object.entries(approvalForm).forEach(([key, form]) => {
        if (!form.enabled) {
          return;
        }
        approvals[key] = {
          enabled: true,
          applies_to: form.applies_to,
          roles: form.roles.split(',').map((r) => r.trim()).filter(Boolean),
          min_approvals: Number(form.min_approvals || 1)
        };
      });
      const payload = { ...parsed, approvals: Object.keys(approvals).length ? approvals : undefined };
      return api.post('/v1/workflows', payload);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workflows'] });
    },
    onError: (err: any) => setError(err.message)
  });

  return (
    <div className="grid" style={{ gap: 16 }}>
      <div className="split">
        <div className="panel">
          <div style={{ fontWeight: 600, marginBottom: 8 }}>Workflows</div>
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Version</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {workflows.map((wf) => (
                <tr key={wf.workflowId} onClick={() => setSelected(wf)} style={{ cursor: 'pointer' }}>
                  <td>{wf.name}</td>
                  <td>{wf.version}</td>
                  <td><span className="badge">{wf.status}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="panel">
          <div style={{ fontWeight: 600, marginBottom: 12 }}>Workflow DAG</div>
          {spec ? (
            <DagView steps={steps} edges={edges} />
          ) : (
            <div style={{ color: 'var(--muted)' }}>Select a workflow to view definition.</div>
          )}
          {spec?.approvals && (
            <div style={{ marginTop: 16 }}>
              <div style={{ fontWeight: 600, marginBottom: 6 }}>Approvals</div>
              <pre className="code-block">{JSON.stringify(spec.approvals, null, 2)}</pre>
            </div>
          )}
        </div>
      </div>
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Create Workflow</div>
        <ErrorBanner message={error} />
        <div className="grid cols-2" style={{ marginTop: 8 }}>
          {Object.entries(approvalForm).map(([key, form]) => (
            <div key={key} className="panel compact">
              <div className="toolbar" style={{ justifyContent: 'space-between' }}>
                <div className="mono">{key}</div>
                <label className="mono">
                  <input
                    type="checkbox"
                    checked={form.enabled}
                    onChange={(e) => setApprovalForm({ ...approvalForm, [key]: { ...form, enabled: e.target.checked } })}
                  /> Enable
                </label>
              </div>
              <div style={{ display: 'grid', gap: 6, marginTop: 6 }}>
                <select className="select" value={form.applies_to} onChange={(e) => setApprovalForm({ ...approvalForm, [key]: { ...form, applies_to: e.target.value } })}>
                  <option value="HUMAN">HUMAN</option>
                  <option value="AGENT">AGENT</option>
                  <option value="BOTH">BOTH</option>
                </select>
                <input className="input" placeholder="Roles (comma separated)" value={form.roles} onChange={(e) => setApprovalForm({ ...approvalForm, [key]: { ...form, roles: e.target.value } })} />
                <input className="input" placeholder="Min approvals" value={form.min_approvals} onChange={(e) => setApprovalForm({ ...approvalForm, [key]: { ...form, min_approvals: e.target.value } })} />
              </div>
            </div>
          ))}
        </div>
        <div style={{ marginTop: 12 }}>
          <textarea className="textarea" value={specText} onChange={(e) => setSpecText(e.target.value)} />
          <div className="toolbar" style={{ marginTop: 8 }}>
            <button className="button" onClick={() => createWorkflow.mutate()}>Create Workflow</button>
            <span className="muted">Workflow definition JSON. Approvals will be merged.</span>
          </div>
        </div>
      </div>
    </div>
  );
}
