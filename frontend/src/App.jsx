import { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import VerifyEmail from './pages/VerifyEmail';
import AssetList from './pages/AssetList';
import AssetDetail from './pages/AssetDetail';
import Analytics from './pages/Analytics';
import ExchangeRates from './pages/ExchangeRates';
import LicenseExpired from './pages/LicenseExpired';
import NavBar from './components/NavBar';
import { checkForUpdates } from './api';
import './App.css';

function ProtectedRoute({ children }) {
  const token = localStorage.getItem('token');
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

function Layout({ children, updateAvailable }) {
  const location = useLocation();
  const publicPaths = ['/login', '/register', '/verify-email', '/license-expired'];
  const showNav = !publicPaths.includes(location.pathname);

  return (
    <>
      {updateAvailable && (
        <div className="update-banner">Update available</div>
      )}
      {showNav && <NavBar />}
      {children}
    </>
  );
}

export default function App() {
  const [updateAvailable, setUpdateAvailable] = useState(false);

  useEffect(() => {
    checkForUpdates().then((data) => {
      setUpdateAvailable(data.updatesAvailable);
    });
  }, []);

  return (
    <BrowserRouter>
      <Layout updateAvailable={updateAvailable}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/verify-email" element={<VerifyEmail />} />
          <Route path="/license-expired" element={<LicenseExpired />} />
          <Route path="/assets" element={<ProtectedRoute><AssetList /></ProtectedRoute>} />
          <Route path="/assets/:id" element={<ProtectedRoute><AssetDetail /></ProtectedRoute>} />
          <Route path="/analytics" element={<ProtectedRoute><Analytics /></ProtectedRoute>} />
          <Route path="/exchange-rates" element={<ProtectedRoute><ExchangeRates /></ProtectedRoute>} />
          <Route path="*" element={<Navigate to="/assets" replace />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}
