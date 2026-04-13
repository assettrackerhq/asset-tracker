package banking

import "context"

// BankProvider defines the interface for bank account linking providers.
type BankProvider interface {
	Name() string
	CreateLinkToken(ctx context.Context, userID string) (string, error)
	ExchangeToken(ctx context.Context, publicToken string) (string, error)
	FetchAccounts(ctx context.Context, accessToken string) ([]Account, error)
}

// Account represents a bank account fetched from a provider.
type Account struct {
	ExternalID  string  `json:"external_id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
	Institution string  `json:"institution"`
}

// LinkedAccount represents a bank account stored in the database.
type LinkedAccount struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Source      string  `json:"source"`
	ExternalID  *string `json:"external_id"`
	Institution *string `json:"institution"`
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
	UpdatedAt   string  `json:"updated_at"`
}
