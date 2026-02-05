import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { User } from '../api/types';
import { useAuth } from '../components/AuthProvider';

export default function UsersPage() {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const { data: usersData } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get<{ users: User[] }>('/v1/users?limit=100')
  });
  const { data: agentsData } = useQuery({
    queryKey: ['agents'],
    queryFn: () => api.get<{ agents: User[] }>('/v1/agents?limit=100')
  });

  const [userForm, setUserForm] = useState({ username: '', password: '', role: 'OPERATOR' });
  const [agentForm, setAgentForm] = useState({ username: '', password: '', role: 'OPERATOR', owner_user_id: '' });

  const createUser = useMutation({
    mutationFn: () => api.post('/v1/users', userForm),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] })
  });

  const createAgent = useMutation({
    mutationFn: () => api.post('/v1/agents', agentForm),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agents'] })
  });

  const users = usersData?.users || [];
  const agents = agentsData?.agents || [];
  const humanUsers = useMemo(() => users.filter((u) => u.type === 'HUMAN'), [users]);

  useEffect(() => {
    if (!agentForm.owner_user_id && humanUsers.length) {
      setAgentForm((prev) => ({ ...prev, owner_user_id: humanUsers[0].userId }));
    }
  }, [humanUsers, agentForm.owner_user_id]);

  const canCreateUser = userForm.username && userForm.password;
  const canCreateAgent = agentForm.username && agentForm.password && agentForm.owner_user_id;

  if (user?.role !== 'ADMIN') {
    return (
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Users</div>
        <div className="muted">Admin only.</div>
      </div>
    );
  }

  return (
    <div className="grid cols-2">
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Users</div>
        <table className="data-table">
          <thead>
            <tr>
              <th>Username</th>
              <th>Role</th>
              <th>Type</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.userId}>
                <td>{u.username}</td>
                <td>{u.role}</td>
                <td>{u.type}</td>
                <td><span className="badge">{u.status}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
        <div style={{ marginTop: 12, display: 'grid', gap: 8 }}>
          <div style={{ fontWeight: 600 }}>Create User</div>
          <input className="input" placeholder="Username" value={userForm.username} onChange={(e) => setUserForm({ ...userForm, username: e.target.value })} />
          <input className="input" placeholder="Password" type="password" value={userForm.password} onChange={(e) => setUserForm({ ...userForm, password: e.target.value })} />
          <select className="select" value={userForm.role} onChange={(e) => setUserForm({ ...userForm, role: e.target.value })}>
            <option value="ADMIN">ADMIN</option>
            <option value="OPERATOR">OPERATOR</option>
            <option value="VIEWER">VIEWER</option>
          </select>
          <button className="button" onClick={() => createUser.mutate()} disabled={!canCreateUser}>Create</button>
        </div>
      </div>
      <div className="panel">
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Agents</div>
        <table className="data-table">
          <thead>
            <tr>
              <th>Username</th>
              <th>Role</th>
              <th>Owner</th>
            </tr>
          </thead>
          <tbody>
            {agents.map((u) => (
              <tr key={u.userId}>
                <td>{u.username}</td>
                <td>{u.role}</td>
                <td className="mono">{u.ownerUserId?.slice(0, 8)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <div style={{ marginTop: 12, display: 'grid', gap: 8 }}>
          <div style={{ fontWeight: 600 }}>Create Agent</div>
          <input className="input" placeholder="Username" value={agentForm.username} onChange={(e) => setAgentForm({ ...agentForm, username: e.target.value })} />
          <input className="input" placeholder="Password" type="password" value={agentForm.password} onChange={(e) => setAgentForm({ ...agentForm, password: e.target.value })} />
          <select className="select" value={agentForm.role} onChange={(e) => setAgentForm({ ...agentForm, role: e.target.value })}>
            <option value="ADMIN">ADMIN</option>
            <option value="OPERATOR">OPERATOR</option>
            <option value="VIEWER">VIEWER</option>
          </select>
          <select className="select" value={agentForm.owner_user_id} onChange={(e) => setAgentForm({ ...agentForm, owner_user_id: e.target.value })}>
            {humanUsers.length ? (
              humanUsers.map((u) => (
                <option key={u.userId} value={u.userId}>{u.username}</option>
              ))
            ) : (
              <option value="">No human users</option>
            )}
          </select>
          <button className="button" onClick={() => createAgent.mutate()} disabled={!canCreateAgent}>Create Agent</button>
        </div>
      </div>
    </div>
  );
}
