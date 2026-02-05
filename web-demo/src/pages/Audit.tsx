import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import type { AuditLog, AuditQueryResult } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';
import { useAuth } from '../components/AuthProvider';

function buildQuery(params: Record<string, string>) {
  const search = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v) {
      search.set(k, v);
    }
  });
  return search.toString();
}

export default function AuditPage() {
  const { user } = useAuth();
  const [filters, setFilters] = useState({
    entityType: '',
    entityId: '',
    action: '',
    actor: '',
    riskLevel: '',
    traceId: '',
    cursor: '',
    limit: '50'
  });
  const [selected, setSelected] = useState<AuditLog | null>(null);
  const queryString = buildQuery(filters);

  const { data, error, refetch, isFetching } = useQuery({
    queryKey: ['audit', queryString],
    queryFn: () => api.get<AuditQueryResult>(`/v1/admin/audit?${queryString}`),
    enabled: false
  });

  const logs = data?.logs || [];
  const pagination = data?.pagination;

  const canSearch = useMemo(() => true, []);

  if (user?.role !== 'ADMIN') {
    return (
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Audit</div>
        <div className="muted">Admin only.</div>
      </div>
    );
  }

  return (
    <div className="split">
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Audit Query</div>
        <div className="grid cols-2" style={{ marginBottom: 10 }}>
          <input className="input" placeholder="Entity Type" value={filters.entityType} onChange={(e) => setFilters({ ...filters, entityType: e.target.value })} />
          <input className="input" placeholder="Entity ID" value={filters.entityId} onChange={(e) => setFilters({ ...filters, entityId: e.target.value })} />
          <input className="input" placeholder="Action" value={filters.action} onChange={(e) => setFilters({ ...filters, action: e.target.value })} />
          <input className="input" placeholder="Actor" value={filters.actor} onChange={(e) => setFilters({ ...filters, actor: e.target.value })} />
          <input className="input" placeholder="Risk Level" value={filters.riskLevel} onChange={(e) => setFilters({ ...filters, riskLevel: e.target.value })} />
          <input className="input" placeholder="Trace ID" value={filters.traceId} onChange={(e) => setFilters({ ...filters, traceId: e.target.value })} />
          <input className="input" placeholder="Cursor" value={filters.cursor} onChange={(e) => setFilters({ ...filters, cursor: e.target.value })} />
          <input className="input" placeholder="Limit" value={filters.limit} onChange={(e) => setFilters({ ...filters, limit: e.target.value })} />
        </div>
        <div className="toolbar">
          <button className="button" onClick={() => refetch()} disabled={!canSearch || isFetching}>Run Query</button>
          {pagination?.hasMore && <span className="muted">Has more results</span>}
        </div>
        <div style={{ marginTop: 12 }}>
          <ErrorBanner message={(error as Error | undefined)?.message} />
        </div>
        <table className="data-table" style={{ marginTop: 12 }}>
          <thead>
            <tr>
              <th>ID</th>
              <th>Entity</th>
              <th>Action</th>
              <th>Actor</th>
              <th>Risk</th>
              <th>Time</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((log) => (
              <tr key={log.auditId} onClick={() => setSelected(log)} style={{ cursor: 'pointer' }}>
                <td className="mono">{log.auditId}</td>
                <td className="mono">{log.entityType}:{log.entityId?.slice(0, 8)}</td>
                <td>{log.action}</td>
                <td className="mono">{log.actor}</td>
                <td><span className="badge">{log.riskLevel}</span></td>
                <td className="mono">{log.createdAt?.slice(0, 19)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Audit Detail</div>
        {selected ? (
          <div style={{ display: 'grid', gap: 10 }}>
            <div className="mono">{selected.auditId}</div>
            <div className="mono">Entity: {selected.entityType} / {selected.entityId}</div>
            <div className="mono">Action: {selected.action}</div>
            <div className="mono">Actor: {selected.actor}</div>
            <div className="mono">Risk: {selected.riskLevel}</div>
            <div className="mono">Trace: {selected.traceId}</div>
            <div className="mono">Time: {selected.createdAt}</div>
          </div>
        ) : (
          <div className="muted">Run a query and select a record.</div>
        )}
      </div>
    </div>
  );
}
