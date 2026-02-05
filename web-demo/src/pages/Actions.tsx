import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { api } from '../api/client';
import type { ActionEvidence, ActionTransition } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';

export default function ActionsPage() {
  const [actionId, setActionId] = useState('');
  const [evidence, setEvidence] = useState<ActionEvidence | null>(null);
  const [transitions, setTransitions] = useState<ActionTransition[]>([]);
  const [error, setError] = useState<string | undefined>();

  const fetchAll = useMutation({
    mutationFn: async () => {
      setError(undefined);
      const [ev, tr] = await Promise.all([
        api.get<ActionEvidence>(`/v1/actions/${actionId}/evidence`),
        api.get<{ transitions: ActionTransition[] }>(`/v1/actions/${actionId}/transitions`)
      ]);
      return { ev, tr };
    },
    onSuccess: (res) => {
      setEvidence(res.ev);
      setTransitions(res.tr.transitions || []);
    },
    onError: (err: any) => setError(err.message)
  });

  const ackAction = useMutation({
    mutationFn: async () => api.post(`/v1/actions/${actionId}/ack`),
    onError: (err: any) => setError(err.message)
  });

  const resolveAction = useMutation({
    mutationFn: async () => api.post(`/v1/actions/${actionId}/resolve`),
    onError: (err: any) => setError(err.message)
  });

  const canQuery = actionId.length > 0;

  return (
    <div className="split">
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Action Lookup</div>
        <ErrorBanner message={error} />
        <div style={{ display: 'grid', gap: 8, marginTop: 8 }}>
          <input className="input" placeholder="Action ID" value={actionId} onChange={(e) => setActionId(e.target.value)} />
          <div className="toolbar">
            <button className="button" onClick={() => fetchAll.mutate()} disabled={!canQuery}>Fetch Evidence</button>
            <button className="button secondary" onClick={() => ackAction.mutate()} disabled={!canQuery}>Ack</button>
            <button className="button secondary" onClick={() => resolveAction.mutate()} disabled={!canQuery}>Resolve</button>
          </div>
        </div>
        <div style={{ marginTop: 14 }}>
          <div className="mono" style={{ marginBottom: 6 }}>Transitions</div>
          <table className="data-table">
            <thead>
              <tr>
                <th>From</th>
                <th>To</th>
                <th>At</th>
                <th>Reason</th>
              </tr>
            </thead>
            <tbody>
              {transitions.map((t, idx) => (
                <tr key={`${t.actionId}-${idx}`}>
                  <td>{t.fromStatus}</td>
                  <td>{t.toStatus}</td>
                  <td className="mono">{t.transitionedAt?.slice(0, 19)}</td>
                  <td className="mono">{t.reason || ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Action Evidence</div>
        {evidence ? (
          <pre className="code-block">{JSON.stringify(evidence, null, 2)}</pre>
        ) : (
          <div className="muted">Fetch an action to view evidence.</div>
        )}
      </div>
    </div>
  );
}
