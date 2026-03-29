import React, { useState, useEffect } from 'react';
import { secretsFetch } from '../secretsApi';

export default function KeySourceModal({ title, keyType, keyUsage, initialTab, onSelect, onClose }) {
  const [tab, setTab] = useState(initialTab || 'generate');
  const [providers, setProviders] = useState([]);
  const [keys, setKeys] = useState([]);
  const [selectedProvider, setSelectedProvider] = useState('');
  const [keyName, setKeyName] = useState('');
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState('');
  const [loadingKeys, setLoadingKeys] = useState(false);

  useEffect(() => {
    secretsFetch('/providers').then(data => {
      setProviders((data || []).filter(p => p.active));
    }).catch(() => {});
  }, []);

  useEffect(() => {
    if (tab === 'existing') {
      setLoadingKeys(true);
      secretsFetch('/keys').then(data => {
        const filtered = (data || []).filter(k => {
          if (keyType && k.keyType !== keyType) return false;
          if (keyUsage && k.keyUsage !== keyUsage) return false;
          return k.active;
        });
        setKeys(filtered);
      }).catch(() => setKeys([]))
        .finally(() => setLoadingKeys(false));
    }
  }, [tab, keyType, keyUsage]);

  const handleGenerate = async () => {
    if (!selectedProvider || !keyName) {
      setError('Provider and key name are required');
      return;
    }
    setGenerating(true);
    setError('');
    try {
      const result = await secretsFetch('/keys/generate', 'POST', {
        name: keyName,
        keyType: keyType,
        keyUsage: keyUsage || 'general',
        providerId: Number(selectedProvider),
      });
      // Export the key material
      const exported = await secretsFetch(`/keys/${result.id}/export`, 'POST');
      onSelect(exported.material, result.id);
    } catch (e) {
      setError(e.message || 'Generation failed');
    } finally {
      setGenerating(false);
    }
  };

  const handleSelectExisting = async (key) => {
    setError('');
    try {
      const exported = await secretsFetch(`/keys/${key.id}/export`, 'POST');
      onSelect(exported.material, key.id);
    } catch (e) {
      setError(e.message || 'Export failed');
    }
  };

  return (
    <div className="admin-modal-overlay" onClick={onClose}>
      <div className="admin-modal" onClick={e => e.stopPropagation()} style={{ maxWidth: 560 }}>
        <h3 style={{ marginBottom: 16 }}>{title || 'Key from Secrets Manager'}</h3>
        {error && <div className="admin-msg admin-msg-error" style={{ marginBottom: 12 }}>{error}</div>}

        <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
          <button className={`btn btn-small ${tab === 'generate' ? 'btn-primary' : 'btn-secondary'}`} onClick={() => setTab('generate')}>Generate New</button>
          <button className={`btn btn-small ${tab === 'existing' ? 'btn-primary' : 'btn-secondary'}`} onClick={() => setTab('existing')}>Use Existing</button>
        </div>

        {tab === 'generate' && (
          <div className="form-grid">
            <div className="form-field">
              <label className="form-label">Provider</label>
              <select className="filter-select" value={selectedProvider} onChange={e => setSelectedProvider(e.target.value)}>
                <option value="">Select provider...</option>
                {providers.map(p => <option key={p.id} value={p.id}>{p.name} ({p.providerType})</option>)}
              </select>
            </div>
            <div className="form-field">
              <label className="form-label">Key Name</label>
              <input className="filter-input" value={keyName} onChange={e => setKeyName(e.target.value)} placeholder={`e.g. master-key-${Date.now()}`} />
            </div>
            <div className="form-field">
              <label className="form-label">Key Type</label>
              <input className="filter-input" value={keyType} readOnly style={{ opacity: 0.7 }} />
            </div>
            {keyUsage && (
              <div className="form-field">
                <label className="form-label">Key Usage</label>
                <input className="filter-input" value={keyUsage} readOnly style={{ opacity: 0.7 }} />
              </div>
            )}
            <div className="form-field" style={{ gridColumn: '1 / -1' }}>
              <button className="btn btn-primary" onClick={handleGenerate} disabled={generating}>
                {generating ? 'Generating...' : 'Generate Key'}
              </button>
            </div>
          </div>
        )}

        {tab === 'existing' && (
          <div>
            {loadingKeys ? (
              <div className="spinner-wrap" style={{ padding: 24 }}><div className="spinner" /></div>
            ) : keys.length === 0 ? (
              <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: 24 }}>
                No matching keys found ({keyType}{keyUsage ? ` / ${keyUsage}` : ''})
              </div>
            ) : (
              <div style={{ maxHeight: 300, overflowY: 'auto' }}>
                <table style={{ width: '100%' }}>
                  <thead>
                    <tr>
                      <th style={{ textAlign: 'left', padding: '6px 8px', fontSize: 11 }}>Name</th>
                      <th style={{ textAlign: 'left', padding: '6px 8px', fontSize: 11 }}>Type</th>
                      <th style={{ textAlign: 'left', padding: '6px 8px', fontSize: 11 }}>Provider</th>
                      <th style={{ padding: '6px 8px', fontSize: 11 }}></th>
                    </tr>
                  </thead>
                  <tbody>
                    {keys.map(k => (
                      <tr key={k.id}>
                        <td style={{ padding: '6px 8px', fontSize: 12 }}><strong>{k.name}</strong></td>
                        <td style={{ padding: '6px 8px', fontSize: 12 }}><span className="badge badge-muted">{k.keyType}</span></td>
                        <td style={{ padding: '6px 8px', fontSize: 12 }}>{k.providerId}</td>
                        <td style={{ padding: '6px 8px' }}>
                          <button className="btn btn-primary btn-small" onClick={() => handleSelectExisting(k)}>Select</button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
        </div>
      </div>
    </div>
  );
}
