import React, { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../AuthContext';
import { secretsFetch } from '../secretsApi';

const PROVIDER_TYPES = ['internal', 'hsm', 'ca'];

const HSM_PRESETS = {
  'payshield-10k': {
    label: 'Thales payShield 10000',
    config: {
      model: 'Thales payShield 10000',
      preset: 'payshield-10k',
      protocol: 'TCP',
      port: 1500,
      headerLength: 4,
      firmwareVersion: '1.4A0000',
      hostCommandTimeout: 15,
      connectionTimeout: 10,
      maxConnections: 16,
      lmkIdentifier: 0,
      slotId: 0,
      libraryPath: '/opt/thales/payshield10k/lib/libethsm.so',
      pin: '',
      address: '',
    },
  },
  'payshield-9k': {
    label: 'Thales payShield 9000',
    config: {
      model: 'Thales payShield 9000',
      preset: 'payshield-9k',
      protocol: 'TCP',
      port: 1500,
      headerLength: 2,
      firmwareVersion: '3.20',
      hostCommandTimeout: 30,
      connectionTimeout: 15,
      maxConnections: 8,
      lmkIdentifier: 0,
      slotId: 0,
      libraryPath: '/opt/thales/payshield9k/lib/libethsm.so',
      pin: '',
      address: '',
    },
  },
  custom: {
    label: 'Custom HSM',
    config: {
      model: '',
      preset: 'custom',
      protocol: 'TCP',
      port: 1500,
      headerLength: 4,
      firmwareVersion: '',
      hostCommandTimeout: 15,
      connectionTimeout: 10,
      maxConnections: 8,
      lmkIdentifier: 0,
      slotId: 0,
      libraryPath: '',
      pin: '',
      address: '',
    },
  },
};

export default function SecurityProviders() {
  const { canDo } = useAuth();
  const [providers, setProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState(null);

  const load = useCallback(async () => {
    try {
      const data = await secretsFetch('/providers');
      setProviders(data || []);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load providers' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (p) => {
    if (!confirm(`Delete provider "${p.name}"?`)) return;
    try {
      await secretsFetch(`/providers/${p.id}`, 'DELETE');
      setMsg({ type: 'success', text: 'Provider deleted' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: e.message || 'Failed to delete provider' });
    }
  };

  const handleTest = async (p) => {
    try {
      const result = await secretsFetch(`/providers/${p.id}/test`, 'POST');
      if (result.success) {
        setMsg({ type: 'success', text: `Connection test passed for "${p.name}"` });
      } else {
        setMsg({ type: 'error', text: `Connection test failed: ${result.error}` });
      }
    } catch (e) {
      setMsg({ type: 'error', text: 'Test request failed' });
    }
  };

  const typeBadge = (type) => {
    const cls = { internal: 'badge-green', hsm: 'badge-cyan', ca: 'badge-orange' };
    return <span className={`badge ${cls[type] || 'badge-muted'}`}>{type.toUpperCase()}</span>;
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <>
      <div className="page-header">
        <h1>Security Providers</h1>
        <p>Manage cryptographic backends: Internal, HSM, and Certificate Authority providers</p>
      </div>

      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">Providers ({providers.length})</span>
          {canDo('WriteProviders') && <button className="btn btn-primary btn-small" onClick={() => { setEditing(null); setShowForm(true); }}>Add Provider</button>}
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th><th>Type</th><th>Status</th><th>Keys</th><th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {providers.map(p => (
                <tr key={p.id}>
                  <td><strong>{p.name}</strong></td>
                  <td>{typeBadge(p.providerType)}</td>
                  <td><span className={`badge ${p.active ? 'badge-green' : 'badge-muted'}`}>{p.active ? 'Active' : 'Inactive'}</span></td>
                  <td>{p.keysCount}</td>
                  <td>
                    <div style={{display:'flex', gap:4}}>
                      {canDo('WriteProviders') && p.providerType !== 'internal' && <button className="btn btn-secondary btn-small" onClick={() => handleTest(p)}>Test</button>}
                      {canDo('WriteProviders') && <button className="btn btn-secondary btn-small" onClick={() => { setEditing(p); setShowForm(true); }}>Edit</button>}
                      {p.providerType === 'hsm' && (
                        <button className="btn btn-secondary btn-small" onClick={() => window.location.href = '#/hsm/endpoints'} title="View linked HSM endpoint">
                          Endpoint
                        </button>
                      )}
                      {canDo('WriteProviders') && p.providerType !== 'internal' && <button className="btn btn-danger btn-small" onClick={() => handleDelete(p)}>Delete</button>}
                    </div>
                  </td>
                </tr>
              ))}
              {providers.length === 0 && <tr><td colSpan={5} style={{textAlign:'center',padding:32,color:'var(--text-muted)'}}>No providers</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {showForm && <ProviderForm provider={editing} onClose={() => setShowForm(false)} onDone={() => { setShowForm(false); load(); setMsg({type:'success', text: editing ? 'Provider updated' : 'Provider created'}); }} />}
    </>
  );
}

function ProviderForm({ provider, onClose, onDone }) {
  const [type, setType] = useState(provider?.providerType || 'internal');
  const [name, setName] = useState(provider?.name || '');
  const [active, setActive] = useState(provider?.active ?? true);
  const [config, setConfig] = useState(() => {
    if (provider?.config) {
      try { return typeof provider.config === 'string' ? JSON.parse(provider.config) : provider.config; }
      catch { return {}; }
    }
    return {};
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const submit = async () => {
    setSaving(true);
    try {
      const body = { name, providerType: type, config, active };
      if (provider) {
        await secretsFetch(`/providers/${provider.id}`, 'PUT', body);
      } else {
        await secretsFetch('/providers', 'POST', body);
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
        <h3 style={{ marginBottom: 16 }}>{provider ? 'Edit' : 'Add'} Provider</h3>
        {!provider && type === 'hsm' && (
          <div style={{ background:'var(--cyan-dim)', color:'var(--cyan)', padding:'8px 12px', borderRadius:'var(--radius)', fontSize:12, marginBottom:12 }}>
            A matching HSM Endpoint will be auto-created with this provider's configuration
          </div>
        )}
        {error && <div className="admin-msg admin-msg-error" style={{marginBottom:12}}>{error}</div>}
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Name</label>
            <input className="filter-input" value={name} onChange={e => setName(e.target.value)} placeholder="Provider name" />
          </div>
          {!provider && (
            <div className="form-field">
              <label className="form-label">Type</label>
              <select className="filter-select" value={type} onChange={e => { setType(e.target.value); setConfig({}); }}>
                {PROVIDER_TYPES.map(t => <option key={t} value={t}>{t.toUpperCase()}</option>)}
              </select>
            </div>
          )}
          <div className="form-field">
            <label className="form-label">Active</label>
            <div className="admin-toggle-wrap">
              <button type="button" className={`admin-toggle ${active ? 'on' : ''}`} onClick={() => setActive(!active)}>
                <span className="admin-toggle-knob" />
              </button>
              <span style={{fontSize:13, color:'var(--text-secondary)'}}>{active ? 'Active' : 'Inactive'}</span>
            </div>
          </div>
        </div>

        {type === 'hsm' && (
          <div style={{marginTop:16}}>
            <div style={{fontSize:12, fontWeight:600, color:'var(--text-secondary)', textTransform:'uppercase', letterSpacing:'0.04em', marginBottom:12, paddingBottom:8, borderBottom:'1px solid var(--border)'}}>HSM Configuration</div>

            {/* Preset selector */}
            {!provider && (
              <div style={{display:'flex', gap:8, marginBottom:16}}>
                {Object.entries(HSM_PRESETS).map(([key, spec]) => {
                  const selected = (config.preset || 'custom') === key;
                  return (
                    <button
                      key={key}
                      type="button"
                      className={`btn btn-small ${selected ? 'btn-primary' : 'btn-secondary'}`}
                      onClick={() => {
                        setConfig({...spec.config});
                        if (!name && key !== 'custom') setName(spec.label);
                      }}
                    >
                      {spec.label}
                    </button>
                  );
                })}
              </div>
            )}

            {/* Spec info banner for presets */}
            {config.preset && config.preset !== 'custom' && (
              <div style={{
                background:'var(--cyan-dim)', color:'var(--cyan)',
                padding:'10px 14px', borderRadius:'var(--radius)',
                fontSize:12, marginBottom:16, lineHeight:1.6,
              }}>
                <strong>{HSM_PRESETS[config.preset]?.label}</strong>
                {config.preset === 'payshield-10k' && (
                  <span> — Next-gen payment HSM. AES-256, RSA up to 4096-bit, 3DES. EFTSec Ethernet protocol, 4-byte header. Up to 10,000 RSA 2048 sign/sec. Dual-LMK, FIPS 140-2 L3 / PCI HSM v3.</span>
                )}
                {config.preset === 'payshield-9k' && (
                  <span> — Legacy payment HSM. 3DES, AES-256, RSA up to 4096-bit. 2-byte header. Up to 1,500 RSA 2048 sign/sec. Single LMK, FIPS 140-2 L3 / PCI HSM v2.</span>
                )}
              </div>
            )}

            <div className="form-grid">
              <div className="form-field">
                <label className="form-label">Model</label>
                <input className="filter-input" value={config.model || ''} onChange={e => setConfig({...config, model:e.target.value})}
                  placeholder="Thales payShield 10000" readOnly={config.preset && config.preset !== 'custom'} />
              </div>
              <div className="form-field">
                <label className="form-label">Address</label>
                <input className="filter-input" value={config.address || ''} onChange={e => setConfig({...config, address:e.target.value})} placeholder="10.0.1.50" />
              </div>
              <div className="form-field">
                <label className="form-label">Port</label>
                <input className="filter-input" type="number" value={config.port || 1500} onChange={e => setConfig({...config, port:Number(e.target.value)})} />
              </div>
              <div className="form-field">
                <label className="form-label">Protocol</label>
                <select className="filter-select" value={config.protocol || 'TCP'} onChange={e => setConfig({...config, protocol:e.target.value})}>
                  <option value="TCP">TCP</option><option value="TLS">TLS</option><option value="UDP">UDP</option>
                </select>
              </div>
              <div className="form-field">
                <label className="form-label">Header Length (bytes)</label>
                <input className="filter-input" type="number" value={config.headerLength ?? 4} onChange={e => setConfig({...config, headerLength:Number(e.target.value)})} />
                <span style={{fontSize:11, color:'var(--text-muted)'}}>
                  {config.preset === 'payshield-10k' ? 'PS10K uses 4-byte headers' : config.preset === 'payshield-9k' ? 'PS9K uses 2-byte headers' : 'Message header length'}
                </span>
              </div>
              <div className="form-field">
                <label className="form-label">Firmware Version</label>
                <input className="filter-input" value={config.firmwareVersion || ''} onChange={e => setConfig({...config, firmwareVersion:e.target.value})}
                  placeholder={config.preset === 'payshield-10k' ? '1.4A0000' : config.preset === 'payshield-9k' ? '3.20' : 'e.g. 1.0'} />
              </div>
            </div>

            <div style={{fontSize:12, fontWeight:600, color:'var(--text-secondary)', textTransform:'uppercase', letterSpacing:'0.04em', marginTop:20, marginBottom:12, paddingBottom:8, borderBottom:'1px solid var(--border)'}}>Connection Settings</div>
            <div className="form-grid">
              <div className="form-field">
                <label className="form-label">Host Command Timeout (sec)</label>
                <input className="filter-input" type="number" value={config.hostCommandTimeout ?? 15} onChange={e => setConfig({...config, hostCommandTimeout:Number(e.target.value)})} />
              </div>
              <div className="form-field">
                <label className="form-label">Connection Timeout (sec)</label>
                <input className="filter-input" type="number" value={config.connectionTimeout ?? 10} onChange={e => setConfig({...config, connectionTimeout:Number(e.target.value)})} />
              </div>
              <div className="form-field">
                <label className="form-label">Max Connections</label>
                <input className="filter-input" type="number" value={config.maxConnections ?? 8} onChange={e => setConfig({...config, maxConnections:Number(e.target.value)})} />
              </div>
              <div className="form-field">
                <label className="form-label">LMK Identifier</label>
                <input className="filter-input" type="number" value={config.lmkIdentifier ?? 0} onChange={e => setConfig({...config, lmkIdentifier:Number(e.target.value)})} />
                <span style={{fontSize:11, color:'var(--text-muted)'}}>LMK set index (0-based)</span>
              </div>
            </div>

            <div style={{fontSize:12, fontWeight:600, color:'var(--text-secondary)', textTransform:'uppercase', letterSpacing:'0.04em', marginTop:20, marginBottom:12, paddingBottom:8, borderBottom:'1px solid var(--border)'}}>PKCS#11 Settings</div>
            <div className="form-grid">
              <div className="form-field">
                <label className="form-label">Library Path</label>
                <input className="filter-input" value={config.libraryPath || ''} onChange={e => setConfig({...config, libraryPath:e.target.value})}
                  placeholder="/opt/thales/lib/libethsm.so" />
              </div>
              <div className="form-field">
                <label className="form-label">Slot ID</label>
                <input className="filter-input" type="number" value={config.slotId || 0} onChange={e => setConfig({...config, slotId:Number(e.target.value)})} />
              </div>
              <div className="form-field">
                <label className="form-label">PIN</label>
                <input className="filter-input" type="password" value={config.pin || ''} onChange={e => setConfig({...config, pin:e.target.value})} />
              </div>
            </div>
          </div>
        )}

        {type === 'ca' && (
          <div style={{marginTop:16}}>
            <div style={{fontSize:12, fontWeight:600, color:'var(--text-secondary)', textTransform:'uppercase', letterSpacing:'0.04em', marginBottom:12, paddingBottom:8, borderBottom:'1px solid var(--border)'}}>CA Configuration</div>
            <div className="form-grid">
              <div className="form-field">
                <label className="form-label">Vendor</label>
                <input className="filter-input" value={config.vendor || ''} onChange={e => setConfig({...config, vendor:e.target.value})} placeholder="DigiCert" />
              </div>
              <div className="form-field">
                <label className="form-label">API URL</label>
                <input className="filter-input" value={config.apiUrl || ''} onChange={e => setConfig({...config, apiUrl:e.target.value})} placeholder="https://api.digicert.com" />
              </div>
              <div className="form-field">
                <label className="form-label">API Key</label>
                <input className="filter-input" type="password" value={config.apiKey || ''} onChange={e => setConfig({...config, apiKey:e.target.value})} />
              </div>
            </div>
          </div>
        )}

        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit} disabled={saving || !name}>{saving ? 'Saving...' : 'Save'}</button>
        </div>
      </div>
    </div>
  );
}
