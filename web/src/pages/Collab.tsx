import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type {
  CollabArtifact,
  CollabArtifactsResponse,
  CollabClaim,
  CollabDecision,
  CollabEvent,
  CollabEventsResponse,
  CollabOpenStepsResponse,
  CollabParticipant,
  CollabParticipantsResponse,
  CollabSSEMessage,
  CollabSession,
  CollabStep,
  WorkflowDefinition
} from '../api/types';
import ErrorBanner from '../components/ErrorBanner';
import { useAuth } from '../components/AuthProvider';

type JoinType = 'HUMAN' | 'AGENT';
type VoteChoice = 'APPROVE' | 'REJECT';
type StreamStatus = 'idle' | 'connecting' | 'live' | 'error';

function toErrorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message;
  }
  return 'Request failed';
}

function shortID(value?: string) {
  if (!value) {
    return 'n/a';
  }
  return value.length > 12 ? `${value.slice(0, 8)}...${value.slice(-4)}` : value;
}

function formatTime(value?: string) {
  if (!value) {
    return 'n/a';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function badgeClass(status: string) {
  const upper = status.toUpperCase();
  if (upper.includes('FAILED') || upper.includes('REJECTED') || upper.includes('ERROR')) {
    return 'badge danger';
  }
  if (upper.includes('PENDING') || upper.includes('REVIEW')) {
    return 'badge warning';
  }
  return 'badge';
}

export default function CollabPage() {
  const queryClient = useQueryClient();
  const { actor, user } = useAuth();
  const [error, setError] = useState<string | undefined>();

  const [sessionForm, setSessionForm] = useState({
    workflowId: '',
    title: 'coauthor_review_publish',
    context: '{\n  "topic": "Execution Hub V2"\n}'
  });
  const [sessionInput, setSessionInput] = useState('');
  const [activeSessionId, setActiveSessionId] = useState('');

  const [joinForm, setJoinForm] = useState({
    type: 'HUMAN' as JoinType,
    ref: '',
    capabilities: 'draft,review',
    trustScore: '80'
  });
  const [participant, setParticipant] = useState<CollabParticipant | null>(null);
  const [leaseSeconds, setLeaseSeconds] = useState('900');
  const [onlyMyOperable, setOnlyMyOperable] = useState(true);
  const [selectedStepIds, setSelectedStepIds] = useState<string[]>([]);
  const [batchMessage, setBatchMessage] = useState('');

  const [selectedStepId, setSelectedStepId] = useState('');
  const [handoffToParticipantId, setHandoffToParticipantId] = useState('');
  const [handoffComment, setHandoffComment] = useState('');

  const [artifactKind, setArtifactKind] = useState('draft');
  const [artifactContent, setArtifactContent] = useState('{\n  "summary": "Ready for review"\n}');

  const [decisionPolicy, setDecisionPolicy] = useState('{\n  "min_approvals": 1\n}');
  const [decisionDeadline, setDecisionDeadline] = useState('');
  const [decisionId, setDecisionId] = useState('');
  const [voteChoice, setVoteChoice] = useState<VoteChoice>('APPROVE');
  const [voteComment, setVoteComment] = useState('');

  const [resolveWithClaimOwner, setResolveWithClaimOwner] = useState(true);

  const [streamStatus, setStreamStatus] = useState<StreamStatus>('idle');
  const [streamEvents, setStreamEvents] = useState<CollabEvent[]>([]);

  const workflowsQuery = useQuery({
    queryKey: ['workflows'],
    queryFn: () => api.get<{ workflows: WorkflowDefinition[] }>('/v1/workflows?limit=200')
  });
  const workflows = workflowsQuery.data?.workflows || [];

  useEffect(() => {
    if (!sessionForm.workflowId && workflows.length) {
      setSessionForm((prev) => ({ ...prev, workflowId: workflows[0].workflowId }));
    }
  }, [sessionForm.workflowId, workflows]);

  useEffect(() => {
    setParticipant(null);
    setSelectedStepId('');
    setSelectedStepIds([]);
    setBatchMessage('');
    setDecisionId('');
    setStreamEvents([]);
  }, [activeSessionId]);

  const sessionQuery = useQuery({
    queryKey: ['collab', 'session', activeSessionId],
    queryFn: () => api.get<CollabSession>(`/v2/collab/sessions/${activeSessionId}`),
    enabled: !!activeSessionId
  });

  const participantsQuery = useQuery({
    queryKey: ['collab', 'participants', activeSessionId],
    queryFn: () => api.get<CollabParticipantsResponse>(`/v2/collab/sessions/${activeSessionId}/participants?limit=200`),
    enabled: !!activeSessionId
  });

  const openStepsQuery = useQuery({
    queryKey: ['collab', 'openSteps', activeSessionId, participant?.participantId || '', onlyMyOperable],
    queryFn: () => {
      const search = new URLSearchParams();
      search.set('limit', '100');
      if (onlyMyOperable && participant?.participantId) {
        search.set('participant_id', participant.participantId);
      }
      return api.get<CollabOpenStepsResponse>(`/v2/collab/sessions/${activeSessionId}/steps/open?${search.toString()}`);
    },
    enabled: !!activeSessionId
  });

  const eventsQuery = useQuery({
    queryKey: ['collab', 'events', activeSessionId],
    queryFn: () => api.get<CollabEventsResponse>(`/v2/collab/sessions/${activeSessionId}/events?limit=200`),
    enabled: !!activeSessionId
  });

  const stepQuery = useQuery({
    queryKey: ['collab', 'step', selectedStepId],
    queryFn: () => api.get<CollabStep>(`/v2/collab/steps/${selectedStepId}`),
    enabled: !!selectedStepId
  });

  const artifactsQuery = useQuery({
    queryKey: ['collab', 'artifacts', selectedStepId],
    queryFn: () => api.get<CollabArtifactsResponse>(`/v2/collab/steps/${selectedStepId}/artifacts`),
    enabled: !!selectedStepId
  });

  const participants = participantsQuery.data?.participants || [];
  const openSteps = openStepsQuery.data?.steps || [];
  const timeline = eventsQuery.data?.events || [];
  const artifacts = artifactsQuery.data?.artifacts || [];

  useEffect(() => {
    if (participant || !participants.length) {
      return;
    }
    const actorRef = actor?.trim();
    if (actorRef) {
      const byActor = participants.find((item) => item.ref === actorRef);
      if (byActor) {
        setParticipant(byActor);
        return;
      }
    }
    if (user) {
      const fallbackRef = `${user.type === 'AGENT' ? 'agent' : 'user'}:${user.username}`;
      const byUser = participants.find((item) => item.ref === fallbackRef);
      if (byUser) {
        setParticipant(byUser);
      }
    }
  }, [actor, participant, participants, user]);

  useEffect(() => {
    if (!selectedStepId && openSteps.length) {
      setSelectedStepId(openSteps[0].stepId);
    }
  }, [openSteps, selectedStepId]);

  useEffect(() => {
    if (!activeSessionId) {
      setStreamStatus('idle');
      return;
    }

    setStreamStatus('connecting');
    const clientID = `collab-ui-${Math.random().toString(36).slice(2)}`;
    const es = new EventSource(`/v2/collab/stream?client_id=${clientID}&session_id=${activeSessionId}`);

    es.onopen = () => {
      setStreamStatus('live');
    };

    es.onmessage = (evt) => {
      try {
        const message = JSON.parse(evt.data) as CollabSSEMessage;
        if (message.event !== 'collab' || !message.data) {
          return;
        }
        const incoming: CollabEvent = {
          id: Date.now(),
          eventId: message.data.eventId || message.id || `stream-${Date.now()}`,
          sessionId: message.data.sessionId || activeSessionId,
          stepId: message.data.stepId,
          type: message.data.type || 'COLLAB_EVENT',
          actor: message.data.actor || 'system',
          payload: message.data.payload,
          createdAt: message.data.createdAt || message.timestamp
        };
        setStreamEvents((prev) => [incoming, ...prev].slice(0, 30));
        queryClient.invalidateQueries({ queryKey: ['collab', 'events', activeSessionId] });
        queryClient.invalidateQueries({ queryKey: ['collab', 'openSteps', activeSessionId] });
        if (incoming.stepId) {
          queryClient.invalidateQueries({ queryKey: ['collab', 'step', incoming.stepId] });
          queryClient.invalidateQueries({ queryKey: ['collab', 'artifacts', incoming.stepId] });
        }
      } catch {
        // ignore malformed stream packets
      }
    };

    es.onerror = () => {
      setStreamStatus('error');
    };

    return () => {
      es.close();
    };
  }, [activeSessionId, queryClient]);

  const createSession = useMutation({
    mutationFn: async () => {
      setError(undefined);
      const payload: Record<string, unknown> = {
        workflow_id: sessionForm.workflowId,
        title: sessionForm.title
      };
      const contextText = sessionForm.context.trim();
      if (contextText) {
        payload.context = JSON.parse(contextText);
      }
      return api.post<CollabSession>('/v2/collab/sessions', payload);
    },
    onSuccess: (session) => {
      setActiveSessionId(session.sessionId);
      setSessionInput(session.sessionId);
      queryClient.invalidateQueries({ queryKey: ['collab'] });
    },
    onError: (err) => setError(toErrorMessage(err))
  });

  const joinSession = useMutation({
    mutationFn: async () => {
      if (!activeSessionId) {
        throw new Error('Set session first');
      }
      setError(undefined);
      const trustScore = Number(joinForm.trustScore);
      const capabilities = joinForm.capabilities
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean);
      const payload: Record<string, unknown> = {
        type: joinForm.type,
        capabilities,
        trust_score: Number.isFinite(trustScore) ? trustScore : 0
      };
      if (joinForm.ref.trim()) {
        payload.ref = joinForm.ref.trim();
      }
      return api.post<CollabParticipant>(`/v2/collab/sessions/${activeSessionId}/join`, payload);
    },
    onSuccess: (next) => {
      setParticipant(next);
      queryClient.invalidateQueries({ queryKey: ['collab', 'openSteps', activeSessionId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'participants', activeSessionId] });
    },
    onError: (err) => setError(toErrorMessage(err))
  });

  const claimStep = useMutation({
    mutationFn: async (stepId: string) => {
      if (!participant) {
        throw new Error('Join session first');
      }
      const lease = Number(leaseSeconds);
      return api.post<CollabClaim>(`/v2/collab/steps/${stepId}/claim`, {
        participant_id: participant.participantId,
        lease_seconds: Number.isFinite(lease) ? lease : 900
      });
    },
    onSuccess: (_claim, stepId) => {
      setSelectedStepId(stepId);
      queryClient.invalidateQueries({ queryKey: ['collab', 'openSteps', activeSessionId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'step', stepId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'events', activeSessionId] });
    },
    onError: (err) => setError(toErrorMessage(err))
  });

  const releaseStep = useMutation({
    mutationFn: async () => {
      if (!participant || !selectedStepId) {
        throw new Error('Select a step and join as participant first');
      }
      return api.post(`/v2/collab/steps/${selectedStepId}/release`, {
        participant_id: participant.participantId
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['collab', 'openSteps', activeSessionId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'step', selectedStepId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'events', activeSessionId] });
    },
    onError: (err) => setError(toErrorMessage(err))
  });

  const handoffStep = useMutation({
    mutationFn: async () => {
      if (!participant || !selectedStepId) {
        throw new Error('Select a step and join as participant first');
      }
      if (!handoffToParticipantId.trim()) {
        throw new Error('Target participant id is required');
      }
      const lease = Number(leaseSeconds);
      return api.post<CollabClaim>(`/v2/collab/steps/${selectedStepId}/handoff`, {
        from_participant_id: participant.participantId,
        to_participant_id: handoffToParticipantId.trim(),
        lease_seconds: Number.isFinite(lease) ? lease : 900,
        comment: handoffComment.trim() || undefined
      });
    },
    onSuccess: () => {
      setHandoffComment('');
      queryClient.invalidateQueries({ queryKey: ['collab', 'openSteps', activeSessionId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'step', selectedStepId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'events', activeSessionId] });
    },
    onError: (err) => setError(toErrorMessage(err))
  });

  const submitArtifact = useMutation({
    mutationFn: async () => {
      if (!participant || !selectedStepId) {
        throw new Error('Select a step and join as participant first');
      }
      const content = JSON.parse(artifactContent);
      return api.post<CollabArtifact>(`/v2/collab/steps/${selectedStepId}/artifacts`, {
        participant_id: participant.participantId,
        kind: artifactKind.trim() || undefined,
        content
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['collab', 'step', selectedStepId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'artifacts', selectedStepId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'events', activeSessionId] });
    },
    onError: (err) => setError(toErrorMessage(err))
  });

  const openDecision = useMutation({
    mutationFn: async () => {
      if (!selectedStepId) {
        throw new Error('Select a step first');
      }
      const payload: Record<string, unknown> = {};
      if (decisionPolicy.trim()) {
        payload.policy = JSON.parse(decisionPolicy);
      }
      if (decisionDeadline.trim()) {
        const d = new Date(decisionDeadline);
        if (!Number.isNaN(d.getTime())) {
          payload.deadline = d.toISOString();
        }
      }
      return api.post<CollabDecision>(`/v2/collab/steps/${selectedStepId}/decisions`, payload);
    },
    onSuccess: (decision) => {
      setDecisionId(decision.decisionId);
      queryClient.invalidateQueries({ queryKey: ['collab', 'events', activeSessionId] });
    },
    onError: (err) => setError(toErrorMessage(err))
  });

  const castVote = useMutation({
    mutationFn: async () => {
      if (!participant) {
        throw new Error('Join session first');
      }
      if (!decisionId.trim()) {
        throw new Error('Decision id is required');
      }
      return api.post<CollabDecision>(`/v2/collab/decisions/${decisionId.trim()}/votes`, {
        participant_id: participant.participantId,
        choice: voteChoice,
        comment: voteComment.trim() || undefined
      });
    },
    onSuccess: (decision) => {
      setDecisionId(decision.decisionId);
      queryClient.invalidateQueries({ queryKey: ['collab', 'events', activeSessionId] });
      if (selectedStepId) {
        queryClient.invalidateQueries({ queryKey: ['collab', 'step', selectedStepId] });
      }
    },
    onError: (err) => setError(toErrorMessage(err))
  });

  const resolveStep = useMutation({
    mutationFn: async () => {
      if (!selectedStepId) {
        throw new Error('Select a step first');
      }
      const payload = resolveWithClaimOwner && participant
        ? { participant_id: participant.participantId }
        : undefined;
      return api.post<CollabStep>(`/v2/collab/steps/${selectedStepId}/resolve`, payload);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['collab', 'session', activeSessionId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'openSteps', activeSessionId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'events', activeSessionId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'step', selectedStepId] });
    },
    onError: (err) => setError(toErrorMessage(err))
  });

  const selectedStep = stepQuery.data;
  const session = sessionQuery.data;
  const combinedError = useMemo(() => {
    if (error) {
      return error;
    }
    const firstQueryError = [
      workflowsQuery.error,
      sessionQuery.error,
      participantsQuery.error,
      openStepsQuery.error,
      eventsQuery.error,
      stepQuery.error,
      artifactsQuery.error
    ].find(Boolean);
    return firstQueryError ? toErrorMessage(firstQueryError) : undefined;
  }, [error, workflowsQuery.error, sessionQuery.error, participantsQuery.error, openStepsQuery.error, eventsQuery.error, stepQuery.error, artifactsQuery.error]);

  const handleUseSession = () => {
    const next = sessionInput.trim();
    if (!next) {
      setError('Session id is required');
      return;
    }
    setError(undefined);
    setActiveSessionId(next);
  };

  const toggleStepSelection = (stepId: string) => {
    setSelectedStepIds((prev) => {
      if (prev.includes(stepId)) {
        return prev.filter((id) => id !== stepId);
      }
      return [...prev, stepId];
    });
  };

  const selectAllVisible = () => {
    const visible = openSteps.map((step) => step.stepId);
    setSelectedStepIds((prev) => Array.from(new Set([...prev, ...visible])));
  };

  const clearSelection = () => {
    setSelectedStepIds([]);
    setBatchMessage('');
  };

  const runBatch = async (
    label: string,
    stepIds: string[],
    run: (stepId: string) => Promise<void>
  ) => {
    if (!stepIds.length) {
      setError('Select at least one step');
      return;
    }
    setError(undefined);
    setBatchMessage(`${label}: running ${stepIds.length} items...`);
    let success = 0;
    const failures: string[] = [];

    for (const stepId of stepIds) {
      try {
        await run(stepId);
        success++;
      } catch (err) {
        failures.push(`${shortID(stepId)} ${toErrorMessage(err)}`);
      }
    }

    const failed = failures.length;
    setBatchMessage(`${label}: ${success} success / ${failed} failed`);
    if (failed > 0) {
      setError(`Batch partial failure: ${failures[0]}`);
    }
    queryClient.invalidateQueries({ queryKey: ['collab', 'openSteps', activeSessionId] });
    queryClient.invalidateQueries({ queryKey: ['collab', 'events', activeSessionId] });
    queryClient.invalidateQueries({ queryKey: ['collab', 'participants', activeSessionId] });
    if (selectedStepId) {
      queryClient.invalidateQueries({ queryKey: ['collab', 'step', selectedStepId] });
      queryClient.invalidateQueries({ queryKey: ['collab', 'artifacts', selectedStepId] });
    }
  };

  const runBatchClaim = async () => {
    if (!participant) {
      setError('Join session first');
      return;
    }
    const lease = Number(leaseSeconds);
    await runBatch('Batch claim', selectedStepIds, async (stepId) => {
      await api.post(`/v2/collab/steps/${stepId}/claim`, {
        participant_id: participant.participantId,
        lease_seconds: Number.isFinite(lease) ? lease : 900
      });
    });
  };

  const runBatchRelease = async () => {
    if (!participant) {
      setError('Join session first');
      return;
    }
    await runBatch('Batch release', selectedStepIds, async (stepId) => {
      await api.post(`/v2/collab/steps/${stepId}/release`, {
        participant_id: participant.participantId
      });
    });
  };

  const runBatchResolve = async () => {
    await runBatch('Batch resolve', selectedStepIds, async (stepId) => {
      const payload = resolveWithClaimOwner && participant
        ? { participant_id: participant.participantId }
        : undefined;
      await api.post(`/v2/collab/steps/${stepId}/resolve`, payload);
    });
  };

  return (
    <div className="v2-page">
      <section className="panel v2-hero reveal">
        <div>
          <div className="tag">V2 Collaboration</div>
          <h2 style={{ margin: '10px 0 6px' }}>Leaderless Multi-User Console</h2>
          <div className="muted">
            Session + claim + handoff + artifact + vote + resolve. Actor: <span className="mono">{actor || 'user:self'}</span>
          </div>
        </div>
        <div className="v2-metrics">
          <div className="v2-metric-card">
            <div className="mono">Session</div>
            <div className="v2-metric-value">{activeSessionId ? shortID(activeSessionId) : 'none'}</div>
            <div className={badgeClass(session?.status || 'IDLE')}>{session?.status || 'IDLE'}</div>
          </div>
          <div className="v2-metric-card">
            <div className="mono">Open Steps</div>
            <div className="v2-metric-value">{openSteps.length}</div>
            <div className="muted">claimable now</div>
          </div>
          <div className="v2-metric-card">
            <div className="mono">Participant</div>
            <div className="v2-metric-value">{participant ? shortID(participant.participantId) : 'none'}</div>
            <div className="mono">{participant?.ref || 'join required'}</div>
          </div>
          <div className="v2-metric-card">
            <div className="mono">Stream</div>
            <div className="v2-stream">
              <span className={`dot ${streamStatus}`} />
              <span>{streamStatus.toUpperCase()}</span>
            </div>
            <div className="muted">{streamEvents.length} recent events</div>
          </div>
        </div>
      </section>

      <ErrorBanner message={combinedError} />

      <div className="v2-layout reveal">
        <section className="panel v2-column">
          <div className="v2-section-title">Session Setup</div>
          <div className="v2-stack">
            <select
              className="select"
              value={sessionForm.workflowId}
              onChange={(e) => setSessionForm({ ...sessionForm, workflowId: e.target.value })}
            >
              {workflows.map((wf) => (
                <option key={wf.workflowId} value={wf.workflowId}>
                  {wf.name} v{wf.version}
                </option>
              ))}
            </select>
            <input
              className="input"
              placeholder="Session title"
              value={sessionForm.title}
              onChange={(e) => setSessionForm({ ...sessionForm, title: e.target.value })}
            />
            <textarea
              className="textarea json-area"
              value={sessionForm.context}
              onChange={(e) => setSessionForm({ ...sessionForm, context: e.target.value })}
            />
            <button className="button" onClick={() => createSession.mutate()} disabled={!sessionForm.workflowId || createSession.isPending}>
              {createSession.isPending ? 'Creating...' : 'Create Session'}
            </button>
          </div>

          <div className="v2-divider" />

          <div className="v2-section-title">Attach Existing Session</div>
          <div className="toolbar">
            <input
              className="input"
              placeholder="Session ID"
              value={sessionInput}
              onChange={(e) => setSessionInput(e.target.value)}
            />
            <button className="button secondary" onClick={handleUseSession}>Attach</button>
          </div>
          {session && (
            <div className="panel compact" style={{ marginTop: 10, background: 'var(--panel-2)' }}>
              <div style={{ fontWeight: 600 }}>{session.name}</div>
              <div className="mono">Task: {shortID(session.taskId)}</div>
              <div className="mono">Trace: {shortID(session.traceId)}</div>
              <div className="mono">Updated: {formatTime(session.updatedAt)}</div>
            </div>
          )}

          <div className="v2-divider" />

          <div className="v2-section-title">Join Session</div>
          <div className="v2-stack">
            <select
              className="select"
              value={joinForm.type}
              onChange={(e) => setJoinForm({ ...joinForm, type: e.target.value as JoinType })}
            >
              <option value="HUMAN">HUMAN</option>
              <option value="AGENT">AGENT</option>
            </select>
            <input
              className="input"
              placeholder="Ref (optional)"
              value={joinForm.ref}
              onChange={(e) => setJoinForm({ ...joinForm, ref: e.target.value })}
            />
            <input
              className="input"
              placeholder="Capabilities (comma separated)"
              value={joinForm.capabilities}
              onChange={(e) => setJoinForm({ ...joinForm, capabilities: e.target.value })}
            />
            <input
              className="input"
              placeholder="Trust score"
              value={joinForm.trustScore}
              onChange={(e) => setJoinForm({ ...joinForm, trustScore: e.target.value })}
            />
            <button className="button" onClick={() => joinSession.mutate()} disabled={!activeSessionId || joinSession.isPending}>
              {joinSession.isPending ? 'Joining...' : 'Join'}
            </button>
          </div>

          <div className="v2-divider" />

          <div className="v2-section-title">Participants</div>
          <div className="toolbar">
            <button className="button secondary" onClick={() => participantsQuery.refetch()} disabled={!activeSessionId}>
              Refresh
            </button>
            <span className="mono">{participants.length} total</span>
          </div>
          <div className="stream-list" style={{ maxHeight: 220 }}>
            {participants.length ? (
              participants.map((p) => {
                const active = participant?.participantId === p.participantId;
                return (
                  <div
                    key={p.participantId}
                    className="stream-item"
                    style={active ? { borderColor: 'var(--accent)', boxShadow: '0 0 0 2px rgba(15, 118, 110, 0.14)' } : undefined}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
                      <span className="tag">{p.type}</span>
                      <button className="button secondary" onClick={() => setParticipant(p)}>
                        {active ? 'Using' : 'Use'}
                      </button>
                    </div>
                    <div className="mono">{p.ref}</div>
                    <div className="mono">id: {shortID(p.participantId)}</div>
                    <div className="mono">capabilities: {(p.capabilities || []).join(', ') || 'none'}</div>
                    <div className="mono">trust: {p.trustScore}</div>
                    <div className="mono">last seen: {formatTime(p.lastSeenAt)}</div>
                  </div>
                );
              })
            ) : (
              <div className="muted">No participants yet. Join session to create one.</div>
            )}
          </div>
        </section>

        <section className="panel v2-column">
          <div className="v2-section-title">Open Steps</div>
          <div className="toolbar" style={{ marginBottom: 10 }}>
            <input
              className="input"
              placeholder="Lease seconds"
              value={leaseSeconds}
              onChange={(e) => setLeaseSeconds(e.target.value)}
            />
            <label className="mono" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <input
                type="checkbox"
                checked={onlyMyOperable}
                onChange={(e) => setOnlyMyOperable(e.target.checked)}
                disabled={!participant}
              />
              仅我可操作
            </label>
            <button className="button secondary" onClick={selectAllVisible} disabled={!openSteps.length}>
              全选可见
            </button>
            <button className="button secondary" onClick={clearSelection} disabled={!selectedStepIds.length}>
              清空选择
            </button>
            <button className="button secondary" onClick={() => openStepsQuery.refetch()} disabled={!activeSessionId}>
              Refresh
            </button>
          </div>
          <div className="toolbar" style={{ marginBottom: 10 }}>
            <span className="mono">已选: {selectedStepIds.length}</span>
            <button className="button" onClick={runBatchClaim} disabled={!participant || !selectedStepIds.length}>
              批量 Claim
            </button>
            <button className="button secondary" onClick={runBatchRelease} disabled={!participant || !selectedStepIds.length}>
              批量 Release
            </button>
            <button className="button secondary" onClick={runBatchResolve} disabled={!selectedStepIds.length}>
              批量 Resolve
            </button>
            {batchMessage && <span className="mono">{batchMessage}</span>}
          </div>
          <div className="step-list">
            {openSteps.length ? (
              openSteps.map((step) => (
                <div
                  key={step.stepId}
                  className={`step-item ${selectedStepId === step.stepId ? 'active' : ''}`}
                  onClick={() => setSelectedStepId(step.stepId)}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
                    <div>
                      <label className="mono" style={{ display: 'inline-flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                        <input
                          type="checkbox"
                          checked={selectedStepIds.includes(step.stepId)}
                          onChange={(evt) => {
                            evt.stopPropagation();
                            toggleStepSelection(step.stepId);
                          }}
                        />
                        Pick
                      </label>
                      <div style={{ fontWeight: 600 }}>{step.name}</div>
                      <div className="mono">{step.stepKey}</div>
                    </div>
                    <span className={badgeClass(step.status)}>{step.status}</span>
                  </div>
                  <div className="mono">depends_on: {(step.dependsOn || []).join(', ') || 'none'}</div>
                  <div className="mono">required_capabilities: {(step.requiredCapabilities || []).join(', ') || 'none'}</div>
                  <div style={{ marginTop: 8 }}>
                    <button
                      className="button"
                      onClick={(evt) => {
                        evt.stopPropagation();
                        claimStep.mutate(step.stepId);
                      }}
                      disabled={!participant || claimStep.isPending}
                    >
                      Claim
                    </button>
                  </div>
                </div>
              ))
            ) : (
              <div className="muted">No claimable step. Check dependencies or participant capabilities.</div>
            )}
          </div>

          <div className="v2-divider" />

          <div className="v2-section-title">Step Workspace</div>
          {selectedStep ? (
            <div className="v2-stack">
              <div className="panel compact" style={{ background: 'var(--panel-2)' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
                  <div>
                    <div style={{ fontWeight: 600 }}>{selectedStep.name}</div>
                    <div className="mono">{selectedStep.stepKey}</div>
                  </div>
                  <span className={badgeClass(selectedStep.status)}>{selectedStep.status}</span>
                </div>
                <div className="mono">Step: {selectedStep.stepId}</div>
                <div className="mono">TTL: {selectedStep.leaseTtlSeconds}s</div>
              </div>

              <div className="toolbar">
                <button className="button secondary" onClick={() => releaseStep.mutate()} disabled={!participant || releaseStep.isPending}>
                  Release
                </button>
                <label className="mono" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <input
                    type="checkbox"
                    checked={resolveWithClaimOwner}
                    onChange={(e) => setResolveWithClaimOwner(e.target.checked)}
                  />
                  resolve with participant id
                </label>
                <button className="button" onClick={() => resolveStep.mutate()} disabled={resolveStep.isPending}>
                  Resolve
                </button>
              </div>

              <div className="v2-subsection">
                <div style={{ fontWeight: 600 }}>Handoff</div>
                <input
                  className="input"
                  placeholder="To participant_id"
                  value={handoffToParticipantId}
                  onChange={(e) => setHandoffToParticipantId(e.target.value)}
                />
                <input
                  className="input"
                  placeholder="Comment (optional)"
                  value={handoffComment}
                  onChange={(e) => setHandoffComment(e.target.value)}
                />
                <button className="button secondary" onClick={() => handoffStep.mutate()} disabled={!participant || handoffStep.isPending}>
                  Handoff
                </button>
              </div>

              <div className="v2-subsection">
                <div style={{ fontWeight: 600 }}>Submit Artifact</div>
                <input
                  className="input"
                  placeholder="kind"
                  value={artifactKind}
                  onChange={(e) => setArtifactKind(e.target.value)}
                />
                <textarea
                  className="textarea json-area"
                  value={artifactContent}
                  onChange={(e) => setArtifactContent(e.target.value)}
                />
                <button className="button" onClick={() => submitArtifact.mutate()} disabled={!participant || submitArtifact.isPending}>
                  Submit Artifact
                </button>
              </div>

              <div className="v2-subsection">
                <div style={{ fontWeight: 600 }}>Decision + Vote</div>
                <textarea
                  className="textarea json-area"
                  value={decisionPolicy}
                  onChange={(e) => setDecisionPolicy(e.target.value)}
                />
                <input
                  className="input"
                  type="datetime-local"
                  value={decisionDeadline}
                  onChange={(e) => setDecisionDeadline(e.target.value)}
                />
                <button className="button secondary" onClick={() => openDecision.mutate()} disabled={openDecision.isPending}>
                  Open Decision
                </button>
                <input
                  className="input"
                  placeholder="decision_id"
                  value={decisionId}
                  onChange={(e) => setDecisionId(e.target.value)}
                />
                <div className="toolbar">
                  <select className="select" value={voteChoice} onChange={(e) => setVoteChoice(e.target.value as VoteChoice)}>
                    <option value="APPROVE">APPROVE</option>
                    <option value="REJECT">REJECT</option>
                  </select>
                  <input
                    className="input"
                    placeholder="vote comment"
                    value={voteComment}
                    onChange={(e) => setVoteComment(e.target.value)}
                  />
                </div>
                <button className="button" onClick={() => castVote.mutate()} disabled={!participant || castVote.isPending}>
                  Cast Vote
                </button>
              </div>
            </div>
          ) : (
            <div className="muted">Select a step to open workspace.</div>
          )}
        </section>

        <section className="panel v2-column">
          <div className="v2-section-title">Realtime Stream</div>
          <div className="stream-list">
            {streamEvents.length ? (
              streamEvents.map((evt) => (
                <div key={evt.eventId} className="stream-item">
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
                    <span className="tag">{evt.type}</span>
                    <span className="mono">{formatTime(evt.createdAt)}</span>
                  </div>
                  <div className="mono">actor: {evt.actor}</div>
                  <div className="mono">step: {shortID(evt.stepId)}</div>
                </div>
              ))
            ) : (
              <div className="muted">No stream events yet.</div>
            )}
          </div>

          <div className="v2-divider" />

          <div className="v2-section-title">Event Timeline</div>
          <div className="stream-list">
            {timeline.length ? (
              timeline.map((evt) => (
                <div key={evt.eventId} className="stream-item">
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
                    <span className="tag">{evt.type}</span>
                    <span className="mono">{formatTime(evt.createdAt)}</span>
                  </div>
                  <div className="mono">actor: {evt.actor}</div>
                  <div className="mono">step: {shortID(evt.stepId)}</div>
                </div>
              ))
            ) : (
              <div className="muted">No events found.</div>
            )}
          </div>

          <div className="v2-divider" />

          <div className="v2-section-title">Step Artifacts</div>
          <div className="stream-list">
            {artifacts.length ? (
              artifacts.map((artifact) => (
                <div key={artifact.artifactId} className="stream-item">
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
                    <span className="tag">{artifact.kind}</span>
                    <span className="mono">v{artifact.version}</span>
                  </div>
                  <div className="mono">producer: {shortID(artifact.producerId)}</div>
                  <pre className="code-block" style={{ margin: '8px 0 0' }}>
                    {JSON.stringify(artifact.content, null, 2)}
                  </pre>
                </div>
              ))
            ) : (
              <div className="muted">No artifacts for selected step.</div>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
