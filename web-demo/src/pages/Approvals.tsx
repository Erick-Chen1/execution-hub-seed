import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { Approval } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';

function buildQuery(params: Record<string, string>) {
  const search = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v) {
      search.set(k, v);
    }
  });
  return search.toString();
}

export default function ApprovalsPage() {
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState<Approval | null>(null);
  const [filters, setFilters] = useState({
    status: '',
    entity_type: '',
    entity_id: '',
    operation: '',
    requested_by: '',
    limit: '100'
  });
  const queryString = buildQuery(filters);
  const { data, error } = useQuery({
    queryKey: ['approvals', queryString],
    queryFn: () => api.get<{ approvals: Approval[] }>(`/v1/approvals?${queryString}`)
  });
  const approvals = data?.approvals || [];

  const decide = useMutation({
    mutationFn: ({ id, decision }: { id: string; decision: 'APPROVE' | 'REJECT' }) =>
      api.post(`/v1/approvals/${id}/decide`, { decision }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['approvals'] })
  });

  const filtered = useMemo(() => approvals, [approvals]);

  return (
    <div className="split">
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Approvals</div>
        <div className="grid cols-2" style={{ marginBottom: 10 }}>
          <input className="input" placeholder="Status" value={filters.status} onChange={(e) => setFilters({ ...filters, status: e.target.value })} />
          <input className="input" placeholder="Entity Type" value={filters.entity_type} onChange={(e) => setFilters({ ...filters, entity_type: e.target.value })} />
          <input className="input" placeholder="Entity ID" value={filters.entity_id} onChange={(e) => setFilters({ ...filters, entity_id: e.target.value })} />
          <input className="input" placeholder="Operation" value={filters.operation} onChange={(e) => setFilters({ ...filters, operation: e.target.value })} />
          <input className="input" placeholder="Requested By" value={filters.requested_by} onChange={(e) => setFilters({ ...filters, requested_by: e.target.value })} />
          <input className="input" placeholder="Limit" value={filters.limit} onChange={(e) => setFilters({ ...filters, limit: e.target.value })} />
        </div>
        <ErrorBanner message={(error as Error | undefined)?.message} />
        <table className="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Entity</th>
              <th>Operation</th>
              <th>Status</th>
              <th>Approvals</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((a) => (
              <tr key={a.approvalId} onClick={() => setSelected(a)} style={{ cursor: 'pointer' }}>
                <td className="mono">{a.approvalId}</td>
                <td className="mono">{a.entityType}:{a.entityId?.slice(0, 8)}</td>
                <td>{a.operation}</td>
                <td><span className="badge">{a.status}</span></td>
                <td>{a.approvalsCount}/{a.rejectionsCount}</td>
                <td>
                  <button
                    className="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      decide.mutate({ id: a.approvalId, decision: 'APPROVE' });
                    }}
                    disabled={a.status !== 'PENDING'}
                  >
                    Approve
                  </button>
                  <button
                    className="button secondary"
                    style={{ marginLeft: 8 }}
                    onClick={(e) => {
                      e.stopPropagation();
                      decide.mutate({ id: a.approvalId, decision: 'REJECT' });
                    }}
                    disabled={a.status !== 'PENDING'}
                  >
                    Reject
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Detail</div>
        {selected ? (
          <pre className="code-block">{JSON.stringify(selected, null, 2)}</pre>
        ) : (
          <div className="muted">Select an approval for details.</div>
        )}
      </div>
    </div>
  );
}
