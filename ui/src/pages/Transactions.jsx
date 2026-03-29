import React, { useEffect, useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { fetchJSON } from '../api';
import { StatusBadge, formatDate } from './Dashboard';
import { NetworkBadge } from '../components/NetworkBadge';
import { SearchInput } from '../components/SearchInput';
import { useNetworks, networkColorMap } from '../hooks/useNetworks';
import {
  PieChart, Pie, Cell, Tooltip, ResponsiveContainer,
  AreaChart, Area, XAxis, YAxis, CartesianGrid,
} from 'recharts';

const PER_PAGE = 20;
const STATUS_COLORS = {
  Y: '#009464', N: '#d63031', C: '#d4a017', U: '#8b92a5',
  D: '#0891b2', A: '#009464', R: '#e17055',
};
const FLOW_COLORS = ['#009464', '#d4a017', '#0891b2'];

const CustomTooltipStyle = {
  backgroundColor: '#ffffff', border: '1px solid #e2e6ed',
  borderRadius: 6, padding: '8px 12px', fontSize: 12, color: '#1a1f2b',
  boxShadow: '0 4px 12px rgba(0,0,0,0.07)',
};
const PIE_LABEL = ({ name, percent }) =>
  percent > 0.04 ? `${name} ${(percent * 100).toFixed(0)}%` : '';

export default function Transactions() {
  const { networks: netEntries } = useNetworks();
  const [rows, setRows] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filters, setFilters] = useState({ status: '', network: '', channel: '', search: '' });

  // Chart data from dashboard endpoint
  const [chartData, setChartData] = useState(null);

  const load = useCallback(() => {
    setLoading(true);
    fetchJSON('/transactions', { ...filters, page, per_page: PER_PAGE })
      .then(d => { setRows(d.data || []); setTotal(d.total || 0); })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [page, filters]);

  useEffect(() => { load(); }, [load]);
  useEffect(() => {
    fetchJSON('/dashboard').then(setChartData).catch(() => {});
  }, []);

  const totalPages = Math.ceil(total / PER_PAGE) || 1;
  const setFilter = (k, v) => { setFilters(f => ({ ...f, [k]: v })); setPage(1); };

  // Derive chart data
  const networkData = (chartData?.networks || []).map(n => ({ name: n.network, value: n.count }));
  const colorMap = networkColorMap(netEntries);
  const networkColors = networkData.map(n => colorMap[n.name] || '#6366f1');

  const stats = chartData?.stats || {};
  const statusData = [
    { name: 'Auth', value: stats.frictionless || 0 },
    { name: 'Challenge', value: stats.challenged || 0 },
    { name: 'Failed', value: stats.failed || 0 },
    { name: 'Unavailable', value: stats.unavailable || 0 },
  ].filter(d => d.value > 0);
  const statusPieColors = ['#009464', '#d4a017', '#d63031', '#8b92a5'];

  const flowData = [
    { name: 'Frictionless', value: stats.frictionless || 0 },
    { name: 'OTP', value: stats.challenged || 0 },
    { name: 'OOB', value: 0 },
  ].filter(d => d.value > 0);

  const activityData = (chartData?.activity || [])
    .sort((a, b) => a.hour.localeCompare(b.hour))
    .map(p => ({ hour: p.hour.slice(11, 16), total: p.total, auth: p.auth }));

  return (
    <>
      <div className="page-header">
        <h1>Transactions</h1>
        <p>3DS authentication transaction records</p>
      </div>

      <div className="chart-grid">
        <ChartCard title="By Network">
          {networkData.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <PieChart>
                <Pie data={networkData} dataKey="value" nameKey="name" cx="50%" cy="50%"
                  outerRadius={65} innerRadius={35} paddingAngle={2}
                  label={PIE_LABEL} labelLine={false}>
                  {networkData.map((_, i) => <Cell key={i} fill={networkColors[i]} />)}
                </Pie>
                <Tooltip contentStyle={CustomTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          ) : <EmptyChart />}
        </ChartCard>

        <ChartCard title="By Status">
          {statusData.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <PieChart>
                <Pie data={statusData} dataKey="value" nameKey="name" cx="50%" cy="50%"
                  outerRadius={65} innerRadius={35} paddingAngle={2}
                  label={PIE_LABEL} labelLine={false}>
                  {statusData.map((_, i) => <Cell key={i} fill={statusPieColors[i]} />)}
                </Pie>
                <Tooltip contentStyle={CustomTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          ) : <EmptyChart />}
        </ChartCard>

        <ChartCard title="By Flow">
          {flowData.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <PieChart>
                <Pie data={flowData} dataKey="value" nameKey="name" cx="50%" cy="50%"
                  outerRadius={65} innerRadius={35} paddingAngle={2}
                  label={PIE_LABEL} labelLine={false}>
                  {flowData.map((_, i) => <Cell key={i} fill={FLOW_COLORS[i]} />)}
                </Pie>
                <Tooltip contentStyle={CustomTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          ) : <EmptyChart />}
        </ChartCard>

        <ChartCard title="Activity">
          {activityData.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={activityData}>
                <defs>
                  <linearGradient id="gt" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#009464" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#009464" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e6ed" />
                <XAxis dataKey="hour" tick={{ fill: '#8b92a5', fontSize: 10 }} tickLine={false} axisLine={false} />
                <YAxis tick={{ fill: '#8b92a5', fontSize: 10 }} tickLine={false} axisLine={false} width={25} />
                <Tooltip contentStyle={CustomTooltipStyle} />
                <Area type="monotone" dataKey="total" stroke="#009464" fill="url(#gt)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : <EmptyChart />}
        </ChartCard>
      </div>

      <div className="table-card">
        <div className="table-toolbar">
          <div className="card-title">All Transactions ({total})</div>
          <div className="table-filters">
            <SearchInput value={filters.search} onChange={v => setFilter('search', v)} placeholder="Search transactions…" />
            <select className="filter-select" value={filters.status} onChange={e => setFilter('status', e.target.value)}>
              <option value="">All Status</option>
              {['Y', 'N', 'C', 'U', 'D', 'A', 'R'].map(s => (
                <option key={s} value={s}>{s}</option>
              ))}
            </select>
            <select className="filter-select" value={filters.network} onChange={e => setFilter('network', e.target.value)}>
              <option value="">All Networks</option>
              {['visa', 'mastercard', 'amex', 'discover'].map(n => (
                <option key={n} value={n}>{n.charAt(0).toUpperCase() + n.slice(1)}</option>
              ))}
            </select>
            <select className="filter-select" value={filters.channel} onChange={e => setFilter('channel', e.target.value)}>
              <option value="">All Channels</option>
              <option value="01">App (01)</option>
              <option value="02">Browser (02)</option>
              <option value="03">3RI (03)</option>
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
                  <th>ACS Trans ID</th>
                  <th>DS Trans ID</th>
                  <th>Network</th>
                  <th>Channel</th>
                  <th>Status</th>
                  <th>Merchant</th>
                  <th>Amount</th>
                  <th>Attempts</th>
                  <th>Created</th>
                  <th>Completed</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {rows.map(tx => (
                  <tr key={tx.acsTransID}>
                    <td className="mono truncate" title={tx.acsTransID}>{tx.acsTransID.slice(0, 8)}…</td>
                    <td className="mono truncate" title={tx.dsTransID}>{tx.dsTransID.slice(0, 8)}…</td>
                    <td><NetworkBadge network={tx.network} /></td>
                    <td><ChannelBadge ch={tx.deviceChannel} /></td>
                    <td><StatusBadge status={tx.transStatus} /></td>
                    <td>{tx.merchantName}</td>
                    <td className="mono">{tx.purchaseAmount || '—'}{tx.purchaseCurrency ? ` ${tx.purchaseCurrency}` : ''}</td>
                    <td style={{ textAlign: 'center' }}>{tx.challengeAttempts}</td>
                    <td>{formatDate(tx.createdAt)}</td>
                    <td>{formatDate(tx.completedAt)}</td>
                    <td>
                      <Link to={`/transactions/${tx.acsTransID}`} className="btn btn-small btn-secondary" style={{ textDecoration: 'none' }}>
                        View
                      </Link>
                    </td>
                  </tr>
                ))}
                {rows.length === 0 && (
                  <tr><td colSpan={11} className="empty-state">No transactions found</td></tr>
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

function ChannelBadge({ ch }) {
  const map = { '01': ['badge-cyan', 'App'], '02': ['badge-orange', 'Browser'], '03': ['badge-muted', '3RI'] };
  const [cls, label] = map[ch] || ['badge-muted', ch || '—'];
  return <span className={`badge ${cls}`}>{label}</span>;
}

function ChartCard({ title, children }) {
  return (
    <div className="chart-card">
      <div className="card-title">{title}</div>
      {children}
    </div>
  );
}

function EmptyChart() {
  return <div style={{ height: 200, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-muted)', fontSize: 12 }}>No data</div>;
}

export function Pagination({ page, totalPages, total, perPage, onPageChange }) {
  const from = (page - 1) * perPage + 1;
  const to = Math.min(page * perPage, total);

  const pageNums = [];
  const start = Math.max(1, page - 2);
  const end = Math.min(totalPages, page + 2);
  for (let i = start; i <= end; i++) pageNums.push(i);

  return (
    <div className="pagination">
      <span>{total > 0 ? `Showing ${from}–${to} of ${total}` : 'No results'}</span>
      <div className="pagination-buttons">
        <button disabled={page <= 1} onClick={() => onPageChange(page - 1)}>Prev</button>
        {pageNums.map(n => (
          <button key={n} className={n === page ? 'active' : ''} onClick={() => onPageChange(n)}>
            {n}
          </button>
        ))}
        <button disabled={page >= totalPages} onClick={() => onPageChange(page + 1)}>Next</button>
      </div>
    </div>
  );
}