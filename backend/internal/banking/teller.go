package banking

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type TellerProvider struct {
	applicationID string
	environment   string
	httpClient    *http.Client
}

func NewTellerProvider(applicationID, environment string) *TellerProvider {
	return &TellerProvider{
		applicationID: applicationID,
		environment:   environment,
		httpClient:    &http.Client{},
	}
}

func (t *TellerProvider) Name() string {
	return "teller"
}

func (t *TellerProvider) CreateLinkToken(_ context.Context, _ string) (string, error) {
	// Teller Connect is initialized client-side with the application ID.
	// No server-side link token is needed.
	return "", nil
}

func (t *TellerProvider) ExchangeToken(_ context.Context, accessToken string) (string, error) {
	// Teller Connect returns the access token directly to the frontend.
	// No exchange step needed — pass through.
	return accessToken, nil
}

func (t *TellerProvider) FetchAccounts(ctx context.Context, accessToken string) ([]Account, error) {
	baseURL := "https://api.teller.io"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/accounts", nil)
	if err != nil {
		return nil, fmt.Errorf("teller create request: %w", err)
	}
	req.SetBasicAuth(accessToken, "")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teller fetch accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("teller accounts returned status %d", resp.StatusCode)
	}

	var tellerAccounts []tellerAccount
	if err := json.NewDecoder(resp.Body).Decode(&tellerAccounts); err != nil {
		return nil, fmt.Errorf("teller decode accounts: %w", err)
	}

	var accounts []Account
	for _, ta := range tellerAccounts {
		balance, err := t.fetchBalance(ctx, accessToken, ta.ID)
		if err != nil {
			log.Printf("teller: failed to fetch balance for account %s: %v", ta.ID, err)
			balance = 0
		}

		accounts = append(accounts, Account{
			ExternalID:  ta.ID,
			Name:        ta.Name,
			Type:        ta.Type,
			Balance:     balance,
			Currency:    ta.Currency,
			Institution: ta.Institution.Name,
		})
	}

	return accounts, nil
}

func (t *TellerProvider) fetchBalance(ctx context.Context, accessToken, accountID string) (float64, error) {
	baseURL := "https://api.teller.io"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/accounts/"+accountID+"/balances", nil)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(accessToken, "")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("teller balances returned status %d", resp.StatusCode)
	}

	var bal tellerBalance
	if err := json.NewDecoder(resp.Body).Decode(&bal); err != nil {
		return 0, err
	}

	return bal.Available, nil
}

type tellerAccount struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Currency    string            `json:"currency"`
	Institution tellerInstitution `json:"institution"`
}

type tellerInstitution struct {
	Name string `json:"name"`
}

type tellerBalance struct {
	Available float64 `json:"available,string"`
}
