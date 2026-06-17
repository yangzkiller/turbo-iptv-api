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

// favoriteResponse struct for a favorite item returned by the API
type favoriteResponse struct {
	ID          string `json:"id"`
	ContentKey  string `json:"contentKey"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Logo        string `json:"logo"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	SeriesName  string `json:"seriesName,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

// addFavoriteRequest struct for the JSON body of the add favorite endpoint
type addFavoriteRequest struct {
	Name        string `json:"name"`        // Display name of the content
	Category    string `json:"category"`    // Optional category label
	Logo        string `json:"logo"`        // Optional logo URL
	URL         string `json:"url"`         // Stream or content URL
	ContentType string `json:"contentType"` // Content type: live, movie or series
	SeriesName  string `json:"seriesName"`  // Optional series title for episodes
}

// addFavoriteData struct for validated favorite input ready for persistence
type addFavoriteData struct {
	Name        string // Required display name
	Category    string // Optional category
	Logo        string // Optional logo URL
	URL         string // Required content URL
	ContentType string // Required content type
	SeriesName  string // Optional series name
	ContentKey  string // Hash derived from URL for deduplication
}

// ListFavorites function to return all favorites of a playlist for the authenticated user
func (h *Handler) ListFavorites(c *fiber.Ctx) error {
	// Resolve authenticated user and playlist ownership
	userID, playlistID, err := h.favoritePlaylistContext(c)
	if err != nil {
		return err
	}

	// Fetch favorites ordered by most recent first
	rows, err := h.DB.Query(c.Context(), `
		SELECT id::text, content_key, name, COALESCE(category, ''), COALESCE(logo, ''), url,
		       content_type, COALESCE(series_name, ''), created_at
		FROM favorites
		WHERE user_id = $1 AND playlist_id = $2
		ORDER BY created_at DESC
	`, userID, playlistID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao listar favoritos"})
	}
	defer rows.Close()

	// Map database rows to API response items
	items := make([]favoriteResponse, 0)
	for rows.Next() {
		item, err := scanFavoriteResponse(rows)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Erro ao ler favoritos"})
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"favorites": items})
}

// AddFavorite function to create or update a favorite item in a playlist
func (h *Handler) AddFavorite(c *fiber.Ctx) error {
	// Resolve authenticated user and playlist ownership
	userID, playlistID, err := h.favoritePlaylistContext(c)
	if err != nil {
		return err
	}

	// Parse JSON body into request struct
	var req addFavoriteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Dados inválidos"})
	}

	// Validate and normalize input fields
	data, errMsg := validateAddFavoriteRequest(req)
	if errMsg != "" {
		return c.Status(400).JSON(fiber.Map{"error": errMsg})
	}

	// Insert or update favorite (unique per user, playlist and content key)
	id, err := h.upsertFavorite(c.Context(), userID, playlistID, data)
	if err != nil {
		return respondFavoriteError(c, err)
	}

	return c.Status(201).JSON(fiber.Map{"id": id, "contentKey": data.ContentKey})
}

// DeleteFavorite function to remove a favorite by its database ID
func (h *Handler) DeleteFavorite(c *fiber.Ctx) error {
	return h.deleteFavorite(c, c.Params("id"), "")
}

// DeleteFavoriteByKey function to remove a favorite by its content key
func (h *Handler) DeleteFavoriteByKey(c *fiber.Ctx) error {
	return h.deleteFavorite(c, "", c.Params("contentKey"))
}

// favoritePlaylistContext function to resolve user, playlist and ownership for favorite routes
func (h *Handler) favoritePlaylistContext(c *fiber.Ctx) (string, string, error) {
	userID := auth.UserID(c)
	playlistID := c.Params("playlistId")

	if err := h.ensurePlaylistOwner(c, userID, playlistID); err != nil {
		return "", "", c.Status(404).JSON(fiber.Map{"error": "Playlist não encontrada"})
	}

	return userID, playlistID, nil
}

// validateAddFavoriteRequest function to validate favorite input before persistence
func validateAddFavoriteRequest(req addFavoriteRequest) (*addFavoriteData, string) {
	if req.URL == "" || req.Name == "" || req.ContentType == "" {
		return nil, "name, url e contentType são obrigatórios"
	}

	return &addFavoriteData{
		Name:        req.Name,
		Category:    req.Category,
		Logo:        req.Logo,
		URL:         req.URL,
		ContentType: req.ContentType,
		SeriesName:  req.SeriesName,
		ContentKey:  util.ContentKey(req.URL),
	}, ""
}

// upsertFavorite function to insert or update a favorite in the database
func (h *Handler) upsertFavorite(ctx context.Context, userID, playlistID string, data *addFavoriteData) (string, error) {
	var id string
	err := h.DB.QueryRow(ctx, `
		INSERT INTO favorites (user_id, playlist_id, content_key, name, category, logo, url, content_type, series_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, playlist_id, content_key) DO UPDATE SET
			name = EXCLUDED.name,
			category = EXCLUDED.category,
			logo = EXCLUDED.logo,
			url = EXCLUDED.url,
			content_type = EXCLUDED.content_type,
			series_name = EXCLUDED.series_name
		RETURNING id::text
	`, userID, playlistID, data.ContentKey, data.Name, util.NullIfEmpty(data.Category), util.NullIfEmpty(data.Logo), data.URL, data.ContentType, util.NullIfEmpty(data.SeriesName)).Scan(&id)
	return id, err
}

// deleteFavorite function to remove a favorite by ID or content key
func (h *Handler) deleteFavorite(c *fiber.Ctx, favoriteID, contentKey string) error {
	userID := auth.UserID(c)
	playlistID := c.Params("playlistId")

	var query string
	var args []any
	if favoriteID != "" {
		query = `
			DELETE FROM favorites
			WHERE id = $1 AND user_id = $2 AND playlist_id = $3
		`
		args = []any{favoriteID, userID, playlistID}
	} else {
		query = `
			DELETE FROM favorites
			WHERE user_id = $1 AND playlist_id = $2 AND content_key = $3
		`
		args = []any{userID, playlistID, contentKey}
	}

	tag, err := h.DB.Exec(c.Context(), query, args...)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao remover favorito"})
	}
	if tag.RowsAffected() == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Favorito não encontrado"})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// scanFavoriteResponse function to map a database row into a favorite API response item
func scanFavoriteResponse(rows interface {
	Scan(dest ...any) error
}) (favoriteResponse, error) {
	var item favoriteResponse
	var createdAt time.Time
	if err := rows.Scan(&item.ID, &item.ContentKey, &item.Name, &item.Category, &item.Logo, &item.URL, &item.ContentType, &item.SeriesName, &createdAt); err != nil {
		return favoriteResponse{}, err
	}
	item.CreatedAt = createdAt.Format(time.RFC3339)
	return item, nil
}

// respondFavoriteError function to map database errors to HTTP responses on favorite save
func respondFavoriteError(c *fiber.Ctx, err error) error {
	return c.Status(500).JSON(fiber.Map{"error": "Erro ao salvar favorito"})
}
