import { useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { api } from '../api/client';
import type { EvidenceBundle, TrustEventIngestRequest } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';

const defaultPayload = `{
  "sample": true,
  "signal": "temp"
}`;

export default function TrustPage() {
  const [eventForm, setEventForm] = useState<TrustEventIngestRequest>({
    source_type: 'sensor',
    source_id: '',
    event_type: '',
    payload: {},
    schema_version: 'v1'
  });
  const [payloadText, setPayloadText] = useState(defaultPayload);
  const [ingestResult, setIngestResult] = useState<Record<string, unknown> | null>(null);
  const [ingestError, setIngestError] = useState<string | undefined>();

  const [evidenceType, setEvidenceType] = useState('EVENT');
  const [evidenceSubject, setEvidenceSubject] = useState('');
  const [evidence, setEvidence] = useState<EvidenceBundle | null>(null);
  const [evidenceError, setEvidenceError] = useState<string | undefined>();

  const ingestMutation = useMutation({
    mutationFn: async () => {
      setIngestError(undefined);
      const payload = JSON.parse(payloadText);
      const req = { ...eventForm, payload };
      return api.post('/v1/trust/events', req);
    },
    onSuccess: (res) => {
      setIngestResult(res as Record<string, unknown>);
    },
    onError: (err: any) => setIngestError(err.message)
  });

  const evidenceMutation = useMutation({
    mutationFn: async () => {
      setEvidenceError(undefined);
      const res = await api.get<EvidenceBundle>(`/v1/trust/evidence/${evidenceType}/${evidenceSubject}`);
      return res;
    },
    onSuccess: (res) => setEvidence(res),
    onError: (err: any) => setEvidenceError(err.message)
  });

  const canIngest = useMemo(() => eventForm.source_id && eventForm.event_type, [eventForm]);
  const canQuery = useMemo(() => evidenceSubject, [evidenceSubject]);

  return (
    <div className="grid cols-2">
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Ingest Trust Event</div>
        <ErrorBanner message={ingestError} />
        <div style={{ display: 'grid', gap: 8, marginTop: 8 }}>
          <input className="input" placeholder="Source Type" value={eventForm.source_type} onChange={(e) => setEventForm({ ...eventForm, source_type: e.target.value })} />
          <input className="input" placeholder="Source ID" value={eventForm.source_id} onChange={(e) => setEventForm({ ...eventForm, source_id: e.target.value })} />
          <input className="input" placeholder="Event Type" value={eventForm.event_type} onChange={(e) => setEventForm({ ...eventForm, event_type: e.target.value })} />
          <input className="input" placeholder="Schema Version" value={eventForm.schema_version} onChange={(e) => setEventForm({ ...eventForm, schema_version: e.target.value })} />
          <textarea className="textarea" value={payloadText} onChange={(e) => setPayloadText(e.target.value)} />
          <button className="button" onClick={() => ingestMutation.mutate()} disabled={!canIngest}>Ingest</button>
          {ingestResult && (
            <pre className="code-block">{JSON.stringify(ingestResult, null, 2)}</pre>
          )}
        </div>
      </div>
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Evidence Bundle</div>
        <ErrorBanner message={evidenceError} />
        <div style={{ display: 'grid', gap: 8, marginTop: 8 }}>
          <select className="select" value={evidenceType} onChange={(e) => setEvidenceType(e.target.value)}>
            <option value="EVENT">EVENT</option>
            <option value="ACTION">ACTION</option>
            <option value="BATCH">BATCH</option>
            <option value="TASK">TASK</option>
          </select>
          <input className="input" placeholder="Subject ID" value={evidenceSubject} onChange={(e) => setEvidenceSubject(e.target.value)} />
          <button className="button" onClick={() => evidenceMutation.mutate()} disabled={!canQuery}>Fetch Evidence</button>
          {evidence && (
            <pre className="code-block">{JSON.stringify(evidence, null, 2)}</pre>
          )}
        </div>
      </div>
    </div>
  );
}
