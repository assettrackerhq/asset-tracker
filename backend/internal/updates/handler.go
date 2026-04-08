package updates

import (
	"encoding/json"
	"net/http"
)

// Handler checks the Replicated SDK for available application updates.
type Handler struct {
	sdkEndpoint string
}

// NewHandler creates a Handler that queries the Replicated SDK at the given base URL.
func NewHandler(sdkEndpoint string) *Handler {
	return &Handler{sdkEndpoint: sdkEndpoint}
}

// Check queries the SDK and returns {"updatesAvailable": true/false}.
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	available := h.hasUpdates(r)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"updatesAvailable": available,
	})
}

func (h *Handler) hasUpdates(r *http.Request) bool {
	url := h.sdkEndpoint + "/api/v1/app/updates"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var releases []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return false
	}

	return len(releases) > 0
}
