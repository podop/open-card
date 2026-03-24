const BASE = '/api';

export async function fetchJSON(path, params = {}, method = 'GET', body = null) {
  const url = new URL(BASE + path, window.location.origin);
  for (const [k, v] of Object.entries(params)) {
    if (v !== '' && v != null) url.searchParams.set(k, v);
  }
  const opts = { method, headers: {} };

  // Attach auth token if available
  const token = localStorage.getItem('accessToken');
  if (token) {
    opts.headers['Authorization'] = `Bearer ${token}`;
  }

  if (body !== null) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(url.toString(), opts);

  // On 401: try refresh, then retry once
  if (res.status === 401) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      opts.headers['Authorization'] = `Bearer ${localStorage.getItem('accessToken')}`;
      const retry = await fetch(url.toString(), opts);
      if (!retry.ok) throw new Error(`HTTP ${retry.status}`);
      return retry.json();
    }
    // Refresh failed — redirect to login
    localStorage.removeItem('accessToken');
    localStorage.removeItem('refreshToken');
    window.location.href = '/login';
    throw new Error('Session expired');
  }

  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

async function tryRefresh() {
  const rt = localStorage.getItem('refreshToken');
  if (!rt) return false;
  try {
    const res = await fetch('/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refreshToken: rt }),
    });
    if (!res.ok) return false;
    const data = await res.json();
    localStorage.setItem('accessToken', data.accessToken);
    localStorage.setItem('refreshToken', data.refreshToken);
    return true;
  } catch {
    return false;
  }
}
