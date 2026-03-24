import React, { useEffect, useState, useCallback } from 'react';
import { fetchJSON } from '../api';
import { FlowBadge } from './Scenarios';
import { Pagination } from './Transactions';
import { SearchInput } from '../components/SearchInput';
import {
  PieChart, Pie, Cell, Tooltip, ResponsiveContainer,
  BarChart, Bar, XAxis, YAxis, CartesianGrid,
} from 'recharts';

const PER_PAGE = 20;
const FLOW_COLORS = {
  frictionless: '#009464',
  challenge_otp: '#d4a017',
  challenge_oob: '#0891b2',
  decoupled: '#6366f1',
};
const STATUS_COLORS = { completed: '#009464', failed: '#d63031', pending: '#8b92a5', running: '#d4a017' };

const CustomTooltipStyle = {
  backgroundColor: '#ffffff', border: '1px solid #e2e6ed',
  borderRadius: 6, padding: '8px 12px', fontSize: 12, color: '#1a1f2b',
  boxShadow: '0 4px 12px rgba(0,0,0,0.07)',
};

const PIE_LABEL = ({ name, percent }) =>
  percent > 0.04 ? `${name} ${(percent * 100).toFixed(0)}%` : '';

export default function RunStats() {
  const [stats, setStats] = useState(null);
  const [flowStats, setFlowStats] = useState([]);
  const [recent, setRecent] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [runSearch, setRunSearch] = useState('');

  const loadStats = useCallback(() => {
    fetchJSON('/scenario-runs/stats')
      .then(d => { setStats(d.stats); setFlowStats(d.byFlow || []); })
      .catch(() => {});
  }, []);

  const loadRecent = useCallback(() => {
    setLoading(true);
    fetchJSON('/scenario-runs', { search: runSearch, page, per_page: PER_PAGE })
      .then(d => { setRecent(d.data || []); setTotal(d.total || 0); })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [page, runSearch]);

  useEffect(() => { loadStats(); loadRecent(); }, [loadStats, loadRecent]);

  const totalPages = Math.ceil(total / PER_PAGE) || 1;

  // Derive chart data
  const flowPieData = flowStats.map(f => ({
    name: f.flowType.replace('_', ' '),
    value: f.total,
  }));
  const flowPieColors = flowStats.map(f => FLOW_COLORS[f.flowType] || '#8b92a5');

  const statusPieData = stats ? [
    { name: 'Completed', value: stats.completedRuns },
    { name: 'Failed', value: stats.failedRuns },
    { name: 'Other', value: Math.max(0, stats.totalRuns - stats.completedRuns - stats.failedRuns) },
  ].filter(d => d.value > 0) : [];
  const statusPieColors = ['#009464', '#d63031', '#8b92a5'];

  const barData = flowStats.map(f => ({
    name: f.flowType.replace('challenge_', '').toUpperCase(),
    Completed: f.completed,
    Failed: f.failed,
  }));

  return (
    <>
      <div className="page-header">
        <h1>Run Statistics</h1>
        <p>Aggregate results from scenario test executions</p>
      </div>

      {/* Stat cards */}
      {stats && (
        <div className="stat-grid">
          <StatCard label="Total Runs" value={stats.totalRuns} icon="🧪" bg="var(--accent-dim)" />
          <StatCard label="Completed" value={stats.completedRuns} icon="✅" bg="var(--green-dim)" />
          <StatCard label="Failed" value={stats.failedRuns} icon="❌" bg="var(--red-dim)" />
          <StatCard label="Avg Duration" value={`${Math.round(stats.avgDurationMs)}ms`} icon="⏱" bg="var(--cyan-dim)" />
          <StatCard label="Success Rate"
            value={stats.totalRuns > 0 ? `${((stats.completedRuns / stats.totalRuns) * 100).toFixed(1)}%` : '—'}
            icon="📈" bg="var(--accent-dim)" />
        </div>
      )}

      {/* Charts */}
      <div className="chart-grid" style={{ gridTemplateColumns: 'repeat(3, 1fr)' }}>
        <ChartCard title="By Flow Type">
          {flowPieData.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <PieChart>
                <Pie data={flowPieData} dataKey="value" nameKey="name" cx="50%" cy="50%"
                  outerRadius={75} innerRadius={40} paddingAngle={2}
                  label={PIE_LABEL} labelLine={false}>
                  {flowPieData.map((_, i) => <Cell key={i} fill={flowPieColors[i]} />)}
                </Pie>
                <Tooltip contentStyle={CustomTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          ) : <EmptyChart />}
        </ChartCard>

        <ChartCard title="By Status">
          {statusPieData.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <PieChart>
                <Pie data={statusPieData} dataKey="value" nameKey="name" cx="50%" cy="50%"
                  outerRadius={75} innerRadius={40} paddingAngle={2}
                  label={PIE_LABEL} labelLine={false}>
                  {statusPieData.map((_, i) => <Cell key={i} fill={statusPieColors[i]} />)}
                </Pie>
                <Tooltip contentStyle={CustomTooltipStyle} />
              </PieChart>
            </ResponsiveContainer>
          ) : <EmptyChart />}
        </ChartCard>

        <ChartCard title="Results by Flow">
          {barData.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={barData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e6ed" />
                <XAxis dataKey="name" tick={{ fill: '#8b92a5', fontSize: 11 }} tickLine={false} axisLine={false} />
                <YAxis tick={{ fill: '#8b92a5', fontSize: 11 }} tickLine={false} axisLine={false} width={30} />
                <Tooltip contentStyle={CustomTooltipStyle} />
                <Bar dataKey="Completed" fill="#009464" radius={[4, 4, 0, 0]} />
                <Bar dataKey="Failed" fill="#d63031" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : <EmptyChart />}
        </ChartCard>
      </div>

      {/* Per-flow breakdown table */}
      {flowStats.length > 0 && (
        <div className="table-card" style={{ marginBottom: 24 }}>
          <div className="table-toolbar">
            <div className="card-title">Breakdown by Flow</div>
          </div>
          <div className="table-wrap">
            <table>
              <thead>
                <tr><th>Flow Type</th><th>Total</th><th>Completed</th><th>Failed</th><th>Success Rate</th></tr>
              </thead>
              <tbody>
                {flowStats.map(f => (
                  <tr key={f.flowType}>
                    <td><FlowBadge flow={f.flowType} /></td>
                    <td className="mono">{f.total}</td>
                    <td className="mono" style={{ color: 'var(--green)' }}>{f.completed}</td>
                    <td className="mono" style={{ color: 'var(--red)' }}>{f.failed}</td>
                    <td className="mono">{f.total > 0 ? ((f.completed / f.total) * 100).toFixed(1) + '%' : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Recent runs */}
      <div className="table-card">
        <div className="table-toolbar">
          <div className="card-title">Recent Runs ({total})</div>
          <SearchInput value={runSearch} onChange={v => { setRunSearch(v); setPage(1); }} placeholder="Search runs…" />
        </div>
        <div className="table-wrap">
          {loading ? (
            <div className="spinner-wrap"><div className="spinner" /></div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>ID</th><th>Scenario</th><th>Flow</th><th>Status</th>
                  <th>Duration</th><th>Error</th><th>Started</th>
                </tr>
              </thead>
              <tbody>
                {recent.map(r => (
                  <tr key={r.id}>
                    <td className="mono">{r.id}</td>
                    <td style={{ fontWeight: 500, color: 'var(--text-primary)' }}>{r.scenarioName}</td>
                    <td><FlowBadge flow={r.flowType} /></td>
                    <td><StatusBadgeMini status={r.status} /></td>
                    <td className="mono">{r.durationMs}ms</td>
                    <td className="mono truncate" style={{ maxWidth: 200, color: 'var(--red)' }} title={r.errorMsg}>
                      {r.errorMsg || '—'}
                    </td>
                    <td>{formatDate(r.startedAt)}</td>
                  </tr>
                ))}
                {recent.length === 0 && (
                  <tr><td colSpan={7} className="empty-state">No runs yet</td></tr>
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

function StatCard({ icon, bg, label, value }) {
  return (
    <div className="stat-card">
      <div className="stat-icon" style={{ background: bg }}>
        <span style={{ fontSize: 18 }}>{icon}</span>
      </div>
      <div className="stat-info">
        <div className="stat-label">{label}</div>
        <div className="stat-value">{value ?? '—'}</div>
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

function EmptyChart() {
  return <div style={{ height: 220, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-muted)', fontSize: 12 }}>No data</div>;
}

function StatusBadgeMini({ status }) {
  const map = { completed: 'green', failed: 'red', pending: 'muted', running: 'yellow' };
  const c = map[status] || 'muted';
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
      <span className={`badge-dot badge-dot-${c}`} />
      {status}
    </span>
  );
}

function formatDate(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  return d.toLocaleString('en-GB', {
    day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  });
}