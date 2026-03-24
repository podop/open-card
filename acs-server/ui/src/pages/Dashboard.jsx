import React, { useEffect, useState } from 'react';
import { fetchJSON } from '../api';
import { NetworkBadge } from '../components/NetworkBadge';
import { SearchInput } from '../components/SearchInput';
import { useNetworks, networkColorMap } from '../hooks/useNetworks';
import {
  PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend,
  AreaChart, Area, XAxis, YAxis, CartesianGrid,
} from 'recharts';

const STATUS_COLORS = {
  Y: '#009464', N: '#d63031', C: '#d4a017', U: '#8b92a5',
  D: '#0891b2', A: '#009464', R: '#e17055', I: '#8b92a5',
};

const STATUS_LABELS = {
  Y: 'Authenticated', N: 'Not Auth', C: 'Challenge', U: 'Unavailable',
  D: 'Decoupled', A: 'Attempt', R: 'Rejected', I: 'Info Only',
};

const PIE_LABEL = ({ name, percent }) =>
  percent > 0.04 ? `${name} ${(percent * 100).toFixed(0)}%` : '';

function StatCard({ icon, bg, label, value, sub }) {
  return (
    <div className="stat-card">
      <div className="stat-icon" style={{ background: bg }}>
        <span style={{ fontSize: 18 }}>{icon}</span>
      </div>
      <div className="stat-info">
        <div className="stat-label">{label}</div>
        <div className="stat-value">{value ?? '—'}</div>
        {sub && <div className="stat-sub">{sub}</div>}
      </div>
    </div>
  );
}

function ChartCard({ title, children }) {
  return (
    <div className="chart-card">
      <div className="card-title">{title}</div>
      {children}
    </div>
  );
}

const CustomTooltipStyle = {
  backgroundColor: '#ffffff', border: '1px solid #e2e6ed',
  borderRadius: 6, padding: '8px 12px', fontSize: 12, color: '#1a1f2b',
  boxShadow: '0 4px 12px rgba(0,0,0,0.07)',
};

