import React, { useState } from 'react';
import { BrowserRouter, Routes, Route, NavLink, Navigate, useLocation } from 'react-router-dom';
import { AuthProvider, useAuth } from './AuthContext';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Transactions from './pages/Transactions';
import AuditLogs from './pages/AuditLogs';
import Scenarios from './pages/Scenarios';
import ScenarioRunner from './pages/ScenarioRunner';
import RunStats from './pages/RunStats';
import TransactionDrilldown from './pages/TransactionDrilldown';
import Admin from './pages/Admin';
import CardRanges from './pages/CardRanges';
import MFASettings from './pages/MFASettings';
import IdentitySettings from './pages/IdentitySettings';
import DSEmulator from './pages/DSEmulator';
import KeyManagement from './pages/KeyManagement';
import RotationPolicies from './pages/RotationPolicies';
import SecurityProviders from './pages/SecurityProviders';
import HSMDashboard from './pages/HSMDashboard';
import HSMEndpoints from './pages/HSMEndpoints';
import HSMSessions from './pages/HSMSessions';
import HSMConsole from './pages/HSMConsole';
import HSMAudit from './pages/HSMAudit';

const icons = {
  dashboard: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/>
      <rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>
    </svg>
  ),
  transactions: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/>
    </svg>
  ),
  audit: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
      <polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/>
      <line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/>
    </svg>
  ),
  scenarios: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2"/>
      <rect x="9" y="3" width="6" height="4" rx="1"/>
      <path d="M9 14l2 2 4-4"/>
    </svg>
  ),
  runner: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polygon points="5 3 19 12 5 21 5 3"/>
    </svg>
  ),
  stats: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/>
      <line x1="6" y1="20" x2="6" y2="14"/>
    </svg>
  ),
  admin: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
    </svg>
  ),
  cardRanges: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1" y="4" width="22" height="16" rx="2" ry="2"/><line x1="1" y1="10" x2="23" y2="10"/>
    </svg>
  ),
  mfa: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>
    </svg>
  ),
  identity: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
    </svg>
  ),
  dsOps: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="2"/><path d="M16.24 7.76a6 6 0 0 1 0 8.49M7.76 16.24a6 6 0 0 1 0-8.49"/>
      <path d="M19.07 4.93a10 10 0 0 1 0 14.14M4.93 19.07a10 10 0 0 1 0-14.14"/>
    </svg>
  ),
  keys: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.78 7.78 5.5 5.5 0 0 1 7.78-7.78zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/>
    </svg>
  ),
  rotation: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
      <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
    </svg>
  ),
  providers: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
      <line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
    </svg>
  ),
  hsm: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="7" width="20" height="14" rx="2"/><path d="M12 11v4"/><circle cx="12" cy="17" r="1"/>
      <path d="M6 7V5a6 6 0 0 1 12 0v2"/>
    </svg>
  ),
  hsmEndpoint: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="4" y="4" width="16" height="16" rx="2"/><line x1="9" y1="9" x2="9.01" y2="9"/>
      <line x1="15" y1="9" x2="15.01" y2="9"/><path d="M9 15h6"/>
    </svg>
  ),
  hsmConsole: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
    </svg>
  ),
  hsmAudit: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
      <polyline points="14 2 14 8 20 8"/><line x1="12" y1="18" x2="12" y2="12"/><line x1="9" y1="15" x2="15" y2="15"/>
    </svg>
  ),
  pin: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 17v5"/><path d="M8 3h8"/><path d="M9 3v5l-3 4v1h12v-1l-3-4V3"/>
    </svg>
  ),
  panelClose: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="15 18 9 12 15 6"/>
    </svg>
  ),
  logout: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/>
    </svg>
  ),
};

