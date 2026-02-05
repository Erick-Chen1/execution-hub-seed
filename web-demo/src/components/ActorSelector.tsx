import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import type { User } from '../api/types';
import { useAuth } from './AuthProvider';

export default function ActorSelector() {
  const { user, actor, setActor } = useAuth();
  const { data } = useQuery({
    queryKey: ['agents'],
    queryFn: () => api.get<{ agents: User[] }>('/v1/agents?limit=50'),
    enabled: !!user
  });
  const agents = data?.agents || [];

  const initialAgent = useMemo(() => {
    if (actor && actor.startsWith('agent:')) {
      return actor.slice('agent:'.length);
    }
    return '';
  }, [actor]);

  const [mode, setMode] = useState<'human' | 'agent'>(initialAgent ? 'agent' : 'human');
  const [agent, setAgent] = useState<string>(initialAgent);

  useEffect(() => {
    if (!actor) {
      if (mode !== 'human') {
        setMode('human');
      }
      return;
    }
    if (actor.startsWith('agent:')) {
      const name = actor.slice('agent:'.length);
      if (mode !== 'agent') {
        setMode('agent');
      }
      if (agent !== name) {
        setAgent(name);
      }
    }
  }, [actor, agent, mode]);

  useEffect(() => {
    if (mode === 'agent') {
      if (!agent && agents.length) {
        setAgent(agents[0].username);
        return;
      }
      if (agent) {
        setActor(`agent:${agent}`);
      }
    } else if (actor) {
      setActor(undefined);
    }
  }, [mode, agent, agents, setActor, actor]);

  return (
    <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
      <select className="select" value={mode} onChange={(e) => setMode(e.target.value as 'human' | 'agent')}>
        <option value="human">Act as Me</option>
        <option value="agent">Act as Agent</option>
      </select>
      {mode === 'agent' && (
        <select className="select" value={agent} onChange={(e) => setAgent(e.target.value)} disabled={!agents.length}>
          {agents.length ? (
            agents.map((a) => (
              <option key={a.userId} value={a.username}>{a.username}</option>
            ))
          ) : (
            <option value="">No agents</option>
          )}
        </select>
      )}
    </div>
  );
}
