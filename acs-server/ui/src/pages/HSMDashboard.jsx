import React, { useState, useEffect, useCallback } from 'react';
import { secretsFetch } from '../secretsApi';
import { useNavigate } from 'react-router-dom';

const STATUS_BADGE = {
  online: 'badge-green',
  offline: 'badge-red',
  degraded: 'badge-yellow',
  draining: 'badge-orange',
};

export default function HSMDashboard() {
  const [endpoints, setEndpoints] = useState([]);
  const [sessions, setSessions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const navigate = useNavigate();

  const load = useCallback(async () => {
    try {
      const [eps, sess] = await Promise.all([
        secretsFetch('/hsm/v1/endpoints'),
        secretsFetch('/hsm/v1/sessions'),
      ]);
      setEndpoints(eps || []);
      setSessions(sess || []);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load HSM data' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const onlineCount = endpoints.filter(e => e.status === 'online').length;
  const degradedCount = endpoints.filter(e => e.status === 'degraded').length;
  const activeSessions = sessions.filter(s => s.status === 'active').length;

  const handleAction = async (id, action) => {
    try {
      await secretsFetch(`/hsm/v1/endpoints/${id}/${action}`, 'POST');
      setMsg({ type: 'success', text: `Endpoint ${action} successful` });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: `Failed to ${action} endpoint` });
    }
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <>
      <div className="page-header">
        <h1>HSM Gateway</h1>
        <p>Hardware Security Module gateway dashboard — monitor endpoints, sessions, and routing</p>
      </div>

      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      {/* Summary cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 16, marginBottom: 24 }}>
        <div className="table-card" style={{ padding: 20, textAlign: 'center', cursor: 'pointer' }} onClick={() => navigate('/hsm/endpoints')}>
          <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--green)' }}>{onlineCount}</div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 4 }}>Online Endpoints</div>
          <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>{endpoints.length} total</div>
        </div>
        <div className="table-card" style={{ padding: 20, textAlign: 'center' }}>
          <div style={{ fontSize: 28, fontWeight: 700, color: degradedCount > 0 ? 'var(--yellow)' : 'var(--text-muted)' }}>{degradedCount}</div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 4 }}>Degraded</div>
        </div>
        <div className="table-card" style={{ padding: 20, textAlign: 'center', cursor: 'pointer' }} onClick={() => navigate('/hsm/sessions')}>
          <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--cyan)' }}>{activeSessions}</div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 4 }}>Active Sessions</div>
        </div>
        <div className="table-card" style={{ padding: 20, textAlign: 'center', cursor: 'pointer' }} onClick={() => navigate('/hsm/audit')}>
          <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--text-secondary)' }}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" style={{width:28,height:28}}>
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
              <polyline points="14 2 14 8 20 8"/>
            </svg>
          </div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 4 }}>Audit Log</div>
        </div>
      </div>

      {/* Endpoints table */}
      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">HSM Endpoints</span>
          <button className="btn btn-primary btn-small" onClick={() => navigate('/hsm/endpoints')}>Manage Endpoints</button>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th><th>Model</th><th>Address</th><th>Status</th>
                <th>Pool</th><th>Weight</th><th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {endpoints.map(ep => (
                <tr key={ep.id}>
                  <td><strong>{ep.name}</strong></td>
                  <td><span className="badge badge-cyan">{ep.model}</span></td>
                  <td style={{fontFamily:'monospace',fontSize:12}}>{ep.address}:{ep.port}</td>
                  <td><span className={`badge ${STATUS_BADGE[ep.status] || 'badge-muted'}`}>{ep.status}</span></td>
                  <td style={{fontSize:12}}>
                    {ep.pool ? `${ep.pool.active}/${ep.pool.total} active` : '—'}
                  </td>
                  <td>{ep.weight}</td>
                  <td>
                    <div style={{display:'flex', gap:4}}>
                      {ep.status === 'offline' && <button className="btn btn-secondary btn-small" onClick={() => handleAction(ep.id, 'reconnect')}>Reconnect</button>}
                      {(ep.status === 'online' || ep.status === 'degraded') && <button className="btn btn-secondary btn-small" onClick={() => handleAction(ep.id, 'drain')}>Drain</button>}
                      {ep.status === 'draining' && <button className="btn btn-secondary btn-small" onClick={() => handleAction(ep.id, 'resume')}>Resume</button>}
                    </div>
                  </td>
                </tr>
              ))}
              {endpoints.length === 0 && (
                <tr><td colSpan={7} style={{textAlign:'center',padding:32,color:'var(--text-muted)'}}>
                  No HSM endpoints configured. <button className="btn btn-primary btn-small" style={{marginLeft:8}} onClick={() => navigate('/hsm/endpoints')}>Add Endpoint</button>
                </td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}
