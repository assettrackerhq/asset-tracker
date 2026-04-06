import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { listAssets, createAsset, updateAsset, deleteAsset } from '../api';

export default function AssetList() {
  const [assets, setAssets] = useState([]);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({ id: '', name: '', description: '' });
  const navigate = useNavigate();

  useEffect(() => {
    loadAssets();
  }, []);

  async function loadAssets() {
    try {
      const data = await listAssets();
      setAssets(data);
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      if (editingId) {
        await updateAsset(editingId, formData.name, formData.description || null);
      } else {
        await createAsset(formData.id, formData.name, formData.description || null);
      }
      setShowForm(false);
      setEditingId(null);
      setFormData({ id: '', name: '', description: '' });
      await loadAssets();
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleDelete(id) {
    if (!confirm('Delete this asset?')) return;
    try {
      await deleteAsset(id);
      await loadAssets();
    } catch (err) {
      setError(err.message);
    }
  }

  function startEdit(asset) {
    setEditingId(asset.id);
    setFormData({ id: asset.id, name: asset.name, description: asset.description || '' });
    setShowForm(true);
  }

  function handleLogout() {
    localStorage.removeItem('token');
    navigate('/login');
  }

  return (
    <div className="container">
      <div className="header">
        <h1>My Assets</h1>
        <div>
          <button className="primary" onClick={() => { setShowForm(true); setEditingId(null); setFormData({ id: '', name: '', description: '' }); }}>
            Add Asset
          </button>
          <button className="secondary" onClick={handleLogout}>Logout</button>
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      {showForm && (
        <form onSubmit={handleSubmit} style={{ marginBottom: '24px', padding: '16px', background: 'white', borderRadius: '8px' }}>
          {!editingId && (
            <div className="form-group">
              <label>ID</label>
              <input value={formData.id} onChange={(e) => setFormData({ ...formData, id: e.target.value })} required />
            </div>
          )}
          <div className="form-group">
            <label>Name</label>
            <input value={formData.name} onChange={(e) => setFormData({ ...formData, name: e.target.value })} required />
          </div>
          <div className="form-group">
            <label>Description</label>
            <textarea value={formData.description} onChange={(e) => setFormData({ ...formData, description: e.target.value })} />
          </div>
          <button type="submit" className="primary">{editingId ? 'Update' : 'Create'}</button>
          <button type="button" className="secondary" onClick={() => { setShowForm(false); setEditingId(null); }}>Cancel</button>
        </form>
      )}

      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Name</th>
            <th>Description</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {assets.map((asset) => (
            <tr key={asset.id}>
              <td><a href="#" onClick={(e) => { e.preventDefault(); navigate(`/assets/${asset.id}`); }}>{asset.id}</a></td>
              <td>{asset.name}</td>
              <td>{asset.description}</td>
              <td className="actions">
                <button className="secondary" onClick={() => startEdit(asset)}>Edit</button>
                <button className="danger" onClick={() => handleDelete(asset.id)}>Delete</button>
              </td>
            </tr>
          ))}
          {assets.length === 0 && (
            <tr><td colSpan="4" style={{ textAlign: 'center', color: '#999' }}>No assets yet</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
