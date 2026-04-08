package license

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	checkInterval = 60 * time.Second
	gracePeriod   = 15 * time.Minute
)

// Status represents the cached license state.
type Status struct {
	Valid       bool
	Message     string
	LastChecked time.Time
}

// Checker periodically polls the SDK and caches license validity.
type Checker struct {
	client *Client
	mu     sync.RWMutex
	status Status
	now    func() time.Time // for testing
}

// NewChecker creates a Checker that polls the given license client.
func NewChecker(client *Client) *Checker {
	return &Checker{
		client: client,
		now:    time.Now,
		status: Status{Valid: false, Message: "License status unknown — waiting for first check"},
	}
}

// Run polls the SDK on the given interval until the context is cancelled.
// It performs an immediate check before entering the loop.
func (c *Checker) Run(ctx context.Context) {
	c.check(ctx)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.check(ctx)
		}
	}
}

func (c *Checker) check(ctx context.Context) {
	info, err := c.client.LicenseInfo(ctx)
	if err != nil {
		log.Printf("license checker: SDK unreachable: %v", err)
		c.mu.Lock()
		defer c.mu.Unlock()
		// If we've never had a successful check, stay invalid
		if c.status.LastChecked.IsZero() {
			return
		}
		// Grace period: if last successful check was recent enough, keep current state
		if c.now().Sub(c.status.LastChecked) > gracePeriod {
			c.status.Valid = false
			c.status.Message = "License validation unavailable — please check your connection"
		}
		return
	}

	valid, message := evaluateLicense(info, c.now())

	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = Status{
		Valid:       valid,
		Message:     message,
		LastChecked: c.now(),
	}
}

func evaluateLicense(info *LicenseInfoResponse, now time.Time) (bool, string) {
	if info.ExpirationTime == nil || *info.ExpirationTime == "" {
		return true, ""
	}
	expiry, err := time.Parse(time.RFC3339, *info.ExpirationTime)
	if err != nil {
		return false, "License has an invalid expiration date"
	}
	if now.After(expiry) {
		return false, "Your license expired on " + expiry.Format("January 2, 2006") + ". Contact your administrator."
	}
	return true, ""
}

// CurrentStatus returns the cached license status.
func (c *Checker) CurrentStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}
