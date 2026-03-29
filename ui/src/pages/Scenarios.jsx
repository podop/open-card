import React, { useEffect, useState, useCallback } from 'react';
import { fetchJSON } from '../api';
import { NetworkBadge, NetworkIcon } from '../components/NetworkBadge';
import { SearchInput } from '../components/SearchInput';
import { useNetworks } from '../hooks/useNetworks';

const FLOW_TYPES = [
  { value: 'frictionless', label: 'Frictionless' },
  { value: 'challenge_otp', label: 'Challenge (OTP)' },
  { value: 'challenge_oob', label: 'Challenge (OOB)' },
  { value: 'decoupled', label: 'Decoupled' },
  { value: 'challenge_unavailable', label: 'Challenge (Unavailable)' },
  { value: 'oreq', label: 'OReq/ORes (Operational)' },
];

const CHANNELS = [
  { value: '01', label: 'App (01)' },
  { value: '02', label: 'Browser (02)' },
  { value: '03', label: '3RI (03)' },
];


const ACTIONS = {
  frictionless: [{ value: 'none', label: 'N/A (Frictionless)' }],
  challenge_otp: [
    { value: 'submit_otp', label: 'Submit OTP (success)' },
    { value: 'cancel', label: 'Cancel' },
    { value: 'timeout', label: 'Timeout' },
  ],
  challenge_oob: [
    { value: 'oob_approve', label: 'OOB Approve' },
    { value: 'oob_deny', label: 'OOB Deny' },
    { value: 'oob_fallback', label: 'OOB Fallback → OTP' },
    { value: 'cancel', label: 'Cancel' },
    { value: 'timeout', label: 'Timeout' },
  ],
  decoupled: [
    { value: 'oob_approve', label: 'OOB Approve' },
    { value: 'oob_deny', label: 'OOB Deny' },
    { value: 'oob_fallback', label: 'OOB Fallback → OTP' },
    { value: 'cancel', label: 'Cancel' },
    { value: 'timeout', label: 'Timeout' },
  ],
  challenge_unavailable: [{ value: 'none', label: 'N/A (Verify fallback)' }],
  oreq: [
    { value: 'available', label: 'Available (01)' },
    { value: 'temp_unavailable', label: 'Temporarily Unavailable (02)' },
    { value: 'perm_unavailable', label: 'Permanently Unavailable (03)' },
  ],
};

const CARD_TEMPLATES = {
  visa:       '4000000000001000',
  mastercard: '5200000000001005',
  amex:       '340000000001007',
  discover:   '6011000000001002',
};

const DEFAULT = {
  name: '', description: '', flowType: 'frictionless', deviceChannel: '02',
  network: 'visa', cardNumber: CARD_TEMPLATES.visa, merchantName: 'Test Merchant',
  merchantCountry: '840', purchaseAmount: '1000', purchaseCurrency: '840',
  challengeAction: 'none',
};

