import React, { useState, useEffect } from 'react';
import { secretsFetch } from '../secretsApi';

const TABS = ['Keys', 'PIN', 'EMV', 'Cards', 'Admin'];

export default function HSMConsole() {
  const [tab, setTab] = useState('Keys');
  const [endpoints, setEndpoints] = useState([]);
  const [result, setResult] = useState(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    secretsFetch('/hsm/v1/endpoints').then(d => setEndpoints(d || [])).catch(() => {});
  }, []);

  const execute = async (path, body) => {
    setLoading(true);
    setResult(null);
    try {
      const data = await secretsFetch(`/hsm/v1${path}`, 'POST', body);
      setResult({ success: true, data });
    } catch (e) {
      setResult({ success: false, error: e.message });
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <div className="page-header">
        <h1>HSM Console</h1>
        <p>Interactive command interface for sending operations to HSM endpoints</p>
      </div>

      <div className="admin-tabs" style={{ marginBottom: 20 }}>
        {TABS.map(t => (
          <button key={t} className={`admin-tab ${tab === t ? 'active' : ''}`} onClick={() => { setTab(t); setResult(null); }}>{t}</button>
        ))}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20 }}>
        <div className="table-card" style={{ padding: 20 }}>
          <h4 style={{ marginBottom: 16 }}>{tab} Operations</h4>
          {tab === 'Keys' && <KeysTab onExecute={execute} />}
          {tab === 'PIN' && <PINTab onExecute={execute} />}
          {tab === 'EMV' && <EMVTab onExecute={execute} />}
          {tab === 'Cards' && <CardsTab onExecute={execute} />}
          {tab === 'Admin' && <AdminTab onExecute={execute} endpoints={endpoints} />}
        </div>

        <div className="table-card" style={{ padding: 20 }}>
          <h4 style={{ marginBottom: 16 }}>Response</h4>
          {loading && <div className="spinner-wrap"><div className="spinner" /></div>}
          {result && !loading && (
            <div>
              <div className={`admin-msg ${result.success ? 'admin-msg-success' : 'admin-msg-error'}`} style={{marginBottom:12}}>
                {result.success ? 'Command executed successfully' : `Error: ${result.error}`}
              </div>
              {result.data && (
                <pre style={{
                  background:'var(--bg-card-alt)', padding:16, borderRadius:'var(--radius)',
                  fontSize:12, overflow:'auto', maxHeight:400, whiteSpace:'pre-wrap',
                }}>{JSON.stringify(result.data, null, 2)}</pre>
              )}
            </div>
          )}
          {!result && !loading && (
            <div style={{textAlign:'center',padding:32,color:'var(--text-muted)',fontSize:13}}>
              Execute a command to see the response
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function KeysTab({ onExecute }) {
  const [op, setOp] = useState('generate');
  const [form, setForm] = useState({ keyType:'ZPK', scheme:'U', lmkKeyType:1, keyLength:2048, publicExponent:'65537' });
  const f = (k, v) => setForm({...form, [k]: v});

  return (
    <>
      <div className="form-field" style={{marginBottom:12}}>
        <label className="form-label">Operation</label>
        <select className="filter-select" value={op} onChange={e => setOp(e.target.value)}>
          <option value="generate">Generate Symmetric Key (A0)</option>
          <option value="generate-rsa">Generate RSA Key (EI)</option>
          <option value="validate-check-value">Validate Check Value (BU)</option>
        </select>
      </div>
      {op === 'generate' && (
        <div className="form-grid">
          <div className="form-field"><label className="form-label">Key Scheme</label><select className="filter-select" value={form.scheme} onChange={e=>f('scheme',e.target.value)}><option value="U">U (Variant)</option><option value="T">T (TR-31 AES)</option><option value="Z">Z (Legacy)</option></select></div>
          <div className="form-field"><label className="form-label">LMK Key Type</label><input className="filter-input" type="number" value={form.lmkKeyType} onChange={e=>f('lmkKeyType',Number(e.target.value))} /></div>
        </div>
      )}
      {op === 'generate-rsa' && (
        <div className="form-grid">
          <div className="form-field"><label className="form-label">Key Length (bits)</label><select className="filter-select" value={form.keyLength} onChange={e=>f('keyLength',Number(e.target.value))}><option value={1024}>1024</option><option value={2048}>2048</option><option value={3072}>3072</option><option value={4096}>4096</option></select></div>
          <div className="form-field"><label className="form-label">Public Exponent</label><select className="filter-select" value={form.publicExponent} onChange={e=>f('publicExponent',e.target.value)}><option value="65537">65537</option><option value="3">3</option></select></div>
        </div>
      )}
      {op === 'validate-check-value' && (
        <div className="form-grid">
          <div className="form-field"><label className="form-label">Key (hex)</label><input className="filter-input" value={form.key||''} onChange={e=>f('key',e.target.value)} placeholder="Key under LMK" /></div>
          <div className="form-field"><label className="form-label">Key Type Code</label><input className="filter-input" type="number" value={form.lmkKeyType} onChange={e=>f('lmkKeyType',Number(e.target.value))} /></div>
        </div>
      )}
      <button className="btn btn-primary" style={{marginTop:16}} onClick={() => {
        if (op === 'generate') onExecute('/keys/generate', { scheme: form.scheme, lmkKeyType: form.lmkKeyType });
        else if (op === 'generate-rsa') onExecute('/keys/generate-rsa', { keyLength: form.keyLength, publicExponent: form.publicExponent });
        else if (op === 'validate-check-value') onExecute('/keys/validate-check-value', { keyType: form.lmkKeyType, key: form.key });
      }}>Execute</button>
    </>
  );
}

function PINTab({ onExecute }) {
  const [op, setOp] = useState('verify');
  const [form, setForm] = useState({ tpk:'', pinBlock:'', pinBlockFormat:'01', pan:'', offset:'', checkLength:4, decimalisationTable:'', validationData:'' });
  const f = (k, v) => setForm({...form, [k]: v});

  return (
    <>
      <div className="form-field" style={{marginBottom:12}}>
        <label className="form-label">Operation</label>
        <select className="filter-select" value={op} onChange={e => setOp(e.target.value)}>
          <option value="verify">Verify PIN (DA)</option>
          <option value="translate">Translate PIN (CA)</option>
          <option value="generate-offset">Generate PIN Offset (DE)</option>
        </select>
      </div>
      <div className="form-grid">
        <div className="form-field"><label className="form-label">TPK/PVK (hex)</label><input className="filter-input" value={form.tpk} onChange={e=>f('tpk',e.target.value)} /></div>
        <div className="form-field"><label className="form-label">PIN Block (hex)</label><input className="filter-input" value={form.pinBlock} onChange={e=>f('pinBlock',e.target.value)} /></div>
        <div className="form-field"><label className="form-label">PIN Block Format</label><select className="filter-select" value={form.pinBlockFormat} onChange={e=>f('pinBlockFormat',e.target.value)}><option value="01">01 - ISO 9564-1</option><option value="03">03 - ISO 9564-3</option><option value="05">05 - ISO 9564-4</option></select></div>
        <div className="form-field"><label className="form-label">PAN</label><input className="filter-input" value={form.pan} onChange={e=>f('pan',e.target.value)} /></div>
      </div>
      <button className="btn btn-primary" style={{marginTop:16}} onClick={() => {
        if (op === 'verify') onExecute('/pin/verify', form);
        else if (op === 'translate') onExecute('/pin/translate', { sourceTPK:form.tpk, sourcePINBlock:form.pinBlock, sourceFormat:form.pinBlockFormat, destFormat:form.pinBlockFormat, pan:form.pan, maxPINLength:12 });
        else onExecute('/pin/generate-offset', { pvk:form.tpk, pinBlock:form.pinBlock, pinBlockFormat:form.pinBlockFormat, pan:form.pan, checkLength:form.checkLength });
      }}>Execute</button>
    </>
  );
}

function EMVTab({ onExecute }) {
  const [form, setForm] = useState({ schemeId:0, mkAc:'', pan:'', panSeqNo:'00', atc:'', un:'', transData:'', arqc:'' });
  const f = (k, v) => setForm({...form, [k]: v});

  return (
    <>
      <div style={{fontSize:12,color:'var(--text-muted)',marginBottom:12}}>Validate ARQC (KQ command)</div>
      <div className="form-grid">
        <div className="form-field"><label className="form-label">Scheme</label><select className="filter-select" value={form.schemeId} onChange={e=>f('schemeId',Number(e.target.value))}><option value={0}>Visa / Common</option><option value={1}>Mastercard</option></select></div>
        <div className="form-field"><label className="form-label">MK-AC (hex)</label><input className="filter-input" value={form.mkAc} onChange={e=>f('mkAc',e.target.value)} /></div>
        <div className="form-field"><label className="form-label">PAN</label><input className="filter-input" value={form.pan} onChange={e=>f('pan',e.target.value)} /></div>
        <div className="form-field"><label className="form-label">PAN Seq No</label><input className="filter-input" value={form.panSeqNo} onChange={e=>f('panSeqNo',e.target.value)} /></div>
        <div className="form-field"><label className="form-label">ATC (hex)</label><input className="filter-input" value={form.atc} onChange={e=>f('atc',e.target.value)} /></div>
        <div className="form-field"><label className="form-label">Unpredictable Number</label><input className="filter-input" value={form.un} onChange={e=>f('un',e.target.value)} /></div>
        <div className="form-field" style={{gridColumn:'1/-1'}}><label className="form-label">Transaction Data (hex)</label><input className="filter-input" value={form.transData} onChange={e=>f('transData',e.target.value)} /></div>
        <div className="form-field"><label className="form-label">ARQC (hex)</label><input className="filter-input" value={form.arqc} onChange={e=>f('arqc',e.target.value)} /></div>
      </div>
      <button className="btn btn-primary" style={{marginTop:16}} onClick={() => onExecute('/emv/arqc/validate', form)}>Validate ARQC</button>
    </>
  );
}

function CardsTab({ onExecute }) {
  const [op, setOp] = useState('verify-cvv');
  const [form, setForm] = useState({ cvk:'', pan:'', expDate:'', serviceCode:'', cvv:'' });
  const f = (k, v) => setForm({...form, [k]: v});

  return (
    <>
      <div className="form-field" style={{marginBottom:12}}>
        <label className="form-label">Operation</label>
        <select className="filter-select" value={op} onChange={e => setOp(e.target.value)}>
          <option value="verify-cvv">Verify CVV (CY)</option>
          <option value="generate-cvv">Generate CVV (CW)</option>
        </select>
      </div>
      <div className="form-grid">
        <div className="form-field"><label className="form-label">CVK (hex)</label><input className="filter-input" value={form.cvk} onChange={e=>f('cvk',e.target.value)} /></div>
        <div className="form-field"><label className="form-label">PAN</label><input className="filter-input" value={form.pan} onChange={e=>f('pan',e.target.value)} /></div>
        <div className="form-field"><label className="form-label">Expiry (YYMM)</label><input className="filter-input" value={form.expDate} onChange={e=>f('expDate',e.target.value)} maxLength={4} /></div>
        <div className="form-field"><label className="form-label">Service Code</label><input className="filter-input" value={form.serviceCode} onChange={e=>f('serviceCode',e.target.value)} maxLength={3} /></div>
        {op === 'verify-cvv' && <div className="form-field"><label className="form-label">CVV</label><input className="filter-input" value={form.cvv} onChange={e=>f('cvv',e.target.value)} maxLength={3} /></div>}
      </div>
      <button className="btn btn-primary" style={{marginTop:16}} onClick={() => {
        if (op === 'verify-cvv') onExecute('/cards/verify-cvv', form);
        else onExecute('/cards/generate-cvv', { cvk:form.cvk, pan:form.pan, expDate:form.expDate, serviceCode:form.serviceCode });
      }}>Execute</button>
    </>
  );
}

function AdminTab({ onExecute, endpoints }) {
  const [op, setOp] = useState('test');
  const [epId, setEpId] = useState(endpoints[0]?.id || 0);
  const [rawCmd, setRawCmd] = useState('');

  return (
    <>
      <div className="form-field" style={{marginBottom:12}}>
        <label className="form-label">Operation</label>
        <select className="filter-select" value={op} onChange={e => setOp(e.target.value)}>
          <option value="test">Test Command (NC diagnostics)</option>
          <option value="raw">Raw Command</option>
        </select>
      </div>
      <div className="form-grid">
        <div className="form-field">
          <label className="form-label">Endpoint</label>
          <select className="filter-select" value={epId} onChange={e => setEpId(Number(e.target.value))}>
            {endpoints.map(ep => <option key={ep.id} value={ep.id}>{ep.name} ({ep.address}:{ep.port})</option>)}
            {endpoints.length === 0 && <option value={0}>No endpoints available</option>}
          </select>
        </div>
        {op === 'raw' && (
          <div className="form-field" style={{gridColumn:'1/-1'}}>
            <label className="form-label">Command (command code + data, ASCII or hex)</label>
            <input className="filter-input" value={rawCmd} onChange={e => setRawCmd(e.target.value)} placeholder="NC or A0001U..." style={{fontFamily:'monospace'}} />
          </div>
        )}
      </div>
      <button className="btn btn-primary" style={{marginTop:16}} disabled={!epId} onClick={() => {
        if (op === 'test') onExecute('/admin/test-command', { endpointId: epId });
        else onExecute('/admin/raw-command', { endpointId: epId, commandHex: rawCmd });
      }}>Execute</button>
    </>
  );
}