const navigationGroups = [
  {
    id: 'analytics',
    label: 'Analytics',
    icon: icons.dashboard,
    items: [{ to: '/', label: 'Dashboard', end: true }],
  },
  {
    id: 'data',
    label: 'Data',
    icon: icons.transactions,
    items: [
      { to: '/transactions', label: 'Transactions' },
      { to: '/audit-logs', label: 'Audit Logs' },
    ],
  },
  {
    id: 'testing',
    label: 'Testing',
    icon: icons.scenarios,
    items: [
      { to: '/scenarios', label: 'Scenarios' },
      { to: '/runner', label: 'Run Scenario' },
      { to: '/run-stats', label: 'Run Statistics' },
      { to: '/ds-emulator', label: 'DS Emulator' },
    ],
  },
  {
    id: 'keys',
    label: 'Key Mgmt',
    icon: icons.keys,
    items: [
      { to: '/keys', label: 'Keys' },
      { to: '/rotation-policies', label: 'Rotation Policies' },
      { to: '/security-providers', label: 'Providers' },
    ],
  },
  {
    id: 'hsm',
    label: 'HSM',
    icon: icons.hsm,
    items: [
      { to: '/hsm', label: 'Dashboard', end: true },
      { to: '/hsm/endpoints', label: 'Endpoints' },
      { to: '/hsm/sessions', label: 'Sessions' },
      { to: '/hsm/console', label: 'Console' },
      { to: '/hsm/audit', label: 'Audit Log' },
    ],
  },
  {
    id: 'admin',
    label: 'Admin',
    icon: icons.admin,
    items: [
      { to: '/admin', label: 'Settings', end: true },
      { to: '/admin/card-ranges', label: 'Card BIN Ranges' },
      { to: '/admin/mfa', label: 'MFA Settings' },
      { to: '/admin/identity', label: 'Identity' },
    ],
  },
];

function matchesRoute(pathname, item) {
  if (item.end) return pathname === item.to;
  return pathname === item.to || pathname.startsWith(`${item.to}/`);
}

function findActiveGroup(pathname) {
  return navigationGroups.find((group) => group.items.some((item) => matchesRoute(pathname, item))) ?? navigationGroups[0];
}

function ProtectedRoute({ children }) {
  const { isAuthenticated, loading } = useAuth();
  if (loading) return <div className="spinner-wrap"><div className="spinner" /></div>;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return children;
}

