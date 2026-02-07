import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { api, setDefaultActor } from '../api/client';
import type { User } from '../api/types';

interface AuthContextValue {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
  actor: string | undefined;
  setActor: (actor?: string) => void;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [actor, setActorState] = useState<string | undefined>(() => {
    if (typeof window === 'undefined') {
      return undefined;
    }
    const stored = window.localStorage.getItem('actor_override');
    return stored || undefined;
  });

  useEffect(() => {
    setDefaultActor(actor);
    if (typeof window === 'undefined') {
      return;
    }
    if (actor) {
      window.localStorage.setItem('actor_override', actor);
    } else {
      window.localStorage.removeItem('actor_override');
    }
  }, [actor]);

  const refresh = async () => {
    try {
      const res = await api.get<User>('/v1/auth/me');
      setUser(res);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  const login = async (username: string, password: string) => {
    await api.post('/v1/auth/login', { username, password });
    await refresh();
  };

  const logout = async () => {
    await api.post('/v1/auth/logout');
    setUser(null);
    setActorState(undefined);
  };

  const setActor = (next?: string) => {
    setActorState(next);
  };

  const value = useMemo(
    () => ({ user, loading, login, logout, refresh, actor, setActor }),
    [user, loading, actor]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return ctx;
}
