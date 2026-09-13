import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  createService,
  createServiceFromTemplate,
  listServices,
} from '../api/client';
import type { Service } from '../api/client';

const DEFAULT_TEAM_ID = '22222222-2222-2222-2222-222222222222';

export default function ServicesPage() {
  const [services, setServices] = useState<Service[]>([]);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [repo, setRepo] = useState('');

  async function load() {
    try {
      setServices(await listServices());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load');
    }
  }

  useEffect(() => {
    load();
  }, []);

  function clearForm() {
    setName('');
    setSlug('');
    setDescription('');
    setRepo('');
  }

  function buildInput() {
    return {
      team_id: DEFAULT_TEAM_ID,
      name,
      slug,
      description,
      repository_url: repo,
      owner_email: 'admin@nimbus.local',
    };
  }

  async function handleRegisterOnly(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setSuccess('');

    try {
      await createService(buildInput());
      clearForm();
      setSuccess('Service registered in catalog.');
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed');
    }
  }

  async function handleCreateFromTemplate(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setSuccess('');

    try {
      const result = await createServiceFromTemplate({
        template_id: 'go-api',
        ...buildInput(),
      });

      clearForm();
      setSuccess(
        `Golden path created at ${result.generated_path} (${result.generated_files.length} files).`
      );
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Template create failed');
    }
  }

  return (
    <div>
      <h2>Service Catalog</h2>

      {error && <p style={{ color: 'red' }}>{error}</p>}
      {success && <p style={{ color: 'green' }}>{success}</p>}

      <form
        style={{ marginBottom: 24, padding: 16, border: '1px solid #ddd' }}
        onSubmit={(e) => e.preventDefault()}
      >
        <h3>Create Service</h3>
        <p style={{ marginTop: 0, color: '#555' }}>
          <strong>Golden path</strong> generates Dockerfile, Helm, Kustomize, CI stub.
          <strong> Catalog only</strong> creates the metadata row.
        </p>

        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 12 }}>
          <input
            placeholder="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
          <input
            placeholder="slug"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            required
          />
          <input
            placeholder="Description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            style={{ width: 200 }}
          />
          <input
            placeholder="Repo URL"
            value={repo}
            onChange={(e) => setRepo(e.target.value)}
            style={{ width: 260 }}
          />
        </div>

        <div style={{ display: 'flex', gap: 8 }}>
          <button type="button" onClick={handleCreateFromTemplate}>
            Create from golden path (go-api)
          </button>
          <button type="button" onClick={handleRegisterOnly}>
            Register in catalog only
          </button>
        </div>
      </form>

      <table width="100%" cellPadding={8} style={{ borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ textAlign: 'left', borderBottom: '1px solid #ddd' }}>
            <th>Name</th>
            <th>Slug</th>
            <th>Owner</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {(services ?? []).map((s) => (
            <tr key={s.id} style={{ borderBottom: '1px solid #eee' }}>
              <td>{s.name}</td>
              <td>{s.slug}</td>
              <td>{s.owner_email}</td>
              <td>
                <Link to={`/services/${s.id}`}>View</Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}