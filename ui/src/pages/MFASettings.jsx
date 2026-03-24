import React, { useState, useEffect, useCallback } from 'react';
import { fetchJSON } from '../api';
import { secretsFetch } from '../secretsApi';

const PROVIDER_TYPES = [
  { value: 'internal', label: 'Internal' },
  { value: 'external', label: 'External' },
];

const FACTOR_TYPES = [
  { value: 'card_pin', label: 'Card PIN' },
  { value: 'authenticator', label: 'Authenticator App' },
  { value: 'duo', label: 'DUO Mobile' },
  { value: 'push', label: 'Push Notification' },
  { value: 'biometric', label: 'Biometric' },
  { value: 'custom', label: 'Custom' },
];

export default function MFASettings() {
  const [providers, setProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editProvider, setEditProvider] = useState(null);

  const load = useCallback(async () => {
    try {
      const data = await fetchJSON('/mfa-providers');
      setProviders(data.data || []);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load MFA providers' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (id, name) => {
    if (!confirm(`Delete MFA provider "${name}"? This cannot be undone.`)) return;
    try {
      await fetchJSON(`/mfa-providers/${id}`, {}, 'DELETE');
      setMsg({ type: 'success', text: `${name} deleted` });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: `Delete failed: ${e.message}` });
    }
  };

  const handleToggle = async (provider) => {
    try {
      await fetchJSON(`/mfa-providers/${provider.id}`, {}, 'PUT', {
        ...provider,
        active: !provider.active,
      });
      setMsg({ type: 'success', text: `${provider.displayName} ${!provider.active ? 'activated' : 'deactivated'}` });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: `Toggle failed: ${e.message}` });
    }
  };

  return (
    <>
      <div className="page-header">
        <h1>MFA Settings</h1>
        <p>Manage multi-factor authentication providers for second factor verification</p>
      </div>

      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      <OTPGenerationSettings />

      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
        <button className="btn btn-primary" onClick={() => { setEditProvider(null); setShowForm(true); }}>
          + Add Provider
        </button>
      </div>

      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">MFA Providers ({providers.length})</span>
        </div>
        <div className="table-wrap">
          {loading ? (
            <div className="spinner-wrap"><div className="spinner" /></div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Type</th>
                  <th>Factor</th>
                  <th>Status</th>
                  <th>Description</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {providers.map(p => (
                  <tr key={p.id}>
                    <td style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
                      {p.displayName}
                      <div style={{ fontSize: 11, color: 'var(--text-muted)', fontWeight: 400 }}>{p.name}</div>
                    </td>
                    <td>
                      <span className={`badge ${p.type === 'internal' ? 'badge-cyan' : 'badge-orange'}`}>
                        {p.type === 'internal' ? 'Internal' : 'External'}
                      </span>
                    </td>
                    <td>
                      <span className="badge badge-muted">
                        {FACTOR_TYPES.find(f => f.value === p.factorType)?.label || p.factorType}
                      </span>
                    </td>
                    <td>
                      <button
                        className={`admin-toggle ${p.active ? 'on' : ''}`}
                        onClick={() => handleToggle(p)}
                        style={{ transform: 'scale(0.8)' }}
                      >
                        <span className="admin-toggle-knob" />
                      </button>
                    </td>
                    <td style={{ fontSize: 12, color: 'var(--text-secondary)', maxWidth: 250 }}>{p.description || '-'}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn btn-small btn-secondary" onClick={() => { setEditProvider(p); setShowForm(true); }}>Edit</button>
                        <button className="btn btn-small btn-danger" onClick={() => handleDelete(p.id, p.displayName)}>Delete</button>
                      </div>
                    </td>
                  </tr>
                ))}
                {providers.length === 0 && (
                  <tr><td colSpan={6} className="empty-state">No MFA providers configured</td></tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </div>

      <div className="table-card" style={{ marginTop: 24 }}>
        <div className="table-toolbar">
          <span className="card-title">How It Works</span>
        </div>
        <div style={{ padding: '16px 20px', fontSize: 13, color: 'var(--text-secondary)', lineHeight: 1.6 }}>
          <p style={{ margin: '0 0 8px' }}>
            <strong>Internal providers</strong> are built into the ACS and handle second factor verification natively (e.g. Card PIN).
          </p>
          <p style={{ margin: '0 0 8px' }}>
            <strong>External providers</strong> integrate with third-party services (e.g. Authenticator apps, DUO Mobile) for additional verification.
          </p>
          <p style={{ margin: '0 0 8px' }}>
            To enable second factor on a flow, go to <strong>Admin &gt; BIN Configuration</strong> and set the <em>Second Factor Mode</em> on network rules or BIN exclusions.
          </p>
          <p style={{ margin: 0 }}>
            Modes: <strong>None</strong> = no second factor, <strong>OTP</strong> = apply to OTP challenges, <strong>OOB</strong> = apply to OOB challenges, <strong>Both</strong> = apply to all challenges.
          </p>
        </div>
      </div>

      {showForm && (
        <ProviderForm
          provider={editProvider}
          onClose={() => setShowForm(false)}
          onSaved={() => {
            setShowForm(false);
            load();
            setMsg({ type: 'success', text: editProvider ? 'Provider updated' : 'Provider created' });
          }}
          onError={e => setMsg({ type: 'error', text: e })}
        />
      )}
    </>
  );
}

function OTPGenerationSettings() {
  const [source, setSource] = useState('internal');
  const [endpointId, setEndpointId] = useState('');
  const [endpoints, setEndpoints] = useState([]);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    // Load current config
    fetchJSON('/config').then(data => {
      if (data?.otp) {
        setSource(data.otp.source || 'internal');
        setEndpointId(data.otp.hsmEndpointId || '');
      }
      setLoaded(true);
    }).catch(() => setLoaded(true));

    // Load HSM endpoints
    secretsFetch('/hsm/v1/endpoints').then(data => {
      setEndpoints((data || []).filter(ep => ep.enabled));
    }).catch(() => {});
  }, []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await fetchJSON('/config', {}, 'PUT', {
        otp: {
          source,
          hsmEndpointId: source === 'hsm' ? Number(endpointId) : 0,
        },
      });
      setMsg({ type: 'success', text: 'OTP generation settings saved' });
    } catch (e) {
      setMsg({ type: 'error', text: `Failed: ${e.message}` });
    } finally {
      setSaving(false);
    }
  };

  if (!loaded) return null;

  return (
    <div className="table-card" style={{ marginBottom: 24 }}>
      <div className="table-toolbar">
        <span className="card-title">OTP Generation Settings</span>
      </div>
      <div style={{ padding: '16px 20px' }}>
        {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 12 }}>{msg.text}</div>}
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Random Number Source</label>
            <select className="filter-select" value={source} onChange={e => setSource(e.target.value)}>
              <option value="internal">Internal (crypto/rand)</option>
              <option value="hsm">Secrets Manager / HSM</option>
            </select>
          </div>
          {source === 'hsm' && (
            <div className="form-field">
              <label className="form-label">HSM Endpoint</label>
              <select className="filter-select" value={endpointId} onChange={e => setEndpointId(e.target.value)}>
                <option value="">Select endpoint...</option>
                {endpoints.map(ep => (
                  <option key={ep.id} value={ep.id}>{ep.name} ({ep.model} - {ep.address}:{ep.port})</option>
                ))}
              </select>
              <span style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 4, display: 'block' }}>
                OTP random bytes will be generated using the HSM's hardware random number generator for FIPS-compliant randomness
              </span>
            </div>
          )}
        </div>
        <div style={{ marginTop: 12, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-primary btn-small" onClick={handleSave} disabled={saving || (source === 'hsm' && !endpointId)}>
            {saving ? 'Saving...' : 'Save OTP Settings'}
          </button>
        </div>
      </div>
    </div>
  );
}

