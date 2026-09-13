import { Link, Outlet, useNavigate } from 'react-router-dom';
import { clearToken } from '../api/client';

export default function Layout() {
  const navigate = useNavigate();

  function logout() {
    clearToken();
    navigate('/login');
  }

  return (
    <div style={{ fontFamily: 'system-ui', maxWidth: 960, margin: '0 auto', padding: 24 }}>
      <header style={{ display: 'flex', gap: 16, marginBottom: 24, borderBottom: '1px solid #ddd', paddingBottom: 12 }}>
        <strong>Nimbus</strong>
        <Link to="/services">Services</Link>
        <Link to="/audit">Audit</Link>
        <button onClick={logout} style={{ marginLeft: 'auto' }}>Logout</button>
      </header>
      <Outlet />
    </div>
  );
}