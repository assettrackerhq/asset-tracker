import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { login, getLicenseStatus, resendVerification } from '../api';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [licenseValid, setLicenseValid] = useState(true);
  const [licenseMessage, setLicenseMessage] = useState('');
  const [unverifiedUserId, setUnverifiedUserId] = useState(null);
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
    setUnverifiedUserId(null);
    try {
      const data = await login(username, password);
      localStorage.setItem('token', data.token);
      navigate('/assets');
    } catch (err) {
      if (err.message === 'email_not_verified') {
        // The 403 response is caught by the request() helper which throws the error string.
        // We need to get the user_id from the response. Since request() only throws the error
        // string, we handle this by attempting login again to get the full response.
        setError('Please verify your email before logging in.');
        // Re-fetch to get user_id
        try {
          const resp = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
          });
          if (resp.status === 403) {
            const data = await resp.json();
            if (data.error === 'email_not_verified') {
              setUnverifiedUserId(data.user_id);
            }
          }
        } catch {
          // ignore
        }
      } else {
        setError(err.message);
      }
    }
  }

  function handleGoToVerify() {
    navigate('/verify-email', { state: { userId: unverifiedUserId } });
  }

  async function handleResendAndVerify() {
    if (unverifiedUserId) {
      try {
        await resendVerification(unverifiedUserId);
      } catch {
        // ignore
      }
      navigate('/verify-email', { state: { userId: unverifiedUserId } });
    }
  }

  return (
    <div className="auth-form">
      <h1>Login</h1>
      {!licenseValid && (
        <p className="error">{licenseMessage}</p>
      )}
      {error && <p className="error">{error}</p>}
      {unverifiedUserId && (
        <p className="info">
          <button type="button" onClick={handleGoToVerify} style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', padding: 0, fontSize: '14px' }}>Enter verification code</button>
          {' or '}
          <button type="button" onClick={handleResendAndVerify} style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', padding: 0, fontSize: '14px' }}>resend code</button>
        </p>
      )}
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
