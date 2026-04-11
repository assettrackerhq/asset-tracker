package license

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Client queries the Replicated SDK for license entitlement fields.
type Client struct {
	sdkEndpoint string
}

// NewClient creates a Client that talks to the Replicated SDK at the given base URL.
func NewClient(sdkEndpoint string) *Client {
	return &Client{sdkEndpoint: sdkEndpoint}
}

type licenseFieldResponse struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// entitlement represents a single entitlement field from the SDK license info response.
type entitlement struct {
	Title     string `json:"title"`
	Value     any    `json:"value"`
	ValueType string `json:"valueType"`
}

// LicenseInfoResponse represents the response from the SDK license info endpoint.
type LicenseInfoResponse struct {
	LicenseID    string                `json:"licenseID"`
	LicenseType  string                `json:"licenseType"`
	Entitlements map[string]entitlement `json:"entitlements"`
}

// ExpirationTime returns the expires_at entitlement value, or nil if not set.
func (r *LicenseInfoResponse) ExpirationTime() *string {
	if r.Entitlements == nil {
		return nil
	}
	ea, ok := r.Entitlements["expires_at"]
	if !ok {
		return nil
	}
	if s, ok := ea.Value.(string); ok && s != "" {
		return &s
	}
	return nil
}

// LicenseInfo queries the SDK for the full license info.
func (c *Client) LicenseInfo(ctx context.Context) (*LicenseInfoResponse, error) {
	url := c.sdkEndpoint + "/api/v1/license/info"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("license: failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("license: failed to query SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("license: SDK returned status %d", resp.StatusCode)
	}

	var info LicenseInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("license: failed to decode response: %w", err)
	}

	return &info, nil
}

// UserLimit queries the SDK for the user_limit license field and returns it as an int.
// Returns 1 as default if the field is not set or the SDK is unreachable.
func (c *Client) UserLimit(ctx context.Context) (int, error) {
	url := c.sdkEndpoint + "/api/v1/license/fields/user_limit"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 1, fmt.Errorf("license: failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 1, fmt.Errorf("license: failed to query SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 1, fmt.Errorf("license: SDK returned status %d", resp.StatusCode)
	}

	var field licenseFieldResponse
	if err := json.NewDecoder(resp.Body).Decode(&field); err != nil {
		return 1, fmt.Errorf("license: failed to decode response: %w", err)
	}

	return parseIntValue(field.Value)
}

// AnalyticsEnabled queries the SDK for the analytics_enabled license field.
// Returns true as default if the field is not set or the SDK is unreachable.
func (c *Client) AnalyticsEnabled(ctx context.Context) (bool, error) {
	url := c.sdkEndpoint + "/api/v1/license/fields/analytics_enabled"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return true, fmt.Errorf("license: failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return true, fmt.Errorf("license: failed to query SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return true, fmt.Errorf("license: SDK returned status %d", resp.StatusCode)
	}

	var field licenseFieldResponse
	if err := json.NewDecoder(resp.Body).Decode(&field); err != nil {
		return true, fmt.Errorf("license: failed to decode response: %w", err)
	}

	return parseBoolValue(field.Value)
}

func parseBoolValue(v any) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		return strings.EqualFold(val, "true") || val == "1", nil
	default:
		return true, fmt.Errorf("license: unexpected value type %T for bool field", v)
	}
}

func parseIntValue(v any) (int, error) {
	switch val := v.(type) {
	case float64:
		return int(val), nil
	case string:
		n, err := strconv.Atoi(val)
		if err != nil {
			return 1, fmt.Errorf("license: cannot parse user_limit %q as int: %w", val, err)
		}
		return n, nil
	default:
		return 1, fmt.Errorf("license: unexpected value type %T", v)
	}
}
