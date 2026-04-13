package banking

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Syncer struct {
	db        *pgxpool.Pool
	providers map[string]BankProvider
	interval  time.Duration
}

func NewSyncer(db *pgxpool.Pool, providers map[string]BankProvider) *Syncer {
	return &Syncer{
		db:        db,
		providers: providers,
		interval:  24 * time.Hour,
	}
}

func (s *Syncer) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAll(ctx)
		}
	}
}

func (s *Syncer) syncAll(ctx context.Context) {
	log.Println("banking: starting daily sync")

	rows, err := s.db.Query(ctx,
		`SELECT DISTINCT ON (source, access_token) user_id, source, access_token
		 FROM assets
		 WHERE source != 'manual' AND access_token IS NOT NULL`,
	)
	if err != nil {
		log.Printf("banking: sync query error: %v", err)
		return
	}
	defer rows.Close()

	type syncEntry struct {
		userID      string
		source      string
		accessToken string
	}
	var entries []syncEntry
	for rows.Next() {
		var e syncEntry
		if err := rows.Scan(&e.userID, &e.source, &e.accessToken); err != nil {
			log.Printf("banking: sync scan error: %v", err)
			continue
		}
		entries = append(entries, e)
	}

	synced := 0
	for _, e := range entries {
		provider, ok := s.providers[e.source]
		if !ok {
			continue
		}

		accounts, err := provider.FetchAccounts(ctx, e.accessToken)
		if err != nil {
			log.Printf("banking: sync fetch error for %s user %s: %v", e.source, e.userID, err)
			continue
		}

		for _, acct := range accounts {
			assetID := e.source + "-" + acct.ExternalID
			_, err := s.db.Exec(ctx,
				`INSERT INTO asset_value_points (asset_id, user_id, value, currency)
				 VALUES ($1, $2, $3, $4)`,
				assetID, e.userID, acct.Balance, acct.Currency,
			)
			if err != nil {
				log.Printf("banking: sync value point error: %v", err)
				continue
			}
			synced++
		}
	}

	log.Printf("banking: daily sync complete, %d value points updated", synced)
}
