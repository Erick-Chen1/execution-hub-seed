import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { Step, Task, TaskEvidence, WorkflowDefinition } from '../api/types';
import DagView from '../components/DagView';
import DataTable from '../components/DataTable';
import type { ColumnDef } from '@tanstack/react-table';
import ErrorBanner from '../components/ErrorBanner';

export default function TasksPage() {
  const queryClient = useQueryClient();
  const { data } = useQuery({
    queryKey: ['tasks'],
    queryFn: () => api.get<{ tasks: Task[] }>('/v1/tasks?limit=50')
  });
  const workflowsQuery = useQuery({
    queryKey: ['workflows'],
    queryFn: () => api.get<{ workflows: WorkflowDefinition[] }>('/v1/workflows?limit=100')
  });
  const tasks = data?.tasks || [];
  const workflows = workflowsQuery.data?.workflows || [];
  const [selected, setSelected] = useState<Task | null>(null);
  const [taskError, setTaskError] = useState<string | undefined>();
  const [createForm, setCreateForm] = useState({
    title: '',
    workflow_id: '',
    context: '{\n  "priority": "normal"\n}'
  });
  const [evidence, setEvidence] = useState<TaskEvidence | null>(null);

  const stepsQuery = useQuery({
    queryKey: ['steps', selected?.taskId],
    queryFn: () => api.get<{ steps: Step[] }>(`/v1/tasks/${selected?.taskId}/steps`),
    enabled: !!selected?.taskId
  });

  const startTask = useMutation({
    mutationFn: async () => {
      if (!selected) return null;
      setTaskError(undefined);
      return api.post(`/v1/tasks/${selected.taskId}/start`);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['tasks'] })
  });

  const cancelTask = useMutation({
    mutationFn: async () => {
      if (!selected) return null;
      setTaskError(undefined);
      return api.post(`/v1/tasks/${selected.taskId}/cancel`);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['tasks'] })
  });

  const ackStep = useMutation({
    mutationFn: async (stepId: string) => {
      setTaskError(undefined);
      return api.post(`/v1/tasks/${selected?.taskId}/steps/${stepId}/ack`, { comment: 'Acked in UI' });
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['steps', selected?.taskId] })
  });

  const resolveStep = useMutation({
    mutationFn: async (stepId: string) => {
      setTaskError(undefined);
      return api.post(`/v1/tasks/${selected?.taskId}/steps/${stepId}/resolve`, { evidence: { ok: true } });
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['steps', selected?.taskId] })
  });

  const createTask = useMutation({
    mutationFn: async () => {
      setTaskError(undefined);
      let context: Record<string, unknown> | undefined;
      if (createForm.context) {
        context = JSON.parse(createForm.context);
      }
      return api.post('/v1/tasks', {
        title: createForm.title,
        workflow_id: createForm.workflow_id,
        context
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tasks'] });
      setCreateForm({ title: '', workflow_id: '', context: '{\n  "priority": "normal"\n}' });
    },
    onError: (err: any) => setTaskError(err.message)
  });

  const fetchEvidence = useMutation({
    mutationFn: async () => {
      if (!selected) return null;
      const res = await api.get<TaskEvidence>(`/v1/tasks/${selected.taskId}/evidence`);
      return res;
    },
    onSuccess: (res) => {
      if (res) {
        setEvidence(res);
      }
    },
    onError: (err: any) => setTaskError(err.message)
  });

  const steps = stepsQuery.data?.steps || [];
  const dagSteps = steps.map((s) => ({ id: s.stepId, name: s.name, status: s.status }));
  const dagEdges = steps.flatMap((s) => (s.dependsOn || []).map((d) => ({ from: d, to: s.stepId })));

  const approvalHint = useMemo(() => {
    const last = (startTask.data as any) || (cancelTask.data as any) || (ackStep.data as any) || (resolveStep.data as any);
    if (last && last.status === 'PENDING_APPROVAL') {
      return `Approval pending: ${last.approval?.approvalId}`;
    }
    return null;
  }, [startTask.data, cancelTask.data, ackStep.data, resolveStep.data]);

  useEffect(() => {
    if (!selected && tasks.length) {
      setSelected(tasks[0]);
    }
  }, [tasks, selected]);

  useEffect(() => {
    if (!createForm.workflow_id && workflows.length) {
      setCreateForm((prev) => ({ ...prev, workflow_id: workflows[0].workflowId }));
    }
  }, [workflows, createForm.workflow_id]);

  const taskColumns: ColumnDef<Task>[] = [
    { header: 'Title', accessorKey: 'title' },
    {
      header: 'Status',
      cell: (info) => <span className="badge">{info.row.original.status}</span>
    },
    {
      header: 'Workflow',
      cell: (info) => <span className="mono">{info.row.original.workflowId.slice(0, 8)}</span>
    }
  ];

  return (
    <div className="split">
      <div className="panel">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div style={{ fontWeight: 600 }}>Tasks</div>
        </div>
        <ErrorBanner message={taskError} />
        {approvalHint && <div style={{ marginTop: 8, color: 'var(--warning)' }}>{approvalHint}</div>}
        <div style={{ marginTop: 10 }}>
          <DataTable data={tasks} columns={taskColumns} onRowClick={setSelected} />
        </div>
        <div style={{ display: 'flex', gap: 10, marginTop: 12 }}>
          <button className="button" onClick={() => startTask.mutate()} disabled={!selected}>Start</button>
          <button className="button secondary" onClick={() => cancelTask.mutate()} disabled={!selected}>Cancel</button>
          <button className="button secondary" onClick={() => fetchEvidence.mutate()} disabled={!selected}>Evidence</button>
        </div>
        <div style={{ marginTop: 16 }}>
          <div style={{ fontWeight: 600, marginBottom: 8 }}>Create Task</div>
          <div style={{ display: 'grid', gap: 8 }}>
            <input className="input" placeholder="Title" value={createForm.title} onChange={(e) => setCreateForm({ ...createForm, title: e.target.value })} />
            <select className="select" value={createForm.workflow_id} onChange={(e) => setCreateForm({ ...createForm, workflow_id: e.target.value })}>
              {workflows.map((wf) => (
                <option key={wf.workflowId} value={wf.workflowId}>{wf.name} v{wf.version}</option>
              ))}
            </select>
            <textarea className="textarea" value={createForm.context} onChange={(e) => setCreateForm({ ...createForm, context: e.target.value })} />
            <button className="button" onClick={() => createTask.mutate()} disabled={!createForm.title || !createForm.workflow_id}>Create</button>
          </div>
        </div>
      </div>
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Steps</div>
        <DagView steps={dagSteps} edges={dagEdges} />
        <table className="data-table" style={{ marginTop: 12 }}>
          <thead>
            <tr>
              <th>Step</th>
              <th>Status</th>
              <th>Executor</th>
              <th>Action</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {steps.map((s) => (
              <tr key={s.stepId}>
                <td>{s.name}</td>
                <td><span className="badge">{s.status}</span></td>
                <td className="mono">{s.executorRef}</td>
                <td className="mono">{s.actionId?.slice(0, 8)}</td>
                <td>
                  <div style={{ display: 'flex', gap: 6 }}>
                    <button className="button secondary" onClick={() => ackStep.mutate(s.stepId)}>Ack</button>
                    <button className="button" onClick={() => resolveStep.mutate(s.stepId)}>Resolve</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {evidence && (
          <div style={{ marginTop: 12 }}>
            <div className="mono" style={{ marginBottom: 6 }}>Task Evidence</div>
            <pre className="code-block">{JSON.stringify(evidence, null, 2)}</pre>
          </div>
        )}
      </div>
    </div>
  );
}
