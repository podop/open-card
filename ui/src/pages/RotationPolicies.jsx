import React, { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../AuthContext';
import { secretsFetch } from '../secretsApi';

const KEY_TYPES = ['RSA-1024','RSA-2048','RSA-3072','RSA-4096','AES-128','AES-192','AES-256','3DES','X509','*'];

export default function RotationPolicies() {
  const { canDo } = useAuth();
  const [policies, setPolicies] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState(null);

  const load = useCallback(async () => {
    try {
      const data = await secretsFetch('/rotation-policies');
      setPolicies(data || []);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load policies' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (p) => {
    if (!confirm(`Delete policy "${p.name}"?`)) return;
    try {
      await secretsFetch(`/rotation-policies/${p.id}`, 'DELETE');
      setMsg({ type: 'success', text: 'Policy deleted' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to delete policy' });
    }
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <>
      <div className="page-header">
        <h1>Rotation Policies</h1>
        <p>Define key rotation schedules and auto-rotation rules</p>
      </div>

      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">Policies ({policies.length})</span>
          {canDo('WriteRotationPolicies') && <button className="btn btn-primary btn-small" onClick={() => { setEditing(null); setShowForm(true); }}>Create Policy</button>}
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th><th>Key Type</th><th>Interval</th><th>Notify Before</th>
                <th>Auto-Rotate</th><th>Template</th><th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {policies.map(p => (
                <tr key={p.id}>
                  <td><strong>{p.name}</strong>{p.description && <><br/><span style={{color:'var(--text-muted)', fontSize:11}}>{p.description}</span></>}</td>
                  <td><span className="badge badge-cyan">{p.keyType}</span></td>
                  <td>{p.rotationInterval}</td>
                  <td>{p.notifyBefore}</td>
                  <td><span className={`badge ${p.autoRotate ? 'badge-green' : 'badge-muted'}`}>{p.autoRotate ? 'Yes' : 'No'}</span></td>
                  <td>{p.isTemplate ? <span className="badge badge-orange">Template</span> : '—'}</td>
                  <td>
                    <div style={{display:'flex', gap:4}}>
                      {canDo('WriteRotationPolicies') && <button className="btn btn-secondary btn-small" onClick={() => { setEditing(p); setShowForm(true); }}>Edit</button>}
                      {canDo('WriteRotationPolicies') && <button className="btn btn-danger btn-small" onClick={() => handleDelete(p)}>Delete</button>}
                    </div>
                  </td>
                </tr>
              ))}
              {policies.length === 0 && <tr><td colSpan={7} style={{textAlign:'center',padding:32,color:'var(--text-muted)'}}>No policies</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {showForm && <PolicyForm policy={editing} onClose={() => setShowForm(false)} onDone={() => { setShowForm(false); load(); setMsg({type:'success', text: editing ? 'Policy updated' : 'Policy created'}); }} />}
    </>
  );
}

function PolicyForm({ policy, onClose, onDone }) {
  const [form, setForm] = useState({
    name: policy?.name || '',
    description: policy?.description || '',
    keyType: policy?.keyType || 'RSA-2048',
    rotationInterval: policy?.rotationInterval || '1 year',
    notifyBefore: policy?.notifyBefore || '30 days',
    autoRotate: policy?.autoRotate || false,
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const submit = async () => {
    setSaving(true);
    try {
      if (policy) {
        await secretsFetch(`/rotation-policies/${policy.id}`, 'PUT', form);
      } else {
        await secretsFetch('/rotation-policies', 'POST', form);
      }
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
        <h3 style={{ marginBottom: 16 }}>{policy ? 'Edit' : 'Create'} Rotation Policy</h3>
        {error && <div className="admin-msg admin-msg-error" style={{marginBottom:12}}>{error}</div>}
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Name</label>
            <input className="filter-input" value={form.name} onChange={e => setForm({...form, name:e.target.value})} placeholder="Policy name" />
          </div>
          <div className="form-field">
            <label className="form-label">Description</label>
            <input className="filter-input" value={form.description} onChange={e => setForm({...form, description:e.target.value})} />
          </div>
          <div className="form-field">
            <label className="form-label">Key Type</label>
            <select className="filter-select" value={form.keyType} onChange={e => setForm({...form, keyType:e.target.value})}>
              {KEY_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Rotation Interval</label>
            <input className="filter-input" value={form.rotationInterval} onChange={e => setForm({...form, rotationInterval:e.target.value})} placeholder="1 year" />
          </div>
          <div className="form-field">
            <label className="form-label">Notify Before</label>
            <input className="filter-input" value={form.notifyBefore} onChange={e => setForm({...form, notifyBefore:e.target.value})} placeholder="30 days" />
          </div>
          <div className="form-field">
            <label className="form-label">Auto-Rotate</label>
            <div className="admin-toggle-wrap">
              <button type="button" className={`admin-toggle ${form.autoRotate ? 'on' : ''}`} onClick={() => setForm({...form, autoRotate:!form.autoRotate})}>
                <span className="admin-toggle-knob" />
              </button>
              <span style={{fontSize:13, color:'var(--text-secondary)'}}>{form.autoRotate ? 'Enabled' : 'Disabled'}</span>
            </div>
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit} disabled={saving || !form.name}>{saving ? 'Saving...' : 'Save'}</button>
        </div>
      </div>
    </div>
  );
}
