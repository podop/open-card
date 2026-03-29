import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';

const AuthContext = createContext(null);

export function useAuth() {
  return useContext(AuthContext);
}

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [token, setToken] = useState(() => localStorage.getItem('accessToken'));
  const [loading, setLoading] = useState(true);

  const logout = useCallback(() => {
    const rt = localStorage.getItem('refreshToken');
    if (rt) {
      fetch('/auth/logout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refreshToken: rt }),
      }).catch(() => {});
    }
    localStorage.removeItem('accessToken');
    localStorage.removeItem('refreshToken');
    setToken(null);
    setUser(null);
  }, []);

  const refresh = useCallback(async () => {
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
      setToken(data.accessToken);
      setUser(data.user);
      return true;
    } catch {
      return false;
    }
  }, []);

  const login = useCallback(async (username, password) => {
    const res = await fetch('/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || 'Login failed');
    }
    const data = await res.json();
    localStorage.setItem('accessToken', data.accessToken);
    localStorage.setItem('refreshToken', data.refreshToken);
    setToken(data.accessToken);
    setUser(data.user);
  }, []);

  // On mount: validate existing token
  useEffect(() => {
    if (!token) {
      setLoading(false);
      return;
    }
    fetch('/auth/me', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(res => {
        if (!res.ok) throw new Error('invalid');
        return res.json();
      })
      .then(data => {
        setUser(data);
        setLoading(false);
      })
      .catch(() => {
        // Try refresh
        refresh().then(ok => {
          if (!ok) {
            localStorage.removeItem('accessToken');
            localStorage.removeItem('refreshToken');
            setToken(null);
          }
          setLoading(false);
        });
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Auto-refresh timer: refresh token 2 min before expiry
  useEffect(() => {
    if (!token) return;
    try {
      const payload = JSON.parse(atob(token.split('.')[1]));
      const expiresIn = (payload.exp * 1000) - Date.now() - 120000; // 2 min before
      if (expiresIn <= 0) {
        refresh();
        return;
      }
      const timer = setTimeout(() => refresh(), expiresIn);
      return () => clearTimeout(timer);
    } catch {
      // invalid token format
    }
  }, [token, refresh]);

  const canDo = useCallback((action) => {
    if (!user?.actions) return false;
    if (Array.isArray(action)) return action.some(a => user.actions.includes(a));
    return user.actions.includes(action);
  }, [user]);

  const value = {
    user,
    token,
    loading,
    login,
    logout,
    refresh,
    canDo,
    isAuthenticated: !!token && !!user,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
