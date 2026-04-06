import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { register } from '../api';

export default function Register() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const navigate = useNavigate();

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      const data = await register(username, password);
      localStorage.setItem('token', data.token);
      navigate('/assets');
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="auth-form">
      <h1>Register</h1>
      {error && <p className="error">{error}</p>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
        </div>
        <button type="submit" className="primary">Register</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Already have an account? <Link to="/login">Login</Link>
      </p>
    </div>
  );
}
