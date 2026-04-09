package supportbundle

import (
	"encoding/json"
	"net/http"
)

// Handler generates support bundles via the Replicated SDK.
type Handler struct {
	sdkEndpoint string
}

// NewHandler creates a Handler that calls the Replicated SDK at the given base URL.
func NewHandler(sdkEndpoint string) *Handler {
	return &Handler{sdkEndpoint: sdkEndpoint}
}

// Generate triggers support bundle collection and upload to the Vendor Portal.
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	url := h.sdkEndpoint + "/api/v1/supportbundle"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, nil)
	if err != nil {
		http.Error(w, `{"error":"failed to create request"}`, http.StatusInternalServerError)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":"failed to reach support bundle service"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		http.Error(w, `{"error":"support bundle generation failed"}`, http.StatusBadGateway)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Support bundle generated and uploaded",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	result["message"] = "Support bundle generated and uploaded"
	json.NewEncoder(w).Encode(result)
}
