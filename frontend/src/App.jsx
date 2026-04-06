import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import AssetList from './pages/AssetList';
import AssetDetail from './pages/AssetDetail';
import './App.css';

function ProtectedRoute({ children }) {
  const token = localStorage.getItem('token');
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        <Route path="/assets" element={<ProtectedRoute><AssetList /></ProtectedRoute>} />
        <Route path="/assets/:id" element={<ProtectedRoute><AssetDetail /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/assets" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
