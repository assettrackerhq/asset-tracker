import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { login, getLicenseStatus } from '../api';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [licenseValid, setLicenseValid] = useState(true);
  const [licenseMessage, setLicenseMessage] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    getLicenseStatus().then((status) => {
      setLicenseValid(status.valid);
      if (!status.valid) {
        setLicenseMessage(status.message || 'License is invalid.');
      }
    });
  }, []);

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      const data = await login(username, password);
      localStorage.setItem('token', data.token);
      navigate('/assets');
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="auth-form">
      <h1>Login</h1>
      {!licenseValid && (
        <p className="error">{licenseMessage}</p>
      )}
      {error && <p className="error">{error}</p>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </div>
        <button type="submit" className="primary" disabled={!licenseValid}>Login</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Don't have an account? <Link to="/register">Register</Link>
      </p>
    </div>
  );
}
