import React, { useEffect, useRef, useState } from 'react';
import Highcharts from 'highcharts/highmaps';
import worldTopology from '@highcharts/map-collection/custom/world.topo.json';
import { fetchJSON } from '../api';
import { NetworkBadge } from '../components/NetworkBadge';
import { useNetworks, networkColorMap } from '../hooks/useNetworks';

const RANGE_OPTIONS = [
  { id: 'hourly', label: '24H' },
  { id: 'monthly', label: '30D' },
  { id: 'yearly', label: '12M' },
];

const AUTH_STATUSES = new Set(['Y', 'A']);
const FAIL_STATUSES = new Set(['N', 'R', 'U']);
const CHART_FAIL_STATUSES = new Set(['N', 'R']);
const UNAVAILABLE_STATUSES = new Set(['U']);
const FLOW_LABELS = {
  frictionless: 'Frictionless',
  otp: 'OTP',
  oob: 'OOB',
};
const FLOW_COLORS = {
  frictionless: '#0f8f67',
  otp: '#cf8a18',
  oob: '#2f6fed',
};
const STATUS_COLORS = {
  total: '#8b92a5',
  authenticated: '#0f8f67',
  failed: '#d63031',
};
const COUNTRY_POINTS = {
  '032': { lat: -38.4161, lon: -63.6167, name: 'Argentina' },
  '036': { lat: -25.2744, lon: 133.7751, name: 'Australia' },
  '076': { lat: -14.235, lon: -51.9253, name: 'Brazil' },
  '124': { lat: 56.1304, lon: -106.3468, name: 'Canada' },
  '156': { lat: 35.8617, lon: 104.1954, name: 'China' },
  '250': { lat: 46.2276, lon: 2.2137, name: 'France' },
  '276': { lat: 51.1657, lon: 10.4515, name: 'Germany' },
  '356': { lat: 20.5937, lon: 78.9629, name: 'India' },
  '380': { lat: 41.8719, lon: 12.5674, name: 'Italy' },
  '392': { lat: 36.2048, lon: 138.2529, name: 'Japan' },
  '410': { lat: 35.9078, lon: 127.7669, name: 'South Korea' },
  '484': { lat: 23.6345, lon: -102.5528, name: 'Mexico' },
  '566': { lat: 9.082, lon: 8.6753, name: 'Nigeria' },
  '578': { lat: 60.472, lon: 8.4689, name: 'Norway' },
  '643': { lat: 61.524, lon: 105.3188, name: 'Russia' },
  '682': { lat: 23.8859, lon: 45.0792, name: 'Saudi Arabia' },
  '702': { lat: 1.3521, lon: 103.8198, name: 'Singapore' },
  '710': { lat: -30.5595, lon: 22.9375, name: 'South Africa' },
  '724': { lat: 40.4637, lon: -3.7492, name: 'Spain' },
  '756': { lat: 46.8182, lon: 8.2275, name: 'Switzerland' },
  '784': { lat: 23.4241, lon: 53.8478, name: 'UAE' },
  '792': { lat: 38.9637, lon: 35.2433, name: 'Turkey' },
  '804': { lat: 48.3794, lon: 31.1656, name: 'Ukraine' },
  '818': { lat: 26.8206, lon: 30.8025, name: 'Egypt' },
  '826': { lat: 55.3781, lon: -3.436, name: 'United Kingdom' },
  '840': { lat: 37.0902, lon: -95.7129, name: 'United States' },
};

function ChartCard({ title, actions, children, className = '' }) {
  return (
    <div className={`chart-card dashboard-card ${className}`.trim()}>
      <div className="dashboard-card-header">
        <div>
          <div className="card-title">{title}</div>
        </div>
        {actions}
      </div>
      {children}
    </div>
  );
}

