import React, { useState, useEffect, useRef } from 'react';

export function SearchInput({ value, onChange, placeholder = 'Search…' }) {
  const [local, setLocal] = useState(value || '');
  const timer = useRef(null);

  useEffect(() => { setLocal(value || ''); }, [value]);

  const handleChange = (e) => {
    const v = e.target.value;
    setLocal(v);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => onChange(v), 300);
  };

  const clear = () => {
    setLocal('');
    clearTimeout(timer.current);
    onChange('');
  };

  return (
    <div className="search-input-wrap">
      <svg className="search-input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
        strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
      </svg>
      <input
        type="text"
        className="search-input"
        value={local}
        onChange={handleChange}
        placeholder={placeholder}
      />
      {local && (
        <button className="search-input-clear" onClick={clear} title="Clear search">&times;</button>
      )}
    </div>
  );
}