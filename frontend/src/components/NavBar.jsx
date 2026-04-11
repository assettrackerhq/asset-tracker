import { NavLink, useNavigate } from 'react-router-dom';

export default function NavBar({ updateAvailable, analyticsEnabled }) {
  const navigate = useNavigate();

  function handleLogout() {
    localStorage.removeItem('token');
    navigate('/login');
  }

  return (
    <nav className="nav-bar">
      <div className="nav-links">
        <NavLink to="/assets" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>Assets</NavLink>
        {analyticsEnabled && <NavLink to="/analytics" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>Analytics</NavLink>}
        {analyticsEnabled && <NavLink to="/exchange-rates" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>Exchange Rates</NavLink>}
      </div>
      <div className="nav-right">
        {updateAvailable && <span className="update-badge">Update available</span>}
        <button className="secondary" onClick={handleLogout}>Logout</button>
      </div>
    </nav>
  );
}
