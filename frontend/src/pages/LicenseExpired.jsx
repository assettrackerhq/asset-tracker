import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getLicenseStatus } from '../api';

export default function LicenseExpired() {
  const [checking, setChecking] = useState(false);
  const [message, setMessage] = useState('');
  const navigate = useNavigate();

  async function handleCheckAgain() {
    setChecking(true);
    setMessage('');
    try {
      const status = await getLicenseStatus();
      if (status.valid) {
        navigate('/login');
      } else {
        setMessage(status.message || 'License is still invalid.');
      }
    } catch {
      setMessage('Unable to check license status.');
    } finally {
      setChecking(false);
    }
  }

  return (
    <div className="auth-form">
      <h1>License Expired</h1>
      <p className="license-expired-text">
        Your license has expired or is invalid. Access to the application is
        currently unavailable.
      </p>
      <p className="license-expired-text">
        Please contact your administrator to renew your license.
      </p>
      {message && <p className="error">{message}</p>}
      <button className="primary" onClick={handleCheckAgain} disabled={checking}>
        {checking ? 'Checking...' : 'Check Again'}
      </button>
    </div>
  );
}
