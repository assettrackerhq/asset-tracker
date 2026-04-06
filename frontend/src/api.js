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

export function register(username, password) {
  return request('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
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
