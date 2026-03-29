import React, { useState, useEffect, useCallback } from 'react';

const TABS = ['Configure', 'Run Commands'];

const ERROR_CODES = {
  '00': 'No error',
  '01': 'Verification failure',
  '02': 'Bad key type',
  '03': 'Bad key length',
  '04': 'Bad source key',
  '05': 'Bad message length',
  '10': 'Parity error',
  '13': 'No LMK loaded',
  '15': 'Bad input',
  '20': 'PIN block error',
  '27': 'Bad scheme',
  '41': 'Internal error',
  '68': 'Command disabled',
  '90': 'Not authorized',
};

const ALL_COMMANDS = [
  { code: 'NC', name: 'Perform Diagnostics', resp: 'ND', category: 'Diagnostics' },
  { code: 'NO', name: 'HSM Status', resp: 'NP', category: 'Diagnostics' },
  { code: 'A0', name: 'Generate Key', resp: 'A1', category: 'Key Mgmt' },
  { code: 'A6', name: 'Import Key', resp: 'A7', category: 'Key Mgmt' },
  { code: 'A8', name: 'Export Key', resp: 'A9', category: 'Key Mgmt' },
  { code: 'BU', name: 'Key Check Value', resp: 'BV', category: 'Key Mgmt' },
  { code: 'B0', name: 'Translate Key Scheme', resp: 'B1', category: 'Key Mgmt' },
  { code: 'EI', name: 'Generate RSA Key Pair', resp: 'EJ', category: 'Key Mgmt' },
  { code: 'BA', name: 'Encrypt Clear PIN', resp: 'BB', category: 'PIN' },
  { code: 'DA', name: 'Verify PIN (IBM)', resp: 'DB', category: 'PIN' },
  { code: 'DC', name: 'Verify PIN (Visa PVV)', resp: 'DD', category: 'PIN' },
  { code: 'DE', name: 'Generate PIN Offset', resp: 'DF', category: 'PIN' },
  { code: 'DG', name: 'Generate PVV', resp: 'DH', category: 'PIN' },
  { code: 'CA', name: 'Translate PIN (ZPK→ZPK)', resp: 'CB', category: 'PIN' },
  { code: 'FA', name: 'Translate PIN Block', resp: 'FB', category: 'PIN' },
  { code: 'CW', name: 'Generate CVV', resp: 'CX', category: 'CVV' },
  { code: 'CY', name: 'Verify CVV', resp: 'CZ', category: 'CVV' },
  { code: 'M0', name: 'Encrypt Data Block', resp: 'M1', category: 'Crypto' },
  { code: 'M2', name: 'Decrypt Data Block', resp: 'M3', category: 'Crypto' },
  { code: 'MS', name: 'Generate MAC', resp: 'MT', category: 'Crypto' },
  { code: 'KQ', name: 'Verify ARQC / Generate ARPC', resp: 'KR', category: 'EMV' },
  { code: 'N0', name: 'Generate Random Data', resp: 'N1', category: 'Random' },
];

