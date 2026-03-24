import React, { useState, useEffect, useCallback } from 'react';
import { secretsFetch } from '../secretsApi';

const STRATEGIES = ['round-robin', 'capability', 'weighted', 'sticky'];

export default function HSMSessions() {
  const [sessions, setSessions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [showForm, setShowForm] = useState(false);

  const load = useCallback(async () => {
    try {
      const data = await secretsFetch('/hsm/v1/sessions');
      setSessions(data || []);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load sessions' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleClose = async (id) => {
    try {
      await secretsFetch(`/hsm/v1/sessions/${id}`, 'DELETE');
      setMsg({ type: 'success', text: 'Session closed' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to close session' });
    }
  };

  const handlePing = async (id) => {
    try {
      await secretsFetch(`/hsm/v1/sessions/${id}/ping`, 'POST');
      setMsg({ type: 'success', text: 'Session pinged' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: 'Ping failed' });
    }
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <>
      <div className="page-header">
        <h1>HSM Sessions</h1>
        <p>Manage gateway sessions for sticky routing and audit grouping</p>
      </div>

      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">Sessions ({sessions.length})</span>
          <button className="btn btn-primary btn-small" onClick={() => setShowForm(true)}>Create Session</button>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th><th>Client</th><th>Strategy</th><th>Status</th>
                <th>Last Ping</th><th>TTL</th><th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map(s => (
                <tr key={s.id}>
                  <td style={{fontFamily:'monospace',fontSize:11}}>{s.id.substring(0,8)}...</td>
                  <td>{s.clientLabel || '—'}</td>
                  <td><span className="badge badge-cyan">{s.strategy}</span></td>
                  <td><span className={`badge ${s.status==='active' ? 'badge-green' : s.status==='idle' ? 'badge-yellow' : 'badge-muted'}`}>{s.status}</span></td>
                  <td style={{fontSize:12}}>{new Date(s.lastPing).toLocaleTimeString()}</td>
                  <td>{s.ttlSeconds}s</td>
                  <td>
                    <div style={{display:'flex',gap:4}}>
                      {s.status === 'active' && <button className="btn btn-secondary btn-small" onClick={() => handlePing(s.id)}>Ping</button>}
                      {s.status !== 'closed' && <button className="btn btn-danger btn-small" onClick={() => handleClose(s.id)}>Close</button>}
                    </div>
                  </td>
                </tr>
              ))}
              {sessions.length === 0 && <tr><td colSpan={7} style={{textAlign:'center',padding:32,color:'var(--text-muted)'}}>No active sessions</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {showForm && <SessionForm onClose={() => setShowForm(false)} onDone={() => { setShowForm(false); load(); setMsg({type:'success', text:'Session created'}); }} />}
    </>
  );
}

function SessionForm({ onClose, onDone }) {
  const [form, setForm] = useState({ clientLabel: '', strategy: 'round-robin', ttlSeconds: 300 });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const submit = async () => {
    setSaving(true);
    try {
      await secretsFetch('/hsm/v1/sessions', 'POST', form);
      onDone();
    } catch (e) {
      setError(e.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()}>
        <h3 style={{ marginBottom: 16 }}>Create Session</h3>
        {error && <div className="admin-msg admin-msg-error" style={{marginBottom:12}}>{error}</div>}
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Client Label</label>
            <input className="filter-input" value={form.clientLabel} onChange={e => setForm({...form, clientLabel:e.target.value})} placeholder="my-service" />
          </div>
          <div className="form-field">
            <label className="form-label">Strategy</label>
            <select className="filter-select" value={form.strategy} onChange={e => setForm({...form, strategy:e.target.value})}>
              {STRATEGIES.map(s => <option key={s} value={s}>{s}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">TTL (seconds)</label>
            <input className="filter-input" type="number" value={form.ttlSeconds} onChange={e => setForm({...form, ttlSeconds:Number(e.target.value)})} />
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit} disabled={saving}>{saving ? 'Creating...' : 'Create'}</button>
        </div>
      </div>
    </div>
  );
}
