import React, { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { fetchJSON } from '../api';
import { StatusBadge, formatDate } from './Dashboard';
import { NetworkBadge } from '../components/NetworkBadge';

const STATUS_COLORS = {
  Y: '#009464', N: '#d63031', C: '#d4a017', U: '#8b92a5', D: '#0891b2', A: '#009464', R: '#e17055',
};

const EVENT_LABELS = {
  auth_frictionless: 'Frictionless Auth',
  auth_challenge_start: 'Challenge Started',
  challenge_success: 'Challenge Success',
  challenge_fail: 'Challenge Failed',
  challenge_cancel: 'Challenge Cancelled',
  challenge_timeout: 'Challenge Timeout',
  decoupled_start: 'OOB Initiated',
  decoupled_complete: 'OOB Complete',
};

const EVENT_DESCRIPTIONS = {
  auth_frictionless: 'Transaction authenticated without cardholder interaction (risk-based).',
  auth_challenge_start: 'Challenge flow initiated. OTP generated and sent to cardholder.',
  challenge_success: 'Cardholder submitted the correct OTP. Transaction authenticated.',
  challenge_fail: 'Incorrect OTP submitted. Attempt recorded.',
  challenge_cancel: 'Cardholder cancelled the challenge. Transaction denied.',
  challenge_timeout: 'Challenge timer expired. Transaction status set to unavailable.',
  decoupled_start: 'Out-of-band authentication initiated. Push notification sent.',
  decoupled_complete: 'Out-of-band authentication completed.',
};

export default function TransactionDrilldown() {
  const { acsTransID } = useParams();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [expandedMsg, setExpandedMsg] = useState(null);

  useEffect(() => {
    setLoading(true);
    fetchJSON(`/transactions/${acsTransID}`)
      .then(setData)
      .catch(() => setData(null))
      .finally(() => setLoading(false));
  }, [acsTransID]);

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;
  if (!data) return (
    <div className="page-header">
      <h1>Transaction Not Found</h1>
      <p><Link to="/transactions" style={{ color: 'var(--accent)' }}>Back to Transactions</Link></p>
    </div>
  );

  const tx = data.transaction;
  const messages = data.messages || [];
  const auditLogs = data.auditLogs || [];

  // Build flow steps from audit logs
  const flowSteps = buildFlowSteps(tx, auditLogs);

  return (
    <>
      <div className="page-header">
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <Link to="/transactions" className="btn btn-small btn-secondary" style={{ textDecoration: 'none' }}>
            &larr; Back
          </Link>
          <div>
            <h1 style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              Transaction Drilldown
              <StatusBadge status={tx.transStatus} />
            </h1>
            <p className="mono" style={{ fontSize: 13, marginTop: 2 }}>{tx.acsTransID}</p>
          </div>
        </div>
      </div>

      {/* Flow Graph */}
      <div className="table-card" style={{ marginBottom: 24 }}>
        <div className="table-toolbar">
          <div className="card-title">Transaction Flow</div>
        </div>
        <div style={{ padding: '24px 16px', overflowX: 'auto' }}>
          <div className="flow-graph">
            {flowSteps.map((step, i) => (
              <React.Fragment key={i}>
                {i > 0 && <div className="flow-arrow" />}
                <div className={`flow-node flow-node-${step.color}`} title={step.tooltip}>
                  <div className="flow-node-icon">{step.icon}</div>
                  <div className="flow-node-label">{step.label}</div>
                  <div className="flow-node-time">{step.time}</div>
                </div>
              </React.Fragment>
            ))}
          </div>
        </div>
      </div>

      {/* Transaction Details */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 24 }}>
        <div className="table-card">
          <div className="table-toolbar"><div className="card-title">Transaction Info</div></div>
          <div style={{ padding: 16 }}>
            <InfoRow label="ACS Trans ID" value={tx.acsTransID} mono />
            <InfoRow label="DS Trans ID" value={tx.dsTransID} mono />
            <InfoRow label="Server Trans ID" value={tx.serverTransID} mono />
            <InfoRow label="Status" value={<StatusBadge status={tx.transStatus} />} />
            <InfoRow label="Network" value={<NetworkBadge network={tx.network} />} />
            <InfoRow label="Device Channel" value={tx.deviceChannel} />
            <InfoRow label="Challenge Attempts" value={tx.challengeAttempts} />
          </div>
        </div>
        <div className="table-card">
          <div className="table-toolbar"><div className="card-title">Merchant & Payment</div></div>
          <div style={{ padding: 16 }}>
            <InfoRow label="Merchant" value={tx.merchantName} />
            <InfoRow label="Country" value={tx.merchantCountry} />
            <InfoRow label="Amount" value={tx.purchaseAmount || '—'} mono />
            <InfoRow label="Currency" value={tx.purchaseCurrency || '—'} />
            <InfoRow label="Auth Value" value={tx.authValue || '—'} mono />
            <InfoRow label="ECI" value={tx.eci || '—'} mono />
            <InfoRow label="Created" value={formatDate(tx.createdAt)} />
            <InfoRow label="Completed" value={formatDate(tx.completedAt)} />
          </div>
        </div>
      </div>

      {/* Messages (Request/Response) */}
      <div className="table-card" style={{ marginBottom: 24 }}>
        <div className="table-toolbar">
          <div className="card-title">Protocol Messages ({messages.length})</div>
        </div>
        {messages.length === 0 ? (
          <div className="empty-state" style={{ padding: 24 }}>No messages recorded</div>
        ) : (
          <div style={{ padding: 0 }}>
            {messages.map((msg, i) => (
              <div key={msg.id || i} className="msg-row">
                <div className="msg-header" onClick={() => setExpandedMsg(expandedMsg === i ? null : i)}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span className={`badge ${msg.direction === 'inbound' ? 'badge-cyan' : 'badge-green'}`}>
                      {msg.direction === 'inbound' ? 'IN' : 'OUT'}
                    </span>
                    <span style={{ fontWeight: 600 }}>{msg.messageType}</span>
                  </div>
                  <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>{formatDate(msg.createdAt)}</span>
                  <span style={{ fontSize: 12, color: 'var(--text-muted)', marginLeft: 'auto', cursor: 'pointer' }}>
                    {expandedMsg === i ? '▼' : '▶'}
                  </span>
                </div>
                {expandedMsg === i && (
                  <div className="msg-body">
                    <pre className="msg-json">{formatJSON(msg.content)}</pre>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Audit Trail */}
      <div className="table-card">
        <div className="table-toolbar">
          <div className="card-title">Audit Trail ({auditLogs.length})</div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Event</th>
                <th>Status</th>
                <th>Detail</th>
                <th>Time</th>
              </tr>
            </thead>
            <tbody>
              {auditLogs.map(log => (
                <tr key={log.id}>
                  <td style={{ fontWeight: 500 }}>{EVENT_LABELS[log.eventType] || log.eventType}</td>
                  <td><StatusBadge status={log.transStatus} /></td>
                  <td style={{ maxWidth: 400 }}>
                    {log.detail ? (
                      <details>
                        <summary style={{ cursor: 'pointer', fontSize: 12, color: 'var(--text-muted)' }}>View detail</summary>
                        <pre style={{ fontSize: 11, marginTop: 4, whiteSpace: 'pre-wrap', color: 'var(--text-secondary)' }}>
                          {formatJSON(log.detail)}
                        </pre>
                      </details>
                    ) : '—'}
                  </td>
                  <td>{formatDate(log.createdAt)}</td>
                </tr>
              ))}
              {auditLogs.length === 0 && (
                <tr><td colSpan={4} className="empty-state">No audit entries</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}

function InfoRow({ label, value, mono }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--border)', fontSize: 13 }}>
      <span style={{ color: 'var(--text-muted)' }}>{label}</span>
      <span className={mono ? 'mono' : ''} style={{ color: 'var(--text-primary)', textAlign: 'right', maxWidth: '60%', wordBreak: 'break-all' }}>
        {value}
      </span>
    </div>
  );
}

function formatJSON(obj) {
  if (!obj) return '';
  if (typeof obj === 'string') {
    try { return JSON.stringify(JSON.parse(obj), null, 2); }
    catch { return obj; }
  }
  return JSON.stringify(obj, null, 2);
}

function buildFlowSteps(tx, auditLogs) {
  const steps = [];
  const ts = (iso) => {
    if (!iso) return '';
    const d = new Date(iso);
    return d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  // AReq → ARes is always the first step
  steps.push({
    label: 'AReq / ARes',
    icon: '1',
    color: 'green',
    time: ts(tx.createdAt),
    tooltip: 'Authentication Request received from DS. ACS evaluated risk and returned Authentication Response.',
  });

  for (const log of auditLogs) {
    switch (log.eventType) {
      case 'auth_frictionless':
        steps.push({
          label: 'Frictionless',
          icon: '✓',
          color: 'green',
          time: ts(log.createdAt),
          tooltip: EVENT_DESCRIPTIONS.auth_frictionless,
        });
        break;
      case 'auth_challenge_start':
        steps.push({
          label: 'Challenge',
          icon: '🔒',
          color: 'yellow',
          time: ts(log.createdAt),
          tooltip: EVENT_DESCRIPTIONS.auth_challenge_start,
        });
        break;
      case 'challenge_success':
        steps.push({
          label: 'OTP Verified',
          icon: '✓',
          color: 'green',
          time: ts(log.createdAt),
          tooltip: EVENT_DESCRIPTIONS.challenge_success,
        });
        break;
      case 'challenge_fail':
        steps.push({
          label: 'OTP Failed',
          icon: '✗',
          color: 'red',
          time: ts(log.createdAt),
          tooltip: EVENT_DESCRIPTIONS.challenge_fail,
        });
        break;
      case 'challenge_cancel':
        steps.push({
          label: 'Cancelled',
          icon: '✗',
          color: 'red',
          time: ts(log.createdAt),
          tooltip: EVENT_DESCRIPTIONS.challenge_cancel,
        });
        break;
      case 'challenge_timeout':
        steps.push({
          label: 'Timeout',
          icon: '⏱',
          color: 'gray',
          time: ts(log.createdAt),
          tooltip: EVENT_DESCRIPTIONS.challenge_timeout,
        });
        break;
      case 'decoupled_start':
        steps.push({
          label: 'OOB Started',
          icon: '📱',
          color: 'cyan',
          time: ts(log.createdAt),
          tooltip: EVENT_DESCRIPTIONS.decoupled_start,
        });
        break;
      case 'decoupled_complete':
        steps.push({
          label: 'OOB Complete',
          icon: log.transStatus === 'Y' ? '✓' : '✗',
          color: log.transStatus === 'Y' ? 'green' : 'red',
          time: ts(log.createdAt),
          tooltip: EVENT_DESCRIPTIONS.decoupled_complete,
        });
        break;
      default:
        break;
    }
  }

  // Final status if not captured by audit
  if (steps.length === 1) {
    const statusMap = {
      Y: { label: 'Authenticated', icon: '✓', color: 'green' },
      N: { label: 'Denied', icon: '✗', color: 'red' },
      U: { label: 'Unavailable', icon: '?', color: 'gray' },
      C: { label: 'Pending', icon: '…', color: 'yellow' },
      D: { label: 'Decoupled', icon: '📱', color: 'cyan' },
      R: { label: 'Rejected', icon: '✗', color: 'red' },
    };
    const s = statusMap[tx.transStatus] || { label: tx.transStatus, icon: '?', color: 'gray' };
    steps.push({ ...s, time: ts(tx.completedAt), tooltip: `Final transaction status: ${tx.transStatus}` });
  }

  return steps;
}