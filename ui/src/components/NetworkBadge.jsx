import React from 'react';

const NETWORK_ICONS = {
  visa: (
    <svg viewBox="0 0 48 16" width="32" height="11" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M19.5 1.2L16.2 14.8H13L16.3 1.2H19.5Z" fill="#1A1F71"/>
      <path d="M31.2 1.5C30.5 1.2 29.4 1 28 1C24.7 1 22.3 2.8 22.3 5.3C22.3 7.2 24 8.2 25.3 8.8C26.6 9.4 27.1 9.8 27.1 10.4C27.1 11.3 26 11.7 25 11.7C23.6 11.7 22.8 11.5 21.7 11L21.2 10.8L20.7 14C21.5 14.4 23 14.7 24.5 14.7C28 14.7 30.4 13 30.4 10.3C30.4 8.8 29.5 7.7 27.5 6.8C26.3 6.2 25.6 5.8 25.6 5.2C25.6 4.6 26.3 4 27.7 4C28.9 4 29.8 4.2 30.4 4.5L30.8 4.7L31.2 1.5Z" fill="#1A1F71"/>
      <path d="M35.6 1.2C34.8 1.2 34.2 1.4 33.8 2.3L28.8 14.8H32.3L33 12.8H37.2L37.6 14.8H40.8L38 1.2H35.6ZM33.9 10.2C34.2 9.3 35.5 5.8 35.5 5.8L36.6 10.2H33.9Z" fill="#1A1F71"/>
      <path d="M12 1.2L8.7 10.6L8.4 9L7.2 2.4C7 1.6 6.4 1.2 5.7 1.2H0.1L0 1.5C1.2 1.8 2.6 2.3 3.4 2.8L6.4 14.8H10L15.5 1.2H12Z" fill="#1A1F71"/>
    </svg>
  ),
  mastercard: (
    <svg viewBox="0 0 38 24" width="24" height="15" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="14" cy="12" r="10" fill="#EB001B"/>
      <circle cx="24" cy="12" r="10" fill="#F79E1B"/>
      <path d="M19 4.6A10 10 0 0 0 15.5 12 10 10 0 0 0 19 19.4 10 10 0 0 0 22.5 12 10 10 0 0 0 19 4.6Z" fill="#FF5F00"/>
    </svg>
  ),
  amex: (
    <svg viewBox="0 0 40 16" width="28" height="11" xmlns="http://www.w3.org/2000/svg">
      <rect width="40" height="16" rx="2" fill="#006FCF"/>
      <text x="20" y="11.5" textAnchor="middle" fill="#fff" fontSize="9" fontWeight="700" fontFamily="Arial,sans-serif">AMEX</text>
    </svg>
  ),
  discover: (
    <svg viewBox="0 0 48 16" width="32" height="11" xmlns="http://www.w3.org/2000/svg">
      <rect width="48" height="16" rx="2" fill="#fff" stroke="#FF6600" strokeWidth="1.5"/>
      <circle cx="28" cy="8" r="6" fill="#FF6600"/>
      <text x="14" y="11" textAnchor="middle" fill="#231F20" fontSize="7" fontWeight="700" fontFamily="Arial,sans-serif">D</text>
    </svg>
  ),
};

const NETWORK_BADGE_COLORS = {
  visa: { bg: '#f0f1f8', border: '#c8cbe0', color: '#1A1F71' },
  mastercard: { bg: '#fef3f0', border: '#f0c8bd', color: '#252525' },
  amex: { bg: '#eef5fd', border: '#b8d4f0', color: '#006FCF' },
  discover: { bg: '#fff5ee', border: '#f0d4b8', color: '#FF6600' },
};

export function NetworkBadge({ network }) {
  const icon = NETWORK_ICONS[network];
  const colors = NETWORK_BADGE_COLORS[network] || { bg: 'var(--surface-secondary)', border: 'var(--border)', color: 'var(--text-secondary)' };
  const label = network ? network.charAt(0).toUpperCase() + network.slice(1) : '—';

  return (
    <span className="network-badge" style={{
      display: 'inline-flex', alignItems: 'center', gap: 5,
      padding: '3px 8px 3px 6px', borderRadius: 6,
      background: colors.bg, border: `1px solid ${colors.border}`,
      fontSize: 12, fontWeight: 500, color: colors.color,
      whiteSpace: 'nowrap',
    }}>
      {icon || null}
      {label}
    </span>
  );
}

export function NetworkIcon({ network, size = 20 }) {
  const icon = NETWORK_ICONS[network];
  if (!icon) return null;
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', transform: size !== 20 ? `scale(${size / 20})` : undefined }}>
      {icon}
    </span>
  );
}

export function networkOptionLabel(network) {
  const labels = { visa: 'Visa', mastercard: 'Mastercard', amex: 'Amex', discover: 'Discover' };
  return labels[network] || network;
}