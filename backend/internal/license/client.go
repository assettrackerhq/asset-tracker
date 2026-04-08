package license

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
