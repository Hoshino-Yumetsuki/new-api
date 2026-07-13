/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import 'cap-widget';
import React, { useEffect, useRef } from 'react';

/** Semi Design tokens → Cap widget CSS variables (follows ink-battles / capjs.js.org/guide/widget). */
const capWidgetThemeStyle = {
  '--cap-background': 'var(--semi-color-bg-1)',
  '--cap-border-color': 'var(--semi-color-border)',
  '--cap-border-radius': '14px',
  '--cap-color': 'var(--semi-color-text-0)',
  '--cap-checkbox-border': '1px solid var(--semi-color-border)',
  '--cap-checkbox-background': 'var(--semi-color-fill-0)',
  '--cap-spinner-color': 'var(--semi-color-primary)',
  '--cap-spinner-background-color': 'var(--semi-color-primary-light-default)',
};

const CapWidget = ({ apiEndpoint, onVerify, onExpire, onReady, className }) => {
  const ref = useRef(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const handleSolve = (event) => {
      const token = event.detail?.token;
      if (token) onVerify(token);
    };
    const handleExpire = () => onExpire?.();

    el.addEventListener('solve', handleSolve);
    el.addEventListener('error', handleExpire);
    el.addEventListener('reset', handleExpire);

    return () => {
      el.removeEventListener('solve', handleSolve);
      el.removeEventListener('error', handleExpire);
      el.removeEventListener('reset', handleExpire);
    };
  }, [apiEndpoint, onVerify, onExpire]);

  useEffect(() => {
    onReady?.();
  }, [apiEndpoint, onReady]);

  return (
    <div className={className} style={capWidgetThemeStyle}>
      <cap-widget ref={ref} data-cap-api-endpoint={apiEndpoint} />
    </div>
  );
};

export default CapWidget;