import React, { useState, useEffect, useCallback } from 'react';
import { fetchJSON } from '../api';

const AVAILABILITY_OPTIONS = [
  { value: '01', label: 'Available' },
  { value: '02', label: 'Temporarily Unavailable' },
  { value: '03', label: 'Permanently Unavailable' },
];

function availabilityLabel(code) {
  const opt = AVAILABILITY_OPTIONS.find(o => o.value === code);
  return opt ? opt.label : code || 'N/A';
}

// Call the DS emulator API directly (proxied at /ds-api)
async function dsFetch(path, body = null) {
  const opts = { method: body ? 'POST' : 'GET', headers: {} };
  const token = localStorage.getItem('accessToken');
  if (token) opts.headers['Authorization'] = `Bearer ${token}`;
  if (body) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch('/ds-api' + path, opts);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `HTTP ${res.status}`);
  }
  return res.json();
}

const TABS = [
  { key: 'settings', label: 'Settings' },
  { key: 'operations', label: 'Operations' },
  { key: 'log', label: 'Communication Log' },
];

export default function DSEmulator() {
  const [tab, setTab] = useState('settings');

  return (
    <>
      <div className="page-header">
        <h1>DS Emulator</h1>
        <p>Directory Server emulator for testing 3DS message flows (RReq/RRes, OReq/ORes)</p>
      </div>

      <div className="admin-tabs" style={{ marginBottom: 24 }}>
        {TABS.map(t => (
          <button key={t.key}
            className={`admin-tab${tab === t.key ? ' active' : ''}`}
            onClick={() => setTab(t.key)}>
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'settings' && <SettingsTab />}
      {tab === 'operations' && <OperationsTab />}
      {tab === 'log' && <CommunicationLogTab />}
    </>
  );
}

// ── Settings Tab ──────────────────────────────────────────────────────────

function SettingsTab() {
  const [dsStatus, setDsStatus] = useState(null);
  const [acsStatus, setAcsStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [emulatorReachable, setEmulatorReachable] = useState(false);

  useEffect(() => {
    const loadAll = async () => {
      // Try to reach DS emulator
      try {
        const ds = await dsFetch('/status');
        setDsStatus(ds);
        setEmulatorReachable(true);
      } catch {
        setEmulatorReachable(false);
      }
      // Load ACS config
      try {
        const acs = await fetchJSON('/oreq-status');
        setAcsStatus(acs);
      } catch { /* ignore */ }
      setLoading(false);
    };
    loadAll();
  }, []);

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  return (
    <>
      {/* Emulator Status */}
      <div className="table-card" style={{ marginBottom: 24 }}>
        <div className="table-toolbar">
          <span className="card-title">DS Emulator Status</span>
          <span className={`badge ${emulatorReachable ? 'badge-green' : 'badge-red'}`}>
            {emulatorReachable ? 'Active' : 'Unreachable'}
          </span>
        </div>
        <div style={{ padding: '16px 20px' }}>
          {emulatorReachable ? (
            <div className="form-grid">
              <div className="form-field">
                <label className="form-label">ACS URL (target)</label>
                <div className="form-value mono">{dsStatus?.acsURL || 'Not configured'}</div>
              </div>
              <div className="form-field">
                <label className="form-label">RReq/RRes processed</label>
                <div className="form-value">{dsStatus?.rreqTotal ?? 0}</div>
              </div>
              <div className="form-field">
                <label className="form-label">OReq/ORes processed</label>
                <div className="form-value">{dsStatus?.oreqTotal ?? 0}</div>
              </div>
            </div>
          ) : (
            <div style={{ color: 'var(--text-secondary)', fontSize: 13 }}>
              DS emulator service is not running or not reachable. When inactive, the ACS uses the real DS URL configured below for RReq delivery.
            </div>
          )}
        </div>
      </div>

      {/* ACS DS Configuration */}
      <div className="table-card" style={{ marginBottom: 24 }}>
        <div className="table-toolbar">
          <span className="card-title">ACS Configuration</span>
        </div>
        <div style={{ padding: '16px 20px' }}>
          <div className="form-grid">
            <div className="form-field">
              <label className="form-label">DS URL</label>
              <div className="form-value mono">{acsStatus?.dsURL || 'Not configured (local only)'}</div>
              <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 4 }}>
                When the DS emulator is active, this points to the emulator. Otherwise, RReq messages are sent to this URL.
              </div>
            </div>
            <div className="form-field">
              <label className="form-label">ACS Operator ID</label>
              <div className="form-value mono">{acsStatus?.acsOperatorID || 'Not configured'}</div>
            </div>
            <div className="form-field">
              <label className="form-label">ACS Reference Number</label>
              <div className="form-value mono">{acsStatus?.acsReferenceNumber || 'Not configured'}</div>
            </div>
            <div className="form-field">
              <label className="form-label">Supported Versions</label>
              <div className="form-value">{acsStatus?.supportedVersions?.join(', ') || 'N/A'}</div>
            </div>
            <div className="form-field">
              <label className="form-label">Card Ranges</label>
              <div className="form-value">{acsStatus?.cardRangeCount ?? 0} ranges registered</div>
            </div>
          </div>
        </div>
      </div>

      {/* TLS Info */}
      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">TLS Certificates</span>
        </div>
        <div style={{ padding: '16px 20px' }}>
          <div style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
            The DS emulator runs in plaintext HTTP mode for local development. In production, mutual TLS (mTLS) is configured per-network via the network certificate settings in the Admin &rarr; Settings page. Each network (Visa, Mastercard, etc.) can have its own client certificate for DS communication.
          </div>
          <div className="form-grid" style={{ marginTop: 16 }}>
            <div className="form-field">
              <label className="form-label">Emulator TLS</label>
              <div className="form-value"><span className="badge badge-muted">Plaintext (dev)</span></div>
            </div>
            <div className="form-field">
              <label className="form-label">Per-network mTLS</label>
              <div className="form-value"><span className="badge badge-muted">Configured in Network Settings</span></div>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

// ── Operations Tab ────────────────────────────────────────────────────────

function OperationsTab() {
  const [history, setHistory] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [msg, setMsg] = useState(null);
  const [availability, setAvailability] = useState('01');
  const [expandedId, setExpandedId] = useState(null);
  const perPage = 20;

  const loadHistory = useCallback(async () => {
    try {
      const data = await fetchJSON('/oreq-history', { page, per_page: perPage });
      setHistory(data.data || []);
      setTotal(data.total || 0);
    } catch {
      setMsg({ type: 'error', text: 'Failed to load OReq history' });
    }
  }, [page]);

  useEffect(() => {
    loadHistory().finally(() => setLoading(false));
  }, [loadHistory]);

  const handleSendOReq = async () => {
    setSending(true);
    setMsg(null);
    try {
      await dsFetch('/send-oreq', { availability });
      setMsg({ type: 'success', text: 'DS sent OReq to ACS successfully' });
      setTimeout(() => { loadHistory(); }, 500);
    } catch (e) {
      setMsg({ type: 'error', text: 'OReq failed: ' + e.message });
    } finally {
      setSending(false);
    }
  };

  const totalPages = Math.ceil(total / perPage);

  return (
    <>
      {msg && <div className={`admin-msg admin-msg-${msg.type}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      {/* Send OReq */}
      <div className="table-card" style={{ marginBottom: 24 }}>
        <div className="table-toolbar">
          <span className="card-title">Send OReq (DS &rarr; ACS)</span>
        </div>
        <div style={{ padding: '16px 20px' }}>
          <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 12 }}>
            Trigger the DS emulator to send an OReq to this ACS. The ACS validates and responds with ORes.
          </p>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <select className="filter-select" value={availability} onChange={e => setAvailability(e.target.value)}>
              {AVAILABILITY_OPTIONS.map(o => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
            <button className="btn btn-primary btn-small" onClick={handleSendOReq} disabled={sending}>
              {sending ? 'Sending...' : 'Trigger DS \u2192 Send OReq'}
            </button>
          </div>
        </div>
      </div>

      {/* OReq/ORes History (from ACS perspective) */}
      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">OReq/ORes History ({total})</span>
          <button className="btn btn-secondary btn-small" onClick={loadHistory}>Refresh</button>
        </div>
        <div className="table-wrap">
          {loading ? (
            <div className="spinner-wrap"><div className="spinner" /></div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>ID</th><th>Time</th><th>Type</th><th>Direction</th>
                  <th>Availability</th><th>Version</th><th></th>
                </tr>
              </thead>
              <tbody>
                {history.length === 0 && (
                  <tr><td colSpan="7" className="empty-state">No operational messages yet. Trigger an OReq above.</td></tr>
                )}
                {history.map(m => {
                  const content = typeof m.content === 'object' ? m.content : {};
                  const isExpanded = expandedId === m.id;
                  return (
                    <React.Fragment key={m.id}>
                      <tr>
                        <td>{m.id}</td>
                        <td>{new Date(m.createdAt).toLocaleString()}</td>
                        <td><span className={`badge ${m.messageType === 'OReq' ? 'badge-blue' : 'badge-green'}`}>{m.messageType}</span></td>
                        <td><span className={`badge ${m.direction === 'inbound' ? 'badge-yellow' : 'badge-green'}`}>{m.direction}</span></td>
                        <td>{availabilityLabel(content.acsAvailability)}</td>
                        <td>{content.messageVersion || 'N/A'}</td>
                        <td>
                          <button className="btn btn-secondary btn-small" onClick={() => setExpandedId(isExpanded ? null : m.id)}>
                            {isExpanded ? 'Hide' : 'JSON'}
                          </button>
                        </td>
                      </tr>
                      {isExpanded && (
                        <tr>
                          <td colSpan="7">
                            <pre style={{ background: 'var(--bg-secondary)', padding: 12, borderRadius: 6, fontSize: 12, maxHeight: 300, overflow: 'auto', margin: 0 }}>
                              {JSON.stringify(content, null, 2)}
                            </pre>
                          </td>
                        </tr>
                      )}
                    </React.Fragment>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
        {totalPages > 1 && (
          <div className="table-pagination">
            <button className="btn btn-secondary btn-small" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>Prev</button>
            <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>Page {page} of {totalPages}</span>
            <button className="btn btn-secondary btn-small" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next</button>
          </div>
        )}
      </div>
    </>
  );
}

// ── Communication Log Tab ─────────────────────────────────────────────────

function CommunicationLogTab() {
  const [entries, setEntries] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [expandedId, setExpandedId] = useState(null);
  const [typeFilter, setTypeFilter] = useState('all');
  const [error, setError] = useState(null);
  const perPage = 30;

  const load = useCallback(async () => {
    try {
      setError(null);
      const data = await dsFetch(`/communication-log?page=${page}&per_page=${perPage}`);
      setEntries(data.data || []);
      setTotal(data.total || 0);
    } catch (e) {
      setError('DS emulator is not reachable. Communication log is only available when the emulator is active.');
      setEntries([]);
    }
  }, [page]);

  useEffect(() => {
    load().finally(() => setLoading(false));
  }, [load]);

  const filtered = typeFilter === 'all'
    ? entries
    : entries.filter(e => e.messageType === typeFilter);

  const totalPages = Math.ceil(total / perPage);

  const msgTypeBadge = (type) => {
    const map = {
      RReq: 'badge-yellow', RRes: 'badge-green',
      OReq: 'badge-blue', ORes: 'badge-cyan',
    };
    return <span className={`badge ${map[type] || 'badge-muted'}`}>{type}</span>;
  };

  const dirBadge = (dir) => (
    <span className={`badge ${dir === 'inbound' ? 'badge-yellow' : 'badge-green'}`}>{dir}</span>
  );

  return (
    <>
      {error && <div className="admin-msg admin-msg-error" style={{ marginBottom: 16 }}>{error}</div>}

      <div className="table-card">
        <div className="table-toolbar">
          <span className="card-title">Communication Log ({total})</span>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <select className="filter-select" value={typeFilter} onChange={e => setTypeFilter(e.target.value)}>
              <option value="all">All Types</option>
              <option value="RReq">RReq</option>
              <option value="RRes">RRes</option>
              <option value="OReq">OReq</option>
              <option value="ORes">ORes</option>
            </select>
            <button className="btn btn-secondary btn-small" onClick={load}>Refresh</button>
          </div>
        </div>
        <div className="table-wrap">
          {loading ? (
            <div className="spinner-wrap"><div className="spinner" /></div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>#</th><th>Timestamp</th><th>Type</th><th>Direction</th>
                  <th>Summary</th><th></th>
                </tr>
              </thead>
              <tbody>
                {filtered.length === 0 && (
                  <tr><td colSpan="6" className="empty-state">
                    {error ? 'Emulator offline' : 'No messages recorded yet. Run a test scenario or trigger an OReq to see communication here.'}
                  </td></tr>
                )}
                {filtered.map(e => {
                  const isExpanded = expandedId === e.id + '_' + e.messageType;
                  const key = e.id + '_' + e.messageType;
                  return (
                    <React.Fragment key={key}>
                      <tr>
                        <td>{e.id}</td>
                        <td>{new Date(e.timestamp).toLocaleString()}</td>
                        <td>{msgTypeBadge(e.messageType)}</td>
                        <td>{dirBadge(e.direction)}</td>
                        <td><span className="mono" style={{ fontSize: 12 }}>{e.summary}</span></td>
                        <td>
                          <button className="btn btn-secondary btn-small"
                            onClick={() => setExpandedId(isExpanded ? null : key)}>
                            {isExpanded ? 'Hide' : 'JSON'}
                          </button>
                        </td>
                      </tr>
                      {isExpanded && (
                        <tr>
                          <td colSpan="6">
                            <pre style={{ background: 'var(--bg-secondary)', padding: 12, borderRadius: 6, fontSize: 12, maxHeight: 300, overflow: 'auto', margin: 0 }}>
                              {JSON.stringify(e.content, null, 2)}
                            </pre>
                          </td>
                        </tr>
                      )}
                    </React.Fragment>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
        {totalPages > 1 && (
          <div className="table-pagination">
            <button className="btn btn-secondary btn-small" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>Prev</button>
            <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>Page {page} of {totalPages}</span>
            <button className="btn btn-secondary btn-small" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next</button>
          </div>
        )}
      </div>
    </>
  );
}
