const BASE = '/secrets';

export async function secretsFetch(path, method = 'GET', body = null) {
  const url = BASE + path;
  const opts = { method, headers: {} };

  const token = localStorage.getItem('accessToken');
  if (token) {
    opts.headers['Authorization'] = `Bearer ${token}`;
  }

  if (body !== null) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }

  const res = await fetch(url, opts);

  if (res.status === 401) {
    const rt = localStorage.getItem('refreshToken');
    if (rt) {
      const refreshRes = await fetch('/auth/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refreshToken: rt }),
      });
      if (refreshRes.ok) {
        const data = await refreshRes.json();
        localStorage.setItem('accessToken', data.accessToken);
        localStorage.setItem('refreshToken', data.refreshToken);
        opts.headers['Authorization'] = `Bearer ${data.accessToken}`;
        const retry = await fetch(url, opts);
        if (!retry.ok) throw new Error(`HTTP ${retry.status}`);
        return retry.json();
      }
    }
    localStorage.removeItem('accessToken');
    localStorage.removeItem('refreshToken');
    window.location.href = '/login';
    throw new Error('Session expired');
  }

  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}
