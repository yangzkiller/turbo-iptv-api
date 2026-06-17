package handler

import (
	// Standard packages
	"context" // For context management
	"time"    // For timestamp formatting

	// External packages
	"github.com/gofiber/fiber/v2" // For Fiber HTTP framework

	// Internal packages
	"turbo-iptv-api/internal/auth" // For authenticated user context
	"turbo-iptv-api/internal/util" // For content key and SQL helpers
)

// progressResponse struct for a watch progress item returned by the API
type progressResponse struct {
	ID              string   `json:"id"`
	ContentKey      string   `json:"contentKey"`
	Name            string   `json:"name"`
	Category        string   `json:"category"`
	Logo            string   `json:"logo"`
	URL             string   `json:"url"`
	ContentType     string   `json:"contentType"`
	SeriesName      string   `json:"seriesName,omitempty"`
	Season          *int     `json:"season,omitempty"`
	Episode         *int     `json:"episode,omitempty"`
	PositionSeconds float64  `json:"positionSeconds"`
	DurationSeconds *float64 `json:"durationSeconds,omitempty"`
	UpdatedAt       string   `json:"updatedAt"`
}

// upsertProgressRequest struct for the JSON body of the upsert progress endpoint
type upsertProgressRequest struct {
	Name            string   `json:"name"`            // Display name of the content
	Category        string   `json:"category"`        // Optional category label
	Logo            string   `json:"logo"`            // Optional logo URL
	URL             string   `json:"url"`             // Stream or content URL
	ContentType     string   `json:"contentType"`     // Content type: live, movie or series
	SeriesName      string   `json:"seriesName"`      // Optional series title for episodes
	Season          *int     `json:"season"`          // Optional season number
	Episode         *int     `json:"episode"`         // Optional episode number
	PositionSeconds float64  `json:"positionSeconds"` // Playback position in seconds
	DurationSeconds *float64 `json:"durationSeconds"` // Optional total duration in seconds
}

// upsertProgressData struct for validated progress input ready for persistence
type upsertProgressData struct {
	Name            string   // Required display name
	Category        string   // Optional category
	Logo            string   // Optional logo URL
	URL             string   // Required content URL
	ContentType     string   // Required content type
	SeriesName      string   // Optional series name
	Season          *int     // Optional season number
	Episode         *int     // Optional episode number
	PositionSeconds float64  // Playback position in seconds
	DurationSeconds *float64 // Optional total duration
	ContentKey      string   // Hash derived from URL for deduplication
}

// ListProgress function to return recent watch progress items for a playlist
func (h *Handler) ListProgress(c *fiber.Ctx) error {
	// Resolve authenticated user and playlist ownership
	userID, playlistID, err := h.progressPlaylistContext(c)
	if err != nil {
		return err
	}

	limit := c.QueryInt("limit", 20)

	// Fetch progress ordered by most recently watched
	rows, err := h.DB.Query(c.Context(), `
		SELECT id::text, content_key, name, COALESCE(category, ''), COALESCE(logo, ''), url,
		       content_type, COALESCE(series_name, ''), season, episode,
		       position_seconds, duration_seconds, updated_at
		FROM watch_progress
		WHERE user_id = $1 AND playlist_id = $2
		ORDER BY updated_at DESC
		LIMIT $3
	`, userID, playlistID, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao listar progresso"})
	}
	defer rows.Close()

	// Map database rows to API response items
	items := make([]progressResponse, 0)
	for rows.Next() {
		item, err := scanProgressResponse(rows)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Erro ao ler progresso"})
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"progress": items})
}

// GetProgress function to return watch progress for a single content item
func (h *Handler) GetProgress(c *fiber.Ctx) error {
	// Resolve authenticated user and playlist ownership
	userID, playlistID, err := h.progressPlaylistContext(c)
	if err != nil {
		return err
	}

	contentKey := c.Params("contentKey")

	// Fetch single progress record by content key
	item, err := h.fetchProgressByKey(c.Context(), userID, playlistID, contentKey)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Progresso não encontrado"})
	}

	return c.JSON(item)
}

// UpsertProgress function to create or update watch progress for a content item
func (h *Handler) UpsertProgress(c *fiber.Ctx) error {
	// Resolve authenticated user and playlist ownership
	userID, playlistID, err := h.progressPlaylistContext(c)
	if err != nil {
		return err
	}

	// Parse JSON body into request struct
	var req upsertProgressRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Dados inválidos"})
	}

	// Validate and normalize input fields
	data, errMsg := validateUpsertProgressRequest(req)
	if errMsg != "" {
		return c.Status(400).JSON(fiber.Map{"error": errMsg})
	}

	// Insert or update progress (unique per user, playlist and content key)
	id, err := h.upsertProgress(c.Context(), userID, playlistID, data)
	if err != nil {
		return respondProgressError(c, err)
	}

	return c.JSON(fiber.Map{"id": id, "contentKey": data.ContentKey})
}