export default function Scenarios() {
  const { networks: netEntries } = useNetworks();
  const networkNames = netEntries.filter(n => n.active).map(n => n.name);
  const [scenarios, setScenarios] = useState([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(null); // null = list, 'new' or id
  const [form, setForm] = useState({ ...DEFAULT });
  const [search, setSearch] = useState('');

  const load = useCallback(() => {
    setLoading(true);
    fetchJSON('/scenarios')
      .then(d => setScenarios(d.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(); }, [load]);

  const setField = (k, v) => {
    setForm(f => {
      const next = { ...f, [k]: v };
      if (k === 'flowType') {
        const acts = ACTIONS[v] || [];
        next.challengeAction = acts[0]?.value || 'none';
      }
      if (k === 'network') {
        next.cardNumber = CARD_TEMPLATES[v] || CARD_TEMPLATES.visa;
      }
      return next;
    });
  };

  const startNew = () => { setForm({ ...DEFAULT }); setEditing('new'); };
  const startEdit = (s) => { setForm({ ...s }); setEditing(s.id); };
  const cancel = () => { setEditing(null); };

  const save = async () => {
    try {
      if (editing === 'new') {
        await fetchJSON('/scenarios', {}, 'POST', form);
      } else {
        await fetchJSON(`/scenarios/${editing}`, {}, 'PUT', form);
      }
      setEditing(null);
      load();
    } catch (e) {
      alert('Save failed: ' + e.message);
    }
  };

  const remove = async (id) => {
    if (!confirm('Delete this scenario?')) return;
    try {
      await fetchJSON(`/scenarios/${id}`, {}, 'DELETE');
      load();
    } catch (e) {
      alert('Delete failed: ' + e.message);
    }
  };

  if (editing !== null) {
    const actions = ACTIONS[form.flowType] || [];
    return (
      <>
        <div className="page-header">
          <h1>{editing === 'new' ? 'New Scenario' : 'Edit Scenario'}</h1>
          <p>Configure a test scenario for E2E 3DS flow testing</p>
        </div>
        <div className="table-card" style={{ padding: 24 }}>
          <div className="form-grid">
            <FormField label="Name">
              <input className="filter-input" style={{ width: '100%' }}
                value={form.name} onChange={e => setField('name', e.target.value)}
                placeholder="e.g. Visa OTP Success" />
            </FormField>
            <FormField label="Description">
              <input className="filter-input" style={{ width: '100%' }}
                value={form.description} onChange={e => setField('description', e.target.value)}
                placeholder="Optional description" />
            </FormField>
            <FormField label="Flow Type">
              <select className="filter-select" style={{ width: '100%' }}
                value={form.flowType} onChange={e => setField('flowType', e.target.value)}>
                {FLOW_TYPES.map(f => <option key={f.value} value={f.value}>{f.label}</option>)}
              </select>
            </FormField>
            <FormField label="Challenge Action">
              <select className="filter-select" style={{ width: '100%' }}
                value={form.challengeAction} onChange={e => setField('challengeAction', e.target.value)}>
                {actions.map(a => <option key={a.value} value={a.value}>{a.label}</option>)}
              </select>
            </FormField>
            {form.flowType !== 'oreq' && (<>
            <FormField label="Network">
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <select className="filter-select" style={{ flex: 1 }}
                  value={form.network} onChange={e => setField('network', e.target.value)}>
                  {networkNames.map(n => <option key={n} value={n}>{n.charAt(0).toUpperCase() + n.slice(1)}</option>)}
                </select>
                <NetworkIcon network={form.network} size={24} />
              </div>
            </FormField>
            <FormField label="Device Channel">
              <select className="filter-select" style={{ width: '100%' }}
                value={form.deviceChannel} onChange={e => setField('deviceChannel', e.target.value)}>
                {CHANNELS.map(c => <option key={c.value} value={c.value}>{c.label}</option>)}
              </select>
            </FormField>
            <FormField label="Card Number">
              <input className="filter-input mono" style={{ width: '100%' }}
                value={form.cardNumber} onChange={e => setField('cardNumber', e.target.value)} />
            </FormField>
            <FormField label="Merchant Name">
              <input className="filter-input" style={{ width: '100%' }}
                value={form.merchantName} onChange={e => setField('merchantName', e.target.value)} />
            </FormField>
            <FormField label="Merchant Country (ISO)">
              <input className="filter-input mono" style={{ width: '100%', maxWidth: 100 }}
                value={form.merchantCountry} onChange={e => setField('merchantCountry', e.target.value)} />
            </FormField>
            <FormField label="Purchase Amount">
              <input className="filter-input mono" style={{ width: '100%', maxWidth: 150 }}
                value={form.purchaseAmount} onChange={e => setField('purchaseAmount', e.target.value)} />
            </FormField>
            <FormField label="Purchase Currency (ISO)">
              <input className="filter-input mono" style={{ width: '100%', maxWidth: 100 }}
                value={form.purchaseCurrency} onChange={e => setField('purchaseCurrency', e.target.value)} />
            </FormField>
            </>)}
          </div>
          <div style={{ display: 'flex', gap: 8, marginTop: 24 }}>
            <button className="btn btn-primary" onClick={save}>Save Scenario</button>
            <button className="btn btn-secondary" onClick={cancel}>Cancel</button>
          </div>
        </div>
      </>
    );
  }

  return (
    <>
      <div className="page-header">
        <h1>Test Scenarios</h1>
        <p>Create and manage E2E 3DS authentication test scenarios</p>
      </div>

      <div style={{ marginBottom: 16 }}>
        <button className="btn btn-primary" onClick={startNew}>+ New Scenario</button>
      </div>

      <div className="table-card">
        <div className="table-toolbar">
          <div className="card-title">Scenarios ({scenarios.length})</div>
          <SearchInput value={search} onChange={setSearch} placeholder="Search scenarios…" />
        </div>
        <div className="table-wrap">
          {loading ? (
            <div className="spinner-wrap"><div className="spinner" /></div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Flow</th>
                  <th>Action</th>
                  <th>Network</th>
                  <th>Channel</th>
                  <th>Card</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {scenarios.filter(s => {
                  if (!search) return true;
                  const q = search.toLowerCase();
                  return [s.name, s.description, s.flowType, s.network, s.challengeAction, s.cardNumber, s.deviceChannel]
                    .some(v => (v || '').toLowerCase().includes(q));
                }).map(s => (
                  <tr key={s.id}>
                    <td style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{s.name}</td>
                    <td><FlowBadge flow={s.flowType} /></td>
                    <td><span className="badge badge-muted">{s.challengeAction}</span></td>
                    <td><NetworkBadge network={s.network} /></td>
                    <td><ChannelBadge ch={s.deviceChannel} /></td>
                    <td className="mono">{s.cardNumber}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn btn-small btn-secondary" onClick={() => startEdit(s)}>Edit</button>
                        <button className="btn btn-small btn-danger" onClick={() => remove(s.id)}>Delete</button>
                      </div>
                    </td>
                  </tr>
                ))}
                {scenarios.length === 0 && (
                  <tr><td colSpan={7} className="empty-state">No scenarios yet. Create one to get started.</td></tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </>
  );
}

function FormField({ label, children }) {
  return (
    <div className="form-field">
      <label className="form-label">{label}</label>
      {children}
    </div>
  );
}

export function FlowBadge({ flow }) {
  const map = {
    frictionless: ['badge-green', 'Frictionless'],
    challenge_otp: ['badge-yellow', 'OTP Challenge'],
    challenge_oob: ['badge-cyan', 'OOB Challenge'],
    decoupled: ['badge-cyan', 'Decoupled'],
    challenge_unavailable: ['badge-red', 'Unavailable'],
    oreq: ['badge-blue', 'OReq/ORes'],
  };
  const [cls, label] = map[flow] || ['badge-muted', flow];
  return <span className={`badge ${cls}`}>{label}</span>;
}

function ChannelBadge({ ch }) {
  const map = { '01': ['badge-cyan', 'App'], '02': ['badge-orange', 'Browser'], '03': ['badge-muted', '3RI'] };
  const [cls, label] = map[ch] || ['badge-muted', ch || '—'];
  return <span className={`badge ${cls}`}>{label}</span>;
}