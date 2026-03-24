import React, { useEffect, useState, useCallback } from 'react';
import { fetchJSON } from '../api';
import { formatDate } from './Dashboard';
import { Pagination } from './Transactions';
import { SearchInput } from '../components/SearchInput';

const PER_PAGE = 20;

const EVENT_BADGE = {
  auth_frictionless:     ['badge-green',  'Frictionless'],
  auth_challenge_start:  ['badge-yellow', 'Challenge Start'],
  challenge_success:     ['badge-green',  'Challenge OK'],
  challenge_fail:        ['badge-red',    'Challenge Fail'],
  challenge_cancel:      ['badge-orange', 'Cancelled'],
  challenge_timeout:     ['badge-muted',  'Timeout'],
  auth_unavailable:      ['badge-muted',  'Unavailable'],
  method_collected:      ['badge-cyan',   'Method'],
  decoupled_start:       ['badge-cyan',   'OOB Start'],
  decoupled_complete:    ['badge-green',  'OOB Complete'],
};

export default function AuditLogs() {
  const [rows, setRows] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [eventType, setEventType] = useState('');
  const [search, setSearch] = useState('');

  const load = useCallback(() => {
    setLoading(true);
    fetchJSON('/audit-logs', { event_type: eventType, search, page, per_page: PER_PAGE })
      .then(d => { setRows(d.data || []); setTotal(d.total || 0); })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [page, eventType, search]);

  useEffect(() => { load(); }, [load]);

  const totalPages = Math.ceil(total / PER_PAGE) || 1;

  return (
    <>
      <div className="page-header">
        <h1>Audit Logs</h1>
        <p>Authentication lifecycle event trail</p>
      </div>

      <div className="table-card">
        <div className="table-toolbar">
          <div className="card-title">Events ({total})</div>
          <div className="table-filters">
            <SearchInput value={search} onChange={v => { setSearch(v); setPage(1); }} placeholder="Search audit logs…" />
            <select className="filter-select" value={eventType} onChange={e => { setEventType(e.target.value); setPage(1); }}>
              <option value="">All Events</option>
              {Object.entries(EVENT_BADGE).map(([key, [, label]]) => (
                <option key={key} value={key}>{label}</option>
              ))}
            </select>
          </div>
        </div>

        <div className="table-wrap">
          {loading ? (
            <div className="spinner-wrap"><div className="spinner" /></div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Event</th>
                  <th>ACS Trans ID</th>
                  <th>Status</th>
                  <th>Card BIN</th>
                  <th>Channel</th>
                  <th>Version</th>
                  <th>Detail</th>
                  <th>Time</th>
                </tr>
              </thead>
              <tbody>
                {rows.map(row => (
                  <tr key={row.id}>
                    <td className="mono">{row.id}</td>
                    <td><EventBadge type={row.eventType} /></td>
                    <td className="mono truncate" title={row.acsTransID}>{row.acsTransID.slice(0, 8)}…</td>
                    <td><StatusBadgeMini status={row.transStatus} /></td>
                    <td className="mono">{row.cardBIN || '—'}</td>
                    <td><ChannelLabel ch={row.deviceChannel} /></td>
                    <td className="mono">{row.specVersion}</td>
                    <td className="mono truncate" title={detailStr(row.detail)} style={{ maxWidth: 180 }}>
                      {detailStr(row.detail) || '—'}
                    </td>
                    <td>{formatDate(row.createdAt)}</td>
                  </tr>
                ))}
                {rows.length === 0 && (
                  <tr><td colSpan={9} className="empty-state">No audit events found</td></tr>
                )}
              </tbody>
            </table>
          )}
        </div>

        <Pagination page={page} totalPages={totalPages} total={total} perPage={PER_PAGE} onPageChange={setPage} />
      </div>
    </>
  );
}

function EventBadge({ type }) {
  const [cls, label] = EVENT_BADGE[type] || ['badge-muted', type || '—'];
  return <span className={`badge ${cls}`}>{label}</span>;
}

function StatusBadgeMini({ status }) {
  const colors = { Y: 'green', N: 'red', C: 'yellow', U: 'muted', D: 'cyan', A: 'cyan', R: 'orange' };
  const c = colors[status] || 'muted';
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
      <span className={`badge-dot badge-dot-${c}`} />
      {status || '—'}
    </span>
  );
}

function ChannelLabel({ ch }) {
  const map = { '01': 'App', '02': 'Browser', '03': '3RI' };
  return <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>{map[ch] || ch || '—'}</span>;
}

function detailStr(detail) {
  if (!detail) return '';
  if (typeof detail === 'string') return detail;
  try { return JSON.stringify(detail); } catch { return ''; }
}