function AppLayout() {
  const { user, logout } = useAuth();
  const location = useLocation();
  const activeGroup = findActiveGroup(location.pathname);
  const [hoveredGroupId, setHoveredGroupId] = useState(null);
  const [focusedGroupId, setFocusedGroupId] = useState(null);
  const [pinnedGroupId, setPinnedGroupId] = useState(null);

  const openGroupId = hoveredGroupId || focusedGroupId || pinnedGroupId || activeGroup.id;
  const isPanelVisible = Boolean(hoveredGroupId || focusedGroupId || pinnedGroupId);
  const openGroup = navigationGroups.find((group) => group.id === openGroupId) ?? activeGroup;

  const handleShellBlur = (event) => {
    if (!event.currentTarget.contains(event.relatedTarget)) {
      setHoveredGroupId(null);
      setFocusedGroupId(null);
    }
  };

  return (
    <div className={`layout${isPanelVisible ? ' panel-open' : ''}${pinnedGroupId ? ' panel-pinned' : ''}`}>
      <aside
        className={`sidebar-shell${isPanelVisible ? ' panel-visible' : ''}${pinnedGroupId ? ' panel-pinned' : ''}`}
        onMouseLeave={() => setHoveredGroupId(null)}
        onBlur={handleShellBlur}
        onKeyDown={(event) => {
          if (event.key === 'Escape' && !pinnedGroupId) {
            setHoveredGroupId(null);
            setFocusedGroupId(null);
          }
        }}
      >
        <div className="sidebar-rail">
          <div className="sidebar-brand">
            <div className="sidebar-brand-icon">A</div>
            <span className="sidebar-brand-text">ACS</span>
          </div>

          <nav className="sidebar-primary-nav" aria-label="Primary navigation">
            {navigationGroups.map((group) => {
              const isActive = group.id === activeGroup.id;
              const isOpen = group.id === openGroup.id && isPanelVisible;
              return (
                <button
                  key={group.id}
                  type="button"
                  className={`primary-nav-button${isActive ? ' active' : ''}${isOpen ? ' open' : ''}`}
                  onMouseEnter={() => setHoveredGroupId(group.id)}
                  onFocus={() => setFocusedGroupId(group.id)}
                  onClick={() => setPinnedGroupId((current) => (current === group.id ? null : group.id))}
                  aria-controls="secondary-panel"
                  aria-expanded={isOpen || pinnedGroupId === group.id}
                  aria-current={isActive ? 'page' : undefined}
                  title={group.label}
                >
                  <span className="primary-nav-icon">{group.icon}</span>
                  <span className="primary-nav-label">{group.label}</span>
                </button>
              );
            })}
          </nav>

          <div className="sidebar-footer">
            {user && (
              <div className="sidebar-user">
                <span className="sidebar-user-name" title={user.displayName || user.username}>
                  {user.displayName || user.username}
                </span>
                <button className="sidebar-logout-btn" onClick={logout} title="Sign out">
                  {icons.logout}
                </button>
              </div>
            )}
          </div>
        </div>

        {isPanelVisible && (
          <div
            id="secondary-panel"
            className="sidebar-panel visible"
            onMouseEnter={() => setHoveredGroupId(openGroup.id)}
          >
            <div className="sidebar-panel-header">
              <div>
                <div className="sidebar-panel-eyebrow">Navigation</div>
                <h2>{openGroup.label}</h2>
              </div>
              <div className="sidebar-panel-actions">
                <button
                  type="button"
                  className={`sidebar-panel-toggle${pinnedGroupId === openGroup.id ? ' active' : ''}`}
                  onClick={() => setPinnedGroupId((current) => (current === openGroup.id ? null : openGroup.id))}
                  title={pinnedGroupId === openGroup.id ? 'Unpin panel' : 'Pin panel'}
                >
                  {icons.pin}
                </button>
                <button
                  type="button"
                  className="sidebar-panel-toggle"
                  onClick={() => {
                    setPinnedGroupId(null);
                    setHoveredGroupId(null);
                    setFocusedGroupId(null);
                  }}
                  title="Close panel"
                >
                  {icons.panelClose}
                </button>
              </div>
            </div>

            <nav className="sidebar-secondary-nav" aria-label={`${openGroup.label} pages`}>
              {openGroup.items.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  className={({ isActive }) => `secondary-nav-link${isActive ? ' active' : ''}`}
                  onClick={() => {
                    if (!pinnedGroupId) {
                      setHoveredGroupId(null);
                      setFocusedGroupId(null);
                    }
                  }}
                >
                  {item.label}
                </NavLink>
              ))}
            </nav>
          </div>
        )}
      </aside>

      <div className="main-content">
        <div className="page-content">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/transactions" element={<Transactions />} />
            <Route path="/transactions/:acsTransID" element={<TransactionDrilldown />} />
            <Route path="/audit-logs" element={<AuditLogs />} />
            <Route path="/scenarios" element={<Scenarios />} />
            <Route path="/runner" element={<ScenarioRunner />} />
            <Route path="/run-stats" element={<RunStats />} />
            <Route path="/keys" element={<KeyManagement />} />
            <Route path="/rotation-policies" element={<RotationPolicies />} />
            <Route path="/security-providers" element={<SecurityProviders />} />
            <Route path="/hsm" element={<HSMDashboard />} />
            <Route path="/hsm/endpoints" element={<HSMEndpoints />} />
            <Route path="/hsm/sessions" element={<HSMSessions />} />
            <Route path="/hsm/console" element={<HSMConsole />} />
            <Route path="/hsm/audit" element={<HSMAudit />} />
            <Route path="/admin" element={<Admin />} />
            <Route path="/admin/card-ranges" element={<CardRanges />} />
            <Route path="/admin/mfa" element={<MFASettings />} />
            <Route path="/ds-emulator" element={<DSEmulator />} />
            <Route path="/admin/identity" element={<IdentitySettings />} />
          </Routes>
        </div>
      </div>
    </div>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/*"
            element={(
              <ProtectedRoute>
                <AppLayout />
              </ProtectedRoute>
            )}
          />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
