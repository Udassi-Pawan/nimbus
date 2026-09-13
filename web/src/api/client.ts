const API_BASE = import.meta.env.VITE_API_URL ?? '';

function getToken(): string | null {
  return localStorage.getItem('nimbus_token');
}

export function setToken(token: string) {
  localStorage.setItem('nimbus_token', token);
}

export function clearToken() {
  localStorage.removeItem('nimbus_token');
}

export function isLoggedIn(): boolean {
  return !!getToken();
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> ?? {}),
  };

  const token = getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error ?? `Request failed: ${res.status}`);
  }

  return res.json();
}

// Types matching your Go models
export type User = {
  id: string;
  email: string;
  name: string;
  created_at: string;
};

export type ServiceEnvironment = {
  id: string;
  service_id: string;
  name: string;
  namespace: string;
  created_at: string;
};

export type Service = {
  id: string;
  team_id: string;
  name: string;
  slug: string;
  description: string;
  repository_url: string;
  owner_email: string;
  created_at: string;
  updated_at: string;
  environments?: ServiceEnvironment[];
};

export type AuditLog = {
  id: string;
  actor_user_id?: string;
  action: string;
  resource_type: string;
  resource_id: string;
  metadata: Record<string, unknown>;
  created_at: string;
};

export function login(email: string, password: string) {
  return request<{ token: string; user: User }>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export function listServices() {
  return request<Service[]>('/api/v1/services');
}

export function getService(id: string) {
  return request<Service>(`/api/v1/services/${id}`);
}

export function createService(input: {
  team_id: string;
  name: string;
  slug: string;
  description?: string;
  repository_url?: string;
  owner_email?: string;
}) {
  return request<Service>('/api/v1/services', {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

export function listAuditLogs() {
  return request<AuditLog[]>('/api/v1/audit-logs');
}