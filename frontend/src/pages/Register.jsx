import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { register, getUserLimit } from '../api';

export default function Register() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [limitInfo, setLimitInfo] = useState(null);
  const navigate = useNavigate();

  useEffect(() => {
    getUserLimit()
      .then(setLimitInfo)
      .catch(() => {});
  }, []);

  const limitReached = limitInfo && limitInfo.user_count >= limitInfo.user_limit;

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      const data = await register(username, email, password);
      navigate('/verify-email', { state: { userId: data.user_id } });
    } catch (err) {
      setError(err.message);
      getUserLimit().then(setLimitInfo).catch(() => {});
    }
  }

  return (
    <div className="auth-form">
      <h1>Register</h1>
      {limitInfo && (
        <p className={limitReached ? 'error' : 'info'}>
          Users: {limitInfo.user_count} / {limitInfo.user_limit}
          {limitReached && ' — Registration is currently unavailable. Contact your administrator to increase the user limit.'}
        </p>
      )}
      {error && <p className="error">{error}</p>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Email</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
        </div>
        <button type="submit" className="primary" disabled={limitReached}>Register</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Already have an account? <Link to="/login">Login</Link>
      </p>
    </div>
  );
}
