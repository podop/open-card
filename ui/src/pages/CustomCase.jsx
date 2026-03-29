import React, { useState, useRef, useEffect, useCallback } from 'react';
import { fetchJSON } from '../api';
import { useAuth } from '../AuthContext';
import { FlowBadge } from './Scenarios';

const AREQ_TEMPLATE = {
  messageType: 'AReq',
  messageVersion: '2.2.0',
  messageCategory: '01',
  deviceChannel: '02',
  threeDSServerTransID: crypto.randomUUID(),
  dsTransID: crypto.randomUUID(),
  acctNumber: '4000000000001000',
  merchantName: 'Test Merchant',
  merchantCountry: '840',
  purchaseAmount: '1000',
  purchaseCurrency: '840',
  purchaseExponent: 2,
  browserAcceptHeader: 'text/html,application/xhtml+xml',
  browserIP: '127.0.0.1',
  browserUserAgent: 'CustomCase/1.0',
  browserLanguage: 'en',
  browserColorDepth: '24',
  browserScreenHeight: '1080',
  browserScreenWidth: '1920',
  browserTZ: 0,
  browserJavaEnabled: false,
  browserJavascriptEnabled: true,
};

function isOobFlow(flowType) {
  return flowType === 'challenge_oob' || flowType === 'decoupled';
}

