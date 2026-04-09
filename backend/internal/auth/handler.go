package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/assettrackerhq/asset-tracker/backend/internal/email"
	"github.com/assettrackerhq/asset-tracker/backend/internal/license"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db            *pgxpool.Pool
	jwtSecret     string
	licenseClient *license.Client
	verifier      *email.Verifier
}

func NewHandler(db *pgxpool.Pool, jwtSecret string, licenseClient *license.Client, verifier *email.Verifier) *Handler {
	return &Handler{db: db, jwtSecret: jwtSecret, licenseClient: licenseClient, verifier: verifier}
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerResponse struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

type authResponse struct {
	Token string `json:"token"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if req.Username == "" {
		http.Error(w, `{"error":"username is required"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		http.Error(w, `{"error":"a valid email is required"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	// Check license entitlement for user limit
	if err := h.checkUserLimit(r.Context()); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusForbidden)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	userID := uuid.New().String()
	_, err = h.db.Exec(context.Background(),
		"INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4)",
		userID, req.Username, req.Email, string(hash),
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			if strings.Contains(err.Error(), "email") {
				http.Error(w, `{"error":"email already exists"}`, http.StatusConflict)
			} else {
				http.Error(w, `{"error":"username already exists"}`, http.StatusConflict)
			}
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Send verification code
	if err := h.verifier.SendCode(r.Context(), userID, req.Email); err != nil {
		log.Printf("failed to send verification email: %v", err)
		// Don't fail registration — user can resend later
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(registerResponse{
		UserID:  userID,
		Message: "Registration successful. Check your email for a verification code.",
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	var userID, passwordHash string
	var emailVerified bool
	err := h.db.QueryRow(context.Background(),
		"SELECT id, password_hash, email_verified FROM users WHERE username = $1",
		strings.TrimSpace(req.Username),
	).Scan(&userID, &passwordHash, &emailVerified)
	if err != nil {
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
		return
	}

	if !emailVerified {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "email_not_verified",
			"user_id": userID,
			"message": "Please verify your email before logging in. Check your inbox for a verification code.",
		})
		return
	}

	token, err := GenerateToken(userID, h.jwtSecret)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResponse{Token: token})
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.Code == "" {
		http.Error(w, `{"error":"user_id and code are required"}`, http.StatusBadRequest)
		return
	}

	if err := h.verifier.Verify(r.Context(), req.UserID, req.Code); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
		return
	}

	token, err := GenerateToken(req.UserID, h.jwtSecret)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResponse{Token: token})
}

func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	emailAddr, err := h.verifier.GetEmail(r.Context(), req.UserID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	if err := h.verifier.SendCode(r.Context(), req.UserID, emailAddr); err != nil {
		log.Printf("failed to resend verification email: %v", err)
		http.Error(w, `{"error":"failed to send verification email"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Verification code sent. Check your email.",
	})
}

func (h *Handler) checkUserLimit(ctx context.Context) error {
	limit, err := h.licenseClient.UserLimit(ctx)
	if err != nil {
		log.Printf("license: failed to check user_limit, allowing registration: %v", err)
		return nil
	}

	var count int
	err = h.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Printf("license: failed to count users: %v", err)
		return nil
	}

	if count >= limit {
		return fmt.Errorf("user limit reached (%d/%d). Please contact your administrator to increase the user limit in your license.", count, limit)
	}
	return nil
}

// UserLimitInfo returns the current user count and license limit.
func (h *Handler) UserLimitInfo(w http.ResponseWriter, r *http.Request) {
	limit, err := h.licenseClient.UserLimit(r.Context())
	if err != nil {
		log.Printf("license: failed to check user_limit: %v", err)
		limit = 1
	}

	var count int
	if err := h.db.QueryRow(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"user_count": count,
		"user_limit": limit,
	})
}
