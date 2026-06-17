package handler

import (
	// Internal packages
	"turbo-iptv-api/internal/auth" // For authentication service
	"turbo-iptv-api/internal/playlist" // For playlist service

	// External packages
	"github.com/jackc/pgx/v5/pgxpool" // For PostgreSQL connection pool
)

// Handler struct for the handler
type Handler struct {
	DB    *pgxpool.Pool // For database connection
	Auth  *auth.Service // For authentication service
	Store *playlist.Store // For playlist service
}
