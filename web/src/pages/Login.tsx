import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../components/AuthProvider';
import { api } from '../api/client';

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleLogin = async () => {
    setLoading(true);
    setError(null);
    try {
      await login(username, password);
      navigate('/');
    } catch (err) {
      setError((err as Error).message || 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  const handleBootstrap = async () => {
    setLoading(true);
    setError(null);
    try {
      await api.post('/v1/auth/bootstrap', { username, password });
      await login(username, password);
      navigate('/');
    } catch (err) {
      setError((err as Error).message || 'Bootstrap failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="main" style={{ maxWidth: 420, margin: '40px auto' }}>
      <div className="panel">
        <h2 style={{ marginTop: 0 }}>Sign in</h2>
        <p style={{ color: 'var(--muted)' }}>
          Use your username and password. For first run, bootstrap an admin.
        </p>
        <div style={{ display: 'grid', gap: 12 }}>
          <input
            className="input"
            placeholder="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
          <input
            className="input"
            type="password"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          {error && <div style={{ color: 'var(--danger)' }}>{error}</div>}
          <button className="button" onClick={handleLogin} disabled={loading}>
            {loading ? 'Signing in...' : 'Login'}
          </button>
          <button className="button secondary" onClick={handleBootstrap} disabled={loading}>
            Bootstrap Admin
          </button>
        </div>
      </div>
    </div>
  );
}