export default function Dashboard() {
  const { networks: netEntries } = useNetworks();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [recentSearch, setRecentSearch] = useState('');

  useEffect(() => {
    fetchJSON('/dashboard')
      .then(setData)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;
  if (!data) return <div className="empty-state">Failed to load dashboard</div>;

  const { stats, networks, activity, recent } = data;

  // Build status distribution from recent + stats
  const statusData = [
    { name: 'Authenticated', value: stats.frictionless || 0 },
    { name: 'Challenge', value: stats.challenged || 0 },
    { name: 'Failed', value: stats.failed || 0 },
    { name: 'Unavailable', value: stats.unavailable || 0 },
  ].filter(d => d.value > 0);
  const statusColors = ['#009464', '#d4a017', '#d63031', '#8b92a5'];

  // Network pie
  const networkData = (networks || []).map(n => ({
    name: n.network, value: n.count,
  }));
  const colorMap = networkColorMap(netEntries);
  const networkColors = networkData.map(n => colorMap[n.name] || '#6366f1');

  // Confirmation flow breakdown (estimate from recent)
  const otpCount = (recent || []).filter(r => r.challengeAttempts > 0 && r.transStatus !== 'D').length;
  const oobCount = (recent || []).filter(r => r.transStatus === 'D').length;
  const frictionCount = (recent || []).filter(r => r.transStatus === 'Y' && r.challengeAttempts === 0).length;
  const flowData = [
    { name: 'Frictionless', value: stats.frictionless || frictionCount || 1 },
    { name: 'OTP Challenge', value: stats.challenged || otpCount || 0 },
    { name: 'OOB/Decoupled', value: oobCount || 0 },
  ].filter(d => d.value > 0);
  const flowColors = ['#009464', '#d4a017', '#0891b2'];

  // Activity timeline
  const activityData = (activity || []).sort((a, b) => a.hour.localeCompare(b.hour)).map(p => ({
    hour: p.hour.slice(11, 16),
    total: p.total,
    auth: p.auth,
  }));

  return (
    <>
      <div className="page-header">
        <h1>Dashboard</h1>
        <p>3D Secure authentication overview</p>
      </div>

      <div className="stat-grid">
        <StatCard icon="📊" bg="var(--accent-dim)" label="Total Transactions" value={stats.total} />
        <StatCard icon="✅" bg="var(--green-dim)" label="Authenticated" value={stats.authenticated} sub={stats.total ? `${((stats.authenticated / stats.total) * 100).toFixed(1)}% rate` : ''} />
        <StatCard icon="🔐" bg="var(--yellow-dim)" label="Challenged" value={stats.challenged} />
        <StatCard icon="❌" bg="var(--red-dim)" label="Failed" value={stats.failed} />
        <StatCard icon="⚠️" bg="var(--orange-dim)" label="Unavailable" value={stats.unavailable} />
      </div>

      <div className="chart-grid">
        <ChartCard title="By Network">
          {networkData.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <PieChart>
                <Pie data={networkData} dataKey="value" nameKey="name" cx="50%" cy="50%"
                  outerRadius={75} innerRadius={40} paddingAngle={2}
                  label={PIE_LABEL} labelLine={false}>
                  {networkData.map((_, i) => <Cell key={i} fill={networkColors[i]} />)}
                </Pie>
                <Tooltip contentStyle={CustomTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          ) : <div className="empty-state">No data</div>}
        </ChartCard>

        <ChartCard title="By Status">
          {statusData.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <PieChart>
                <Pie data={statusData} dataKey="value" nameKey="name" cx="50%" cy="50%"
                  outerRadius={75} innerRadius={40} paddingAngle={2}
                  label={PIE_LABEL} labelLine={false}>
                  {statusData.map((_, i) => <Cell key={i} fill={statusColors[i % statusColors.length]} />)}
                </Pie>
                <Tooltip contentStyle={CustomTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          ) : <div className="empty-state">No data</div>}
        </ChartCard>

        <ChartCard title="By Flow">
          {flowData.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <PieChart>
                <Pie data={flowData} dataKey="value" nameKey="name" cx="50%" cy="50%"
                  outerRadius={75} innerRadius={40} paddingAngle={2}
                  label={PIE_LABEL} labelLine={false}>
                  {flowData.map((_, i) => <Cell key={i} fill={flowColors[i]} />)}
                </Pie>
                <Tooltip contentStyle={CustomTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          ) : <div className="empty-state">No data</div>}
        </ChartCard>

        <ChartCard title="Activity Timeline">
          {activityData.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <AreaChart data={activityData}>
                <defs>
                  <linearGradient id="gTotal" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#009464" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#009464" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="gAuth" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#009464" stopOpacity={0.2} />
                    <stop offset="95%" stopColor="#009464" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e6ed" />
                <XAxis dataKey="hour" tick={{ fill: '#8b92a5', fontSize: 11 }} tickLine={false} axisLine={false} />
                <YAxis tick={{ fill: '#8b92a5', fontSize: 11 }} tickLine={false} axisLine={false} width={30} />
                <Tooltip contentStyle={CustomTooltipStyle} />
                <Area type="monotone" dataKey="total" stroke="#009464" fill="url(#gTotal)" strokeWidth={2} />
                <Area type="monotone" dataKey="auth" stroke="#007a53" fill="url(#gAuth)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : <div className="empty-state">No data</div>}
        </ChartCard>
      </div>

      {/* Recent transactions mini-table */}
      <div className="table-card">
        <div className="table-toolbar">
          <div className="card-title">Recent Transactions</div>
          <SearchInput value={recentSearch} onChange={setRecentSearch} placeholder="Search recent…" />
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ACS Trans ID</th>
                <th>Network</th>
                <th>Status</th>
                <th>Merchant</th>
                <th>Amount</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {(recent || []).filter(tx => {
                if (!recentSearch) return true;
                const s = recentSearch.toLowerCase();
                return [tx.acsTransID, tx.network, tx.transStatus, tx.merchantName, tx.purchaseAmount, tx.purchaseCurrency]
                  .some(v => (v || '').toLowerCase().includes(s));
              }).map(tx => (
                <tr key={tx.acsTransID}>
                  <td className="mono truncate" title={tx.acsTransID}>{tx.acsTransID.slice(0, 8)}…</td>
                  <td><NetworkBadge network={tx.network} /></td>
                  <td><StatusBadge status={tx.transStatus} /></td>
                  <td>{tx.merchantName}</td>
                  <td className="mono">{tx.purchaseAmount || '—'} {tx.purchaseCurrency || ''}</td>
                  <td>{formatDate(tx.createdAt)}</td>
                </tr>
              ))}
              {(!recent || recent.length === 0) && (
                <tr><td colSpan={6} className="empty-state">No transactions yet</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}

export function StatusBadge({ status }) {
  const map = {
    Y: ['badge-green', 'Authenticated'],
    N: ['badge-red', 'Failed'],
    C: ['badge-yellow', 'Challenge'],
    U: ['badge-muted', 'Unavailable'],
    D: ['badge-cyan', 'Decoupled'],
    A: ['badge-cyan', 'Attempt'],
    R: ['badge-orange', 'Rejected'],
    I: ['badge-muted', 'Info'],
  };
  const [cls, label] = map[status] || ['badge-muted', status || '—'];
  return <span className={`badge ${cls}`}>{label}</span>;
}

export function formatDate(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  return d.toLocaleString('en-GB', {
    day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit', hour12: false,
  });
}