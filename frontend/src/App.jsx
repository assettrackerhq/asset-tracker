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
import LinkedAccounts from './pages/LinkedAccounts';
import NavBar from './components/NavBar';
import { checkForUpdates, getFeatures } from './api';
import './App.css';

function ProtectedRoute({ children }) {
  const token = localStorage.getItem('token');
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

function Layout({ children, updateAvailable, analyticsEnabled, plaidEnabled, tellerEnabled }) {
  const location = useLocation();
  const publicPaths = ['/login', '/register', '/verify-email', '/license-expired'];
  const showNav = !publicPaths.includes(location.pathname);

  return (
    <>
      {showNav && <NavBar updateAvailable={updateAvailable} analyticsEnabled={analyticsEnabled} plaidEnabled={plaidEnabled} tellerEnabled={tellerEnabled} />}
      {children}
    </>
  );
}

export default function App() {
  const [updateAvailable, setUpdateAvailable] = useState(false);
  const [analyticsEnabled, setAnalyticsEnabled] = useState(true);
  const [plaidEnabled, setPlaidEnabled] = useState(false);
  const [tellerEnabled, setTellerEnabled] = useState(false);
  const [tellerApplicationId, setTellerApplicationId] = useState('');

  useEffect(() => {
    checkForUpdates().then((data) => {
      setUpdateAvailable(data.updatesAvailable);
    });
    getFeatures().then((data) => {
      setAnalyticsEnabled(data.analytics_enabled);
      setPlaidEnabled(data.plaid_enabled || false);
      setTellerEnabled(data.teller_enabled || false);
      setTellerApplicationId(data.teller_application_id || '');
    });
  }, []);

  return (
    <BrowserRouter>
      <Layout updateAvailable={updateAvailable} analyticsEnabled={analyticsEnabled} plaidEnabled={plaidEnabled} tellerEnabled={tellerEnabled}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/verify-email" element={<VerifyEmail />} />
          <Route path="/license-expired" element={<LicenseExpired />} />
          <Route path="/assets" element={<ProtectedRoute><AssetList /></ProtectedRoute>} />
          <Route path="/assets/:id" element={<ProtectedRoute><AssetDetail /></ProtectedRoute>} />
          <Route path="/analytics" element={<ProtectedRoute><Analytics /></ProtectedRoute>} />
          <Route path="/exchange-rates" element={<ProtectedRoute><ExchangeRates /></ProtectedRoute>} />
          <Route path="/linked-accounts" element={
            <ProtectedRoute>
              <LinkedAccounts plaidEnabled={plaidEnabled} tellerEnabled={tellerEnabled} tellerApplicationId={tellerApplicationId} />
            </ProtectedRoute>
          } />
          <Route path="*" element={<Navigate to="/assets" replace />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}
