package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	maxRetries := 30
	retryInterval := 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err = pgxpool.New(ctx, databaseURL)
		if err != nil {
			log.Printf("attempt %d/%d: unable to create connection pool: %v", attempt, maxRetries, err)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled waiting for database: %w", ctx.Err())
			case <-time.After(retryInterval):
				continue
			}
		}

		if err = pool.Ping(ctx); err != nil {
			pool.Close()
			log.Printf("attempt %d/%d: unable to ping database: %v", attempt, maxRetries, err)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled waiting for database: %w", ctx.Err())
			case <-time.After(retryInterval):
				continue
			}
		}

		log.Printf("database connection established on attempt %d", attempt)
		return pool, nil
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
}
