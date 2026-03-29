import { useState, useEffect } from 'react';
import { fetchJSON } from '../api';

let cachedNetworks = null;
let fetchPromise = null;

function doFetch() {
  if (!fetchPromise) {
    fetchPromise = fetchJSON('/networks')
      .then(data => {
        cachedNetworks = data.data || [];
        return cachedNetworks;
      })
      .catch(() => {
        cachedNetworks = [];
        return cachedNetworks;
      })
      .finally(() => { fetchPromise = null; });
  }
  return fetchPromise;
}

/** Fetch all networks from API, cached across components. Call invalidate() after mutations. */
export function useNetworks() {
  const [networks, setNetworks] = useState(cachedNetworks || []);
  const [loading, setLoading] = useState(!cachedNetworks);

  useEffect(() => {
    if (cachedNetworks) {
      setNetworks(cachedNetworks);
      setLoading(false);
      return;
    }
    doFetch().then(data => { setNetworks(data); setLoading(false); });
  }, []);

  const invalidate = () => {
    cachedNetworks = null;
    fetchPromise = null;
    setLoading(true);
    doFetch().then(data => { setNetworks(data); setLoading(false); });
  };

  return { networks, loading, invalidate };
}

/** Returns just active network names as string[]. */
export function useActiveNetworkNames() {
  const { networks, loading } = useNetworks();
  return { names: networks.filter(n => n.active).map(n => n.name), loading };
}

/** Build a color map from networks: { visa: '#1A1F71', ... } */
export function networkColorMap(networks) {
  const map = {};
  for (const n of networks) {
    map[n.name] = n.color || '#555d70';
  }
  return map;
}