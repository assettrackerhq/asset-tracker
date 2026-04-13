import { useState, useEffect, useCallback } from 'react';
import { usePlaidLink } from 'react-plaid-link';
import { useTellerConnect } from 'teller-connect-react';
import { listLinkedAccounts, createLinkToken, connectAccount, syncAccounts, unlinkAccount } from '../api';

function PlaidLinkButton({ onSuccess }) {
  const [linkToken, setLinkToken] = useState(null);

  useEffect(() => {
    createLinkToken('plaid').then((data) => {
      setLinkToken(data.link_token);
    });
  }, []);

  const onPlaidSuccess = useCallback(async (publicToken) => {
    await connectAccount('plaid', publicToken);
    onSuccess();
  }, [onSuccess]);

  const { open, ready } = usePlaidLink({
    token: linkToken,
    onSuccess: onPlaidSuccess,
  });

  return (
    <button className="primary" onClick={() => open()} disabled={!ready}>
      Link with Plaid
    </button>
  );
}

function TellerConnectButton({ applicationId, onSuccess }) {
  const onTellerSuccess = useCallback(async (authorization) => {
    await connectAccount('teller', authorization.accessToken);
    onSuccess();
  }, [onSuccess]);

  const { open, ready } = useTellerConnect({
    applicationId: applicationId,
    onSuccess: onTellerSuccess,
  });

  return (
    <button className="primary" onClick={() => open()} disabled={!ready}>
      Link with Teller
    </button>
  );
}

export default function LinkedAccounts({ plaidEnabled, tellerEnabled, tellerApplicationId }) {
  const [accounts, setAccounts] = useState([]);
  const [error, setError] = useState('');
  const [syncing, setSyncing] = useState(false);

  useEffect(() => {
    loadAccounts();
  }, []);

  async function loadAccounts() {
    try {
      const data = await listLinkedAccounts();
      setAccounts(data);
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleSync() {
    setSyncing(true);
    setError('');
    try {
      await syncAccounts();
      await loadAccounts();
    } catch (err) {
      setError(err.message);
    } finally {
      setSyncing(false);
    }
  }

  async function handleUnlink(id) {
    if (!confirm('Unlink this account? All value history will be deleted.')) return;
    try {
      await unlinkAccount(id);
      await loadAccounts();
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="container">
      <div className="header">
        <h1>Linked Accounts</h1>
        <div>
          {plaidEnabled && <PlaidLinkButton onSuccess={loadAccounts} />}
          {tellerEnabled && (
            <TellerConnectButton
              applicationId={tellerApplicationId}
              onSuccess={loadAccounts}
            />
          )}
          {accounts.length > 0 && (
            <button className="secondary" onClick={handleSync} disabled={syncing}>
              {syncing ? 'Syncing...' : 'Sync Now'}
            </button>
          )}
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Institution</th>
            <th>Source</th>
            <th>Balance</th>
            <th>Currency</th>
            <th>Last Updated</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {accounts.map((acct) => (
            <tr key={acct.id}>
              <td>{acct.name}</td>
              <td>{acct.institution}</td>
              <td><span className={`badge ${acct.source}`}>{acct.source}</span></td>
              <td>{acct.balance.toFixed(2)}</td>
              <td>{acct.currency}</td>
              <td>{new Date(acct.updated_at).toLocaleDateString()}</td>
              <td className="actions">
                <button className="danger" onClick={() => handleUnlink(acct.id)}>Unlink</button>
              </td>
            </tr>
          ))}
          {accounts.length === 0 && (
            <tr><td colSpan="7" style={{ textAlign: 'center', color: '#999' }}>No linked accounts yet</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
