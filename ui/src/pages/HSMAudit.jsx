import React, { useState, useEffect, useCallback } from 'react';
import { secretsFetch } from '../secretsApi';

export default function HSMAudit() {
  const [entries, setEntries] = useState([]);
  const [endpoints, setEndpoints] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [filterEndpoint, setFilterEndpoint] = useState('');
  const [limit, setLimit] = useState(100);

  const load = useCallback(async () => {
    try {
      let url = `/hsm/v1/admin/audit?limit=${limit}`;
      if (filterEndpoint) url += `&endpointId=${filterEndpoint}`;
      const [data, eps] = await Promise.all([
        secretsFetch(url),
        secretsFetch('/hsm/v1/endpoints'),
      ]);
      setEntries(data || []);
      setEndpoints(eps || []);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load audit log' });
    } finally {
      setLoading(false);
    }
  }, [filterEndpoint, limit]);

  useEffect(() => { load(); }, [load]);

  const respCodeBadge = (code) => {
    if (!code) return <span className="badge badge-muted">—</span>;
    if (code === '00') return <span className="badge badge-green">00 OK</span>;
    if (code === '01') return <span className="badge badge-yellow">01 Verify Fail</span>;
    return <span className="badge badge-red">{code}</span>;
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <>
      <div className="page-header">
        <h1>HSM Audit Log</h1>
        <p>Command history and audit trail for all HSM gateway operations</p>
      </div>

      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">Audit Entries ({entries.length})</span>
          <div style={{display:'flex', gap:8, alignItems:'center'}}>
            <select className="filter-select" value={filterEndpoint} onChange={e => setFilterEndpoint(e.target.value)} style={{width:180}}>
              <option value="">All Endpoints</option>
              {endpoints.map(ep => <option key={ep.id} value={ep.id}>{ep.name}</option>)}
            </select>
            <select className="filter-select" value={limit} onChange={e => setLimit(Number(e.target.value))} style={{width:100}}>
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={500}>500</option>
            </select>
            <button className="btn btn-secondary btn-small" onClick={load}>Refresh</button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Time</th><th>Endpoint</th><th>Command</th><th>Operation</th>
                <th>Response</th><th>Duration</th><th>Size</th><th>User</th>
              </tr>
            </thead>
            <tbody>
              {entries.map(e => {
                const ep = endpoints.find(ep => ep.id === e.endpointId);
                return (
                  <tr key={e.id}>
                    <td style={{fontSize:11,fontFamily:'monospace',whiteSpace:'nowrap'}}>{new Date(e.createdAt).toLocaleString()}</td>
                    <td>{ep ? <span className="badge badge-cyan" style={{fontSize:10}}>{ep.name}</span> : (e.endpointId || '—')}</td>
                    <td><span className="badge badge-muted" style={{fontFamily:'monospace'}}>{e.commandCode}</span></td>
                    <td style={{fontSize:12}}>{e.apiOperation}</td>
                    <td>{respCodeBadge(e.responseCode)}</td>
                    <td style={{fontSize:12}}>{e.durationMs}ms</td>
                    <td style={{fontSize:11,color:'var(--text-muted)'}}>{e.requestSize}→{e.responseSize}B</td>
                    <td style={{fontSize:12}}>{e.userId || '—'}</td>
                  </tr>
                );
              })}
              {entries.length === 0 && <tr><td colSpan={8} style={{textAlign:'center',padding:32,color:'var(--text-muted)'}}>No audit entries</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}
