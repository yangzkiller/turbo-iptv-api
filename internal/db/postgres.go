package db

import (
	// Standard packages
	"context" // For context management
	"fmt" // For formatting strings
	"time" // For time management

	// External packages
	"github.com/jackc/pgx/v5/pgxpool" // For PostgreSQL connection pool
)

// Connect function to connect to the PostgreSQL database
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	var lastErr error

	// Try to connect to the database 10 times with a delay of 1 second between attempts
	for attempt := 1; attempt <= 10; attempt++ {
		// Create a new PostgreSQL connection pool
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			lastErr = err
		} else if pingErr := pool.Ping(ctx); pingErr != nil {
			pool.Close()
			lastErr = pingErr
		} else {
			return pool, nil
		}

		time.Sleep(time.Duration(attempt) * time.Second)
	}

	// Return an error if the database connection fails
	return nil, fmt.Errorf("connect to postgres: %w", lastErr)
}
