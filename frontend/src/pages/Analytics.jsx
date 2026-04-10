import { useState, useEffect } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { getPortfolioAnalytics } from '../api';

export default function Analytics() {
  const [currency, setCurrency] = useState('USD');
  const [data, setData] = useState(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadData();
  }, [currency]);

  async function loadData() {
    setLoading(true);
    setError('');
    try {
      const result = await getPortfolioAnalytics(currency);
      setData(result);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  function formatValue(value) {
    try {
      return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(value);
    } catch {
      return `${value} ${currency}`;
    }
  }

  return (
    <div className="container">
      <div className="header">
        <h1>Analytics</h1>
        <div className="currency-selector">
          <label>Currency: </label>
          <input
            value={currency}
            onChange={(e) => setCurrency(e.target.value.toUpperCase())}
            maxLength={3}
            style={{ width: '60px', textAlign: 'center' }}
          />
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      {loading && <p className="info">Loading analytics...</p>}

      {!loading && data && (
        <>
          <div className="analytics-summary">
            <div className="summary-card">
              <div className="summary-label">Total Portfolio Value</div>
              <div className="summary-value">{formatValue(data.total_value)}</div>
            </div>
          </div>

          {data.series.length > 0 ? (
            <div className="chart-container">
              <h2>Portfolio Value Over Time</h2>
              <ResponsiveContainer width="100%" height={400}>
                <LineChart data={data.series}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="date" />
                  <YAxis />
                  <Tooltip formatter={(value) => formatValue(value)} />
                  <Line type="monotone" dataKey="value" stroke="#2563eb" strokeWidth={2} dot={{ r: 4 }} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <p style={{ textAlign: 'center', color: '#999', marginTop: '40px' }}>
              No data available. Add value points to your assets to see analytics.
            </p>
          )}
        </>
      )}
    </div>
  );
}