function RangeToggle({ value, onChange }) {
  return (
    <div className="dashboard-toggle">
      {RANGE_OPTIONS.map((option) => (
        <button
          key={option.id}
          type="button"
          className={value === option.id ? 'active' : ''}
          onClick={() => onChange(option.id)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

export default function Dashboard() {
  const { networks: netEntries } = useNetworks();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [selectedRange, setSelectedRange] = useState('hourly');

  useEffect(() => {
    fetchJSON('/dashboard')
      .then(setData)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;
  if (!data) return <div className="empty-state">Failed to load dashboard</div>;

  const analytics = (data.analytics || []).map((row) => ({
    ...row,
    createdAt: new Date(row.createdAt),
  })).filter((row) => !Number.isNaN(row.createdAt.getTime()));
  const rangeData = buildRangeSeries(analytics, selectedRange);
  const filteredTransactions = filterAnalyticsByRange(analytics, selectedRange);
  const colorMap = networkColorMap(netEntries);
  const treeSystems = buildTreeSystems(filteredTransactions, colorMap);
  const mapData = buildMapData(filteredTransactions, colorMap);
  const recent = (data.recent || []).slice(0, 10);
  const treeColumns = Math.min(Math.max(treeSystems.length, 1), 3);

  return (
    <>
      <div className="page-header">
        <h1>Dashboard</h1>
        <p>Professional 3D Secure transaction monitoring across volume, flow, geography, and latest activity.</p>
      </div>

      <div className="dashboard-single-grid">
        <ChartCard
          title="Transaction Volume"
          actions={<RangeToggle value={selectedRange} onChange={setSelectedRange} />}
          className="dashboard-top-card"
        >
          <TransactionVolumeChart data={rangeData} />
        </ChartCard>
      </div>

      <div className="dashboard-tree-grid" style={{ '--dashboard-tree-columns': treeColumns }}>
        {treeSystems.map((system) => (
          <ChartCard key={system.id} title={`${system.name} Flow`} className="dashboard-top-card">
            <PaymentSystemTree data={system} />
          </ChartCard>
        ))}
      </div>

      <div className="dashboard-split-grid dashboard-bottom-grid">
        <ChartCard title="Transaction Distribution" className="dashboard-bottom-card">
          <WorldMap data={mapData} />
        </ChartCard>
        <ChartCard title="Recent Transactions" className="dashboard-bottom-card">
          <RecentTransactionsTable rows={recent} />
        </ChartCard>
      </div>
    </>
  );
}

function TransactionVolumeChart({ data }) {
  const containerRef = useRef(null);

  useEffect(() => {
    if (!containerRef.current || !data.length) return undefined;

    const chart = Highcharts.chart(containerRef.current, {
      chart: {
        type: 'column',
        backgroundColor: 'transparent',
        height: 250,
        spacing: [10, 8, 0, 8],
      },
      title: { text: null },
      credits: { enabled: false },
      legend: {
        align: 'right',
        verticalAlign: 'bottom',
        y: 6,
        itemStyle: {
          color: '#5a6478',
          fontWeight: '500',
        },
      },
      xAxis: {
        categories: data.map((item) => item.label),
        lineColor: '#edf1f5',
        tickColor: '#edf1f5',
        labels: {
          style: { color: '#8b92a5', fontSize: '11px' },
        },
      },
      yAxis: {
        min: 0,
        title: { text: null },
        gridLineColor: '#e8edf3',
        labels: {
          style: { color: '#8b92a5', fontSize: '11px' },
        },
        stackLabels: { enabled: false },
      },
      tooltip: {
        shared: true,
        backgroundColor: '#ffffff',
        borderColor: '#dfe4ea',
        shadow: false,
      },
      plotOptions: {
        column: {
          stacking: 'normal',
          borderRadius: 3,
          pointPadding: 0.08,
          groupPadding: 0.12,
          borderWidth: 0,
        },
        series: {
          animation: false,
        },
      },
      series: [
        {
          name: 'Authenticated',
          color: STATUS_COLORS.authenticated,
          data: data.map((item) => item.authenticated),
        },
        {
          name: 'Failed',
          color: STATUS_COLORS.failed,
          data: data.map((item) => item.failed),
        },
        {
          name: 'Unavailable',
          color: '#8b92a5',
          data: data.map((item) => item.unavailable),
        },
      ],
      accessibility: { enabled: false },
    });

    return () => chart.destroy();
  }, [data]);

  if (!data.length) return <div className="empty-state dashboard-chart-empty">No activity in the selected range</div>;
  return <div ref={containerRef} className="dashboard-chart-frame" />;
}

function PaymentSystemTree({ data }) {
  if (!data?.nodes?.length) return <div className="empty-state dashboard-chart-empty">No flow data in the selected range</div>;
  const width = 760;
  const height = 250;
  const layout = layoutTree(data.nodes, width, height);

  return (
    <div className="dashboard-chart-frame dashboard-tree-frame">
      <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" className="dashboard-svg">
        {layout.links.map((link) => (
          <path
            key={`${link.from.id}-${link.to.id}`}
            d={`M ${link.from.x + 8} ${link.from.y} C ${link.from.x + 34} ${link.from.y}, ${link.to.x - 34} ${link.to.y}, ${link.to.x - 8} ${link.to.y}`}
            className="dashboard-tree-link"
          />
        ))}
        {layout.nodes.map((node) => (
          <g key={node.id}>
            <circle cx={node.x} cy={node.y} r={node.level === 0 ? 8 : 6} fill={node.color} />
            <circle cx={node.x} cy={node.y} r={node.level === 0 ? 14 : 10} fill="none" stroke={node.color} strokeOpacity="0.16" />
            <text x={node.x + 14} y={node.y - 4} className="dashboard-tree-node-name">{node.name}</text>
            <text x={node.x + 14} y={node.y + 11} className="dashboard-tree-node-meta">{node.meta}</text>
          </g>
        ))}
      </svg>
    </div>
  );
}

function WorldMap({ data }) {
  const containerRef = useRef(null);

  useEffect(() => {
    let chart;
    const container = containerRef.current;

    if (!container) return undefined;

    chart = Highcharts.mapChart(container, {
      chart: {
        backgroundColor: 'transparent',
        height: 250,
        spacing: [0, 0, 0, 0],
      },
      title: { text: null },
      credits: { enabled: false },
      legend: { enabled: false },
      mapNavigation: { enabled: false },
      tooltip: {
        backgroundColor: '#ffffff',
        borderColor: '#dfe4ea',
        shadow: false,
        pointFormat: '{point.name}: <b>{point.value}</b>',
      },
      colorAxis: {
        min: 0,
        maxColor: '#9aa3b2',
        minColor: '#e8ebf0',
        visible: false,
      },
      series: [
        {
          type: 'map',
          mapData: worldTopology,
          nullColor: '#eef1f4',
          borderColor: '#d3d8df',
          borderWidth: 0.7,
          states: {
            hover: {
              color: '#d9dee6',
            },
          },
          data: [],
          enableMouseTracking: false,
        },
        {
          type: 'mappoint',
          data: data.map((point) => ({
            name: point.name,
            lat: point.lat,
            lon: point.lon,
            value: point.count,
            color: '#6b7280',
          })),
          marker: {
            radius: 4,
            fillColor: '#6b7280',
            lineColor: '#ffffff',
            lineWidth: 1,
          },
          dataLabels: { enabled: false },
        },
      ],
    });

    return () => {
      if (chart) chart.destroy();
    };
  }, [data]);

  return <div ref={containerRef} className="dashboard-chart-frame dashboard-map-frame" />;
}

function RecentTransactionsTable({ rows }) {
  return (
    <div className="dashboard-recent-list">
      <div className="table-wrap dashboard-recent-table">
        <table>
          <thead>
            <tr>
              <th>AcsTransId</th>
              <th>Payment system</th>
              <th>Status</th>
              <th>Amount</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((tx) => (
              <tr key={tx.acsTransID}>
                <td className="mono truncate" title={tx.acsTransID}>{tx.acsTransID}</td>
                <td><NetworkBadge network={tx.network} /></td>
                <td><StatusBadge status={tx.transStatus} /></td>
                <td className="mono">{formatAmount(tx.purchaseAmount, tx.purchaseCurrency)}</td>
              </tr>
            ))}
            {!rows.length && (
              <tr><td colSpan={4} className="empty-state">No transactions yet</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function buildRangeSeries(rows, range) {
  const now = new Date();
  const buckets = [];
  const bucketMap = new Map();

  if (range === 'hourly') {
    for (let index = 23; index >= 0; index -= 1) {
      const date = new Date(now);
      date.setUTCMinutes(0, 0, 0);
      date.setUTCHours(date.getUTCHours() - index);
      const key = date.toISOString().slice(0, 13);
      const bucket = { key, label: date.toLocaleTimeString('en-GB', { hour: '2-digit', hour12: false }), total: 0, authenticated: 0, failed: 0, unavailable: 0 };
      buckets.push(bucket);
      bucketMap.set(key, bucket);
    }
  } else if (range === 'monthly') {
    for (let index = 29; index >= 0; index -= 1) {
      const date = new Date(now);
      date.setUTCHours(0, 0, 0, 0);
      date.setUTCDate(date.getUTCDate() - index);
      const key = date.toISOString().slice(0, 10);
      const bucket = { key, label: date.toLocaleDateString('en-GB', { day: '2-digit', month: 'short' }), total: 0, authenticated: 0, failed: 0, unavailable: 0 };
      buckets.push(bucket);
      bucketMap.set(key, bucket);
    }
  } else {
    for (let index = 11; index >= 0; index -= 1) {
      const date = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - index, 1));
      const key = `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
      const bucket = { key, label: date.toLocaleDateString('en-GB', { month: 'short' }), total: 0, authenticated: 0, failed: 0, unavailable: 0 };
      buckets.push(bucket);
      bucketMap.set(key, bucket);
    }
  }

  rows.forEach((row) => {
    const key = bucketKey(row.createdAt, range);
    const bucket = bucketMap.get(key);
    if (!bucket) return;
    bucket.total += 1;
    if (AUTH_STATUSES.has(row.transStatus)) bucket.authenticated += 1;
    if (CHART_FAIL_STATUSES.has(row.transStatus)) bucket.failed += 1;
    if (UNAVAILABLE_STATUSES.has(row.transStatus)) bucket.unavailable += 1;
  });

  return buckets;
}

function filterAnalyticsByRange(rows, range) {
  const now = new Date();
  const start = new Date(now);
  if (range === 'hourly') {
    start.setUTCHours(start.getUTCHours() - 24);
  } else if (range === 'monthly') {
    start.setUTCDate(start.getUTCDate() - 30);
  } else {
    start.setUTCMonth(start.getUTCMonth() - 12);
  }
  return rows.filter((row) => row.createdAt >= start);
}

function buildTreeSystems(rows, colorMap) {
  const systems = new Map();

  rows.forEach((row) => {
    const system = row.network || 'unknown';
    const flow = getFlowType(row);
    const outcome = AUTH_STATUSES.has(row.transStatus) ? 'authenticated' : 'failed';
    if (!systems.has(system)) {
      systems.set(system, {
        total: 0,
        authenticated: 0,
        flows: {
          frictionless: { total: 0, authenticated: 0, failed: 0, unavailable: 0 },
          otp: { total: 0, authenticated: 0, failed: 0, unavailable: 0 },
          oob: { total: 0, authenticated: 0, failed: 0, unavailable: 0 },
        },
      });
    }
    const bucket = systems.get(system);
    bucket.total += 1;
    bucket.flows[flow].total += 1;
    if (AUTH_STATUSES.has(row.transStatus)) {
      bucket.authenticated += 1;
      bucket.flows[flow].authenticated += 1;
    } else if (UNAVAILABLE_STATUSES.has(row.transStatus)) {
      bucket.flows[flow].unavailable += 1;
    } else {
      bucket.flows[flow].failed += 1;
    }
  });

  return Array.from(systems.entries())
    .sort((a, b) => b[1].total - a[1].total)
    .map(([system, stats]) => {
      const systemId = `system-${system}`;
      const color = colorMap[system] || '#5a6478';
      const nodes = [{
        id: systemId,
        name: capitalize(system),
        color,
        meta: `${stats.total} transactions`,
      }];

      ['frictionless', 'otp', 'oob'].forEach((flow) => {
        const flowStats = stats.flows[flow];
        if (!flowStats.total) return;
        const flowId = `${systemId}-${flow}`;
        nodes.push({
          id: flowId,
          parent: systemId,
          name: FLOW_LABELS[flow],
          color: FLOW_COLORS[flow],
          meta: `${flowStats.total} transactions`,
        });
        if (flowStats.authenticated) {
          nodes.push({
            id: `${flowId}-authenticated`,
            parent: flowId,
            name: 'Authenticated',
            color: STATUS_COLORS.authenticated,
            meta: `${flowStats.authenticated} successful`,
          });
        }
        if (flowStats.failed) {
          nodes.push({
            id: `${flowId}-failed`,
            parent: flowId,
            name: 'Failed',
            color: STATUS_COLORS.failed,
            meta: `${flowStats.failed} failed`,
          });
        }
        if (flowStats.unavailable) {
          nodes.push({
            id: `${flowId}-unavailable`,
            parent: flowId,
            name: 'Unavailable',
            color: '#8b92a5',
            meta: `${flowStats.unavailable} unavailable`,
          });
        }
      });

      return {
        id: systemId,
        name: capitalize(system),
        total: stats.total,
        authenticated: stats.authenticated,
        color,
        nodes,
      };
    });
}

function layoutTree(points, chartWidth, chartHeight) {
  const nodeMap = new Map(points.map((point) => [point.id, { ...point, children: [] }]));
  const roots = [];
  points.forEach((point) => {
    const node = nodeMap.get(point.id);
    if (point.parent) {
      const parent = nodeMap.get(point.parent);
      if (parent) parent.children.push(node);
    } else {
      roots.push(node);
    }
  });

  const orderedRoots = roots.slice(0, 3);
  const nodes = [];
  const links = [];
  const levelX = [
    chartWidth * 0.06,
    chartWidth * 0.34,
    chartWidth * 0.62,
    chartWidth * 0.86,
  ];
  const topPadding = chartHeight * 0.05;
  const usableHeight = chartHeight * 0.9;
  const rootGap = 1;

  const countLeaves = (node) => {
    if (!node.children.length) return 1;
    return node.children.reduce((sum, child) => sum + countLeaves(child), 0);
  };

  const totalLeaves = orderedRoots.reduce((sum, root) => sum + countLeaves(root), 0);
  const leafSpacing = totalLeaves > 1 ? usableHeight / (totalLeaves - 1) : 0;
  let currentLeafIndex = 0;

  const placeNode = (node, level) => {
    node.level = level;
    node.x = levelX[Math.min(level, levelX.length - 1)];

    if (!node.children.length) {
      node.y = topPadding + (currentLeafIndex * leafSpacing);
      currentLeafIndex += 1;
      nodes.push(node);
      return node.y;
    }

    const childYs = node.children.map((child) => {
      const y = placeNode(child, level + 1);
      links.push({ from: node, to: child });
      return y;
    });

    node.y = (childYs[0] + childYs[childYs.length - 1]) / 2;
    nodes.push(node);
    return node.y;
  };

  orderedRoots.forEach((root, rootIndex) => {
    if (rootIndex > 0) {
      const previousRoot = orderedRoots[rootIndex - 1];
      currentLeafIndex += Math.max(1, Math.round(countLeaves(previousRoot) * (rootGap / leafSpacing)));
    }
    placeNode(root, 0);
  });

  return { nodes, links };
}

function buildMapData(rows, colorMap) {
  const counts = new Map();
  rows.forEach((row) => {
    const normalized = normalizeCountryCode(row.merchantCountry);
    if (!COUNTRY_POINTS[normalized]) return;
    const key = normalized;
    if (!counts.has(key)) {
      counts.set(key, {
        country: key,
        count: 0,
        ...COUNTRY_POINTS[key],
        color: colorMap[row.network] || '#0f8f67',
      });
    }
    counts.get(key).count += 1;
  });
  return Array.from(counts.values()).sort((a, b) => b.count - a.count);
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

function bucketKey(date, range) {
  if (range === 'hourly') return date.toISOString().slice(0, 13);
  if (range === 'monthly') return date.toISOString().slice(0, 10);
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
}

function getFlowType(row) {
  if (row.transStatus === 'D') return 'oob';
  if ((row.challengeAttempts || 0) > 0) return 'otp';
  return 'frictionless';
}

function capitalize(value) {
  if (!value) return 'Unknown';
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function normalizeCountryCode(code) {
  if (!code) return '';
  return String(code).padStart(3, '0');
}

function formatAmount(amount, currency) {
  if (!amount) return '—';
  return `${amount} ${currency || ''}`.trim();
}
