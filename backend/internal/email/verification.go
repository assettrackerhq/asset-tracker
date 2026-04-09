package email

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const codeExpiry = 15 * time.Minute

type Verifier struct {
	db     *pgxpool.Pool
	sender *Sender
}

func NewVerifier(db *pgxpool.Pool, sender *Sender) *Verifier {
	return &Verifier{db: db, sender: sender}
}

func (v *Verifier) SendCode(ctx context.Context, userID, emailAddr string) error {
	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	expiresAt := time.Now().Add(codeExpiry)

	// Delete any existing codes for this user
	_, err = v.db.Exec(ctx, "DELETE FROM verification_codes WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("clear old codes: %w", err)
	}

	_, err = v.db.Exec(ctx,
		"INSERT INTO verification_codes (user_id, code, expires_at) VALUES ($1, $2, $3)",
		userID, code, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("store code: %w", err)
	}

	body := fmt.Sprintf("Your verification code is: %s\n\nThis code expires in 15 minutes.", code)
	if err := v.sender.Send(emailAddr, "Verify your email — Asset Tracker", body); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func (v *Verifier) Verify(ctx context.Context, userID, code string) error {
	var storedCode string
	var expiresAt time.Time

	err := v.db.QueryRow(ctx,
		"SELECT code, expires_at FROM verification_codes WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1",
		userID,
	).Scan(&storedCode, &expiresAt)
	if err != nil {
		return fmt.Errorf("no verification code found — request a new one")
	}

	if time.Now().After(expiresAt) {
		return fmt.Errorf("verification code has expired — request a new one")
	}

	if storedCode != code {
		return fmt.Errorf("incorrect verification code")
	}

	_, err = v.db.Exec(ctx, "UPDATE users SET email_verified = true WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	// Clean up used codes
	_, _ = v.db.Exec(ctx, "DELETE FROM verification_codes WHERE user_id = $1", userID)

	return nil
}

func (v *Verifier) GetEmail(ctx context.Context, userID string) (string, error) {
	var emailAddr string
	err := v.db.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", userID).Scan(&emailAddr)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}
	return emailAddr, nil
}

func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
