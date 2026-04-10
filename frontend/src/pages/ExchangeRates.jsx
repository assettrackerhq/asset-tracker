import { useState, useEffect } from 'react';
import { listExchangeRates, upsertExchangeRate, deleteExchangeRate, fetchExchangeRates } from '../api';

export default function ExchangeRates() {
  const [rates, setRates] = useState([]);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({ base_currency: '', target_currency: '', rate: '' });
  const [fetchBase, setFetchBase] = useState('USD');
  const [fetchStatus, setFetchStatus] = useState('');
  const [fetching, setFetching] = useState(false);

  useEffect(() => {
    loadRates();
  }, []);

  async function loadRates() {
    try {
      const data = await listExchangeRates();
      setRates(data);
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      await upsertExchangeRate(formData.base_currency, formData.target_currency, formData.rate);
      setShowForm(false);
      setEditingId(null);
      setFormData({ base_currency: '', target_currency: '', rate: '' });
      await loadRates();
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleDelete(id) {
    if (!confirm('Delete this exchange rate?')) return;
    try {
      await deleteExchangeRate(id);
      await loadRates();
    } catch (err) {
      setError(err.message);
    }
  }

  function startEdit(rate) {
    setEditingId(rate.id);
    setFormData({
      base_currency: rate.base_currency,
      target_currency: rate.target_currency,
      rate: rate.rate,
    });
    setShowForm(true);
  }

  async function handleFetch() {
    setFetching(true);
    setFetchStatus('');
    setError('');
    try {
      const result = await fetchExchangeRates(fetchBase);
      setFetchStatus(`Fetched ${result.updated} rates for ${result.base}`);
      await loadRates();
    } catch (err) {
      setError(`Failed to fetch rates: ${err.message}`);
    } finally {
      setFetching(false);
    }
  }

  function formatTimestamp(ts) {
    return new Date(ts).toLocaleString();
  }

  return (
    <div className="container">
      <div className="header">
        <h1>Exchange Rates</h1>
        <div>
          <button className="primary" onClick={() => { setShowForm(true); setEditingId(null); setFormData({ base_currency: '', target_currency: '', rate: '' }); }}>
            Add Rate
          </button>
        </div>
      </div>

      {error && <p className="error">{error}</p>}
      {fetchStatus && <p className="success">{fetchStatus}</p>}

      <div style={{ marginBottom: '24px', padding: '16px', background: 'white', borderRadius: '8px', display: 'flex', alignItems: 'center', gap: '12px' }}>
        <label style={{ fontWeight: 600 }}>Fetch rates for base currency:</label>
        <input
          value={fetchBase}
          onChange={(e) => setFetchBase(e.target.value.toUpperCase())}
          maxLength={3}
          style={{ width: '60px', textAlign: 'center', padding: '8px', border: '1px solid #ccc', borderRadius: '4px' }}
        />
        <button className="primary" onClick={handleFetch} disabled={fetching}>
          {fetching ? 'Fetching...' : 'Fetch Current Rates'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} style={{ marginBottom: '24px', padding: '16px', background: 'white', borderRadius: '8px' }}>
          <div className="form-group">
            <label>Base Currency</label>
            <input
              value={formData.base_currency}
              onChange={(e) => setFormData({ ...formData, base_currency: e.target.value.toUpperCase() })}
              maxLength={3}
              required
              disabled={editingId !== null}
            />
          </div>
          <div className="form-group">
            <label>Target Currency</label>
            <input
              value={formData.target_currency}
              onChange={(e) => setFormData({ ...formData, target_currency: e.target.value.toUpperCase() })}
              maxLength={3}
              required
              disabled={editingId !== null}
            />
          </div>
          <div className="form-group">
            <label>Rate</label>
            <input
              type="number"
              step="0.000001"
              value={formData.rate}
              onChange={(e) => setFormData({ ...formData, rate: e.target.value })}
              required
            />
          </div>
          <button type="submit" className="primary">{editingId ? 'Update' : 'Add'}</button>
          <button type="button" className="secondary" onClick={() => { setShowForm(false); setEditingId(null); }}>Cancel</button>
        </form>
      )}

      <table>
        <thead>
          <tr>
            <th>Base</th>
            <th>Target</th>
            <th>Rate</th>
            <th>Updated</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {rates.map((rate) => (
            <tr key={rate.id}>
              <td>{rate.base_currency}</td>
              <td>{rate.target_currency}</td>
              <td>{rate.rate}</td>
              <td>{formatTimestamp(rate.updated_at)}</td>
              <td className="actions">
                <button className="secondary" onClick={() => startEdit(rate)}>Edit</button>
                <button className="danger" onClick={() => handleDelete(rate.id)}>Delete</button>
              </td>
            </tr>
          ))}
          {rates.length === 0 && (
            <tr><td colSpan="5" style={{ textAlign: 'center', color: '#999' }}>No exchange rates configured</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
