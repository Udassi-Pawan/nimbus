import { useCallback, useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  checkConnectivity,
  deployService,
  getService,
  getServiceNetwork,
  listServices,
  syncDeployment,
} from '../api/client';
import type { ConnectivityCheckResult, Service, ServiceNetworkInfo } from '../api/client';

function envNeedsSync(service: Service): boolean {
  return (service.environments ?? []).some(env => {
    const status = env.deployment_status || 'not_deployed';
    if (status === 'not_deployed') return false;
    if (status === 'running') {
      return env.deployment_replicas_ready < env.deployment_replicas_desired;
    }
    return status === 'progressing' || status === 'pending' || status === 'unknown';
  });
}

export default function ServiceDetailPage() {
  const { id } = useParams();
  const [service, setService] = useState<Service | null>(null);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [image, setImage] = useState('');
  const [deployingEnv, setDeployingEnv] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [networkEnv, setNetworkEnv] = useState('dev');
  const [networkInfo, setNetworkInfo] = useState<ServiceNetworkInfo | null>(null);
  const [networkLoading, setNetworkLoading] = useState(false);
  const [peerServices, setPeerServices] = useState<Service[]>([]);
  const [connectTargetId, setConnectTargetId] = useState('');
  const [connectResult, setConnectResult] = useState<ConnectivityCheckResult | null>(null);
  const [connecting, setConnecting] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    const svc = await getService(id);
    setService(svc);
    setImage(prev => (prev ? prev : `${svc.slug}:latest`));
  }, [id]);

  useEffect(() => {
    load().catch(err =>
      setError(err instanceof Error ? err.message : 'Failed')
    );
  }, [load]);

  useEffect(() => {
    if (!service) return;
    const envs = service.environments ?? [];
    if (envs.length && !envs.some(e => e.name === networkEnv)) {
      setNetworkEnv(envs[0].name);
    }
  }, [service, networkEnv]);

  useEffect(() => {
    listServices()
      .then(all => setPeerServices(all.filter(s => s.id !== id)))
      .catch(() => setPeerServices([]));
  }, [id]);

  const loadNetwork = useCallback(async () => {
    if (!id || !networkEnv) return;
    setNetworkLoading(true);
    setConnectResult(null);
    try {
      const info = await getServiceNetwork(id, networkEnv);
      setNetworkInfo(info);
    } catch (err) {
      setNetworkInfo(null);
      setError(err instanceof Error ? err.message : 'Network lookup failed');
    } finally {
      setNetworkLoading(false);
    }
  }, [id, networkEnv]);

  useEffect(() => {
    loadNetwork().catch(() => {});
  }, [loadNetwork]);

  async function handleConnectivityCheck() {
    if (!id || !connectTargetId) return;
    setConnecting(true);
    setConnectResult(null);
    setError('');
    try {
      const result = await checkConnectivity(id, {
        environment: networkEnv,
        target_service_id: connectTargetId,
      });
      setConnectResult(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Connectivity check failed');
    } finally {
      setConnecting(false);
    }
  }

  const syncFromCluster = useCallback(async () => {
    if (!id || !service) return;
    setSyncing(true);
    try {
      for (const env of service.environments ?? []) {
        if ((env.deployment_status || 'not_deployed') === 'not_deployed') {
          continue;
        }
        await syncDeployment(id, { environment: env.name });
      }
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sync failed');
    } finally {
      setSyncing(false);
    }
  }, [id, service, load]);

  useEffect(() => {
    if (!service || !envNeedsSync(service)) return;
    const timer = window.setInterval(() => {
      syncFromCluster().catch(() => {});
    }, 4000);
    return () => window.clearInterval(timer);
  }, [service, syncFromCluster]);

  async function handleDeploy(environment: string) {
    if (!id) return;
    setError('');
    setSuccess('');
    setDeployingEnv(environment);

    try {
      const result = await deployService(id, {
        environment,
        image: image.trim() || undefined,
      });
      setSuccess(
        `Deployed to ${result.workload.namespace}: ${result.workload.replicas_ready}/${result.workload.replicas_desired} ready (${result.workload.deployment_status}).`
      );
      await load();
      if (result.workload.deployment_status !== 'running') {
        await syncFromCluster();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Deploy failed');
    } finally {
      setDeployingEnv('');
    }
  }

  if (error && !service) return <p style={{ color: 'red' }}>{error}</p>;
  if (!service) return <p>Loading...</p>;

  return (
    <div>
      <Link to="/services">← Back</Link>
      <h2>{service.name}</h2>
      <p><strong>Slug:</strong> {service.slug}</p>
      <p><strong>Repo:</strong> {service.repository_url || '—'}</p>
      <p><strong>Owner:</strong> {service.owner_email}</p>

      {error && <p style={{ color: 'red' }}>{error}</p>}
      {success && <p style={{ color: 'green' }}>{success}</p>}

      <div style={{ marginBottom: 16, padding: 12, border: '1px solid #ddd' }}>
        <h3 style={{ marginTop: 0 }}>Deploy to k3d</h3>
        <p style={{ marginTop: 0, color: '#555' }}>
          Deploy runs <strong>helm upgrade --install</strong> on{' '}
          <code>generated/{service.slug}/helm</code>. Build and import the image first:{' '}
          <code>docker build -t {service.slug}:latest generated/{service.slug}</code>
          {' '}then{' '}
          <code>k3d image import {service.slug}:latest -c nimbus</code>
        </p>
        <label>
          Container image:{' '}
          <input
            value={image}
            onChange={e => setImage(e.target.value)}
            style={{ width: 280 }}
          />
        </label>
        <div style={{ marginTop: 8 }}>
          <button type="button" onClick={() => syncFromCluster()} disabled={syncing}>
            {syncing ? 'Syncing from cluster…' : 'Refresh status from cluster'}
          </button>
          <span style={{ marginLeft: 8, color: '#666', fontSize: 14 }}>
            UI reads Postgres; this pulls live state from k3d.
          </span>
        </div>
      </div>

      <h3>Environments</h3>
      <table cellPadding={8} style={{ borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ textAlign: 'left', borderBottom: '1px solid #ddd' }}>
            <th>Env</th>
            <th>Namespace</th>
            <th>Status</th>
            <th>Ready</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {(service.environments ?? []).map(env => (
            <tr key={env.id} style={{ borderBottom: '1px solid #eee' }}>
              <td>{env.name}</td>
              <td><code>{env.namespace}</code></td>
              <td>{env.deployment_status || 'not_deployed'}</td>
              <td>
                {env.deployment_replicas_ready}/{env.deployment_replicas_desired}
              </td>
              <td>
                <button
                  type="button"
                  disabled={deployingEnv === env.name}
                  onClick={() => handleDeploy(env.name)}
                >
                  {deployingEnv === env.name ? 'Deploying…' : `Deploy ${env.name}`}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <div style={{ marginTop: 24, padding: 12, border: '1px solid #ddd' }}>
        <h3 style={{ marginTop: 0 }}>Network</h3>
        <label>
          Environment:{' '}
          <select
            value={networkEnv}
            onChange={e => setNetworkEnv(e.target.value)}
          >
            {(service.environments ?? []).map(env => (
              <option key={env.id} value={env.name}>
                {env.name}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          style={{ marginLeft: 8 }}
          onClick={() => loadNetwork()}
          disabled={networkLoading}
        >
          {networkLoading ? 'Loading…' : 'Refresh network'}
        </button>

        {networkInfo && (
          <dl style={{ marginTop: 12, display: 'grid', gridTemplateColumns: '140px 1fr', gap: '4px 12px' }}>
            <dt>Cluster IP</dt>
            <dd><code>{networkInfo.service_found ? networkInfo.cluster_ip : '— (not in cluster)'}</code></dd>
            <dt>Endpoints ready</dt>
            <dd>{networkInfo.endpoints_ready}</dd>
            <dt>DNS (short)</dt>
            <dd><code>{networkInfo.dns_short}</code></dd>
            <dt>DNS (FQDN)</dt>
            <dd><code>{networkInfo.dns_fqdn}</code></dd>
            {networkInfo.ingress_url && (
              <>
                <dt>Ingress</dt>
                <dd><a href={networkInfo.ingress_url} target="_blank" rel="noreferrer">{networkInfo.ingress_url}</a></dd>
              </>
            )}
          </dl>
        )}

        <h4 style={{ marginBottom: 8 }}>Connectivity check</h4>
        <p style={{ marginTop: 0, color: '#555', fontSize: 14 }}>
          Runs a short-lived curl Job in the source namespace to hit the target Service FQDN (east–west).
        </p>
        <label>
          Target service:{' '}
          <select
            value={connectTargetId}
            onChange={e => setConnectTargetId(e.target.value)}
            style={{ minWidth: 220 }}
          >
            <option value="">— select —</option>
            {peerServices.map(s => (
              <option key={s.id} value={s.id}>
                {s.name} ({s.slug})
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          style={{ marginLeft: 8 }}
          disabled={!connectTargetId || connecting}
          onClick={() => handleConnectivityCheck()}
        >
          {connecting ? 'Checking…' : 'Run check'}
        </button>
        {connectResult && (
          <p style={{ color: connectResult.ok ? 'green' : '#b45309' }}>
            {connectResult.ok ? 'OK' : 'Failed'}: {connectResult.message}
            {!connectResult.ok && (
              <span style={{ display: 'block', fontSize: 13, color: '#666' }}>
                Target: <code>{connectResult.target_url}</code> ({connectResult.endpoints_ready} endpoints)
              </span>
            )}
          </p>
        )}
      </div>
    </div>
  );
}
