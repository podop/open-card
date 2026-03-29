import React, { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../AuthContext';
import { secretsFetch } from '../secretsApi';

const KEY_TYPES = ['RSA-1024','RSA-2048','RSA-3072','RSA-4096','AES-128','AES-192','AES-256','3DES','X509'];
const KEY_USAGES = ['general','signing','encryption','tls','authentication'];

export default function KeyManagement() {
  const { canDo } = useAuth();
  const [keys, setKeys] = useState([]);
  const [providers, setProviders] = useState([]);
  const [policies, setPolicies] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [filterType, setFilterType] = useState('');
  const [filterProvider, setFilterProvider] = useState('');
  const [filterActive, setFilterActive] = useState('');
  const [modal, setModal] = useState(null);

  const load = useCallback(async () => {
    try {
      const [k, p, pol] = await Promise.all([
        secretsFetch('/keys'),
        secretsFetch('/providers'),
        secretsFetch('/rotation-policies'),
      ]);
      setKeys(k || []);
      setProviders(p || []);
      setPolicies(pol || []);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load data' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const filtered = keys.filter(k => {
    if (filterType && k.keyType !== filterType) return false;
    if (filterProvider && k.providerId !== Number(filterProvider)) return false;
    if (filterActive === 'active' && !k.active) return false;
    if (filterActive === 'inactive' && k.active) return false;
    return true;
  });

  const providerName = (id) => {
    const p = providers.find(x => x.id === id);
    return p ? p.name : '—';
  };

  const handleDelete = async (key) => {
    if (!confirm(`Delete key "${key.name}"?`)) return;
    try {
      await secretsFetch(`/keys/${key.id}`, 'DELETE');
      setMsg({ type: 'success', text: 'Key deleted' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to delete key' });
    }
  };

  const handleRotate = async (key) => {
    if (!confirm(`Rotate key "${key.name}" (v${key.version})?`)) return;
    try {
      await secretsFetch(`/keys/${key.id}/rotate`, 'POST');
      setMsg({ type: 'success', text: 'Key rotated successfully' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: 'Rotation failed' });
    }
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <>
      <div className="page-header">
        <h1>Key Management</h1>
        <p>Generate, import, and manage cryptographic keys and certificates</p>
      </div>

      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">Keys ({filtered.length})</span>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            <select className="filter-select" value={filterType} onChange={e => setFilterType(e.target.value)}>
              <option value="">All Types</option>
              {KEY_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
            </select>
            <select className="filter-select" value={filterProvider} onChange={e => setFilterProvider(e.target.value)}>
              <option value="">All Providers</option>
              {providers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
            <select className="filter-select" value={filterActive} onChange={e => setFilterActive(e.target.value)}>
              <option value="">All Status</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </select>
            {canDo('WriteKeys') && <button className="btn btn-primary btn-small" onClick={() => setModal('generate')}>Generate Key</button>}
            {canDo('WriteKeys') && <button className="btn btn-secondary btn-small" onClick={() => setModal('import')}>Import Key</button>}
            <button className="btn btn-secondary btn-small" onClick={() => setModal('certificate')}>Generate Certificate</button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th><th>Type</th><th>Provider</th><th>Fingerprint</th>
                <th>Status</th><th>Next Rotation</th><th>Version</th><th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map(k => (
                <tr key={k.id}>
                  <td><strong>{k.name}</strong>{k.description && <><br/><span style={{color:'var(--text-muted)', fontSize: 11}}>{k.description}</span></>}</td>
                  <td><span className="badge badge-cyan">{k.keyType}</span></td>
                  <td>{providerName(k.providerId)}</td>
                  <td><span className="mono truncate" title={k.fingerprint}>{k.fingerprint?.slice(0,16)}...</span></td>
                  <td><span className={`badge ${k.active ? 'badge-green' : 'badge-muted'}`}>{k.active ? 'Active' : 'Inactive'}</span></td>
                  <td>{k.nextRotationAt ? new Date(k.nextRotationAt).toLocaleDateString() : '—'}</td>
                  <td>v{k.version}</td>
                  <td>
                    <div style={{display:'flex', gap:4, flexWrap:'wrap'}}>
                      <button className="btn btn-secondary btn-small" onClick={() => setModal({type:'detail', key:k})}>Details</button>
                      {canDo('WriteKeys') && <button className="btn btn-secondary btn-small" onClick={() => handleRotate(k)}>Rotate</button>}
                      {canDo('ManageKeyAccess') && <button className="btn btn-secondary btn-small" onClick={() => setModal({type:'access', key:k})}>Access</button>}
                      <button className="btn btn-secondary btn-small" onClick={() => setModal({type:'usage', key:k})}>Log</button>
                      {canDo('WriteKeys') && <button className="btn btn-secondary btn-small" onClick={() => setModal({type:'policy', key:k})}>Policy</button>}
                      {canDo('WriteKeys') && <button className="btn btn-danger btn-small" onClick={() => handleDelete(k)}>Delete</button>}
                    </div>
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && <tr><td colSpan={8} style={{textAlign:'center',padding:32,color:'var(--text-muted)'}}>No keys found</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {modal === 'generate' && <GenerateKeyModal providers={providers} onClose={() => setModal(null)} onDone={() => { setModal(null); load(); setMsg({type:'success',text:'Key generated'}); }} />}
      {modal === 'import' && <ImportKeyModal providers={providers} onClose={() => setModal(null)} onDone={() => { setModal(null); load(); setMsg({type:'success',text:'Key imported'}); }} />}
      {modal === 'certificate' && <CertificateModal providers={providers} onClose={() => setModal(null)} onDone={() => { setModal(null); load(); setMsg({type:'success',text:'Certificate generated'}); }} />}
      {modal?.type === 'detail' && <KeyDetailModal keyData={modal.key} onClose={() => setModal(null)} />}
      {modal?.type === 'access' && <AccessModal keyData={modal.key} onClose={() => setModal(null)} />}
      {modal?.type === 'usage' && <UsageLogModal keyData={modal.key} onClose={() => setModal(null)} />}
      {modal?.type === 'policy' && <AssignPolicyModal keyData={modal.key} policies={policies} onClose={() => setModal(null)} onDone={() => { setModal(null); load(); }} />}
    </>
  );
}

function GenerateKeyModal({ providers, onClose, onDone }) {
  const [form, setForm] = useState({ name:'', description:'', keyType:'RSA-2048', keyUsage:'general', providerId: providers[0]?.id || 1 });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const submit = async () => {
    setSaving(true);
    try {
      await secretsFetch('/keys/generate', 'POST', form);
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
        <h3 style={{ marginBottom: 16 }}>Generate Key</h3>
        {error && <div className="admin-msg admin-msg-error" style={{marginBottom:12}}>{error}</div>}
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Name</label>
            <input className="filter-input" value={form.name} onChange={e => setForm({...form, name:e.target.value})} placeholder="My RSA Key" />
          </div>
          <div className="form-field">
            <label className="form-label">Description</label>
            <input className="filter-input" value={form.description} onChange={e => setForm({...form, description:e.target.value})} placeholder="Optional description" />
          </div>
          <div className="form-field">
            <label className="form-label">Key Type</label>
            <select className="filter-select" value={form.keyType} onChange={e => setForm({...form, keyType:e.target.value})}>
              {KEY_TYPES.filter(t => t !== 'X509').map(t => <option key={t} value={t}>{t}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Usage</label>
            <select className="filter-select" value={form.keyUsage} onChange={e => setForm({...form, keyUsage:e.target.value})}>
              {KEY_USAGES.map(u => <option key={u} value={u}>{u}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Provider</label>
            <select className="filter-select" value={form.providerId} onChange={e => setForm({...form, providerId:Number(e.target.value)})}>
              {providers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit} disabled={saving || !form.name}>{saving ? 'Generating...' : 'Generate'}</button>
        </div>
      </div>
    </div>
  );
}

function ImportKeyModal({ providers, onClose, onDone }) {
  const [form, setForm] = useState({ name:'', description:'', keyType:'RSA-2048', keyUsage:'general', providerId: providers[0]?.id || 1, publicPem:'', privatePem:'', certPem:'' });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const submit = async () => {
    setSaving(true);
    try {
      await secretsFetch('/keys/import', 'POST', form);
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
        <h3 style={{ marginBottom: 16 }}>Import Key</h3>
        {error && <div className="admin-msg admin-msg-error" style={{marginBottom:12}}>{error}</div>}
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Name</label>
            <input className="filter-input" value={form.name} onChange={e => setForm({...form, name:e.target.value})} placeholder="My Imported Key" />
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
            <label className="form-label">Provider</label>
            <select className="filter-select" value={form.providerId} onChange={e => setForm({...form, providerId:Number(e.target.value)})}>
              {providers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
          </div>
        </div>
        <div style={{marginTop:16, display:'flex', flexDirection:'column', gap:12}}>
          <div className="form-field">
            <label className="form-label">Public Key PEM</label>
            <textarea className="admin-pem-input" rows={4} value={form.publicPem} onChange={e => setForm({...form, publicPem:e.target.value})} placeholder="-----BEGIN PUBLIC KEY-----" />
          </div>
          <div className="form-field">
            <label className="form-label">Private Key PEM</label>
            <textarea className="admin-pem-input" rows={4} value={form.privatePem} onChange={e => setForm({...form, privatePem:e.target.value})} placeholder="-----BEGIN RSA PRIVATE KEY-----" />
          </div>
          <div className="form-field">
            <label className="form-label">Certificate PEM (optional)</label>
            <textarea className="admin-pem-input" rows={4} value={form.certPem} onChange={e => setForm({...form, certPem:e.target.value})} placeholder="-----BEGIN CERTIFICATE-----" />
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit} disabled={saving || !form.name}>{saving ? 'Importing...' : 'Import'}</button>
        </div>
      </div>
    </div>
  );
}

function CertificateModal({ providers, onClose, onDone }) {
  const [form, setForm] = useState({ name:'', description:'', providerId: providers[0]?.id || 1, commonName:'', organization:'', country:'', validityDays:'365', sans:'', keyBits:'2048' });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const submit = async () => {
    setSaving(true);
    try {
      const body = { ...form, sans: form.sans ? form.sans.split(',').map(s => s.trim()) : [], keyBits: Number(form.keyBits), validityDays: Number(form.validityDays) };
      await secretsFetch('/keys/certificate', 'POST', body);
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
        <h3 style={{ marginBottom: 16 }}>Generate Certificate</h3>
        {error && <div className="admin-msg admin-msg-error" style={{marginBottom:12}}>{error}</div>}
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Name</label>
            <input className="filter-input" value={form.name} onChange={e => setForm({...form, name:e.target.value})} placeholder="My TLS Cert" />
          </div>
          <div className="form-field">
            <label className="form-label">Common Name</label>
            <input className="filter-input" value={form.commonName} onChange={e => setForm({...form, commonName:e.target.value})} placeholder="example.com" />
          </div>
          <div className="form-field">
            <label className="form-label">Organization</label>
            <input className="filter-input" value={form.organization} onChange={e => setForm({...form, organization:e.target.value})} />
          </div>
          <div className="form-field">
            <label className="form-label">Country</label>
            <input className="filter-input" value={form.country} onChange={e => setForm({...form, country:e.target.value})} maxLength={2} placeholder="US" />
          </div>
          <div className="form-field">
            <label className="form-label">Validity (days)</label>
            <input className="filter-input" type="number" value={form.validityDays} onChange={e => setForm({...form, validityDays:e.target.value})} />
          </div>
          <div className="form-field">
            <label className="form-label">Key Bits</label>
            <select className="filter-select" value={form.keyBits} onChange={e => setForm({...form, keyBits:e.target.value})}>
              <option value="2048">2048</option><option value="3072">3072</option><option value="4096">4096</option>
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">SANs (comma-separated)</label>
            <input className="filter-input" value={form.sans} onChange={e => setForm({...form, sans:e.target.value})} placeholder="www.example.com, 10.0.0.1" />
          </div>
          <div className="form-field">
            <label className="form-label">Provider</label>
            <select className="filter-select" value={form.providerId} onChange={e => setForm({...form, providerId:Number(e.target.value)})}>
              {providers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit} disabled={saving || !form.name || !form.commonName}>{saving ? 'Generating...' : 'Generate'}</button>
        </div>
      </div>
    </div>
  );
}

function KeyDetailModal({ keyData, onClose }) {
  const fields = [
    ['ID', keyData.id],
    ['UID', keyData.uid],
    ['Name', keyData.name],
    ['Description', keyData.description || '—'],
    ['Type', keyData.keyType],
    ['Usage', keyData.keyUsage],
    ['Key Size', `${keyData.keySizeBits} bits`],
    ['Version', `v${keyData.version}`],
    ['Active', keyData.active ? 'Yes' : 'No'],
    ['Not Before', keyData.notBefore ? new Date(keyData.notBefore).toLocaleString() : '—'],
    ['Not After', keyData.notAfter ? new Date(keyData.notAfter).toLocaleString() : '—'],
    ['Created', new Date(keyData.createdAt).toLocaleString()],
  ];

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()}>
        <h3 style={{ marginBottom: 16 }}>Key Details</h3>
        <div className="admin-readonly-grid">
          {fields.map(([label, value]) => (
            <div className="admin-ro-field" key={label}>
              <span className="admin-ro-label">{label}</span>
              <span className="admin-ro-value">{value}</span>
            </div>
          ))}
          <div className="admin-ro-field" style={{gridColumn:'1 / -1'}}>
            <span className="admin-ro-label">Fingerprint</span>
            <span className="admin-ro-value mono" style={{fontSize:12}}>{keyData.fingerprint}</span>
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-secondary" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}

function AccessModal({ keyData, onClose }) {
  const [access, setAccess] = useState([]);
  const [loading, setLoading] = useState(true);
  const [userId, setUserId] = useState('');
  const [permission, setPermission] = useState('use');

  const loadAccess = useCallback(async () => {
    try {
      const data = await secretsFetch(`/keys/${keyData.id}/access`);
      setAccess(data || []);
    } catch { /* ignore */ }
    finally { setLoading(false); }
  }, [keyData.id]);

  useEffect(() => { loadAccess(); }, [loadAccess]);

  const grant = async () => {
    if (!userId) return;
    try {
      await secretsFetch(`/keys/${keyData.id}/access`, 'POST', { userId: Number(userId), permission });
      setUserId('');
      loadAccess();
    } catch { /* ignore */ }
  };

  const revoke = async (uid) => {
    try {
      await secretsFetch(`/keys/${keyData.id}/access/${uid}`, 'DELETE');
      loadAccess();
    } catch { /* ignore */ }
  };

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()}>
        <h3 style={{ marginBottom: 16 }}>Key Access — {keyData.name}</h3>
        {loading ? <div className="spinner-wrap"><div className="spinner" /></div> : (
          <>
            <div className="table-wrap">
              <table>
                <thead><tr><th>User ID</th><th>Permission</th><th>Granted By</th><th></th></tr></thead>
                <tbody>
                  {access.map(a => (
                    <tr key={a.id}>
                      <td>{a.userId}</td>
                      <td><span className="badge badge-cyan">{a.permission}</span></td>
                      <td>{a.grantedBy}</td>
                      <td><button className="btn btn-danger btn-small" onClick={() => revoke(a.userId)}>Revoke</button></td>
                    </tr>
                  ))}
                  {access.length === 0 && <tr><td colSpan={4} style={{textAlign:'center', color:'var(--text-muted)'}}>No access grants</td></tr>}
                </tbody>
              </table>
            </div>
            <div style={{display:'flex', gap:8, marginTop:16, alignItems:'center'}}>
              <input className="filter-input" placeholder="User ID" value={userId} onChange={e => setUserId(e.target.value)} style={{width:100}} />
              <select className="filter-select" value={permission} onChange={e => setPermission(e.target.value)}>
                <option value="use">use</option><option value="manage">manage</option>
              </select>
              <button className="btn btn-primary btn-small" onClick={grant}>Grant</button>
            </div>
          </>
        )}
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-secondary" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}

function UsageLogModal({ keyData, onClose }) {
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    secretsFetch(`/keys/${keyData.id}/usage`).then(d => setLogs(d || [])).catch(() => {}).finally(() => setLoading(false));
  }, [keyData.id]);

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()} style={{maxWidth:720}}>
        <h3 style={{ marginBottom: 16 }}>Usage Log — {keyData.name}</h3>
        {loading ? <div className="spinner-wrap"><div className="spinner" /></div> : (
          <div className="table-wrap">
            <table>
              <thead><tr><th>Action</th><th>User</th><th>IP</th><th>Time</th></tr></thead>
              <tbody>
                {logs.map(l => (
                  <tr key={l.id}>
                    <td><span className="badge badge-muted">{l.action}</span></td>
                    <td>{l.userId}</td>
                    <td>{l.ipAddress || '—'}</td>
                    <td>{new Date(l.createdAt).toLocaleString()}</td>
                  </tr>
                ))}
                {logs.length === 0 && <tr><td colSpan={4} style={{textAlign:'center', color:'var(--text-muted)'}}>No usage log entries</td></tr>}
              </tbody>
            </table>
          </div>
        )}
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-secondary" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}

function AssignPolicyModal({ keyData, policies, onClose, onDone }) {
  const [policyId, setPolicyId] = useState(policies[0]?.id || '');
  const [saving, setSaving] = useState(false);

  const submit = async () => {
    setSaving(true);
    try {
      await secretsFetch(`/keys/${keyData.id}/policy`, 'POST', { policyId: Number(policyId) });
      onDone();
    } catch { setSaving(false); }
  };

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()}>
        <h3 style={{ marginBottom: 16 }}>Assign Rotation Policy — {keyData.name}</h3>
        <div className="form-field">
          <label className="form-label">Policy</label>
          <select className="filter-select" value={policyId} onChange={e => setPolicyId(e.target.value)}>
            {policies.map(p => <option key={p.id} value={p.id}>{p.name} ({p.rotationInterval})</option>)}
          </select>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit} disabled={saving}>{saving ? 'Assigning...' : 'Assign'}</button>
        </div>
      </div>
    </div>
  );
}