async function hsmFetch(path, method = 'GET', body = null) {
  const opts = { method, headers: {} };
  if (body !== null) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch('/hsm-admin' + path, opts);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export default function HSMEmulator() {
  const [tab, setTab] = useState('Configure');

  return (
    <>
      <div className="page-header">
        <h1>HSM Emulator</h1>
        <p>Configure and interact with the built-in Thales PayShield 10K emulator</p>
      </div>

      <div className="admin-tabs" style={{ marginBottom: 20 }}>
        {TABS.map(t => (
          <button key={t} className={`admin-tab ${tab === t ? 'active' : ''}`} onClick={() => setTab(t)}>{t}</button>
        ))}
      </div>

      {tab === 'Configure' && <ConfigureTab />}
      {tab === 'Run Commands' && <RunCommandsTab />}
    </>
  );
}

// ── Configure Tab ────────────────────────────────────────────────────────────

function ConfigureTab() {
  const [status, setStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState(null);
  const [editForm, setEditForm] = useState(null);

  const load = useCallback(async () => {
    try {
      const data = await hsmFetch('/status');
      setStatus(data);
      setEditForm({
        headerLength: data.headerLength,
        maxConnections: data.maxConnections,
        commandTimeout: data.commandTimeout,
        firmwareVersion: data.firmwareVersion,
        serialNumber: data.serialNumber,
        model: data.model,
        lmkIdentifier: data.lmkIdentifier,
        auditEnabled: data.auditEnabled,
      });
    } catch (e) {
      setMsg({ type: 'error', text: 'Failed to connect to HSM emulator: ' + e.message });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const flash = (type, text) => { setMsg({ type, text }); setTimeout(() => setMsg(null), 4000); };

  const updateConfig = async () => {
    try {
      await hsmFetch('/config', 'PUT', editForm);
      flash('success', 'Configuration updated');
      load();
    } catch (e) {
      flash('error', 'Failed to update: ' + e.message);
    }
  };

  const loadTestLMK = async () => {
    try {
      await hsmFetch('/lmk/load-test', 'POST', {});
      flash('success', 'Test LMK set loaded');
      load();
    } catch (e) {
      flash('error', 'Failed to load test LMK: ' + e.message);
    }
  };

  const generateLMK = async () => {
    try {
      await hsmFetch('/lmk/generate', 'POST', {});
      flash('success', 'Random LMK generated');
      load();
    } catch (e) {
      flash('error', 'Failed to generate LMK: ' + e.message);
    }
  };

  const setAuthorized = async (val) => {
    try {
      await hsmFetch('/authorize', 'POST', { authorized: val });
      flash('success', val ? 'HSM authorized' : 'HSM deauthorized');
      load();
    } catch (e) {
      flash('error', 'Failed: ' + e.message);
    }
  };

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;

  const f = (k, v) => setEditForm({ ...editForm, [k]: v });

  return (
    <>
      {msg && <div className={`admin-msg ${msg.type === 'success' ? 'admin-msg-success' : 'admin-msg-error'}`} style={{ marginBottom: 16 }}>{msg.text}</div>}

      {/* Status Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 24 }}>
        <StatusCard label="LMK Status" value={status?.lmkLoaded ? 'LOADED' : 'NOT LOADED'} color={status?.lmkLoaded ? 'var(--green)' : 'var(--red)'} />
        <StatusCard label="Authorization" value={status?.authorized ? 'AUTHORIZED' : 'NOT AUTHORIZED'} color={status?.authorized ? 'var(--green)' : 'var(--red)'} />
        <StatusCard label="Host Port" value={status?.hostPort} color="var(--accent)" />
        <StatusCard label="Header Length" value={`${status?.headerLength} bytes`} color="var(--accent)" />
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20 }}>
        {/* Left: HSM Identity & Host Interface */}
        <div>
          <div className="table-card" style={{ padding: 20, marginBottom: 20 }}>
            <h4 style={{ marginBottom: 16 }}>HSM Identity</h4>
            {editForm && (
              <div className="form-grid">
                <div className="form-field">
                  <label className="form-label">Model</label>
                  <input className="filter-input" value={editForm.model} onChange={e => f('model', e.target.value)} />
                </div>
                <div className="form-field">
                  <label className="form-label">Serial Number</label>
                  <input className="filter-input" value={editForm.serialNumber} onChange={e => f('serialNumber', e.target.value)} />
                </div>
                <div className="form-field">
                  <label className="form-label">Firmware Version</label>
                  <input className="filter-input" value={editForm.firmwareVersion} onChange={e => f('firmwareVersion', e.target.value)} />
                </div>
                <div className="form-field">
                  <label className="form-label">LMK Identifier</label>
                  <input className="filter-input" type="number" value={editForm.lmkIdentifier} onChange={e => f('lmkIdentifier', Number(e.target.value))} />
                </div>
              </div>
            )}
          </div>

          <div className="table-card" style={{ padding: 20 }}>
            <h4 style={{ marginBottom: 16 }}>Host Interface</h4>
            {editForm && (
              <div className="form-grid">
                <div className="form-field">
                  <label className="form-label">Header Length</label>
                  <select className="filter-select" value={editForm.headerLength} onChange={e => f('headerLength', Number(e.target.value))}>
                    <option value={2}>2 bytes (PayShield 9K compat)</option>
                    <option value={4}>4 bytes (PayShield 10K)</option>
                  </select>
                </div>
                <div className="form-field">
                  <label className="form-label">Max Connections</label>
                  <input className="filter-input" type="number" value={editForm.maxConnections} onChange={e => f('maxConnections', Number(e.target.value))} />
                </div>
                <div className="form-field">
                  <label className="form-label">Command Timeout (sec)</label>
                  <input className="filter-input" type="number" value={editForm.commandTimeout} onChange={e => f('commandTimeout', Number(e.target.value))} />
                </div>
                <div className="form-field">
                  <label className="form-label">Audit Logging</label>
                  <select className="filter-select" value={editForm.auditEnabled ? 'true' : 'false'} onChange={e => f('auditEnabled', e.target.value === 'true')}>
                    <option value="true">Enabled</option>
                    <option value="false">Disabled</option>
                  </select>
                </div>
              </div>
            )}
            <button className="btn btn-primary" style={{ marginTop: 16 }} onClick={updateConfig}>Save Configuration</button>
          </div>
        </div>

        {/* Right: Security & LMK */}
        <div>
          <div className="table-card" style={{ padding: 20, marginBottom: 20 }}>
            <h4 style={{ marginBottom: 16 }}>Security & Authorization</h4>
            <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
              {!status?.authorized ? (
                <button className="btn btn-primary" onClick={() => setAuthorized(true)} disabled={!status?.lmkLoaded}>
                  Authorize HSM
                </button>
              ) : (
                <button className="btn" style={{ background: 'var(--red)', color: '#fff' }} onClick={() => setAuthorized(false)}>
                  Deauthorize HSM
                </button>
              )}
            </div>
            {!status?.lmkLoaded && (
              <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 8 }}>
                LMK must be loaded before the HSM can be authorized.
              </div>
            )}
          </div>

          <div className="table-card" style={{ padding: 20, marginBottom: 20 }}>
            <h4 style={{ marginBottom: 16 }}>LMK Key Management</h4>
            <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
              <button className="btn btn-primary" onClick={loadTestLMK}>Load Test LMK</button>
              <button className="btn" style={{ background: 'var(--orange, #e67e22)', color: '#fff' }} onClick={generateLMK}>Generate Random LMK</button>
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 12 }}>
              Test LMK uses deterministic keys for reproducible results across instances.
              Random LMK generates cryptographically secure keys.
            </div>
          </div>

          {status?.lmkLoaded && <LMKCheckValues />}

          <CommandManagement />
        </div>
      </div>
    </>
  );
}

function StatusCard({ label, value, color }) {
  return (
    <div className="table-card" style={{ padding: '16px 20px', textAlign: 'center' }}>
      <div style={{ fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px', marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 18, fontWeight: 700, color, fontFamily: 'var(--font-mono, monospace)' }}>{value}</div>
    </div>
  );
}

function LMKCheckValues() {
  const [kcvs, setKcvs] = useState(null);
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    hsmFetch('/lmk/check-values').then(d => setKcvs(d)).catch(() => {});
  }, []);

  if (!kcvs?.loaded) return null;

  return (
    <div className="table-card" style={{ padding: 20, marginBottom: 20 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: expanded ? 12 : 0 }}>
        <h4 style={{ margin: 0 }}>LMK Check Values</h4>
        <button className="btn" style={{ fontSize: 11, padding: '4px 10px' }} onClick={() => setExpanded(!expanded)}>
          {expanded ? 'Collapse' : 'Show KCVs'}
        </button>
      </div>
      {expanded && kcvs.checkValues && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '4px 24px', fontSize: 12, fontFamily: 'var(--font-mono, monospace)' }}>
          {kcvs.checkValues.map(cv => (
            <div key={cv.pair} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0', borderBottom: '1px solid var(--border)' }}>
              <span style={{ color: 'var(--text-muted)' }}>LMK {cv.pair}</span>
              <span style={{ fontWeight: 600 }}>{cv.kcv}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function CommandManagement() {
  const [disabled, setDisabled] = useState({});
  const [loaded, setLoaded] = useState(false);
  const [expanded, setExpanded] = useState(false);

  const load = useCallback(async () => {
    try {
      const data = await hsmFetch('/commands/disabled');
      setDisabled(data || {});
      setLoaded(true);
    } catch (e) {
      setLoaded(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const toggle = async (code, isDisabled) => {
    try {
      await hsmFetch('/commands/disabled', 'POST', { command: code, disabled: !isDisabled });
      load();
    } catch (e) { /* ignore */ }
  };

  if (!loaded) return null;

  return (
    <div className="table-card" style={{ padding: 20 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: expanded ? 12 : 0 }}>
        <h4 style={{ margin: 0 }}>Command Management</h4>
        <button className="btn" style={{ fontSize: 11, padding: '4px 10px' }} onClick={() => setExpanded(!expanded)}>
          {expanded ? 'Collapse' : 'Manage Commands'}
        </button>
      </div>
      {expanded && (
        <div style={{ fontSize: 12 }}>
          <div style={{ color: 'var(--text-muted)', marginBottom: 8 }}>Toggle individual commands on/off.</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 4 }}>
            {ALL_COMMANDS.map(cmd => {
              const isDisabled = disabled[cmd.code] === true;
              return (
                <div key={cmd.code} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '4px 8px', borderRadius: 'var(--radius)', background: isDisabled ? 'rgba(231,76,60,0.08)' : 'transparent' }}>
                  <span>
                    <span style={{ fontFamily: 'var(--font-mono, monospace)', fontWeight: 600, marginRight: 6 }}>{cmd.code}</span>
                    <span style={{ color: 'var(--text-muted)' }}>{cmd.name}</span>
                  </span>
                  <button
                    className="btn"
                    style={{ fontSize: 10, padding: '2px 8px', background: isDisabled ? 'var(--red)' : 'var(--green)', color: '#fff', border: 'none', borderRadius: 'var(--radius)', cursor: 'pointer' }}
                    onClick={() => toggle(cmd.code, isDisabled)}
                  >
                    {isDisabled ? 'Disabled' : 'Enabled'}
                  </button>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

// ── Run Commands Tab ─────────────────────────────────────────────────────────

function RunCommandsTab() {
  const [mode, setMode] = useState('wizard');
  const [result, setResult] = useState(null);
  const [loading, setLoading] = useState(false);
  const [history, setHistory] = useState([]);

  const execute = async (command) => {
    setLoading(true);
    setResult(null);
    try {
      const data = await hsmFetch('/command', 'POST', { command });
      setResult(data);
      setHistory(prev => [{ command, result: data, time: new Date().toLocaleTimeString() }, ...prev].slice(0, 50));
    } catch (e) {
      setResult({ success: false, error: e.message });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20 }}>
      {/* Left: Command input */}
      <div>
        <div className="table-card" style={{ padding: 20, marginBottom: 20 }}>
          <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
            <button className={`admin-tab ${mode === 'wizard' ? 'active' : ''}`} onClick={() => setMode('wizard')} style={{ fontSize: 12 }}>Command Wizard</button>
            <button className={`admin-tab ${mode === 'raw' ? 'active' : ''}`} onClick={() => setMode('raw')} style={{ fontSize: 12 }}>Raw Command</button>
          </div>
          {mode === 'wizard' && <CommandWizard onExecute={execute} />}
          {mode === 'raw' && <RawCommand onExecute={execute} />}
        </div>

        {/* History */}
        {history.length > 0 && (
          <div className="table-card" style={{ padding: 20 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
              <h4 style={{ margin: 0 }}>Command History</h4>
              <button className="btn" style={{ fontSize: 11, padding: '4px 10px' }} onClick={() => setHistory([])}>Clear</button>
            </div>
            <div style={{ maxHeight: 300, overflow: 'auto' }}>
              {history.map((h, i) => (
                <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0', borderBottom: '1px solid var(--border)', fontSize: 12, cursor: 'pointer' }} onClick={() => setResult(h.result)}>
                  <span style={{ color: 'var(--text-muted)', minWidth: 60 }}>{h.time}</span>
                  <span style={{ fontFamily: 'var(--font-mono, monospace)', fontWeight: 600 }}>{h.command.substring(0, 2)}</span>
                  <span className={`badge ${h.result?.success ? 'badge-green' : 'badge-red'}`} style={{ fontSize: 10 }}>
                    {h.result?.errorCode || '??'}
                  </span>
                  <span style={{ color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {h.command.length > 20 ? h.command.substring(0, 20) + '...' : h.command}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Right: Response */}
      <div className="table-card" style={{ padding: 20 }}>
        <h4 style={{ marginBottom: 16 }}>Response</h4>
        {loading && <div className="spinner-wrap"><div className="spinner" /></div>}
        {result && !loading && <ResponseDisplay result={result} />}
        {!result && !loading && (
          <div style={{ textAlign: 'center', padding: 40, color: 'var(--text-muted)', fontSize: 13 }}>
            Execute a command to see the response here
          </div>
        )}
      </div>
    </div>
  );
}

function ResponseDisplay({ result }) {
  if (result.error && !result.raw) {
    return <div className="admin-msg admin-msg-error">{result.error}</div>;
  }

  const isSuccess = result.errorCode === '00';
  const cmdInfo = ALL_COMMANDS.find(c => c.code === result.commandCode);

  return (
    <div>
      <div className={`admin-msg ${isSuccess ? 'admin-msg-success' : 'admin-msg-error'}`} style={{ marginBottom: 16 }}>
        {isSuccess ? 'Command executed successfully' : `Error: ${result.errorDescription || result.errorCode}`}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px 16px', fontSize: 12, marginBottom: 16 }}>
        <div>
          <span style={{ color: 'var(--text-muted)' }}>Command: </span>
          <span style={{ fontFamily: 'var(--font-mono, monospace)', fontWeight: 600 }}>{result.commandCode}</span>
          {cmdInfo && <span style={{ color: 'var(--text-muted)', marginLeft: 4 }}>({cmdInfo.name})</span>}
        </div>
        <div>
          <span style={{ color: 'var(--text-muted)' }}>Response: </span>
          <span style={{ fontFamily: 'var(--font-mono, monospace)', fontWeight: 600 }}>{result.responseCode}</span>
        </div>
        <div>
          <span style={{ color: 'var(--text-muted)' }}>Error Code: </span>
          <span className={`badge ${isSuccess ? 'badge-green' : 'badge-red'}`} style={{ fontSize: 10 }}>{result.errorCode}</span>
          <span style={{ marginLeft: 4, color: 'var(--text-muted)' }}>{result.errorDescription}</span>
        </div>
      </div>

      {result.data && (
        <div>
          <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.5px' }}>Response Data</div>
          <pre style={{
            background: 'var(--bg-card-alt)', padding: 16, borderRadius: 'var(--radius)',
            fontSize: 12, overflow: 'auto', maxHeight: 300, whiteSpace: 'pre-wrap',
            fontFamily: 'var(--font-mono, monospace)', wordBreak: 'break-all',
          }}>{result.data}</pre>
        </div>
      )}

      <div style={{ marginTop: 16 }}>
        <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.5px' }}>Raw Response</div>
        <pre style={{
          background: 'var(--bg-card-alt)', padding: 12, borderRadius: 'var(--radius)',
          fontSize: 11, overflow: 'auto', fontFamily: 'var(--font-mono, monospace)', wordBreak: 'break-all',
          color: 'var(--text-secondary)',
        }}>{result.raw}</pre>
      </div>
    </div>
  );
}

function CommandWizard({ onExecute }) {
  const [category, setCategory] = useState('Diagnostics');
  const [cmd, setCmd] = useState('NC');
  const [form, setForm] = useState({});

  const categories = [...new Set(ALL_COMMANDS.map(c => c.category))];
  const cmdsInCat = ALL_COMMANDS.filter(c => c.category === category);

  const f = (k, v) => setForm({ ...form, [k]: v });

  const buildCommand = () => {
    switch (cmd) {
      case 'NC': return 'NC';
      case 'NO': return 'NO';
      case 'A0': return `A0${form.mode || '0'}${(form.keyType || '002').padStart(3, '0')}${form.scheme || 'U'}`;
      case 'N0': return `N0${(form.length || '16').padStart(4, '0')}`;
      case 'BU': return `BU${(form.keyType || '002').padStart(3, '0')}${form.schemePrefix || 'U'}${form.key || ''};1`;
      case 'CW': return `CW${form.schemePrefix || 'U'}${form.cvk || ''}${form.pan || ''};${form.expiry || ''}${form.serviceCode || ''}`;
      case 'CY': return `CY${form.schemePrefix || 'U'}${form.cvk || ''}${form.cvv || ''}${form.pan || ''};${form.expiry || ''}${form.serviceCode || ''}`;
      case 'BA': return `BA${String(form.pin?.length || 4).padStart(2, '0')}${form.pin || '1234'}${form.account || ''}`;
      case 'M0': {
        const dataHex = form.dataHex || '';
        const dataLen = Math.floor(dataHex.length / 2);
        return `M0${form.cipherMode || '00'}0000${(form.keyType || '002').padStart(3, '0')}${form.schemePrefix || 'U'}${form.key || ''}${dataLen.toString(16).toUpperCase().padStart(4, '0')}${dataHex}`;
      }
      case 'M2': {
        const dataHex = form.dataHex || '';
        const dataLen = Math.floor(dataHex.length / 2);
        return `M2${form.cipherMode || '00'}0000${(form.keyType || '002').padStart(3, '0')}${form.schemePrefix || 'U'}${form.key || ''}${dataLen.toString(16).toUpperCase().padStart(4, '0')}${dataHex}`;
      }
      case 'EI': return `EI${(form.keyBits || '2048').padStart(4, '0')}0003`;
      default: return cmd + (form.rawData || '');
    }
  };

  return (
    <>
      <div className="form-grid" style={{ marginBottom: 12 }}>
        <div className="form-field">
          <label className="form-label">Category</label>
          <select className="filter-select" value={category} onChange={e => { setCategory(e.target.value); const first = ALL_COMMANDS.find(c => c.category === e.target.value); if (first) setCmd(first.code); setForm({}); }}>
            {categories.map(c => <option key={c} value={c}>{c}</option>)}
          </select>
        </div>
        <div className="form-field">
          <label className="form-label">Command</label>
          <select className="filter-select" value={cmd} onChange={e => { setCmd(e.target.value); setForm({}); }}>
            {cmdsInCat.map(c => <option key={c.code} value={c.code}>{c.code} — {c.name}</option>)}
          </select>
        </div>
      </div>

      {/* Dynamic fields per command */}
      {cmd === 'A0' && (
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Mode</label>
            <select className="filter-select" value={form.mode || '0'} onChange={e => f('mode', e.target.value)}>
              <option value="0">Generate</option>
              <option value="1">Generate + Export</option>
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Key Type</label>
            <select className="filter-select" value={form.keyType || '002'} onChange={e => f('keyType', e.target.value)}>
              <option value="000">000 — ZMK</option>
              <option value="001">001 — ZPK</option>
              <option value="002">002 — TPK/PVK</option>
              <option value="003">003 — TAK</option>
              <option value="00A">00A — CVK</option>
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Scheme</label>
            <select className="filter-select" value={form.scheme || 'U'} onChange={e => f('scheme', e.target.value)}>
              <option value="Z">Z — Single-length DES</option>
              <option value="U">U — Double-length 3DES</option>
              <option value="T">T — Triple-length 3DES</option>
              <option value="X">X — Double-length variant</option>
            </select>
          </div>
        </div>
      )}

      {cmd === 'N0' && (
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Number of bytes</label>
            <input className="filter-input" type="number" value={form.length || '16'} onChange={e => f('length', e.target.value)} min={1} max={4096} />
          </div>
        </div>
      )}

      {cmd === 'EI' && (
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Key Size (bits)</label>
            <select className="filter-select" value={form.keyBits || '2048'} onChange={e => f('keyBits', e.target.value)}>
              <option value="1024">1024</option>
              <option value="2048">2048</option>
              <option value="3072">3072</option>
              <option value="4096">4096</option>
            </select>
          </div>
        </div>
      )}

      {cmd === 'BU' && (
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Key Type</label>
            <select className="filter-select" value={form.keyType || '002'} onChange={e => f('keyType', e.target.value)}>
              <option value="000">000 — ZMK</option>
              <option value="001">001 — ZPK</option>
              <option value="002">002 — TPK/PVK</option>
              <option value="00A">00A — CVK</option>
            </select>
          </div>
          <div className="form-field" style={{ gridColumn: '1/-1' }}>
            <label className="form-label">Key (hex, under LMK)</label>
            <input className="filter-input" value={form.key || ''} onChange={e => f('key', e.target.value)} placeholder="32/48 hex chars" style={{ fontFamily: 'var(--font-mono, monospace)' }} />
          </div>
          <div className="form-field">
            <label className="form-label">Scheme Prefix</label>
            <select className="filter-select" value={form.schemePrefix || 'U'} onChange={e => f('schemePrefix', e.target.value)}>
              <option value="U">U</option><option value="Z">Z</option><option value="T">T</option><option value="X">X</option>
            </select>
          </div>
        </div>
      )}

      {(cmd === 'CW' || cmd === 'CY') && (
        <div className="form-grid">
          <div className="form-field" style={{ gridColumn: '1/-1' }}>
            <label className="form-label">CVK (hex, under LMK)</label>
            <input className="filter-input" value={form.cvk || ''} onChange={e => f('cvk', e.target.value)} placeholder="32 hex chars (CVK_A + CVK_B)" style={{ fontFamily: 'var(--font-mono, monospace)' }} />
          </div>
          <div className="form-field">
            <label className="form-label">Scheme Prefix</label>
            <select className="filter-select" value={form.schemePrefix || 'U'} onChange={e => f('schemePrefix', e.target.value)}>
              <option value="U">U</option><option value="Z">Z</option>
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">PAN</label>
            <input className="filter-input" value={form.pan || ''} onChange={e => f('pan', e.target.value)} placeholder="4111111111111111" />
          </div>
          <div className="form-field">
            <label className="form-label">Expiry (YYMM)</label>
            <input className="filter-input" value={form.expiry || ''} onChange={e => f('expiry', e.target.value)} maxLength={4} placeholder="2512" />
          </div>
          <div className="form-field">
            <label className="form-label">Service Code</label>
            <input className="filter-input" value={form.serviceCode || ''} onChange={e => f('serviceCode', e.target.value)} maxLength={3} placeholder="101" />
          </div>
          {cmd === 'CY' && (
            <div className="form-field">
              <label className="form-label">CVV to Verify</label>
              <input className="filter-input" value={form.cvv || ''} onChange={e => f('cvv', e.target.value)} maxLength={3} placeholder="123" />
            </div>
          )}
        </div>
      )}

      {cmd === 'BA' && (
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">PIN</label>
            <input className="filter-input" value={form.pin || ''} onChange={e => f('pin', e.target.value)} maxLength={12} placeholder="1234" />
          </div>
          <div className="form-field">
            <label className="form-label">Account Number (12 digits)</label>
            <input className="filter-input" value={form.account || ''} onChange={e => f('account', e.target.value)} maxLength={12} placeholder="123456789012" />
          </div>
        </div>
      )}

      {(cmd === 'M0' || cmd === 'M2') && (
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">Mode</label>
            <select className="filter-select" value={form.cipherMode || '00'} onChange={e => f('cipherMode', e.target.value)}>
              <option value="00">ECB</option>
              <option value="01">CBC (zero IV)</option>
            </select>
          </div>
          <div className="form-field">
            <label className="form-label">Key Type</label>
            <select className="filter-select" value={form.keyType || '002'} onChange={e => f('keyType', e.target.value)}>
              <option value="002">002 — TPK/PVK</option>
              <option value="007">007 — ZEK/DEK</option>
              <option value="008">008 — DEK</option>
            </select>
          </div>
          <div className="form-field" style={{ gridColumn: '1/-1' }}>
            <label className="form-label">Key (hex, under LMK)</label>
            <input className="filter-input" value={form.key || ''} onChange={e => f('key', e.target.value)} style={{ fontFamily: 'var(--font-mono, monospace)' }} placeholder="32 hex chars" />
          </div>
          <div className="form-field">
            <label className="form-label">Scheme Prefix</label>
            <select className="filter-select" value={form.schemePrefix || 'U'} onChange={e => f('schemePrefix', e.target.value)}>
              <option value="U">U</option><option value="Z">Z</option><option value="T">T</option>
            </select>
          </div>
          <div className="form-field" style={{ gridColumn: '1/-1' }}>
            <label className="form-label">Data (hex)</label>
            <input className="filter-input" value={form.dataHex || ''} onChange={e => f('dataHex', e.target.value)} style={{ fontFamily: 'var(--font-mono, monospace)' }} placeholder="AABBCCDD11223344" />
          </div>
        </div>
      )}

      {/* Fallback for commands without a dedicated wizard */}
      {!['NC','NO','A0','N0','EI','BU','CW','CY','BA','M0','M2'].includes(cmd) && (
        <div className="form-field">
          <label className="form-label">Command Data (after command code)</label>
          <input className="filter-input" value={form.rawData || ''} onChange={e => f('rawData', e.target.value)} style={{ fontFamily: 'var(--font-mono, monospace)' }} placeholder="Enter command data fields..." />
        </div>
      )}

      {/* Preview + Execute */}
      <div style={{ marginTop: 16, padding: 12, background: 'var(--bg-card-alt)', borderRadius: 'var(--radius)', fontSize: 12, fontFamily: 'var(--font-mono, monospace)', marginBottom: 12 }}>
        <span style={{ color: 'var(--text-muted)' }}>Wire command: </span>
        <span style={{ fontWeight: 600, wordBreak: 'break-all' }}>{buildCommand()}</span>
      </div>

      <button className="btn btn-primary" onClick={() => onExecute(buildCommand())}>Execute Command</button>
    </>
  );
}

function RawCommand({ onExecute }) {
  const [rawCmd, setRawCmd] = useState('');

  return (
    <>
      <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 12 }}>
        Enter the full command string (command code + data). Example: <code>NC</code>, <code>A00002U</code>, <code>N00016</code>
      </div>
      <div className="form-field">
        <label className="form-label">Command</label>
        <input
          className="filter-input"
          value={rawCmd}
          onChange={e => setRawCmd(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter' && rawCmd) onExecute(rawCmd); }}
          placeholder="NC"
          style={{ fontFamily: 'var(--font-mono, monospace)', fontSize: 14 }}
        />
      </div>
      <button className="btn btn-primary" style={{ marginTop: 16 }} onClick={() => rawCmd && onExecute(rawCmd)} disabled={!rawCmd}>Execute</button>
    </>
  );
}
