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
  deployment_status: string;
  deployment_image: string;
  deployment_replicas_desired: number;
  deployment_replicas_ready: number;
  storage_size?: string;
  storage_class?: string;
  secret_name?: string;
  pvc_phase?: string;
  last_deployed_at?: string;
  created_at: string;
};

export type ServiceDependency = {
  id: string;
  service_id: string;
  depends_on_service_id: string;
  depends_on_name?: string;
  depends_on_slug?: string;
  depends_on_template_id?: string;
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
  template_id: string;
  workload_type: string;
  created_at: string;
  updated_at: string;
  environments?: ServiceEnvironment[];
  dependencies?: ServiceDependency[];
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

export async function listServices() {
    const data = await request<Service[]>('/api/v1/services');
    return data ?? [];
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

export async function listAuditLogs() {
    const data = await request<AuditLog[]>('/api/v1/audit-logs');
    return data ?? [];
  }

  export function createServiceFromTemplate(input: {
    template_id?: string;
    team_id: string;
    name: string;
    slug: string;
    description?: string;
    repository_url?: string;
    owner_email?: string;
  }) {
    return request<CreateFromTemplateResponse>('/api/v1/services/from-template', {
      method: 'POST',
      body: JSON.stringify({
        template_id: input.template_id ?? 'go-api',
        team_id: input.team_id,
        name: input.name,
        slug: input.slug,
        description: input.description ?? '',
        repository_url: input.repository_url ?? '',
        owner_email: input.owner_email ?? '',
      }),
    });
  }

export type CreateFromTemplateResponse = {
  service: Service;
  generated_path: string;
  generated_files: string[];
  template_id: string;
};

export type DeployServiceResponse = {
  environment: ServiceEnvironment;
  workload: {
    namespace: string;
    deployment_name: string;
    image: string;
    replicas_desired: number;
    replicas_ready: number;
    available_replicas: number;
    deployment_status: string;
  };
};

export function deployService(
  serviceId: string,
  input: { environment: string; image?: string; storage_size?: string; storage_class?: string }
) {
  return request<DeployServiceResponse>(`/api/v1/services/${serviceId}/deploy`, {
    method: 'POST',
    body: JSON.stringify({
      environment: input.environment,
      image: input.image ?? '',
      storage_size: input.storage_size ?? '',
      storage_class: input.storage_class ?? '',
    }),
  });
}

export function syncDeployment(
  serviceId: string,
  input: { environment: string }
) {
  return request<DeployServiceResponse>(`/api/v1/services/${serviceId}/sync-deployment`, {
    method: 'POST',
    body: JSON.stringify({ environment: input.environment }),
  });
}

export type ServiceNetworkInfo = {
  environment: string;
  namespace: string;
  service_name: string;
  port: number;
  cluster_ip: string;
  dns_short: string;
  dns_fqdn: string;
  ingress_host?: string;
  ingress_url?: string;
  endpoints_ready: number;
  service_found: boolean;
};

export function getServiceNetwork(serviceId: string, environment: string) {
  const q = new URLSearchParams({ environment });
  return request<ServiceNetworkInfo>(`/api/v1/services/${serviceId}/network?${q}`);
}

export type ConnectivityCheckResult = {
  environment: string;
  source_service_id: string;
  target_service_id: string;
  target_service_slug: string;
  ok: boolean;
  message: string;
  source_namespace: string;
  target_namespace: string;
  target_service_name: string;
  target_url: string;
  endpoints_ready: number;
};

export function checkConnectivity(
  serviceId: string,
  input: { environment: string; target_service_id: string }
) {
  return request<ConnectivityCheckResult>(`/api/v1/services/${serviceId}/connectivity`, {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

export type DataConnectionInfo = {
  environment: string;
  template_id: string;
  host: string;
  port: number;
  database?: string;
  username?: string;
  secret_name: string;
  password_key: string;
  username_key: string;
  database_key?: string;
  pvc_phase: string;
  endpoints_ready: number;
};

export function getDataConnection(serviceId: string, environment: string) {
  const q = new URLSearchParams({ environment });
  return request<DataConnectionInfo>(`/api/v1/services/${serviceId}/data-connection?${q}`);
}

export function listServiceDependencies(serviceId: string) {
  return request<ServiceDependency[]>(`/api/v1/services/${serviceId}/dependencies`);
}

export function addServiceDependency(serviceId: string, dependsOnServiceId: string) {
  return request<ServiceDependency>(`/api/v1/services/${serviceId}/dependencies`, {
    method: 'POST',
    body: JSON.stringify({ depends_on_service_id: dependsOnServiceId }),
  });
}

export async function removeServiceDependency(serviceId: string, dependencyId: string) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch(`${API_BASE}/api/v1/services/${serviceId}/dependencies/${dependencyId}`, {
    method: 'DELETE',
    headers,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error ?? `Request failed: ${res.status}`);
  }
}