function ProviderForm({ provider, onClose, onSaved, onError }) {
  const [form, setForm] = useState({
    name: provider?.name || '',
    displayName: provider?.displayName || '',
    type: provider?.type || 'internal',
    factorType: provider?.factorType || 'card_pin',
    active: provider?.active ?? false,
    description: provider?.description || '',
    config: provider?.config ? JSON.stringify(provider.config, null, 2) : '{}',
  });
  const [saving, setSaving] = useState(false);
  const isEdit = !!provider;

  const handleSave = async () => {
    if (!form.name || !form.displayName) {
      onError('Name and display name are required');
      return;
    }
    let parsedConfig;
    try {
      parsedConfig = JSON.parse(form.config);
    } catch {
      onError('Config must be valid JSON');
      return;
    }
    setSaving(true);
    try {
      const body = {
        ...form,
        config: parsedConfig,
      };
      if (isEdit) {
        await fetchJSON(`/mfa-providers/${provider.id}`, {}, 'PUT', body);
      } else {
        await fetchJSON('/mfa-providers', {}, 'POST', body);
      }
      onSaved();
    } catch (e) {
      onError(`Failed to save: ${e.message}`);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()} style={{ maxWidth: 520 }}>
        <h3 style={{ marginBottom: 16 }}>{isEdit ? 'Edit' : 'Add'} MFA Provider</h3>
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Slug (unique identifier)</label>
            <input
              className="filter-input mono"
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, '') }))}
              placeholder="e.g. card_pin"
              disabled={isEdit}
            />
          </div>
          <div className="form-field">
            <label className="form-label">Display Name</label>
            <input
              className="filter-input"
              value={form.displayName}
              onChange={e => setForm(f => ({ ...f, displayName: e.target.value }))}
              placeholder="e.g. Card PIN Verification"
            />
          </div>
          <div className="form-field">
            <label className="form-label">Provider Type</label>
            <select className="filter-select" value={form.type} onChange={e => setForm(f => ({ ...f, type: e.target.value }))}>
              {PROVIDER_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Factor Type</label>
            <select className="filter-select" value={form.factorType} onChange={e => setForm(f => ({ ...f, factorType: e.target.value }))}>
              {FACTOR_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Active</label>
            <div className="admin-toggle-wrap">
              <button className={`admin-toggle ${form.active ? 'on' : ''}`} onClick={() => setForm(f => ({ ...f, active: !f.active }))}>
                <span className="admin-toggle-knob" />
              </button>
              <span style={{ fontSize: 12 }}>{form.active ? 'Yes' : 'No'}</span>
            </div>
          </div>
          <div className="form-field" style={{ gridColumn: '1 / -1' }}>
            <label className="form-label">Description</label>
            <input
              className="filter-input"
              value={form.description}
              onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
              placeholder="Brief description of this provider"
            />
          </div>
          <div className="form-field" style={{ gridColumn: '1 / -1' }}>
            <label className="form-label">Configuration (JSON)</label>
            <textarea
              className="admin-pem-input"
              rows={4}
              value={form.config}
              onChange={e => setForm(f => ({ ...f, config: e.target.value }))}
              placeholder='{"apiKey": "...", "endpoint": "..."}'
              style={{ fontFamily: 'monospace', fontSize: 12 }}
            />
            <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>
              {form.type === 'external' ? 'Integration settings (API keys, endpoints, etc.)' : 'Internal provider settings (usually empty)'}
            </span>
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : isEdit ? 'Update' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  );
}
