import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { Executor } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';

const defaultConfig = `{
  "notes": "Add executor metadata here"
}`;

export default function ExecutorsPage() {
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState('');
  const [selected, setSelected] = useState<Executor | null>(null);
  const [createForm, setCreateForm] = useState({
    executor_id: '',
    executor_type: 'HUMAN',
    display_name: '',
    capability_tags: ''
  });
  const [metadataText, setMetadataText] = useState(defaultConfig);
  const [error, setError] = useState<string | undefined>();

  const { data } = useQuery({
    queryKey: ['executors'],
    queryFn: () => api.get<{ executors: Executor[] }>('/v1/executors?limit=200')
  });

  const executors = useMemo(() => {
    const list = data?.executors || [];
    if (!filter) return list;
    return list.filter((e) => e.executorId.includes(filter) || e.displayName?.toLowerCase().includes(filter.toLowerCase()));
  }, [data, filter]);

  useEffect(() => {
    if (!selected && executors.length) {
      setSelected(executors[0]);
    }
  }, [executors, selected]);

  const createExecutor = useMutation({
    mutationFn: async () => {
      setError(undefined);
      let metadata: Record<string, unknown> | undefined;
      if (metadataText.trim()) {
        metadata = JSON.parse(metadataText);
      }
      const payload = {
        executor_id: createForm.executor_id,
        executor_type: createForm.executor_type,
        display_name: createForm.display_name,
        capability_tags: createForm.capability_tags ? createForm.capability_tags.split(',').map((t) => t.trim()).filter(Boolean) : [],
        metadata
      };
      return api.post('/v1/executors', payload);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['executors'] });
      setCreateForm({ executor_id: '', executor_type: 'HUMAN', display_name: '', capability_tags: '' });
    },
    onError: (err: any) => setError(err.message)
  });

  const activateExecutor = useMutation({
    mutationFn: async (id: string) => api.post(`/v1/executors/${id}/activate`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['executors'] }),
    onError: (err: any) => setError(err.message)
  });

  const deactivateExecutor = useMutation({
    mutationFn: async (id: string) => api.post(`/v1/executors/${id}/deactivate`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['executors'] }),
    onError: (err: any) => setError(err.message)
  });

  const canCreate = createForm.executor_id && createForm.display_name;

  return (
    <div className="split">
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Executors</div>
        <div className="toolbar" style={{ marginBottom: 10 }}>
          <input className="input" placeholder="Filter by id or name" value={filter} onChange={(e) => setFilter(e.target.value)} />
        </div>
        <table className="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Name</th>
              <th>Type</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {executors.map((e) => (
              <tr key={e.executorId} onClick={() => setSelected(e)} style={{ cursor: 'pointer' }}>
                <td className="mono">{e.executorId}</td>
                <td>{e.displayName}</td>
                <td>{e.executorType}</td>
                <td><span className="badge">{e.status}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Details</div>
        {selected ? (
          <div style={{ display: 'grid', gap: 10 }}>
            <div className="mono">{selected.executorId}</div>
            <div>{selected.displayName}</div>
            <div className="tag">{selected.executorType}</div>
            <div><span className="badge">{selected.status}</span></div>
            <div className="mono">Tags: {(selected.capabilityTags || []).join(', ') || 'none'}</div>
            <div>
              <button className="button" onClick={() => activateExecutor.mutate(selected.executorId)}>Activate</button>
              <button className="button secondary" style={{ marginLeft: 8 }} onClick={() => deactivateExecutor.mutate(selected.executorId)}>Deactivate</button>
            </div>
            <div>
              <div className="mono" style={{ marginBottom: 6 }}>Metadata</div>
              <pre className="code-block">{JSON.stringify(selected.metadata || {}, null, 2)}</pre>
            </div>
          </div>
        ) : (
          <div className="muted">Select an executor to view details.</div>
        )}
        <div style={{ marginTop: 18 }}>
          <div style={{ fontWeight: 600, marginBottom: 8 }}>Create Executor</div>
          <ErrorBanner message={error} />
          <div style={{ display: 'grid', gap: 8, marginTop: 8 }}>
            <input className="input" placeholder="Executor ID" value={createForm.executor_id} onChange={(e) => setCreateForm({ ...createForm, executor_id: e.target.value })} />
            <input className="input" placeholder="Display Name" value={createForm.display_name} onChange={(e) => setCreateForm({ ...createForm, display_name: e.target.value })} />
            <select className="select" value={createForm.executor_type} onChange={(e) => setCreateForm({ ...createForm, executor_type: e.target.value })}>
              <option value="HUMAN">HUMAN</option>
              <option value="AGENT">AGENT</option>
            </select>
            <input className="input" placeholder="Capability tags (comma separated)" value={createForm.capability_tags} onChange={(e) => setCreateForm({ ...createForm, capability_tags: e.target.value })} />
            <textarea className="textarea" value={metadataText} onChange={(e) => setMetadataText(e.target.value)} />
            <button className="button" onClick={() => createExecutor.mutate()} disabled={!canCreate}>Create</button>
          </div>
        </div>
      </div>
    </div>
  );
}
