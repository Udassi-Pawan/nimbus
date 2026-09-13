import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getService } from '../api/client';
import type { Service } from '../api/client';

export default function ServiceDetailPage() {
  const { id } = useParams();
  const [service, setService] = useState<Service | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!id) return;
    getService(id)
      .then(setService)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed'));
  }, [id]);

  if (error) return <p style={{ color: 'red' }}>{error}</p>;
  if (!service) return <p>Loading...</p>;

  return (
    <div>
      <Link to="/services">← Back</Link>
      <h2>{service.name}</h2>
      <p><strong>Slug:</strong> {service.slug}</p>
      <p><strong>Repo:</strong> {service.repository_url || '—'}</p>
      <p><strong>Owner:</strong> {service.owner_email}</p>

      <h3>Environments</h3>
      <table cellPadding={8}>
        <thead><tr><th>Env</th><th>K8s Namespace</th></tr></thead>
        <tbody>
          {(service.environments ?? []).map(env => (
            <tr key={env.id}>
              <td>{env.name}</td>
              <td><code>{env.namespace}</code></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}