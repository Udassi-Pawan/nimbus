import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { createService, listServices } from '../api/client';
import type { Service } from '../api/client';

const DEFAULT_TEAM_ID = '22222222-2222-2222-2222-222222222222';

export default function ServicesPage() {
  const [services, setServices] = useState<Service[]>([]);
  const [error, setError] = useState('');
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [repo, setRepo] = useState('');

  async function load() {
    try {
      setServices(await listServices());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load');
    }
  }

  useEffect(() => { load(); }, []);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await createService({
        team_id: DEFAULT_TEAM_ID,
        name,
        slug,
        repository_url: repo,
        owner_email: 'admin@nimbus.local',
      });
      setName(''); setSlug(''); setRepo('');
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed');
    }
  }

  return (
    <div>
      <h2>Service Catalog</h2>
      {error && <p style={{ color: 'red' }}>{error}</p>}

      <form onSubmit={handleCreate} style={{ marginBottom: 24, padding: 16, border: '1px solid #ddd' }}>
        <h3>Register Service</h3>
        <input placeholder="Name" value={name} onChange={e => setName(e.target.value)} required />
        {' '}
        <input placeholder="slug" value={slug} onChange={e => setSlug(e.target.value)} required />
        {' '}
        <input placeholder="Repo URL" value={repo} onChange={e => setRepo(e.target.value)} style={{ width: 240 }} />
        {' '}
        <button type="submit">Create</button>
      </form>

      <table width="100%" cellPadding={8} style={{ borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ textAlign: 'left', borderBottom: '1px solid #ddd' }}>
            <th>Name</th><th>Slug</th><th>Owner</th><th></th>
          </tr>
        </thead>
        <tbody>
        {(services ?? []).map(s => (            <tr key={s.id} style={{ borderBottom: '1px solid #eee' }}>
              <td>{s.name}</td>
              <td>{s.slug}</td>
              <td>{s.owner_email}</td>
              <td><Link to={`/services/${s.id}`}>View</Link></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}