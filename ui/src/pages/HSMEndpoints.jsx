import React, { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../AuthContext';
import { secretsFetch } from '../secretsApi';

const STATUS_BADGE = { online:'badge-green', offline:'badge-red', degraded:'badge-yellow', draining:'badge-orange' };
const CAPABILITIES = ['keys','pin','emv','cards','p2pe','rkl','tokens','admin'];

const HSM_PRESETS = {
  'payshield-10k-emulator': {
    label: 'Thales payShield 10K Emulator',
    config: { model:'payshield-10k', protocol:'TCP', port:1500, headerLength:2, firmwareVersion:'0650-E000',
      hostCommandTimeout:30, connectionTimeout:10, maxConnections:64, lmkIdentifier:0 },
    defaultCaps: ['keys','pin','emv','cards','p2pe','rkl','tokens','admin'],
    desc: 'Built-in PayShield 10K emulator. 2-byte header, test LMK, auto-authorized.',
  },
  'payshield-10k': {
    label: 'Thales payShield 10000',
    config: { model:'payshield-10k', protocol:'TCP', port:1500, headerLength:4, firmwareVersion:'1.4A0000',
      hostCommandTimeout:15, connectionTimeout:10, maxConnections:16, lmkIdentifier:0 },
    defaultCaps: ['keys','pin','emv','cards','p2pe','rkl','tokens','admin'],
    desc: 'Next-gen payment HSM. 4-byte header, up to 10k RSA 2048 sign/sec. FIPS 140-2 L3.',
  },
  'payshield-9k': {
    label: 'Thales payShield 9000',
    config: { model:'payshield-9k', protocol:'TCP', port:1500, headerLength:2, firmwareVersion:'3.20',
      hostCommandTimeout:30, connectionTimeout:15, maxConnections:8, lmkIdentifier:0 },
    defaultCaps: ['keys','pin','emv','cards','admin'],
    desc: 'Legacy payment HSM. 2-byte header, up to 1.5k RSA 2048 sign/sec. FIPS 140-2 L3.',
  },
  custom: {
    label: 'Custom HSM',
    config: { model:'custom', protocol:'TCP', port:1500, headerLength:4, firmwareVersion:'',
      hostCommandTimeout:15, connectionTimeout:10, maxConnections:8, lmkIdentifier:0 },
    defaultCaps: ['keys','pin','emv','cards','admin'],
    desc: 'Custom HSM endpoint with manual configuration.',
  },
};

export default function HSMEndpoints() {
  const { canDo } = useAuth();
  const [endpoints, setEndpoints] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState(null);

  const load = useCallback(async () => {
    try {
      const data = await secretsFetch('/hsm/v1/endpoints');
      setEndpoints(data || []);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load endpoints' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleAction = async (id, action) => {
    try {
      await secretsFetch(`/hsm/v1/endpoints/${id}/${action}`, 'POST');
      setMsg({ type: 'success', text: `${action} successful` });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: `Failed: ${e.message}` });
    }
  };

  const handleDelete = async (ep) => {
    if (!confirm(`Delete endpoint "${ep.name}"?`)) return;
    try {
      await secretsFetch(`/hsm/v1/endpoints/${ep.id}`, 'DELETE');
      setMsg({ type: 'success', text: 'Endpoint deleted' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: e.message || 'Failed to delete' });
    }
  };

  const handleHealthCheck = async (ep) => {
    try {
      const result = await secretsFetch(`/hsm/v1/endpoints/${ep.id}/health`);
      if (result.healthy) {
        setMsg({ type: 'success', text: `${ep.name}: healthy (${result.durationMs}ms)` });
      } else {
        setMsg({ type: 'error', text: `${ep.name}: unhealthy — ${result.error || 'check failed'}` });
      }
      load();
    } catch (e) {
      setMsg({ type: 'error', text: `Health check failed: ${e.message}` });
    }
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <>
      <div className="page-header">
        <h1>HSM Endpoints</h1>
        <p>Register and manage Hardware Security Module endpoints for the gateway</p>
      </div>

      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">Endpoints ({endpoints.length})</span>
          {canDo('WriteHSMEndpoints') && <button className="btn btn-primary btn-small" onClick={() => { setEditing(null); setShowForm(true); }}>Add Endpoint</button>}
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th><th>Model</th><th>Address</th><th>Status</th>
                <th>Weight</th><th>Capabilities</th><th>Pool</th><th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {endpoints.map(ep => (
                <tr key={ep.id}>
                  <td><strong>{ep.name}</strong></td>
                  <td><span className="badge badge-cyan">{ep.model}</span></td>
                  <td style={{fontFamily:'monospace',fontSize:12}}>{ep.address}:{ep.port}</td>
                  <td><span className={`badge ${STATUS_BADGE[ep.status]||'badge-muted'}`}>{ep.status}</span></td>
                  <td>{ep.weight}</td>
                  <td>
                    <div style={{display:'flex',gap:3,flexWrap:'wrap'}}>
                      {(ep.capabilities||[]).map(c => <span key={c} className="badge badge-muted" style={{fontSize:10,padding:'1px 6px'}}>{c}</span>)}
                    </div>
                  </td>
                  <td style={{fontSize:12}}>
                    {ep.pool ? `${ep.pool.active}A/${ep.pool.idle}I/${ep.pool.total}T` : '—'}
                  </td>
                  <td>
                    <div style={{display:'flex',gap:4,flexWrap:'wrap'}}>
                      <button className="btn btn-secondary btn-small" onClick={() => handleHealthCheck(ep)}>Health</button>
                      {canDo('WriteHSMEndpoints') && ep.status === 'offline' && <button className="btn btn-secondary btn-small" onClick={() => handleAction(ep.id,'reconnect')}>Reconnect</button>}
                      {canDo('WriteHSMEndpoints') && (ep.status==='online'||ep.status==='degraded') && <button className="btn btn-secondary btn-small" onClick={() => handleAction(ep.id,'drain')}>Drain</button>}
                      {canDo('WriteHSMEndpoints') && ep.status === 'draining' && <button className="btn btn-secondary btn-small" onClick={() => handleAction(ep.id,'resume')}>Resume</button>}
                      {canDo('WriteHSMEndpoints') && <button className="btn btn-secondary btn-small" onClick={() => { setEditing(ep); setShowForm(true); }}>Edit</button>}
                      <button className="btn btn-secondary btn-small" onClick={() => window.location.href = '#/secrets/providers'} title="View linked security provider">Provider</button>
                      {canDo('WriteHSMEndpoints') && <button className="btn btn-danger btn-small" onClick={() => handleDelete(ep)}>Delete</button>}
                    </div>
                  </td>
                </tr>
              ))}
              {endpoints.length === 0 && <tr><td colSpan={8} style={{textAlign:'center',padding:32,color:'var(--text-muted)'}}>No endpoints configured</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {showForm && <EndpointForm endpoint={editing} onClose={() => setShowForm(false)} onDone={() => { setShowForm(false); load(); setMsg({type:'success', text: editing ? 'Endpoint updated' : 'Endpoint created'}); }} />}
    </>
  );
}

function EndpointForm({ endpoint, onClose, onDone }) {
  const [preset, setPreset] = useState(endpoint?.model || 'custom');
  const [form, setForm] = useState(() => {
    if (endpoint) return {
      name: endpoint.name, model: endpoint.model, address: endpoint.address, port: endpoint.port,
      protocol: endpoint.protocol, headerLength: endpoint.headerLength,
      firmwareVersion: endpoint.firmwareVersion, hostCommandTimeout: endpoint.hostCommandTimeout,
      connectionTimeout: endpoint.connectionTimeout, maxConnections: endpoint.maxConnections,
      lmkIdentifier: endpoint.lmkIdentifier, weight: endpoint.weight, priority: endpoint.priority,
      capabilities: endpoint.capabilities || [], enabled: endpoint.enabled,
    };
    return { name:'', address:'', weight:100, priority:0, capabilities:[], enabled:true, ...HSM_PRESETS.custom.config };
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const applyPreset = (key) => {
    const p = HSM_PRESETS[key];
    setPreset(key);
    setForm(f => ({ ...f, ...p.config, capabilities: p.defaultCaps, name: f.name || (key !== 'custom' ? p.label : '') }));
  };

  const toggleCap = (cap) => {
    setForm(f => {
      const caps = f.capabilities.includes(cap) ? f.capabilities.filter(c => c !== cap) : [...f.capabilities, cap];
      return { ...f, capabilities: caps };
    });
  };

  const submit = async () => {
    setSaving(true);
    try {
      const body = { ...form, config: JSON.stringify({}) };
      // The backend expects the flat fields, not nested config
      delete body.config;
      const payload = { ...form, config: {} };
      if (endpoint) {
        await secretsFetch(`/hsm/v1/endpoints/${endpoint.id}`, 'PUT', payload);
      } else {
        await secretsFetch('/hsm/v1/endpoints', 'POST', payload);
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
      <div className="admin-modal" onClick={e => e.stopPropagation()} style={{maxWidth:720}}>
        <h3 style={{ marginBottom: 16 }}>{endpoint ? 'Edit' : 'Add'} HSM Endpoint</h3>
        {!endpoint && (
          <div style={{ background:'var(--cyan-dim)', color:'var(--cyan)', padding:'8px 12px', borderRadius:'var(--radius)', fontSize:12, marginBottom:12 }}>
            A matching Security Provider will be auto-created with this endpoint's configuration
          </div>
        )}
        {error && <div className="admin-msg admin-msg-error" style={{marginBottom:12}}>{error}</div>}

        {/* Preset selector */}
        {!endpoint && (
          <div style={{display:'flex', gap:8, marginBottom:16}}>
            {Object.entries(HSM_PRESETS).map(([key, p]) => (
              <button key={key} type="button" className={`btn btn-small ${preset === key ? 'btn-primary' : 'btn-secondary'}`}
                onClick={() => applyPreset(key)}>{p.label}</button>
            ))}
          </div>
        )}

        {/* Preset info */}
        {preset !== 'custom' && HSM_PRESETS[preset] && (
          <div style={{ background:'var(--cyan-dim)', color:'var(--cyan)', padding:'10px 14px', borderRadius:'var(--radius)', fontSize:12, marginBottom:16, lineHeight:1.6 }}>
            <strong>{HSM_PRESETS[preset].label}</strong> — {HSM_PRESETS[preset].desc}
          </div>
        )}

        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Name</label>
            <input className="filter-input" value={form.name} onChange={e => setForm({...form, name:e.target.value})} placeholder="HSM endpoint name" />
          </div>
          <div className="form-field">
            <label className="form-label">Address</label>
            <input className="filter-input" value={form.address} onChange={e => setForm({...form, address:e.target.value})} placeholder="10.0.1.50" />
          </div>
          <div className="form-field">
            <label className="form-label">Port</label>
            <input className="filter-input" type="number" value={form.port} onChange={e => setForm({...form, port:Number(e.target.value)})} />
          </div>
          <div className="form-field">
            <label className="form-label">Protocol</label>
            <select className="filter-select" value={form.protocol} onChange={e => setForm({...form, protocol:e.target.value})}>
              <option value="TCP">TCP</option><option value="TLS">TLS</option>
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Header Length</label>
            <input className="filter-input" type="number" value={form.headerLength} onChange={e => setForm({...form, headerLength:Number(e.target.value)})} />
          </div>
          <div className="form-field">
            <label className="form-label">Firmware Version</label>
            <input className="filter-input" value={form.firmwareVersion} onChange={e => setForm({...form, firmwareVersion:e.target.value})} />
          </div>
        </div>

        <div style={{fontSize:12, fontWeight:600, color:'var(--text-secondary)', textTransform:'uppercase', letterSpacing:'0.04em', marginTop:20, marginBottom:12, paddingBottom:8, borderBottom:'1px solid var(--border)'}}>Gateway Settings</div>
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Weight (0-1000)</label>
            <input className="filter-input" type="number" value={form.weight} onChange={e => setForm({...form, weight:Number(e.target.value)})} />
            <span style={{fontSize:11,color:'var(--text-muted)'}}>Higher weight = more traffic in weighted mode</span>
          </div>
          <div className="form-field">
            <label className="form-label">Priority</label>
            <input className="filter-input" type="number" value={form.priority} onChange={e => setForm({...form, priority:Number(e.target.value)})} />
            <span style={{fontSize:11,color:'var(--text-muted)'}}>0 = primary, 1+ = fallback</span>
          </div>
          <div className="form-field">
            <label className="form-label">Command Timeout (sec)</label>
            <input className="filter-input" type="number" value={form.hostCommandTimeout} onChange={e => setForm({...form, hostCommandTimeout:Number(e.target.value)})} />
          </div>
          <div className="form-field">
            <label className="form-label">Connection Timeout (sec)</label>
            <input className="filter-input" type="number" value={form.connectionTimeout} onChange={e => setForm({...form, connectionTimeout:Number(e.target.value)})} />
          </div>
          <div className="form-field">
            <label className="form-label">Max Connections</label>
            <input className="filter-input" type="number" value={form.maxConnections} onChange={e => setForm({...form, maxConnections:Number(e.target.value)})} />
          </div>
          <div className="form-field">
            <label className="form-label">LMK Identifier</label>
            <input className="filter-input" type="number" value={form.lmkIdentifier} onChange={e => setForm({...form, lmkIdentifier:Number(e.target.value)})} />
          </div>
        </div>

        <div style={{fontSize:12, fontWeight:600, color:'var(--text-secondary)', textTransform:'uppercase', letterSpacing:'0.04em', marginTop:20, marginBottom:12, paddingBottom:8, borderBottom:'1px solid var(--border)'}}>Capabilities</div>
        <div style={{display:'flex', gap:8, flexWrap:'wrap', marginBottom:16}}>
          {CAPABILITIES.map(cap => (
            <button key={cap} type="button" className={`btn btn-small ${form.capabilities.includes(cap) ? 'btn-primary' : 'btn-secondary'}`}
              onClick={() => toggleCap(cap)}>{cap}</button>
          ))}
        </div>

        <div className="form-field" style={{marginBottom:16}}>
          <label className="form-label">Enabled</label>
          <div className="admin-toggle-wrap">
            <button type="button" className={`admin-toggle ${form.enabled ? 'on' : ''}`} onClick={() => setForm({...form, enabled:!form.enabled})}>
              <span className="admin-toggle-knob" />
            </button>
            <span style={{fontSize:13, color:'var(--text-secondary)'}}>{form.enabled ? 'Active' : 'Disabled'}</span>
          </div>
        </div>

        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit} disabled={saving || !form.name || !form.address}>{saving ? 'Saving...' : 'Save'}</button>
        </div>
      </div>
    </div>
  );
}
