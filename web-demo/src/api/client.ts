import { mockApiFetch } from './mock';

const API_BASE = import.meta.env.VITE_API_BASE || '';
export const isDemoMode = import.meta.env.VITE_DEMO_MODE !== 'false';

type RequestOptions = RequestInit & { actor?: string };

let defaultActor: string | undefined;

export function setDefaultActor(actor?: string) {
  defaultActor = actor;
}

export function getDefaultActor() {
  return defaultActor;
}

export async function apiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  if (isDemoMode) {
    return mockApiFetch<T>(path, options);
  }
  const headers: Record<string, string> = {
    'Content-Type': 'application/json'
  };
  if (options.headers) {
    Object.assign(headers, options.headers as Record<string, string>);
  }
  const actor = options.actor ?? defaultActor;
  if (actor) {
    headers['X-Actor'] = actor;
  }
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    ...options,
    headers
  });
  if (!res.ok) {
    const contentType = res.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
      const body = await res.json();
      const message = body?.message || body?.error || res.statusText;
      throw new Error(message);
    }
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  if (res.status === 204) {
    return {} as T;
  }
  return (await res.json()) as T;
}

export const api = {
  get<T>(path: string, actor?: string) {
    return apiFetch<T>(path, { method: 'GET', actor });
  },
  post<T>(path: string, body?: unknown, actor?: string) {
    return apiFetch<T>(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
      actor
    });
  },
  patch<T>(path: string, body?: unknown, actor?: string) {
    return apiFetch<T>(path, {
      method: 'PATCH',
      body: body ? JSON.stringify(body) : undefined,
      actor
    });
  },
  put<T>(path: string, body?: unknown, actor?: string) {
    return apiFetch<T>(path, {
      method: 'PUT',
      body: body ? JSON.stringify(body) : undefined,
      actor
    });
  }
};
