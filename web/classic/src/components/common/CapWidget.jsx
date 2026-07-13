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

const CapWidget = ({ apiEndpoint, onVerify, onExpire, className }) => {
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

  return (
    <cap-widget
      ref={ref}
      className={className}
      data-cap-api-endpoint={apiEndpoint}
    />
  );
};

export default CapWidget;