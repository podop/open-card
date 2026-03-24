import React, { useState, useEffect, useCallback } from 'react';
import { fetchJSON } from '../api';
import { NetworkIcon } from '../components/NetworkBadge';
import { useNetworks } from '../hooks/useNetworks';
import KeySourceModal from '../components/KeySourceModal';

export default function Admin() {
  const [tab, setTab] = useState('settings');

  return (
    <>
      <div className="page-header">
        <h1>Administration</h1>
        <p>Manage ACS core settings, network configuration and certificates</p>
      </div>
      <div className="admin-tabs">
        <button className={`admin-tab${tab === 'settings' ? ' active' : ''}`} onClick={() => setTab('settings')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" width="16" height="16">
            <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
          Core Settings
        </button>
        <button className={`admin-tab${tab === 'networks' ? ' active' : ''}`} onClick={() => setTab('networks')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" width="16" height="16">
            <rect x="2" y="7" width="20" height="14" rx="2" ry="2"/><path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/>
          </svg>
          Networks &amp; Certificates
        </button>
        <button className={`admin-tab${tab === 'bins' ? ' active' : ''}`} onClick={() => setTab('bins')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" width="16" height="16">
            <rect x="1" y="4" width="22" height="16" rx="2" ry="2"/><line x1="1" y1="10" x2="23" y2="10"/>
          </svg>
          BIN Configuration
        </button>
      </div>
      {tab === 'settings' && <CoreSettings />}
      {tab === 'networks' && <NetworksTab />}
      {tab === 'bins' && <BinConfigTab />}
    </>
  );
}

/* ── Core Settings ─────────────────────────────────────────── */

function CoreSettings() {
  const [config, setConfig] = useState(null);
  const [form, setForm] = useState({});
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const data = await fetchJSON('/config');
      setConfig(data);
      setForm({
        baseURL: data.acs?.baseURL || '',
        referenceNumber: data.acs?.referenceNumber || '',
        operatorID: data.acs?.operatorID || '',
        supportedVersions: (data.acs?.supportedVersions || []).join(', '),
        dsURL: data.acs?.dsURL || '',
        logLevel: data.log?.level || 'info',
        logFormat: data.log?.format || 'json',
      });
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load config' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleSave = async () => {
    setSaving(true);
    setMsg(null);
    try {
      await fetchJSON('/config', {}, 'PUT', {
        acs: {
          baseURL: form.baseURL,
          referenceNumber: form.referenceNumber,
          operatorID: form.operatorID,
          supportedVersions: form.supportedVersions.split(',').map(s => s.trim()).filter(Boolean),
          dsURL: form.dsURL,
        },
        log: {
          level: form.logLevel,
          format: form.logFormat,
        },
      });
      setMsg({ type: 'success', text: 'Settings saved successfully' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to save settings: ' + e.message });
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {msg && <div className={`admin-msg admin-msg-${msg.type}`}>{msg.text}</div>}

      <div className="card">
        <div className="card-header">
          <span className="card-title">ACS Configuration</span>
          <span className="card-subtitle">Core 3DS authentication server settings</span>
        </div>
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Base URL</label>
            <input className="filter-input" value={form.baseURL} onChange={e => setForm(f => ({ ...f, baseURL: e.target.value }))} placeholder="https://acs.example.com" />
          </div>
          <div className="form-field">
            <label className="form-label">Reference Number</label>
            <input className="filter-input" value={form.referenceNumber} onChange={e => setForm(f => ({ ...f, referenceNumber: e.target.value }))} placeholder="3DS_LOA_ACS_..." />
          </div>
          <div className="form-field">
            <label className="form-label">Operator ID</label>
            <input className="filter-input" value={form.operatorID} onChange={e => setForm(f => ({ ...f, operatorID: e.target.value }))} placeholder="YOUR_OPERATOR_ID" />
          </div>
          <div className="form-field">
            <label className="form-label">DS URL</label>
            <input className="filter-input" value={form.dsURL} onChange={e => setForm(f => ({ ...f, dsURL: e.target.value }))} placeholder="http://ds:9443" />
          </div>
          <div className="form-field" style={{ gridColumn: '1 / -1' }}>
            <label className="form-label">Supported Versions</label>
            <input className="filter-input" value={form.supportedVersions} onChange={e => setForm(f => ({ ...f, supportedVersions: e.target.value }))} placeholder="2.2.0, 2.3.1" />
          </div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <span className="card-title">Logging</span>
          <span className="card-subtitle">Runtime log configuration</span>
        </div>
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Log Level</label>
            <select className="filter-select" value={form.logLevel} onChange={e => setForm(f => ({ ...f, logLevel: e.target.value }))}>
              <option value="debug">Debug</option>
              <option value="info">Info</option>
              <option value="warn">Warn</option>
              <option value="error">Error</option>
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Log Format</label>
            <select className="filter-select" value={form.logFormat} onChange={e => setForm(f => ({ ...f, logFormat: e.target.value }))}>
              <option value="json">JSON</option>
              <option value="console">Console</option>
            </select>
          </div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <span className="card-title">System Info</span>
          <span className="card-subtitle">Read-only server and crypto information</span>
        </div>
        <div className="admin-readonly-grid">
          <ReadonlyField label="Server" value={`${config?.server?.host || ''}:${config?.server?.port || ''}`} />
          <ReadonlyField label="TLS Cert" value={config?.server?.tlsCertFile || '(not configured)'} />
          <ReadonlyField label="TLS Key" value={config?.server?.tlsKeyFile || '(not configured)'} />
          <ReadonlyField label="SDK Private Key" value={config?.acs?.sdkPrivateKeyFile || '(not configured)'} />
          <ReadonlyField label="Master Key" value={config?.crypto?.masterKeyHex || '(not set)'} mono />
          <ReadonlyField label="PAN Encryption Key" value={config?.crypto?.panEncryptionKeyHex || '(not set)'} mono />
          <ReadonlyField label="Database" value={config?.database?.dsn || ''} mono />
        </div>
      </div>

      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
        <button className="btn btn-secondary" onClick={load}>Reset</button>
        <button className="btn btn-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save Settings'}</button>
      </div>
    </div>
  );
}

function ReadonlyField({ label, value, mono }) {
  return (
    <div className="admin-ro-field">
      <span className="admin-ro-label">{label}</span>
      <span className={`admin-ro-value${mono ? ' mono' : ''}`}>{value}</span>
    </div>
  );
}

/* ── Networks & Certificates ───────────────────────────────── */

function NetworksTab() {
  const { networks, loading: netsLoading, invalidate: invalidateNets } = useNetworks();
  const [config, setConfig] = useState(null);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [expandedNet, setExpandedNet] = useState(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [editNet, setEditNet] = useState(null);

  const loadConfig = useCallback(async () => {
    try {
      const data = await fetchJSON('/config');
      setConfig(data);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load config' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { loadConfig(); }, [loadConfig]);

  if (loading || netsLoading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  const policies = config?.networkPolicies || [];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {msg && <div className={`admin-msg admin-msg-${msg.type}`}>{msg.text}</div>}
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <button className="btn btn-primary btn-small" onClick={() => { setEditNet(null); setShowAddForm(true); }}>Add Network</button>
      </div>
      {networks.map(net => {
        const pol = policies.find(p => p.network === net.name) || {};
        return (
          <NetworkCard
            key={net.name}
            netEntry={net}
            policy={pol}
            expanded={expandedNet === net.name}
            onToggle={() => setExpandedNet(expandedNet === net.name ? null : net.name)}
            onUpdated={() => { loadConfig(); invalidateNets(); setMsg({ type: 'success', text: `${net.displayName} updated` }); }}
            onEdit={() => { setEditNet(net); setShowAddForm(true); }}
            onDelete={async () => {
              if (!confirm(`Delete network "${net.displayName}"? This cannot be undone.`)) return;
              try {
                await fetchJSON(`/networks/${net.name}`, {}, 'DELETE');
                invalidateNets(); loadConfig();
                setMsg({ type: 'success', text: `${net.displayName} deleted` });
              } catch (e) {
                setMsg({ type: 'error', text: `Failed to delete: ${e.message}` });
              }
            }}
            onError={(e) => setMsg({ type: 'error', text: e })}
          />
        );
      })}
      {networks.length === 0 && <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: 40 }}>No networks configured</div>}

      {showAddForm && (
        <NetworkForm
          network={editNet}
          onClose={() => setShowAddForm(false)}
          onSaved={() => { setShowAddForm(false); invalidateNets(); loadConfig(); setMsg({ type: 'success', text: editNet ? 'Network updated' : 'Network created' }); }}
          onError={e => setMsg({ type: 'error', text: e })}
        />
      )}
    </div>
  );
}

const CERT_TYPES = [
  { key: 'server', label: 'Server Certificate', desc: 'TLS server certificate for the ACS endpoint' },
  { key: 'client', label: 'Client Certificate', desc: 'Mutual-TLS client certificate for DS communication' },
  { key: 'signing', label: 'Signing Certificate', desc: 'Message signing certificate (ARes, RReq)' },
];

function NetworkCard({ netEntry, policy, expanded, onToggle, onUpdated, onEdit, onDelete, onError }) {
  const [saving, setSaving] = useState(false);
  const [frictionless, setFrictionless] = useState(policy.allowFrictionless);
  const [masterKey, setMasterKey] = useState('');
  const [certs, setCerts] = useState({}); // { server: info|null, client: info|null, signing: info|null }
  const [certLoading, setCertLoading] = useState(false);

  const network = netEntry.name;

  useEffect(() => {
    setFrictionless(policy.allowFrictionless);
  }, [policy.allowFrictionless]);

  useEffect(() => {
    if (expanded) {
      setCertLoading(true);
      fetchJSON(`/config/network/${network}/certificates`)
        .then(data => setCerts(data.certificates || {}))
        .catch(() => setCerts({}))
        .finally(() => setCertLoading(false));
    }
  }, [expanded, network]);

  const handleSavePolicy = async () => {
    setSaving(true);
    try {
      const body = { allowFrictionless: frictionless };
      if (masterKey) body.masterKeyHex = masterKey;
      await fetchJSON(`/config/network/${network}`, {}, 'PUT', body);
      setMasterKey('');
      onUpdated();
    } catch (e) {
      onError(`Failed to update ${network}: ${e.message}`);
    } finally {
      setSaving(false);
    }
  };

  const handleUploadCert = async (certType, pemData, keyPem) => {
    try {
      const body = { pemData };
      if (keyPem) body.keyPem = keyPem;
      await fetchJSON(`/config/network/${network}/certificate`, { type: certType }, 'POST', body);
      const data = await fetchJSON(`/config/network/${network}/certificates`);
      setCerts(data.certificates || {});
      onUpdated();
    } catch (e) {
      onError(`Failed to upload ${certType} certificate: ${e.message}`);
    }
  };

  const handleDeleteCert = async (certType) => {
    try {
      await fetchJSON(`/config/network/${network}/certificate`, { type: certType }, 'DELETE');
      setCerts(prev => ({ ...prev, [certType]: null }));
      onUpdated();
    } catch (e) {
      onError(`Failed to delete ${certType} certificate: ${e.message}`);
    }
  };

  return (
    <div className={`card admin-network-card${!netEntry.active ? ' admin-network-inactive' : ''}`}>
      <div className="admin-network-header" onClick={onToggle}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{ width: 28, height: 28, borderRadius: 6, background: netEntry.color || '#555d70', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <NetworkIcon network={network} size={20} />
          </div>
          <div>
            <div style={{ fontWeight: 600, fontSize: 14 }}>
              {netEntry.displayName}
              {!netEntry.active && <span className="badge badge-muted" style={{ marginLeft: 8, fontSize: 10 }}>INACTIVE</span>}
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>
              {frictionless ? 'Frictionless allowed' : 'Challenge only'}
              {policy.hasCustomKey && ' \u00b7 Custom key'}
              {policy.hasCertificate && ` \u00b7 ${(policy.certTypes || []).length} cert${(policy.certTypes || []).length !== 1 ? 's' : ''}`}
            </div>
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <button className="btn btn-secondary btn-small" onClick={e => { e.stopPropagation(); onEdit(); }} title="Edit network">Edit</button>
          <button className="btn btn-danger btn-small" onClick={e => { e.stopPropagation(); onDelete(); }} title="Delete network">Delete</button>
          <span className={`badge ${netEntry.active ? (frictionless ? 'badge-green' : 'badge-yellow') : 'badge-muted'}`}>
            {!netEntry.active ? 'Inactive' : frictionless ? 'Frictionless' : 'Challenge'}
          </span>
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" style={{ transform: expanded ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }}>
            <polyline points="6 9 12 15 18 9"/>
          </svg>
        </div>
      </div>

      {expanded && (
        <div className="admin-network-body">
          <div className="admin-section">
            <h4 className="admin-section-title">Network Details</h4>
            <div className="admin-readonly-grid">
              <ReadonlyField label="Slug" value={netEntry.name} mono />
              <ReadonlyField label="Color" value={netEntry.color} />
              <ReadonlyField label="ECI (Full Auth)" value={netEntry.eciFullAuth} />
              <ReadonlyField label="ECI (Attempt)" value={netEntry.eciAttempt} />
              <ReadonlyField label="ECI (Not Auth)" value={netEntry.eciNotAuth} />
            </div>
          </div>

          <div className="admin-section">
            <h4 className="admin-section-title">Flow Policy</h4>
            <div className="form-grid">
              <div className="form-field">
                <label className="form-label">Allow Frictionless</label>
                <div className="admin-toggle-wrap">
                  <button className={`admin-toggle ${frictionless ? 'on' : ''}`} onClick={() => setFrictionless(!frictionless)}>
                    <span className="admin-toggle-knob" />
                  </button>
                  <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>{frictionless ? 'Yes' : 'No'}</span>
                </div>
              </div>
              <MasterKeyField masterKey={masterKey} setMasterKey={setMasterKey} policy={policy} network={network} />
            </div>
            <div style={{ marginTop: 12, display: 'flex', justifyContent: 'flex-end' }}>
              <button className="btn btn-primary btn-small" onClick={handleSavePolicy} disabled={saving}>
                {saving ? 'Saving...' : 'Save Policy'}
              </button>
            </div>
          </div>

          {certLoading ? (
            <div className="admin-section">
              <div className="spinner-wrap" style={{ padding: '20px 0' }}><div className="spinner" /></div>
            </div>
          ) : CERT_TYPES.map(ct => (
            <CertSection
              key={ct.key}
              certType={ct.key}
              label={ct.label}
              description={ct.desc}
              certInfo={certs[ct.key] || null}
              onUpload={(pemData, keyPem) => handleUploadCert(ct.key, pemData, keyPem)}
              onDelete={() => handleDeleteCert(ct.key)}
              showKeyField={ct.key === 'client' || ct.key === 'signing'}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function MasterKeyField({ masterKey, setMasterKey, policy, network }) {
  const [mode, setMode] = useState('manual');
  const [showModal, setShowModal] = useState(false);

  return (
    <div className="form-field">
      <label className="form-label">Custom Master Key (Hex)</label>
      <div style={{ display: 'flex', gap: 6, marginBottom: 6 }}>
        {['manual', 'generate', 'existing'].map(m => (
          <button key={m} type="button" className={`btn btn-small ${mode === m ? 'btn-primary' : 'btn-secondary'}`}
            onClick={() => { setMode(m); if (m !== 'manual') setShowModal(true); }}>
            {m === 'manual' ? 'Manual' : m === 'generate' ? 'Generate New' : 'Use Existing'}
          </button>
        ))}
      </div>
      <input className="filter-input mono" value={masterKey} onChange={e => setMasterKey(e.target.value)}
        placeholder={policy.hasCustomKey ? '(custom key set)' : '(using global key)'} style={{ fontSize: 11 }} />
      <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>64 hex chars. Leave empty to keep current.</span>
      {showModal && (
        <KeySourceModal
          title={`Master Key for ${network}`}
          keyType="AES-256"
          keyUsage="encryption"
          onSelect={(hexValue) => { setMasterKey(hexValue); setShowModal(false); setMode('manual'); }}
          onClose={() => { setShowModal(false); setMode('manual'); }}
        />
      )}
    </div>
  );
}

function NetworkForm({ network, onClose, onSaved, onError }) {
  const [form, setForm] = useState({
    name: network?.name || '',
    displayName: network?.displayName || '',
    active: network?.active ?? true,
    color: network?.color || '#555d70',
    eciFullAuth: network?.eciFullAuth || '05',
    eciAttempt: network?.eciAttempt || '06',
    eciNotAuth: network?.eciNotAuth || '07',
  });
  const [saving, setSaving] = useState(false);
  const isEdit = !!network;

  const handleSave = async () => {
    if (!form.name || !form.displayName) {
      onError('Name and display name are required');
      return;
    }
    setSaving(true);
    try {
      if (isEdit) {
        await fetchJSON(`/networks/${network.name}`, {}, 'PUT', form);
      } else {
        await fetchJSON('/networks', {}, 'POST', form);
      }
      onSaved();
    } catch (e) {
      onError(`Failed to save network: ${e.message}`);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()}>
        <h3 style={{ marginBottom: 16 }}>{isEdit ? 'Edit' : 'Add'} Network</h3>
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Slug (unique identifier)</label>
            <input className="filter-input mono" value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, '') }))} placeholder="e.g. jcb" disabled={isEdit} />
          </div>
          <div className="form-field">
            <label className="form-label">Display Name</label>
            <input className="filter-input" value={form.displayName} onChange={e => setForm(f => ({ ...f, displayName: e.target.value }))} placeholder="e.g. JCB" />
          </div>
          <div className="form-field">
            <label className="form-label">Brand Color</label>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <input type="color" value={form.color} onChange={e => setForm(f => ({ ...f, color: e.target.value }))} style={{ width: 40, height: 32, border: 'none', cursor: 'pointer' }} />
              <input className="filter-input mono" value={form.color} onChange={e => setForm(f => ({ ...f, color: e.target.value }))} style={{ flex: 1 }} />
            </div>
          </div>
          <div className="form-field">
            <label className="form-label">Active</label>
            <div className="admin-toggle-wrap">
              <button className={`admin-toggle ${form.active ? 'on' : ''}`} onClick={() => setForm(f => ({ ...f, active: !f.active }))}><span className="admin-toggle-knob" /></button>
              <span style={{ fontSize: 12 }}>{form.active ? 'Yes' : 'No'}</span>
            </div>
          </div>
          <div className="form-field">
            <label className="form-label">ECI Full Auth</label>
            <input className="filter-input mono" value={form.eciFullAuth} onChange={e => setForm(f => ({ ...f, eciFullAuth: e.target.value }))} placeholder="05" maxLength={4} />
          </div>
          <div className="form-field">
            <label className="form-label">ECI Attempt</label>
            <input className="filter-input mono" value={form.eciAttempt} onChange={e => setForm(f => ({ ...f, eciAttempt: e.target.value }))} placeholder="06" maxLength={4} />
          </div>
          <div className="form-field">
            <label className="form-label">ECI Not Auth</label>
            <input className="filter-input mono" value={form.eciNotAuth} onChange={e => setForm(f => ({ ...f, eciNotAuth: e.target.value }))} placeholder="07" maxLength={4} />
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : isEdit ? 'Update' : 'Create'}</button>
        </div>
      </div>
    </div>
  );
}

function CertSection({ certType, label, description, certInfo, onUpload, onDelete, showKeyField }) {
  const [pemText, setPemText] = useState('');
  const [keyText, setKeyText] = useState('');
  const [uploading, setUploading] = useState(false);

  const handleUpload = async () => {
    if (!pemText.trim()) return;
    setUploading(true);
    try {
      await onUpload(pemText, keyText || undefined);
      setPemText('');
      setKeyText('');
    } finally {
      setUploading(false);
    }
  };

  return (
    <div className="admin-section">
      <h4 className="admin-section-title">
        {label}
        {certInfo && <span className="badge badge-green" style={{ marginLeft: 8, fontSize: 10 }}>UPLOADED</span>}
        {certInfo?.hasKey && <span className="badge badge-cyan" style={{ marginLeft: 4, fontSize: 10 }}>+ KEY</span>}
      </h4>
      <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 10 }}>{description}</div>

      {certInfo ? (
        <div className="admin-cert-info">
          <div className="admin-cert-grid">
            <CertField label="Subject" value={certInfo.subject} />
            <CertField label="Issuer" value={certInfo.issuer} />
            <CertField label="Valid From" value={new Date(certInfo.notBefore).toLocaleDateString()} />
            <CertField label="Valid Until" value={new Date(certInfo.notAfter).toLocaleDateString()} expired={new Date(certInfo.notAfter) < new Date()} />
            <CertField label="Serial Number" value={certInfo.serialNumber} mono />
            <CertField label="Fingerprint (SHA-256)" value={certInfo.fingerprint} mono />
            <CertField label="Uploaded" value={new Date(certInfo.uploadedAt).toLocaleString()} />
            {certInfo.hasKey && <CertField label="Private Key" value="Uploaded (stored securely)" />}
          </div>
          <div style={{ marginTop: 12, display: 'flex', gap: 8 }}>
            <button className="btn btn-danger btn-small" onClick={onDelete}>Remove</button>
          </div>
        </div>
      ) : (
        <div style={{ color: 'var(--text-muted)', fontSize: 13, marginBottom: 8 }}>No {certType} certificate uploaded</div>
      )}

      <div style={{ marginTop: 10 }}>
        <label className="form-label">{certInfo ? 'Replace' : 'Upload'} PEM Certificate</label>
        <textarea
          className="admin-pem-input"
          rows={4}
          value={pemText}
          onChange={e => setPemText(e.target.value)}
          placeholder="-----BEGIN CERTIFICATE-----&#10;MIIBkT...&#10;-----END CERTIFICATE-----"
        />
        {showKeyField && (
          <>
            <label className="form-label" style={{ marginTop: 8 }}>Private Key (PEM) — optional for server, required for client/signing</label>
            <textarea
              className="admin-pem-input"
              rows={4}
              value={keyText}
              onChange={e => setKeyText(e.target.value)}
              placeholder="-----BEGIN PRIVATE KEY-----&#10;MIIEv...&#10;-----END PRIVATE KEY-----"
            />
          </>
        )}
        <div style={{ marginTop: 8, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-primary btn-small" onClick={handleUpload} disabled={uploading || !pemText.trim()}>
            {uploading ? 'Uploading...' : certInfo ? 'Replace' : 'Upload'}
          </button>
        </div>
      </div>
    </div>
  );
}

function CertField({ label, value, mono, expired }) {
  return (
    <div className="admin-cert-field">
      <span className="admin-ro-label">{label}</span>
      <span className={`admin-ro-value${mono ? ' mono' : ''}${expired ? ' admin-expired' : ''}`}>
        {value}
        {expired && <span className="badge badge-red" style={{ marginLeft: 8, fontSize: 10 }}>EXPIRED</span>}
      </span>
    </div>
  );
}

/* ── BIN Configuration ─────────────────────────────────────── */

function BinConfigTab() {
  const { networks } = useNetworks();
  const networkNames = networks.map(n => n.name);
  const networkMap = Object.fromEntries(networks.map(n => [n.name, n]));
  const [rules, setRules] = useState([]);
  const [exclusions, setExclusions] = useState([]);
  const [cardRanges, setCardRanges] = useState([]);
  const [mfaProviders, setMfaProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [filterNet, setFilterNet] = useState('');
  const [showExcForm, setShowExcForm] = useState(false);
  const [editExc, setEditExc] = useState(null);

  const load = useCallback(async () => {
    try {
      const [rulesData, excData, rangesData, mfaData] = await Promise.all([
        fetchJSON('/network-rules'),
        fetchJSON('/bin-exclusions', filterNet ? { network: filterNet } : {}),
        fetchJSON('/card-ranges'),
        fetchJSON('/mfa-providers'),
      ]);
      setRules(rulesData.data || []);
      setExclusions(excData.data || []);
      setCardRanges(rangesData.data || []);
      setMfaProviders((mfaData.data || []).filter(p => p.active));
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load data' });
    } finally {
      setLoading(false);
    }
  }, [filterNet]);

  useEffect(() => { load(); }, [load]);

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {msg && <div className={`admin-msg admin-msg-${msg.type}`}>{msg.text}</div>}

      {/* Network Rules */}
      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">Network Rules</span>
          <span className="card-subtitle" style={{ fontSize: 12, color: 'var(--text-muted)' }}>Default flow settings applied per network</span>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Network</th>
                <th>Frictionless</th>
                <th>OTP</th>
                <th>OOB</th>
                <th>Default Channel</th>
                <th>2nd Factor</th>
                <th>MFA Provider</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {rules.filter(r => networkMap[r.network]).map(r => (
                <NetworkRuleRow key={r.network} rule={r} netEntry={networkMap[r.network]} mfaProviders={mfaProviders} onSaved={() => { load(); setMsg({ type: 'success', text: `${r.network} rule updated` }); }} onError={e => setMsg({ type: 'error', text: e })} />
              ))}
              {rules.filter(r => networkMap[r.network]).length === 0 && <tr><td colSpan={8} className="empty-state">No network rules configured</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {/* Card Ranges (read-only BIN table) */}
      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">Card BIN Ranges</span>
          <div className="table-filters">
            <select className="filter-select" value={filterNet} onChange={e => setFilterNet(e.target.value)}>
              <option value="">All Networks</option>
              {networkNames.map(n => <option key={n} value={n}>{n}</option>)}
            </select>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Network</th>
                <th>Start Range</th>
                <th>End Range</th>
                <th>Protocol</th>
                <th>Active</th>
              </tr>
            </thead>
            <tbody>
              {(filterNet ? cardRanges.filter(r => r.network === filterNet) : cardRanges).map(r => (
                <tr key={r.id}>
                  <td><div style={{ display: 'flex', alignItems: 'center', gap: 8 }}><NetworkIcon network={r.network} size={18} /><span style={{ textTransform: 'capitalize' }}>{r.network}</span></div></td>
                  <td className="mono">{r.startRange}</td>
                  <td className="mono">{r.endRange}</td>
                  <td>{r.minProtocol} - {r.maxProtocol}</td>
                  <td><span className={`badge ${r.active ? 'badge-green' : 'badge-muted'}`}>{r.active ? 'Active' : 'Inactive'}</span></td>
                </tr>
              ))}
              {cardRanges.length === 0 && <tr><td colSpan={5} className="empty-state">No card ranges</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {/* BIN Exclusions */}
      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">BIN Exclusions</span>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>Override network rules for specific BIN ranges</span>
            <button className="btn btn-primary btn-small" onClick={() => { setEditExc(null); setShowExcForm(true); }}>Add Exclusion</button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Network</th>
                <th>Start Range</th>
                <th>End Range</th>
                <th>Description</th>
                <th>Frictionless</th>
                <th>OTP</th>
                <th>OOB</th>
                <th>Channel</th>
                <th>2nd Factor</th>
                <th>Active</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {exclusions.map(exc => (
                <tr key={exc.id}>
                  <td><div style={{ display: 'flex', alignItems: 'center', gap: 8 }}><NetworkIcon network={exc.network} size={18} /><span style={{ textTransform: 'capitalize' }}>{exc.network}</span></div></td>
                  <td className="mono">{exc.startRange}</td>
                  <td className="mono">{exc.endRange}</td>
                  <td>{exc.description || '-'}</td>
                  <td><ToggleBadge on={exc.allowFrictionless} /></td>
                  <td><ToggleBadge on={exc.allowOtp} /></td>
                  <td><ToggleBadge on={exc.allowOob} /></td>
                  <td><span className="badge badge-cyan">{exc.defaultChallengeChannel?.toUpperCase()}</span></td>
                  <td><span className={`badge ${exc.secondFactorMode && exc.secondFactorMode !== 'none' ? 'badge-purple' : 'badge-muted'}`}>{(exc.secondFactorMode || 'none').toUpperCase()}</span></td>
                  <td><span className={`badge ${exc.active ? 'badge-green' : 'badge-muted'}`}>{exc.active ? 'Yes' : 'No'}</span></td>
                  <td>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button className="btn btn-secondary btn-small" onClick={() => { setEditExc(exc); setShowExcForm(true); }}>Edit</button>
                      <button className="btn btn-danger btn-small" onClick={async () => {
                        await fetchJSON(`/bin-exclusions/${exc.id}`, {}, 'DELETE');
                        load();
                        setMsg({ type: 'success', text: 'Exclusion deleted' });
                      }}>Delete</button>
                    </div>
                  </td>
                </tr>
              ))}
              {exclusions.length === 0 && <tr><td colSpan={11} className="empty-state">No BIN exclusions configured</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {showExcForm && (
        <ExclusionForm
          exclusion={editExc}
          networkNames={networkNames}
          mfaProviders={mfaProviders}
          onClose={() => setShowExcForm(false)}
          onSaved={() => { setShowExcForm(false); load(); setMsg({ type: 'success', text: editExc ? 'Exclusion updated' : 'Exclusion created' }); }}
          onError={e => setMsg({ type: 'error', text: e })}
        />
      )}
    </div>
  );
}

function ToggleBadge({ on }) {
  return <span className={`badge ${on ? 'badge-green' : 'badge-red'}`}>{on ? 'Yes' : 'No'}</span>;
}

function NetworkRuleRow({ rule, netEntry, mfaProviders, onSaved, onError }) {
  const inactive = netEntry && !netEntry.active;
  const [form, setForm] = useState({
    allowFrictionless: rule.allowFrictionless,
    allowOtp: rule.allowOtp,
    allowOob: rule.allowOob,
    defaultChallengeChannel: rule.defaultChallengeChannel,
    secondFactorMode: rule.secondFactorMode || 'none',
    mfaProviderId: rule.mfaProviderId || null,
  });
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);

  const update = (key, val) => {
    setForm(f => ({ ...f, [key]: val }));
    setDirty(true);
  };

  const save = async () => {
    setSaving(true);
    try {
      await fetchJSON(`/network-rules/${rule.network}`, {}, 'PUT', form);
      setDirty(false);
      onSaved();
    } catch (e) {
      onError(`Failed to save ${rule.network}: ${e.message}`);
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr style={inactive ? { opacity: 0.45 } : undefined}>
      <td><div style={{ display: 'flex', alignItems: 'center', gap: 8 }}><NetworkIcon network={rule.network} size={18} /><span style={{ textTransform: 'capitalize', fontWeight: 600 }}>{rule.network}</span>{inactive && <span className="badge badge-muted" style={{ fontSize: 9 }}>INACTIVE</span>}</div></td>
      <td><button className={`admin-toggle ${form.allowFrictionless ? 'on' : ''}`} onClick={() => update('allowFrictionless', !form.allowFrictionless)} style={{ transform: 'scale(0.8)' }}><span className="admin-toggle-knob" /></button></td>
      <td><button className={`admin-toggle ${form.allowOtp ? 'on' : ''}`} onClick={() => update('allowOtp', !form.allowOtp)} style={{ transform: 'scale(0.8)' }}><span className="admin-toggle-knob" /></button></td>
      <td><button className={`admin-toggle ${form.allowOob ? 'on' : ''}`} onClick={() => update('allowOob', !form.allowOob)} style={{ transform: 'scale(0.8)' }}><span className="admin-toggle-knob" /></button></td>
      <td>
        <select className="filter-select" value={form.defaultChallengeChannel} onChange={e => update('defaultChallengeChannel', e.target.value)} style={{ fontSize: 12, padding: '4px 8px' }}>
          <option value="otp">OTP</option>
          <option value="oob">OOB</option>
        </select>
      </td>
      <td>
        <select className="filter-select" value={form.secondFactorMode} onChange={e => update('secondFactorMode', e.target.value)} style={{ fontSize: 12, padding: '4px 8px' }}>
          <option value="none">None</option>
          <option value="otp">OTP</option>
          <option value="oob">OOB</option>
          <option value="both">Both</option>
        </select>
      </td>
      <td>
        <select className="filter-select" value={form.mfaProviderId || ''} onChange={e => update('mfaProviderId', e.target.value ? Number(e.target.value) : null)} style={{ fontSize: 12, padding: '4px 8px' }} disabled={form.secondFactorMode === 'none'}>
          <option value="">— None —</option>
          {(mfaProviders || []).map(p => <option key={p.id} value={p.id}>{p.displayName}</option>)}
        </select>
      </td>
      <td>
        <button className="btn btn-primary btn-small" onClick={save} disabled={saving || !dirty}>
          {saving ? '...' : 'Save'}
        </button>
      </td>
    </tr>
  );
}

function ExclusionForm({ exclusion, networkNames, mfaProviders, onClose, onSaved, onError }) {
  const [form, setForm] = useState({
    network: exclusion?.network || (networkNames[0] || 'visa'),
    startRange: exclusion?.startRange || '',
    endRange: exclusion?.endRange || '',
    description: exclusion?.description || '',
    allowFrictionless: exclusion?.allowFrictionless ?? true,
    allowOtp: exclusion?.allowOtp ?? true,
    allowOob: exclusion?.allowOob ?? true,
    defaultChallengeChannel: exclusion?.defaultChallengeChannel || 'otp',
    secondFactorMode: exclusion?.secondFactorMode || 'none',
    mfaProviderId: exclusion?.mfaProviderId || null,
    active: exclusion?.active ?? true,
  });
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!form.startRange || !form.endRange) {
      onError('Start and end range are required');
      return;
    }
    setSaving(true);
    try {
      if (exclusion) {
        await fetchJSON(`/bin-exclusions/${exclusion.id}`, {}, 'PUT', form);
      } else {
        await fetchJSON('/bin-exclusions', {}, 'POST', form);
      }
      onSaved();
    } catch (e) {
      onError(`Failed to save exclusion: ${e.message}`);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()}>
        <h3 style={{ marginBottom: 16 }}>{exclusion ? 'Edit' : 'Add'} BIN Exclusion</h3>
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Network</label>
            <select className="filter-select" value={form.network} onChange={e => setForm(f => ({ ...f, network: e.target.value }))}>
              {networkNames.map(n => <option key={n} value={n}>{n}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Description</label>
            <input className="filter-input" value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))} placeholder="e.g. Premium cards" />
          </div>
          <div className="form-field">
            <label className="form-label">Start Range</label>
            <input className="filter-input mono" value={form.startRange} onChange={e => setForm(f => ({ ...f, startRange: e.target.value }))} placeholder="400000" />
          </div>
          <div className="form-field">
            <label className="form-label">End Range</label>
            <input className="filter-input mono" value={form.endRange} onChange={e => setForm(f => ({ ...f, endRange: e.target.value }))} placeholder="400099" />
          </div>
          <div className="form-field">
            <label className="form-label">Allow Frictionless</label>
            <div className="admin-toggle-wrap">
              <button className={`admin-toggle ${form.allowFrictionless ? 'on' : ''}`} onClick={() => setForm(f => ({ ...f, allowFrictionless: !f.allowFrictionless }))}><span className="admin-toggle-knob" /></button>
              <span style={{ fontSize: 12 }}>{form.allowFrictionless ? 'Yes' : 'No'}</span>
            </div>
          </div>
          <div className="form-field">
            <label className="form-label">Allow OTP</label>
            <div className="admin-toggle-wrap">
              <button className={`admin-toggle ${form.allowOtp ? 'on' : ''}`} onClick={() => setForm(f => ({ ...f, allowOtp: !f.allowOtp }))}><span className="admin-toggle-knob" /></button>
              <span style={{ fontSize: 12 }}>{form.allowOtp ? 'Yes' : 'No'}</span>
            </div>
          </div>
          <div className="form-field">
            <label className="form-label">Allow OOB</label>
            <div className="admin-toggle-wrap">
              <button className={`admin-toggle ${form.allowOob ? 'on' : ''}`} onClick={() => setForm(f => ({ ...f, allowOob: !f.allowOob }))}><span className="admin-toggle-knob" /></button>
              <span style={{ fontSize: 12 }}>{form.allowOob ? 'Yes' : 'No'}</span>
            </div>
          </div>
          <div className="form-field">
            <label className="form-label">Default Challenge Channel</label>
            <select className="filter-select" value={form.defaultChallengeChannel} onChange={e => setForm(f => ({ ...f, defaultChallengeChannel: e.target.value }))}>
              <option value="otp">OTP</option>
              <option value="oob">OOB</option>
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Second Factor Mode</label>
            <select className="filter-select" value={form.secondFactorMode} onChange={e => setForm(f => ({ ...f, secondFactorMode: e.target.value }))}>
              <option value="none">None</option>
              <option value="otp">OTP (apply to OTP challenges)</option>
              <option value="oob">OOB (apply to OOB challenges)</option>
              <option value="both">Both (all challenges)</option>
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">MFA Provider</label>
            <select className="filter-select" value={form.mfaProviderId || ''} onChange={e => setForm(f => ({ ...f, mfaProviderId: e.target.value ? Number(e.target.value) : null }))} disabled={form.secondFactorMode === 'none'}>
              <option value="">— None —</option>
              {(mfaProviders || []).map(p => <option key={p.id} value={p.id}>{p.displayName}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Active</label>
            <div className="admin-toggle-wrap">
              <button className={`admin-toggle ${form.active ? 'on' : ''}`} onClick={() => setForm(f => ({ ...f, active: !f.active }))}><span className="admin-toggle-knob" /></button>
              <span style={{ fontSize: 12 }}>{form.active ? 'Yes' : 'No'}</span>
            </div>
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save'}</button>
        </div>
      </div>
    </div>
  );
}