// DeleteProgress function to remove watch progress for a content item
func (h *Handler) DeleteProgress(c *fiber.Ctx) error {
	userID := auth.UserID(c)
	playlistID := c.Params("playlistId")
	contentKey := c.Params("contentKey")

	tag, err := h.DB.Exec(c.Context(), `
		DELETE FROM watch_progress
		WHERE user_id = $1 AND playlist_id = $2 AND content_key = $3
	`, userID, playlistID, contentKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao remover progresso"})
	}
	if tag.RowsAffected() == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Progresso não encontrado"})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// progressPlaylistContext function to resolve user, playlist and ownership for progress routes
func (h *Handler) progressPlaylistContext(c *fiber.Ctx) (string, string, error) {
	userID := auth.UserID(c)
	playlistID := c.Params("playlistId")

	if err := h.ensurePlaylistOwner(c, userID, playlistID); err != nil {
		return "", "", c.Status(404).JSON(fiber.Map{"error": "Playlist não encontrada"})
	}

	return userID, playlistID, nil
}

// validateUpsertProgressRequest function to validate progress input before persistence
func validateUpsertProgressRequest(req upsertProgressRequest) (*upsertProgressData, string) {
	if req.URL == "" || req.Name == "" || req.ContentType == "" {
		return nil, "name, url e contentType são obrigatórios"
	}

	return &upsertProgressData{
		Name:            req.Name,
		Category:        req.Category,
		Logo:            req.Logo,
		URL:             req.URL,
		ContentType:     req.ContentType,
		SeriesName:      req.SeriesName,
		Season:          req.Season,
		Episode:         req.Episode,
		PositionSeconds: req.PositionSeconds,
		DurationSeconds: req.DurationSeconds,
		ContentKey:      util.ContentKey(req.URL),
	}, ""
}

// upsertProgress function to insert or update watch progress in the database
func (h *Handler) upsertProgress(ctx context.Context, userID, playlistID string, data *upsertProgressData) (string, error) {
	var id string
	err := h.DB.QueryRow(ctx, `
		INSERT INTO watch_progress (
			user_id, playlist_id, content_key, name, category, logo, url, content_type,
			series_name, season, episode, position_seconds, duration_seconds, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW())
		ON CONFLICT (user_id, playlist_id, content_key) DO UPDATE SET
			name = EXCLUDED.name,
			category = EXCLUDED.category,
			logo = EXCLUDED.logo,
			url = EXCLUDED.url,
			content_type = EXCLUDED.content_type,
			series_name = EXCLUDED.series_name,
			season = EXCLUDED.season,
			episode = EXCLUDED.episode,
			position_seconds = EXCLUDED.position_seconds,
			duration_seconds = EXCLUDED.duration_seconds,
			updated_at = NOW()
		RETURNING id::text
	`, userID, playlistID, data.ContentKey, data.Name, util.NullIfEmpty(data.Category), util.NullIfEmpty(data.Logo), data.URL, data.ContentType,
		util.NullIfEmpty(data.SeriesName), data.Season, data.Episode, data.PositionSeconds, data.DurationSeconds).Scan(&id)
	return id, err
}

// fetchProgressByKey function to load a single progress record by content key
func (h *Handler) fetchProgressByKey(ctx context.Context, userID, playlistID, contentKey string) (progressResponse, error) {
	row := h.DB.QueryRow(ctx, `
		SELECT id::text, content_key, name, COALESCE(category, ''), COALESCE(logo, ''), url,
		       content_type, COALESCE(series_name, ''), season, episode,
		       position_seconds, duration_seconds, updated_at
		FROM watch_progress
		WHERE user_id = $1 AND playlist_id = $2 AND content_key = $3
	`, userID, playlistID, contentKey)
	return scanProgressResponse(row)
}

// scanProgressResponse function to map a database row into a progress API response item
func scanProgressResponse(rows interface {
	Scan(dest ...any) error
}) (progressResponse, error) {
	var item progressResponse
	var updatedAt time.Time
	if err := rows.Scan(
		&item.ID, &item.ContentKey, &item.Name, &item.Category, &item.Logo, &item.URL,
		&item.ContentType, &item.SeriesName, &item.Season, &item.Episode,
		&item.PositionSeconds, &item.DurationSeconds, &updatedAt,
	); err != nil {
		return progressResponse{}, err
	}
	item.UpdatedAt = updatedAt.Format(time.RFC3339)
	return item, nil
}

// respondProgressError function to map database errors to HTTP responses on progress save
func respondProgressError(c *fiber.Ctx, err error) error {
	return c.Status(500).JSON(fiber.Map{"error": "Erro ao salvar progresso"})
}
