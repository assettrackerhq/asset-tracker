import { useState } from 'react';
import { useNavigate, useLocation, Link } from 'react-router-dom';
import { verifyEmail, resendVerification } from '../api';

export default function VerifyEmail() {
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [info, setInfo] = useState('');
  const navigate = useNavigate();
  const location = useLocation();
  const userId = location.state?.userId;

  if (!userId) {
    return (
      <div className="auth-form">
        <h1>Verify Email</h1>
        <p className="error">No user ID found. Please <Link to="/register">register</Link> first.</p>
      </div>
    );
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    setInfo('');
    try {
      const data = await verifyEmail(userId, code);
      localStorage.setItem('token', data.token);
      navigate('/assets');
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleResend() {
    setError('');
    setInfo('');
    try {
      const data = await resendVerification(userId);
      setInfo(data.message);
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="auth-form">
      <h1>Verify Email</h1>
      <p className="info">Enter the 6-digit code sent to your email.</p>
      {error && <p className="error">{error}</p>}
      {info && <p className="info">{info}</p>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Verification Code</label>
          <input
            className="verification-code-input"
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
            placeholder="000000"
            maxLength={6}
            required
          />
        </div>
        <button type="submit" className="primary" disabled={code.length !== 6}>Verify</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Didn't receive a code? <button type="button" onClick={handleResend} style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', padding: 0, fontSize: '14px' }}>Resend code</button>
      </p>
    </div>
  );
}
