package banking

import (
	"context"
	"fmt"

	"github.com/plaid/plaid-go/v29/plaid"
)

type PlaidProvider struct {
	client      *plaid.APIClient
	clientID    string
	secret      string
	environment string
}

func NewPlaidProvider(clientID, secret, environment string) *PlaidProvider {
	cfg := plaid.NewConfiguration()
	switch environment {
	case "production":
		cfg.UseEnvironment(plaid.Production)
	default:
		cfg.UseEnvironment(plaid.Sandbox)
	}
	cfg.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	cfg.AddDefaultHeader("PLAID-SECRET", secret)

	return &PlaidProvider{
		client:      plaid.NewAPIClient(cfg),
		clientID:    clientID,
		secret:      secret,
		environment: environment,
	}
}

func (p *PlaidProvider) Name() string {
	return "plaid"
}

func (p *PlaidProvider) CreateLinkToken(ctx context.Context, userID string) (string, error) {
	user := plaid.LinkTokenCreateRequestUser{ClientUserId: userID}
	req := plaid.NewLinkTokenCreateRequest("Asset Tracker", "en", []plaid.CountryCode{plaid.COUNTRYCODE_US}, user)
	req.SetProducts([]plaid.Products{plaid.PRODUCTS_AUTH, plaid.PRODUCTS_TRANSACTIONS})

	resp, _, err := p.client.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*req).Execute()
	if err != nil {
		return "", fmt.Errorf("plaid link token create: %w", err)
	}
	return resp.GetLinkToken(), nil
}

func (p *PlaidProvider) ExchangeToken(ctx context.Context, publicToken string) (string, error) {
	req := plaid.NewItemPublicTokenExchangeRequest(publicToken)
	resp, _, err := p.client.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*req).Execute()
	if err != nil {
		return "", fmt.Errorf("plaid token exchange: %w", err)
	}
	return resp.GetAccessToken(), nil
}

func (p *PlaidProvider) FetchAccounts(ctx context.Context, accessToken string) ([]Account, error) {
	req := plaid.NewAccountsBalanceGetRequest(accessToken)
	resp, _, err := p.client.PlaidApi.AccountsBalanceGet(ctx).AccountsBalanceGetRequest(*req).Execute()
	if err != nil {
		return nil, fmt.Errorf("plaid accounts balance get: %w", err)
	}

	var accounts []Account
	for _, acct := range resp.GetAccounts() {
		balance := acct.GetBalances()

		current := 0.0
		if val, ok := balance.GetCurrentOk(); ok && val != nil {
			current = *val
		}

		currency := "USD"
		if val, ok := balance.GetIsoCurrencyCodeOk(); ok && val != nil {
			currency = *val
		}

		accounts = append(accounts, Account{
			ExternalID:  acct.GetAccountId(),
			Name:        acct.GetName(),
			Type:        string(acct.GetType()),
			Balance:     current,
			Currency:    currency,
			Institution: "Plaid Sandbox",
		})
	}

	return accounts, nil
}