export default function CustomCase() {
  const { canDo } = useAuth();
  const [areqText, setAreqText] = useState(JSON.stringify(AREQ_TEMPLATE, null, 2));
  const [parseError, setParseError] = useState(null);
  const [sending, setSending] = useState(false);
  const [aresData, setAresData] = useState(null);
  const [interactiveInfo, setInteractiveInfo] = useState(null);
  const [otpError, setOtpError] = useState(null);
  const iframeRef = useRef(null);
  const pollRef = useRef(null);

  // Auto-poll for PIN and OTP refresh while interactive panel is active
  const pollCredentials = useCallback(async (acsTransID, flowType) => {
    // Always try PIN (2nd factor may activate after OTP submission)
    try {
      const data = await fetchJSON(`/scenarios/pin/${acsTransID}`);
      if (data.pin) setInteractiveInfo(prev => prev ? { ...prev, pin: data.pin } : prev);
    } catch (_) { /* not yet generated */ }

    // For OTP flows, refresh OTP (it may rotate after failed attempts)
    if (!isOobFlow(flowType)) {
      try {
        const otpData = await fetchJSON(`/scenarios/otp/${acsTransID}`);
        if (otpData.otp) {
          setInteractiveInfo(prev => {
            if (!prev) return prev;
            // Only update if OTP actually changed
            if (prev.otp !== otpData.otp) return { ...prev, otp: otpData.otp };
            return prev;
          });
        }
      } catch (_) { /* timing */ }
    }
  }, []);

  useEffect(() => {
    if (pollRef.current) clearInterval(pollRef.current);
    if (interactiveInfo?.acsTransID) {
      pollRef.current = setInterval(
        () => pollCredentials(interactiveInfo.acsTransID, interactiveInfo.flowType),
        3000,
      );
    }
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, [interactiveInfo?.acsTransID, interactiveInfo?.flowType, pollCredentials]);

  const regenerateIds = () => {
    try {
      const obj = JSON.parse(areqText);
      obj.threeDSServerTransID = crypto.randomUUID();
      obj.dsTransID = crypto.randomUUID();
      setAreqText(JSON.stringify(obj, null, 2));
      setParseError(null);
    } catch (e) {
      setParseError('Invalid JSON — cannot regenerate IDs');
    }
  };

  const refreshOtp = async () => {
    if (!interactiveInfo?.acsTransID) return;
    setOtpError(null);
    try {
      const otpData = await fetchJSON(`/scenarios/otp/${interactiveInfo.acsTransID}`);
      if (otpData.otp) {
        setInteractiveInfo(prev => ({ ...prev, otp: otpData.otp }));
      } else {
        setOtpError('No OTP returned');
      }
    } catch (e) {
      setOtpError('Failed to fetch OTP: ' + e.message);
    }
  };

  const fetchPin = async () => {
    if (!interactiveInfo?.acsTransID) return;
    try {
      const data = await fetchJSON(`/scenarios/pin/${interactiveInfo.acsTransID}`);
      if (data.pin) setInteractiveInfo(prev => ({ ...prev, pin: data.pin }));
    } catch (_) { /* not yet generated */ }
  };

  const sendAreq = async () => {
    setParseError(null);
    setAresData(null);
    setInteractiveInfo(null);
    setOtpError(null);

    let payload;
    try {
      payload = JSON.parse(areqText);
    } catch (e) {
      setParseError('Invalid JSON: ' + e.message);
      return;
    }

    setSending(true);
    try {
      const res = await fetch('/v1/auth', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      const data = await res.json();
      setAresData(data);

      if (data.transStatus === 'C' || data.transStatus === 'D') {
        const acsTransID = data.acsTransID;
        // D = decoupled/OOB, C = OTP challenge
        const flowType = data.transStatus === 'D' ? 'challenge_oob' : 'challenge_otp';

        let otp = null;
        if (flowType === 'challenge_otp') {
          // Retry OTP fetch a few times — audit row may lag slightly behind the response
          for (let attempt = 0; attempt < 3; attempt++) {
            try {
              const otpData = await fetchJSON(`/scenarios/otp/${acsTransID}`);
              if (otpData.otp) { otp = otpData.otp; break; }
            } catch (_) { /* retry */ }
            if (attempt < 2) await new Promise(r => setTimeout(r, 500));
          }
        }

        setInteractiveInfo({ acsTransID, flowType, otp, pin: null });
      }
    } catch (e) {
      setParseError('Request failed: ' + e.message);
    } finally {
      setSending(false);
    }
  };

  const oobAction = async (action) => {
    if (!interactiveInfo?.acsTransID) return;
    const id = interactiveInfo.acsTransID;
    try {
      const post = (url, body) => fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }).then(r => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); });

      if (action === 'approve') {
        await post(`/v1/challenge/${id}/oob/result`, { approved: true });
        alert('OOB approved. Challenge should complete.');
      } else if (action === 'deny') {
        await post(`/v1/challenge/${id}/oob/result`, { approved: false });
        alert('OOB denied. Transaction marked as failed.');
      } else if (action === 'fallback') {
        await post(`/v1/challenge/${id}/oob/fallback`, {});
        // After fallback, switch to OTP view and fetch the new OTP
        let otp = null;
        for (let attempt = 0; attempt < 3; attempt++) {
          try {
            const otpData = await fetchJSON(`/scenarios/otp/${id}`);
            if (otpData.otp) { otp = otpData.otp; break; }
          } catch (_) { /* retry */ }
          if (attempt < 2) await new Promise(r => setTimeout(r, 500));
        }
        setInteractiveInfo(prev => ({ ...prev, flowType: 'challenge_otp', otp }));
        if (iframeRef.current) iframeRef.current.src = `/v1/challenge/${id}`;
      }
    } catch (e) {
      alert('Action failed: ' + e.message);
    }
  };

  const iframeSrc = (acsTransID, flowType) => {
    const base = `/v1/challenge/${acsTransID}`;
    return isOobFlow(flowType) ? base + '?flow=oob' : base;
  };

  const statusColor = (s) => {
    const map = { Y: 'badge-green', C: 'badge-yellow', D: 'badge-cyan', N: 'badge-red', U: 'badge-muted', R: 'badge-red' };
    return map[s] || 'badge-muted';
  };

  const statusLabel = (s) => {
    const map = { Y: 'Frictionless (Y)', C: 'Challenge (C)', D: 'Decoupled (D)', N: 'Not Authenticated (N)', U: 'Unavailable (U)', R: 'Rejected (R)', A: 'Attempt (A)', I: 'Informational (I)' };
    return map[s] || s;
  };

  return (
    <>
      <div className="page-header">
        <h1>Custom Case</h1>
        <p>Build your own AReq and send it to the ACS — full interactive response including challenge, OTP, and OOB</p>
      </div>

      {/* AReq editor */}
      <div className="table-card" style={{ marginBottom: 24 }}>
        <div className="table-toolbar">
          <div className="card-title">Authentication Request (AReq)</div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-small btn-secondary" onClick={regenerateIds}>
              New UUIDs
            </button>
            <button className="btn btn-small btn-secondary" onClick={() => {
              setAreqText(JSON.stringify(AREQ_TEMPLATE, null, 2));
              setParseError(null);
            }}>
              Reset Template
            </button>
          </div>
        </div>
        <div style={{ padding: 16 }}>
          <textarea
            className="custom-case-editor"
            value={areqText}
            onChange={e => { setAreqText(e.target.value); setParseError(null); }}
            spellCheck={false}
          />
          {parseError && (
            <div style={{ color: 'var(--red)', fontSize: 13, marginTop: 8 }}>{parseError}</div>
          )}
          <div style={{ marginTop: 12, display: 'flex', gap: 8 }}>
            <button className="btn btn-primary" disabled={sending || !canDo('RunTestScenario')} onClick={sendAreq}>
              {sending ? 'Sending…' : 'Send AReq'}
            </button>
            <button className="btn btn-secondary" onClick={() => {
              try {
                const formatted = JSON.stringify(JSON.parse(areqText), null, 2);
                setAreqText(formatted);
                setParseError(null);
              } catch (e) {
                setParseError('Cannot format: invalid JSON');
              }
            }}>
              Format JSON
            </button>
          </div>
        </div>
      </div>

      {/* ARes response */}
      {aresData && (
        <div className="table-card" style={{ marginBottom: 24 }}>
          <div className="table-toolbar">
            <div className="card-title">
              Authentication Response (ARes)
              <span className={`badge ${statusColor(aresData.transStatus)}`} style={{ marginLeft: 8 }}>
                {statusLabel(aresData.transStatus)}
              </span>
            </div>
          </div>
          <div style={{ padding: 16 }}>
            <pre className="step-json" style={{ maxHeight: 400 }}>{JSON.stringify(aresData, null, 2)}</pre>
          </div>
        </div>
      )}

      {/* Interactive panel */}
      {interactiveInfo && (
        <div className="interactive-panel" style={{ marginBottom: 24 }}>
          <div className="interactive-grid">
            {/* Left: Challenge iframe */}
            <div className="table-card">
              <div className="table-toolbar">
                <div className="card-title">Challenge Page</div>
                <button className="btn btn-small btn-secondary" onClick={() => {
                  if (iframeRef.current) {
                    iframeRef.current.src = iframeSrc(interactiveInfo.acsTransID, interactiveInfo.flowType);
                  }
                }}>Reload</button>
              </div>
              <div style={{ padding: 0 }}>
                <iframe
                  ref={iframeRef}
                  src={iframeSrc(interactiveInfo.acsTransID, interactiveInfo.flowType)}
                  style={{ width: '100%', height: 520, border: 'none', borderRadius: '0 0 12px 12px', background: '#fff' }}
                  title="Challenge Page"
                />
              </div>
            </div>

            {/* Right: Controls */}
            <div>
              {/* OTP info card — only for OTP flows */}
              {!isOobFlow(interactiveInfo.flowType) && (
                <div className="table-card" style={{ marginBottom: 16 }}>
                  <div className="table-toolbar">
                    <div className="card-title">OTP Code</div>
                  </div>
                  <div style={{ padding: 20, textAlign: 'center' }}>
                    <div className="otp-display">{interactiveInfo.otp || '—'}</div>
                    <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 8 }}>
                      Enter this code in the challenge page
                    </div>
                    {otpError && (
                      <div style={{ fontSize: 12, color: 'var(--red)', marginTop: 4 }}>{otpError}</div>
                    )}
                    <div style={{ display: 'flex', gap: 8, justifyContent: 'center', marginTop: 8 }}>
                      <button className="btn btn-small btn-secondary"
                        onClick={() => { navigator.clipboard.writeText(interactiveInfo.otp || ''); }}>
                        Copy
                      </button>
                      <button className="btn btn-small btn-secondary" onClick={refreshOtp}>
                        Refresh OTP
                      </button>
                    </div>
                  </div>
                </div>
              )}

              {/* PIN info card — auto-polled, shown once available */}
              {interactiveInfo.pin && (
                <div className="table-card" style={{ marginBottom: 16 }}>
                  <div className="table-toolbar">
                    <div className="card-title">Card PIN (2nd Factor)</div>
                  </div>
                  <div style={{ padding: 20, textAlign: 'center' }}>
                    <div className="otp-display">{interactiveInfo.pin}</div>
                    <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 8 }}>
                      Enter this PIN on the second factor page
                    </div>
                    <button className="btn btn-small btn-secondary" style={{ marginTop: 8 }}
                      onClick={() => { navigator.clipboard.writeText(interactiveInfo.pin || ''); }}>
                      Copy
                    </button>
                  </div>
                </div>
              )}
              {!interactiveInfo.pin && (
                <button className="btn btn-small btn-secondary" style={{ marginBottom: 12 }} onClick={fetchPin}>
                  Fetch PIN (if 2nd factor active)
                </button>
              )}

              {/* OOB controls — only for OOB/decoupled flows */}
              {isOobFlow(interactiveInfo.flowType) && (
                <div className="table-card" style={{ marginBottom: 16 }}>
                  <div className="table-toolbar">
                    <div className="card-title">OOB Actions</div>
                  </div>
                  <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 8 }}>
                    <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 4 }}>
                      Simulate the out-of-band authenticator response:
                    </div>
                    <button className="btn btn-primary" onClick={() => oobAction('approve')}>
                      Approve (OOB)
                    </button>
                    <button className="btn btn-danger" onClick={() => oobAction('deny')}>
                      Deny (OOB)
                    </button>
                    <button className="btn btn-secondary" onClick={() => oobAction('fallback')}>
                      Fallback to OTP
                    </button>
                  </div>
                </div>
              )}

              {/* Transaction info */}
              <div className="table-card" style={{ marginBottom: 16 }}>
                <div className="table-toolbar">
                  <div className="card-title">Transaction</div>
                </div>
                <div style={{ padding: 16, fontSize: 12 }}>
                  <div style={{ marginBottom: 6 }}>
                    <span style={{ color: 'var(--text-muted)' }}>ACS Trans ID:</span>
                    <div className="mono" style={{ marginTop: 2, wordBreak: 'break-all' }}>{interactiveInfo.acsTransID}</div>
                  </div>
                  <div style={{ marginBottom: 6 }}>
                    <span style={{ color: 'var(--text-muted)' }}>Flow:</span>
                    <div style={{ marginTop: 2 }}><FlowBadge flow={interactiveInfo.flowType} /></div>
                  </div>
                  <div>
                    <span style={{ color: 'var(--text-muted)' }}>Status:</span>
                    <div style={{ marginTop: 2 }}>
                      <span className={`badge ${statusColor(aresData?.transStatus)}`}>
                        {statusLabel(aresData?.transStatus)}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  );
}