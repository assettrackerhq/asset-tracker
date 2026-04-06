import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { listValuePoints, createValuePoint, updateValuePoint, deleteValuePoint } from '../api';

export default function AssetDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [values, setValues] = useState([]);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({ value: '', currency: 'USD' });

  useEffect(() => {
    loadValues();
  }, [id]);

  async function loadValues() {
    try {
      const data = await listValuePoints(id);
      setValues(data);
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      if (editingId) {
        await updateValuePoint(id, editingId, formData.value, formData.currency);
      } else {
        await createValuePoint(id, formData.value, formData.currency);
      }
      setShowForm(false);
      setEditingId(null);
      setFormData({ value: '', currency: 'USD' });
      await loadValues();
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleDelete(valueId) {
    if (!confirm('Delete this value point?')) return;
    try {
      await deleteValuePoint(id, valueId);
      await loadValues();
    } catch (err) {
      setError(err.message);
    }
  }

  function startEdit(vp) {
    setEditingId(vp.id);
    setFormData({ value: vp.value, currency: vp.currency });
    setShowForm(true);
  }

  function formatTimestamp(ts) {
    return new Date(ts).toLocaleString();
  }

  function formatValue(value, currency) {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(value);
  }

  return (
    <div className="container">
      <div className="header">
        <h1>Asset: {id}</h1>
        <div>
          <button className="primary" onClick={() => { setShowForm(true); setEditingId(null); setFormData({ value: '', currency: 'USD' }); }}>
            Add Value Point
          </button>
          <button className="secondary" onClick={() => navigate('/assets')}>Back</button>
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      {showForm && (
        <form onSubmit={handleSubmit} style={{ marginBottom: '24px', padding: '16px', background: 'white', borderRadius: '8px' }}>
          <div className="form-group">
            <label>Value</label>
            <input type="number" step="0.01" value={formData.value} onChange={(e) => setFormData({ ...formData, value: e.target.value })} required />
          </div>
          <div className="form-group">
            <label>Currency</label>
            <input value={formData.currency} onChange={(e) => setFormData({ ...formData, currency: e.target.value.toUpperCase() })} maxLength={3} required />
          </div>
          <button type="submit" className="primary">{editingId ? 'Update' : 'Add'}</button>
          <button type="button" className="secondary" onClick={() => { setShowForm(false); setEditingId(null); }}>Cancel</button>
        </form>
      )}

      <table>
        <thead>
          <tr>
            <th>Timestamp</th>
            <th>Value</th>
            <th>Currency</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {values.map((vp) => (
            <tr key={vp.id}>
              <td>{formatTimestamp(vp.timestamp)}</td>
              <td>{formatValue(vp.value, vp.currency)}</td>
              <td>{vp.currency}</td>
              <td className="actions">
                <button className="secondary" onClick={() => startEdit(vp)}>Edit</button>
                <button className="danger" onClick={() => handleDelete(vp.id)}>Delete</button>
              </td>
            </tr>
          ))}
          {values.length === 0 && (
            <tr><td colSpan="4" style={{ textAlign: 'center', color: '#999' }}>No value points yet</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
