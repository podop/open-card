import React, { useState, useEffect, useCallback } from 'react';
import { fetchJSON } from '../api';
import { useAuth } from '../AuthContext';
import { NetworkIcon } from '../components/NetworkBadge';
import { useNetworks } from '../hooks/useNetworks';

const PROTOCOLS = ['2.1.0', '2.2.0', '2.3.1'];

export default function CardRanges() {
  const { canDo } = useAuth();
  const { networks } = useNetworks();
  const networkNames = networks.map(n => n.name);
  const [ranges, setRanges] = useState([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [filterNet, setFilterNet] = useState('');
  const [filterActive, setFilterActive] = useState('');
  const [showForm, setShowForm] = useState(false);

  const load = useCallback(async () => {
    try {
      const data = await fetchJSON('/card-ranges');
      setRanges(data.data || []);
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to load card ranges' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const filtered = ranges.filter(r => {
    if (filterNet && r.network !== filterNet) return false;
    if (filterActive === 'active' && !r.active) return false;
    if (filterActive === 'inactive' && r.active) return false;
    return true;
  });

  const handleDelete = async (r) => {
    if (!confirm(`Delete range ${r.startRange} - ${r.endRange}?`)) return;
    try {
      await fetchJSON(`/card-ranges?start=${encodeURIComponent(r.startRange)}&end=${encodeURIComponent(r.endRange)}`, {}, 'DELETE');
      setMsg({ type: 'success', text: 'Card range deactivated' });
      load();
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to delete: ' + e.message });
    }
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <>
      <div className="page-header">
        <h1>Card BIN Ranges</h1>
        <p>Manage BIN ranges for which this ACS provides authentication</p>
      </div>

      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">BIN Ranges ({filtered.length})</span>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <select className="filter-select" value={filterNet} onChange={e => setFilterNet(e.target.value)}>
              <option value="">All Networks</option>
              {networkNames.map(n => <option key={n} value={n}>{n}</option>)}
            </select>
            <select className="filter-select" value={filterActive} onChange={e => setFilterActive(e.target.value)}>
              <option value="">All Status</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </select>
            {canDo('WriteCardRanges') && <button className="btn btn-primary btn-small" onClick={() => setShowForm(true)}>Add BIN Range</button>}
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Network</th>
                <th>Start Range</th>
                <th>End Range</th>
                <th>Min Protocol</th>
                <th>Max Protocol</th>
                <th>Active</th>
                <th>Created</th>
                {canDo('WriteCardRanges') && <th>Actions</th>}
              </tr>
            </thead>
            <tbody>
              {filtered.map(r => (
                <tr key={r.id || `${r.startRange}-${r.endRange}`}>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <NetworkIcon network={r.network} size={18} />
                      <span style={{ textTransform: 'capitalize' }}>{r.network}</span>
                    </div>
                  </td>
                  <td className="mono">{r.startRange}</td>
                  <td className="mono">{r.endRange}</td>
                  <td>{r.minProtocol}</td>
                  <td>{r.maxProtocol}</td>
                  <td>
                    <span className={`badge ${r.active ? 'badge-green' : 'badge-muted'}`}>
                      {r.active ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td style={{ fontSize: 12, color: 'var(--text-muted)' }}>
                    {r.createdAt ? new Date(r.createdAt).toLocaleDateString() : '-'}
                  </td>
                  {canDo('WriteCardRanges') && (
                    <td>
                      <button className="btn btn-danger btn-small" onClick={() => handleDelete(r)} title="Deactivate range">
                        Delete
                      </button>
                    </td>
                  )}
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr><td colSpan={8} className="empty-state">No card ranges found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {showForm && (
        <CardRangeForm
          networkNames={networkNames}
          onClose={() => setShowForm(false)}
          onSaved={() => { setShowForm(false); load(); setMsg({ type: 'success', text: 'BIN range created' }); }}
          onError={e => setMsg({ type: 'error', text: e })}
        />
      )}
    </>
  );
}

function CardRangeForm({ networkNames, onClose, onSaved, onError }) {
  const [form, setForm] = useState({
    network: networkNames[0] || 'visa',
    startRange: '',
    endRange: '',
    minProtocol: '2.2.0',
    maxProtocol: '2.3.1',
  });
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!form.startRange || !form.endRange) {
      onError('Start and end range are required');
      return;
    }
    if (form.startRange > form.endRange) {
      onError('Start range must be less than or equal to end range');
      return;
    }
    setSaving(true);
    try {
      await fetchJSON('/card-ranges', {}, 'POST', form);
      onSaved();
    } catch (e) {
      onError('Failed to create BIN range: ' + e.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()}>
        <h3 style={{ marginBottom: 16 }}>Add BIN Range</h3>
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Network</label>
            <select className="filter-select" value={form.network} onChange={e => setForm(f => ({ ...f, network: e.target.value }))}>
              {networkNames.map(n => <option key={n} value={n}>{n}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Start Range</label>
            <input className="filter-input mono" value={form.startRange} onChange={e => setForm(f => ({ ...f, startRange: e.target.value }))} placeholder="400000" />
            <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>BIN prefix (e.g. 6 digits)</span>
          </div>
          <div className="form-field">
            <label className="form-label">End Range</label>
            <input className="filter-input mono" value={form.endRange} onChange={e => setForm(f => ({ ...f, endRange: e.target.value }))} placeholder="499999" />
          </div>
          <div className="form-field">
            <label className="form-label">Min Protocol</label>
            <select className="filter-select" value={form.minProtocol} onChange={e => setForm(f => ({ ...f, minProtocol: e.target.value }))}>
              {PROTOCOLS.map(p => <option key={p} value={p}>{p}</option>)}
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Max Protocol</label>
            <select className="filter-select" value={form.maxProtocol} onChange={e => setForm(f => ({ ...f, maxProtocol: e.target.value }))}>
              {PROTOCOLS.map(p => <option key={p} value={p}>{p}</option>)}
            </select>
          </div>
        </div>
        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Create'}</button>
        </div>
      </div>
    </div>
  );
}