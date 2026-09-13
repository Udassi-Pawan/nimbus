import { useEffect, useState } from 'react';
import { listAuditLogs } from '../api/client';
import type { AuditLog } from '../api/client';

export default function AuditPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);

  useEffect(() => {
    listAuditLogs().then(setLogs).catch(console.error);
  }, []);

  return (
    <div>
      <h2>Audit Log</h2>
      <table width="100%" cellPadding={8} style={{ borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ textAlign: 'left' }}>
            <th>Time</th><th>Action</th><th>Resource</th><th>ID</th>
          </tr>
        </thead>
        <tbody>
          {logs.map(log => (
            <tr key={log.id} style={{ borderBottom: '1px solid #eee' }}>
              <td>{new Date(log.created_at).toLocaleString()}</td>
              <td>{log.action}</td>
              <td>{log.resource_type}</td>
              <td>{log.resource_id}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}