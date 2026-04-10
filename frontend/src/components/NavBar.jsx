import { NavLink, useNavigate } from 'react-router-dom';

export default function NavBar() {
  const navigate = useNavigate();

  function handleLogout() {
    localStorage.removeItem('token');
    navigate('/login');
  }

  return (
    <nav className="nav-bar">
      <div className="nav-links">
        <NavLink to="/assets" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>Assets</NavLink>
        <NavLink to="/analytics" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>Analytics</NavLink>
        <NavLink to="/exchange-rates" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>Exchange Rates</NavLink>
      </div>
      <button className="secondary" onClick={handleLogout}>Logout</button>
    </nav>
  );
}
