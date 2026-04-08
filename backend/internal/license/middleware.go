package license

import (
	"encoding/json"
	"net/http"
)

type licenseExpiredResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// LicenseMiddleware returns an HTTP middleware that blocks requests when the license is invalid.
func LicenseMiddleware(checker *Checker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status := checker.CurrentStatus()
			if !status.Valid {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				msg := status.Message
				if msg == "" {
					msg = "Your license is invalid. Contact your administrator."
				}
				json.NewEncoder(w).Encode(licenseExpiredResponse{
					Error:   "license_expired",
					Message: msg,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
