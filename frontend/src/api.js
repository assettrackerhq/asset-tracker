const API_BASE = '/api';

async function request(path, options = {}) {
  const token = localStorage.getItem('token');
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (response.status === 403) {
    const data = await response.json().catch(() => ({}));
    if (data.error === 'license_expired') {
      localStorage.removeItem('token');
      window.location.href = '/license-expired';
      return;
    }
    throw new Error(data.error || 'Forbidden');
  }

  if (response.status === 401) {
    localStorage.removeItem('token');
    window.location.href = '/login';
    return;
  }

  if (response.status === 204) {
    return null;
  }

  const data = await response.json();
  if (!response.ok) {
    throw new Error(data.error || 'Request failed');
  }
  return data;
}

export function login(username, password) {
  return request('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

export function register(username, email, password) {
  return request('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ username, email, password }),
  });
}

export function verifyEmail(userId, code) {
  return request('/auth/verify-email', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId, code }),
  });
}

export function resendVerification(userId) {
  return request('/auth/resend-verification', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId }),
  });
}

export function getUserLimit() {
  return request('/auth/user-limit');
}

export async function checkForUpdates() {
  try {
    const res = await fetch(`${API_BASE}/app/updates`);
    if (!res.ok) return { updatesAvailable: false };
    return await res.json();
  } catch {
    return { updatesAvailable: false };
  }
}

export function createSupportBundle() {
  return request('/support-bundle', { method: 'POST' });
}

export function getSupportBundleStatus(name) {
  return request(`/support-bundle/${name}`);
}

export function listAssets() {
  return request('/assets');
}

export function createAsset(id, name, description) {
  return request('/assets', {
    method: 'POST',
    body: JSON.stringify({ id, name, description }),
  });
}

export function updateAsset(id, name, description) {
  return request(`/assets/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, description }),
  });
}

export function deleteAsset(id) {
  return request(`/assets/${id}`, { method: 'DELETE' });
}

export function listValuePoints(assetId) {
  return request(`/assets/${assetId}/values`);
}

export function createValuePoint(assetId, value, currency) {
  return request(`/assets/${assetId}/values`, {
    method: 'POST',
    body: JSON.stringify({ value: parseFloat(value), currency }),
  });
}

export function updateValuePoint(assetId, valueId, value, currency) {
  return request(`/assets/${assetId}/values/${valueId}`, {
    method: 'PUT',
    body: JSON.stringify({ value: parseFloat(value), currency }),
  });
}

export function deleteValuePoint(assetId, valueId) {
  return request(`/assets/${assetId}/values/${valueId}`, { method: 'DELETE' });
}

export async function getLicenseStatus() {
  try {
    const res = await fetch(`${API_BASE}/license/status`);
    if (!res.ok) return { valid: true };
    return await res.json();
  } catch {
    return { valid: true };
  }
}

export function listExchangeRates() {
  return request('/exchange-rates');
}

export function upsertExchangeRate(baseCurrency, targetCurrency, rate) {
  return request('/exchange-rates', {
    method: 'POST',
    body: JSON.stringify({ base_currency: baseCurrency, target_currency: targetCurrency, rate: parseFloat(rate) }),
  });
}

export function deleteExchangeRate(id) {
  return request(`/exchange-rates/${id}`, { method: 'DELETE' });
}

export function fetchExchangeRates(baseCurrency) {
  return request('/exchange-rates/fetch', {
    method: 'POST',
    body: JSON.stringify({ base_currency: baseCurrency }),
  });
}

export function getPortfolioAnalytics(currency) {
  return request(`/analytics/portfolio?currency=${encodeURIComponent(currency)}`);
}
