import React, { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../AuthContext';
import KeySourceModal from '../components/KeySourceModal';

const AUTH_BASE = '/auth';

async function authFetch(path, method = 'GET', body = null) {
  const opts = { method, headers: {} };
  const token = localStorage.getItem('accessToken');
  if (token) opts.headers['Authorization'] = `Bearer ${token}`;
  if (body !== null) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(AUTH_BASE + path, opts);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

// ── Main Page ─────────────────────────────────────────────────

export default function IdentitySettings() {
  const [tab, setTab] = useState('settings');

  return (
    <>
      <div className="page-header">
        <h1>Identity &amp; Access Management</h1>
        <p>JWT authentication, user groups, and access policies</p>
      </div>
      <div className="admin-tabs">
        <button className={`admin-tab${tab === 'settings' ? ' active' : ''}`} onClick={() => setTab('settings')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" width="16" height="16">
            <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
          Settings
        </button>
        <button className={`admin-tab${tab === 'groups' ? ' active' : ''}`} onClick={() => setTab('groups')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" width="16" height="16">
            <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>
          </svg>
          Groups &amp; Policies
        </button>
        <button className={`admin-tab${tab === 'users' ? ' active' : ''}`} onClick={() => setTab('users')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" width="16" height="16">
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>
          </svg>
          Users
        </button>
      </div>
      {tab === 'settings' && <SettingsTab />}
      {tab === 'groups' && <GroupsTab />}
      {tab === 'users' && <UsersTab />}
    </>
  );
}

// ── Settings Tab ──────────────────────────────────────────────

function SettingsTab() {
  const { canDo } = useAuth();
  const [settings, setSettings] = useState(null);
  const [form, setForm] = useState({});
  const [msg, setMsg] = useState(null);

  const load = useCallback(async () => {
    const data = await authFetch('/settings');
    setSettings(data);
    setForm({
      tokenExpiryMinutes: data.tokenExpiryMinutes || '60',
      refreshExpiryHours: data.refreshExpiryHours || '720',
      active: data.active || 'true',
    });
  }, []);

  useEffect(() => { load(); }, [load]);

  const save = async () => {
    try {
      await authFetch('/settings', 'PUT', form);
      setMsg({ type: 'success', text: 'Settings saved' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: e.message });
    }
  };

  const regenerateRSA = async () => {
    if (!window.confirm('Regenerate RSA key pair? All existing tokens will be invalidated.')) return;
    try {
      await authFetch('/settings', 'PUT', { regenerateRSA: true });
      setMsg({ type: 'success', text: 'RSA key pair regenerated. Users will need to re-login.' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: e.message });
    }
  };

  if (!settings) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <div className="card" style={{ marginTop: 16, padding: 24 }}>
      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}
      <div className="form-grid">
        <div className="form-field">
          <label className="form-label">Authentication Active</label>
          <select className="filter-select" value={form.active} onChange={e => setForm(f => ({ ...f, active: e.target.value }))}>
            <option value="true">Active</option>
            <option value="false">Disabled (all requests allowed)</option>
          </select>
        </div>
        <div className="form-field">
          <label className="form-label">Access Token Expiry (minutes)</label>
          <input className="filter-input" type="number" value={form.tokenExpiryMinutes}
            onChange={e => setForm(f => ({ ...f, tokenExpiryMinutes: e.target.value }))} />
        </div>
        <div className="form-field">
          <label className="form-label">Refresh Token Expiry (hours)</label>
          <input className="filter-input" type="number" value={form.refreshExpiryHours}
            onChange={e => setForm(f => ({ ...f, refreshExpiryHours: e.target.value }))} />
        </div>
      </div>
      {canDo('WriteIdentitySettings') && <div style={{ marginTop: 16, display: 'flex', gap: 12 }}>
        <button className="btn btn-primary" onClick={save}>Save Settings</button>
      </div>}

      <RSAKeySection settings={settings} onRegenerated={() => { setMsg({ type: 'success', text: 'RSA key pair updated. Users will need to re-login.' }); load(); }} onError={(e) => setMsg({ type: 'error', text: e })} />
    </div>
  );
}

// ── RSA Key Section ───────────────────────────────────────────

function RSAKeySection({ settings, onRegenerated, onError }) {
  const { canDo } = useAuth();
  const [showModal, setShowModal] = useState(false);
  const [rsaSource, setRsaSource] = useState(null);

  const regenerateInternal = async () => {
    if (!window.confirm('Regenerate RSA key pair internally? All existing tokens will be invalidated.')) return;
    try {
      await authFetch('/settings', 'PUT', { regenerateRSA: true });
      onRegenerated();
    } catch (e) {
      onError(e.message);
    }
  };

  const handleSecretsManagerKey = async (material, keyId) => {
    setShowModal(false);
    try {
      await authFetch('/settings', 'PUT', { rsaFromSecretsManager: true, secretsKeyId: keyId });
      onRegenerated();
    } catch (e) {
      onError(e.message);
    }
  };

  return (
    <div className="admin-section" style={{ marginTop: 24 }}>
      <div className="admin-section-title">RSA Key Pair (RS256)</div>
      <div style={{ fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.6, marginBottom: 16, padding: '10px 14px', background: 'var(--bg-input)', borderRadius: 'var(--radius)', border: '1px solid var(--border)' }}>
        RSA Key Pair (RS256) is used to sign and verify JWT access tokens.
        All authenticated API requests are validated against this public key.
        <strong> Regenerating invalidates all active sessions — users must re-login.</strong>
      </div>

      {canDo('WriteIdentitySettings') && <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <button className="btn btn-danger btn-small" onClick={regenerateInternal}>Regenerate Internally</button>
        <button className="btn btn-secondary btn-small" onClick={() => { setRsaSource('generate'); setShowModal(true); }}>Generate via Secrets Manager</button>
        <button className="btn btn-secondary btn-small" onClick={() => { setRsaSource('existing'); setShowModal(true); }}>Use Existing from Secrets Manager</button>
      </div>}

      <pre style={{
        background: 'var(--bg-input)', padding: 12, borderRadius: 'var(--radius)',
        fontSize: 11, overflow: 'auto', maxHeight: 200,
        whiteSpace: 'pre-wrap', wordBreak: 'break-all',
        border: '1px solid var(--border)',
      }}>
        {settings.rsaPublicKey}
      </pre>

      {showModal && (
        <KeySourceModal
          title="RSA Key for JWT Signing"
          keyType="RSA-2048"
          keyUsage="signing"
          onSelect={handleSecretsManagerKey}
          onClose={() => setShowModal(false)}
        />
      )}
    </div>
  );
}

// ── Groups Tab ────────────────────────────────────────────────

function GroupsTab() {
  const { canDo } = useAuth();
  const [groups, setGroups] = useState([]);
  const [allActions, setAllActions] = useState([]);
  const [editing, setEditing] = useState(null);
  const [msg, setMsg] = useState(null);

  const load = useCallback(async () => {
    const [g, s] = await Promise.all([authFetch('/groups'), authFetch('/settings')]);
    setGroups(g);
    setAllActions(s.allActions || []);
  }, []);

  useEffect(() => { load(); }, [load]);

  const openNew = () => setEditing({ name: '', description: '', actions: [] });
  const openEdit = (g) => setEditing({ ...g, actions: [...(g.actions || [])] });

  const toggleAction = (action) => {
    setEditing(e => {
      const has = e.actions.includes(action);
      return { ...e, actions: has ? e.actions.filter(a => a !== action) : [...e.actions, action] };
    });
  };

  const selectAll = () => setEditing(e => ({ ...e, actions: [...allActions] }));
  const selectNone = () => setEditing(e => ({ ...e, actions: [] }));

  const save = async () => {
    try {
      if (editing.id) {
        await authFetch(`/groups/${editing.id}`, 'PUT', editing);
      } else {
        await authFetch('/groups', 'POST', editing);
      }
      setEditing(null);
      setMsg({ type: 'success', text: 'Group saved' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: e.message });
    }
  };

  const remove = async (id) => {
    if (!window.confirm('Delete this group?')) return;
    await authFetch(`/groups/${id}`, 'DELETE');
    load();
  };

  return (
    <div style={{ marginTop: 16 }}>
      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
        <h3 style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-primary)' }}>User Groups</h3>
        {canDo('ManageGroups') && <button className="btn btn-primary btn-small" onClick={openNew}>Add Group</button>}
      </div>
      <div className="card" style={{ padding: 0 }}>
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Description</th>
              <th>Actions</th>
              <th style={{ width: 140 }}></th>
            </tr>
          </thead>
          <tbody>
            {groups.map(g => (
              <tr key={g.id}>
                <td><strong>{g.name}</strong></td>
                <td>{g.description}</td>
                <td><span className="badge badge-green">{(g.actions || []).length} actions</span></td>
                {canDo('ManageGroups') && (
                  <td>
                    <button className="btn btn-secondary btn-small" onClick={() => openEdit(g)}>Edit</button>{' '}
                    <button className="btn btn-danger btn-small" onClick={() => remove(g.id)}>Delete</button>
                  </td>
                )}
              </tr>
            ))}
            {groups.length === 0 && <tr><td colSpan={4} style={{ textAlign: 'center', color: 'var(--text-muted)', padding: 24 }}>No groups defined</td></tr>}
          </tbody>
        </table>
      </div>

      {editing && (
        <div className="admin-modal-overlay" onClick={() => setEditing(null)}>
          <div className="admin-modal" onClick={e => e.stopPropagation()} style={{ maxWidth: 720 }}>
            <h3 style={{ marginBottom: 16, fontSize: 16, fontWeight: 600 }}>{editing.id ? 'Edit Group' : 'New Group'}</h3>
            <div className="form-grid">
              <div className="form-field">
                <label className="form-label">Name</label>
                <input className="filter-input" value={editing.name} onChange={e => setEditing(ed => ({ ...ed, name: e.target.value }))} />
              </div>
              <div className="form-field">
                <label className="form-label">Description</label>
                <input className="filter-input" value={editing.description} onChange={e => setEditing(ed => ({ ...ed, description: e.target.value }))} />
              </div>
            </div>
            <div style={{ marginTop: 16 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <label className="form-label" style={{ margin: 0 }}>Permissions ({editing.actions.length}/{allActions.length})</label>
                <div style={{ display: 'flex', gap: 8 }}>
                  <button className="btn btn-secondary btn-small" onClick={selectAll}>Select All</button>
                  <button className="btn btn-secondary btn-small" onClick={selectNone}>Clear</button>
                </div>
              </div>
              <div style={{
                display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
                gap: '4px 16px', maxHeight: 300, overflowY: 'auto',
                background: 'var(--bg-input)', padding: 12, borderRadius: 'var(--radius)',
                border: '1px solid var(--border)',
              }}>
                {allActions.map(a => (
                  <label key={a} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, cursor: 'pointer', padding: '3px 0', color: 'var(--text-primary)' }}>
                    <input type="checkbox" checked={editing.actions.includes(a)} onChange={() => toggleAction(a)} />
                    {a}
                  </label>
                ))}
              </div>
            </div>
            <div style={{ marginTop: 16, display: 'flex', gap: 8 }}>
              <button className="btn btn-primary" onClick={save}>Save</button>
              <button className="btn btn-secondary" onClick={() => setEditing(null)}>Cancel</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Users Tab ─────────────────────────────────────────────────

function UsersTab() {
  const { canDo } = useAuth();
  const [users, setUsers] = useState([]);
  const [groups, setGroups] = useState([]);
  const [editing, setEditing] = useState(null);
  const [msg, setMsg] = useState(null);

  const load = useCallback(async () => {
    const [u, g] = await Promise.all([authFetch('/users'), authFetch('/groups')]);
    setUsers(u);
    setGroups(g);
  }, []);

  useEffect(() => { load(); }, [load]);

  const openNew = () => setEditing({ username: '', password: '', displayName: '', active: true, groupIds: [] });
  const openEdit = (u) => setEditing({
    id: u.id,
    username: u.username,
    displayName: u.displayName,
    active: u.active,
    password: '',
    groupIds: (u.groups || []).map(g => g.id),
  });

  const toggleGroup = (gid) => {
    setEditing(e => {
      const has = e.groupIds.includes(gid);
      return { ...e, groupIds: has ? e.groupIds.filter(id => id !== gid) : [...e.groupIds, gid] };
    });
  };

  const save = async () => {
    try {
      const body = { ...editing };
      if (editing.id) {
        const update = { displayName: body.displayName, active: body.active, groupIds: body.groupIds };
        if (body.password) update.password = body.password;
        await authFetch(`/users/${editing.id}`, 'PUT', update);
      } else {
        await authFetch('/users', 'POST', body);
      }
      setEditing(null);
      setMsg({ type: 'success', text: 'User saved' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: e.message });
    }
  };

  const remove = async (id) => {
    if (!window.confirm('Delete this user?')) return;
    await authFetch(`/users/${id}`, 'DELETE');
    load();
  };

  return (
    <div style={{ marginTop: 16 }}>
      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
        <h3 style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-primary)' }}>Users</h3>
        {canDo('ManageUsers') && <button className="btn btn-primary btn-small" onClick={openNew}>Add User</button>}
      </div>
      <div className="card" style={{ padding: 0 }}>
        <table className="data-table">
          <thead>
            <tr>
              <th>Username</th>
              <th>Display Name</th>
              <th>Groups</th>
              <th>Status</th>
              <th style={{ width: 140 }}></th>
            </tr>
          </thead>
          <tbody>
            {users.map(u => (
              <tr key={u.id}>
                <td><strong>{u.username}</strong></td>
                <td>{u.displayName}</td>
                <td>{(u.groups || []).map(g => g.name).join(', ') || <span style={{ color: 'var(--text-muted)' }}>None</span>}</td>
                <td>
                  <span className={`badge ${u.active ? 'badge-green' : 'badge-red'}`}>
                    {u.active ? 'Active' : 'Disabled'}
                  </span>
                </td>
                {canDo('ManageUsers') && (
                  <td>
                    <button className="btn btn-secondary btn-small" onClick={() => openEdit(u)}>Edit</button>{' '}
                    <button className="btn btn-danger btn-small" onClick={() => remove(u.id)}>Delete</button>
                  </td>
                )}
              </tr>
            ))}
            {users.length === 0 && <tr><td colSpan={5} style={{ textAlign: 'center', color: 'var(--text-muted)', padding: 24 }}>No users</td></tr>}
          </tbody>
        </table>
      </div>

      {editing && (
        <div className="admin-modal-overlay" onClick={() => setEditing(null)}>
          <div className="admin-modal" onClick={e => e.stopPropagation()}>
            <h3 style={{ marginBottom: 16, fontSize: 16, fontWeight: 600 }}>{editing.id ? 'Edit User' : 'New User'}</h3>
            <div className="form-grid">
              <div className="form-field">
                <label className="form-label">Username</label>
                <input className="filter-input" value={editing.username} disabled={!!editing.id}
                  onChange={e => setEditing(ed => ({ ...ed, username: e.target.value }))} />
              </div>
              <div className="form-field">
                <label className="form-label">Display Name</label>
                <input className="filter-input" value={editing.displayName}
                  onChange={e => setEditing(ed => ({ ...ed, displayName: e.target.value }))} />
              </div>
              <div className="form-field">
                <label className="form-label">{editing.id ? 'New Password (leave blank to keep)' : 'Password'}</label>
                <input className="filter-input" type="password" value={editing.password}
                  onChange={e => setEditing(ed => ({ ...ed, password: e.target.value }))}
                  required={!editing.id} />
              </div>
              <div className="form-field">
                <label className="form-label">Status</label>
                <select className="filter-select" value={editing.active ? 'true' : 'false'}
                  onChange={e => setEditing(ed => ({ ...ed, active: e.target.value === 'true' }))}>
                  <option value="true">Active</option>
                  <option value="false">Disabled</option>
                </select>
              </div>
            </div>
            <div style={{ marginTop: 16 }}>
              <label className="form-label" style={{ marginBottom: 8, display: 'block' }}>Groups</label>
              <div style={{
                background: 'var(--bg-input)', padding: 12, borderRadius: 'var(--radius)',
                display: 'flex', flexDirection: 'column', gap: 6,
                border: '1px solid var(--border)',
              }}>
                {groups.map(g => (
                  <label key={g.id} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, cursor: 'pointer', color: 'var(--text-primary)' }}>
                    <input type="checkbox" checked={editing.groupIds.includes(g.id)} onChange={() => toggleGroup(g.id)} />
                    <strong>{g.name}</strong>
                    <span style={{ color: 'var(--text-muted)', fontSize: 11 }}>
                      ({(g.actions || []).length} actions)
                    </span>
                  </label>
                ))}
                {groups.length === 0 && <span style={{ color: 'var(--text-muted)', fontSize: 12 }}>No groups created yet</span>}
              </div>
            </div>
            <div style={{ marginTop: 16, display: 'flex', gap: 8 }}>
              <button className="btn btn-primary" onClick={save}>Save</button>
              <button className="btn btn-secondary" onClick={() => setEditing(null)}>Cancel</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
