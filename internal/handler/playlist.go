package handler

import (
	// Standard packages
	"context" // For context management
	"strings" // For string manipulation
	"time"    // For timestamp formatting

	// External packages
	"github.com/gofiber/fiber/v2" // For Fiber HTTP framework

	// Internal packages
	"turbo-iptv-api/internal/auth"    // For authenticated user context
	"turbo-iptv-api/internal/model"   // For content and request models
	"turbo-iptv-api/internal/playlist" // For playlist session and connection logic
	"turbo-iptv-api/internal/util"  // For SQL helpers
)

// playlistResponse struct for a saved playlist returned by the API
type playlistResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceType string `json:"sourceType"`
	DNS        string `json:"dns,omitempty"`
	Username   string `json:"username,omitempty"`
	M3uURL     string `json:"m3uUrl,omitempty"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

// createPlaylistRequest struct for the JSON body of the create playlist endpoint
type createPlaylistRequest struct {
	Name       string `json:"name"`       // Display name of the playlist
	SourceType string `json:"sourceType"` // Source type: xc or m3u
	DNS        string `json:"dns"`        // Xtream Codes server URL
	Username   string `json:"username"`   // Xtream Codes username
	Password   string `json:"password"`   // Xtream Codes password
	M3uURL     string `json:"m3uUrl"`     // M3U playlist URL
}

// createPlaylistData struct for validated playlist input ready for persistence
type createPlaylistData struct {
	Name       string // Trimmed playlist name
	SourceType string // Normalized source type: xc or m3u
	DNS        string // Xtream Codes server URL
	Username   string // Xtream Codes username
	Password   string // Xtream Codes password
	M3uURL     string // M3U playlist URL
}

// updatePlaylistRequest struct for the JSON body of the update playlist endpoint
type updatePlaylistRequest struct {
	Name     string `json:"name"`     // Updated display name
	DNS      string `json:"dns"`      // Updated Xtream Codes server URL
	Username string `json:"username"` // Updated Xtream Codes username
	Password string `json:"password"` // Optional new Xtream Codes password
	M3uURL   string `json:"m3uUrl"`   // Updated M3U playlist URL
}

// updatePlaylistData struct for validated playlist update input
type updatePlaylistData struct {
	Name     string // Trimmed playlist name
	DNS      string // Xtream Codes server URL
	Username string // Xtream Codes username
	Password string // Optional new password (empty keeps current)
	M3uURL   string // M3U playlist URL
}

// savedPlaylistRecord struct for a playlist row loaded from the database
type savedPlaylistRecord struct {
	Name       string // Playlist display name
	SourceType string // Source type: xc or m3u
	DNS        string // Xtream Codes server URL
	Username   string // Xtream Codes username
	Password   string // Xtream Codes password
	M3uURL     string // M3U playlist URL
}

// ListPlaylists function to return all saved playlists of the authenticated user
func (h *Handler) ListPlaylists(c *fiber.Ctx) error {
	userID := auth.UserID(c)

	// Fetch playlists ordered by most recently updated
	rows, err := h.DB.Query(c.Context(), `
		SELECT id::text, name, source_type, COALESCE(dns, ''), COALESCE(xc_username, ''), COALESCE(m3u_url, ''),
		       created_at, updated_at
		FROM user_playlists
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao listar playlists"})
	}
	defer rows.Close()

	// Map database rows to API response items
	items := make([]playlistResponse, 0)
	for rows.Next() {
		item, err := scanPlaylistResponse(rows)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Erro ao ler playlists"})
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"playlists": items})
}

// CreatePlaylist function to save a new playlist for the authenticated user
func (h *Handler) CreatePlaylist(c *fiber.Ctx) error {
	userID := auth.UserID(c)

	// Parse JSON body into request struct
	var req createPlaylistRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Dados inválidos"})
	}

	// Validate and normalize input fields
	data, errMsg := validateCreatePlaylistRequest(req)
	if errMsg != "" {
		return c.Status(400).JSON(fiber.Map{"error": errMsg})
	}

	// Persist playlist in the database
	id, err := h.insertPlaylist(c.Context(), userID, data)
	if err != nil {
		return respondPlaylistError(c, err)
	}

	return c.Status(201).JSON(fiber.Map{
		"id":         id,
		"name":       data.Name,
		"sourceType": data.SourceType,
	})
}

// UpdatePlaylist function to update a saved playlist owned by the authenticated user
func (h *Handler) UpdatePlaylist(c *fiber.Ctx) error {
	userID := auth.UserID(c)
	playlistID := c.Params("id")

	// Parse JSON body into request struct
	var req updatePlaylistRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Dados inválidos"})
	}

	// Load playlist source type and verify ownership
	sourceType, err := h.playlistSourceType(c.Context(), userID, playlistID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Playlist não encontrada"})
	}

	// Validate and normalize update input
	data, errMsg := validateUpdatePlaylistRequest(req)
	if errMsg != "" {
		return c.Status(400).JSON(fiber.Map{"error": errMsg})
	}

	// Apply update based on source type
	if err := h.updateSavedPlaylist(c.Context(), userID, playlistID, sourceType, data); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao atualizar playlist"})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// DeletePlaylist function to remove a saved playlist owned by the authenticated user
func (h *Handler) DeletePlaylist(c *fiber.Ctx) error {
	userID := auth.UserID(c)
	playlistID := c.Params("id")

	tag, err := h.DB.Exec(c.Context(), `
		DELETE FROM user_playlists WHERE id = $1 AND user_id = $2
	`, playlistID, userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao excluir playlist"})
	}
	if tag.RowsAffected() == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Playlist não encontrada"})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// ConnectSavedPlaylist function to connect using a playlist saved by the authenticated user
func (h *Handler) ConnectSavedPlaylist(c *fiber.Ctx) error {
	userID := auth.UserID(c)
	playlistID := c.Params("id")

	// Load saved playlist credentials
	record, err := h.fetchOwnedPlaylist(c.Context(), userID, playlistID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Playlist não encontrada"})
	}

	// Touch updated_at to reflect recent usage
	_, _ = h.DB.Exec(c.Context(), `
		UPDATE user_playlists SET updated_at = NOW() WHERE id = $1
	`, playlistID)

	// Build playlist URL and connect
	playlistURL, playlistType, err := buildPlaylistConnection(record)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return h.connectAndRespond(c, playlistURL, playlistType, fiber.Map{
		"savedPlaylistId":   playlistID,
		"savedPlaylistName": record.Name,
	})
}

// ConnectXC function to connect via Xtream Codes credentials without saving
func (h *Handler) ConnectXC(c *fiber.Ctx) error {
	// Parse JSON body into request struct
	var req model.XCRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Dados inválidos"})
	}

	// Validate required Xtream Codes fields
	if req.DNS == "" || req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "DNS, usuário e senha são obrigatórios"})
	}

	playlistURL := playlist.BuildXCPlaylistURL(req.DNS, req.Username, req.Password)
	return h.connectAndRespond(c, playlistURL, "Xtream Codes API", fiber.Map{"user": req.Username})
}

// ConnectM3U function to connect via M3U URL without saving
func (h *Handler) ConnectM3U(c *fiber.Ctx) error {
	m3uURL := c.Query("url")
	if m3uURL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "URL M3U ausente"})
	}

	return h.connectAndRespond(c, m3uURL, "Playlist M3U", nil)
}

// SessionChannels function to paginate channels from an active playlist session
func (h *Handler) SessionChannels(c *fiber.Ctx) error {
	entry, err := h.getSessionEntry(c)
	if err != nil {
		return err
	}

	return c.JSON(playlist.PageResponse(entry, playlist.ConnectParamsFromCtx(c)))
}

// SessionCategories function to list categories from an active playlist session
func (h *Handler) SessionCategories(c *fiber.Ctx) error {
	entry, err := h.getSessionEntry(c)
	if err != nil {
		return err
	}

	contentType := c.Query("type", model.ContentLive)
	if contentType == model.ContentSeries {
		return c.JSON(fiber.Map{
			"type":       contentType,
			"categories": playlist.ExtractSeriesCategories(entry.Series),
		})
	}

	typeChannels := playlist.FilterChannels(entry.Channels, "", "", contentType)
	return c.JSON(fiber.Map{
		"type":       contentType,
		"categories": playlist.ExtractCategories(typeChannels),
	})
}

// ensurePlaylistOwner function to verify that a playlist belongs to the authenticated user
func (h *Handler) ensurePlaylistOwner(c *fiber.Ctx, userID, playlistID string) error {
	var exists bool
	err := h.DB.QueryRow(c.Context(), `
		SELECT EXISTS(SELECT 1 FROM user_playlists WHERE id = $1 AND user_id = $2)
	`, playlistID, userID).Scan(&exists)
	if err != nil || !exists {
		return fiber.ErrNotFound
	}
	return nil
}

// validateCreatePlaylistRequest function to normalize and validate playlist creation input
func validateCreatePlaylistRequest(req createPlaylistRequest) (*createPlaylistData, string) {
	data := &createPlaylistData{
		Name:       strings.TrimSpace(req.Name),
		SourceType: strings.TrimSpace(strings.ToLower(req.SourceType)),
		DNS:        req.DNS,
		Username:   req.Username,
		Password:   req.Password,
		M3uURL:     strings.TrimSpace(req.M3uURL),
	}

	if data.Name == "" {
		return nil, "Nome da playlist é obrigatório"
	}

	switch data.SourceType {
	case "xc":
		if data.DNS == "" || data.Username == "" || data.Password == "" {
			return nil, "DNS, usuário e senha são obrigatórios para Xtream Codes"
		}
	case "m3u":
		if data.M3uURL == "" {
			return nil, "URL M3U é obrigatória"
		}
	default:
		return nil, "sourceType deve ser xc ou m3u"
	}

	return data, ""
}

// validateUpdatePlaylistRequest function to normalize and validate playlist update input
func validateUpdatePlaylistRequest(req updatePlaylistRequest) (*updatePlaylistData, string) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, "Nome é obrigatório"
	}

	return &updatePlaylistData{
		Name:     name,
		DNS:      req.DNS,
		Username: req.Username,
		Password: req.Password,
		M3uURL:   req.M3uURL,
	}, ""
}

// insertPlaylist function to insert a new saved playlist into the database
func (h *Handler) insertPlaylist(ctx context.Context, userID string, data *createPlaylistData) (string, error) {
	var id string
	err := h.DB.QueryRow(ctx, `
		INSERT INTO user_playlists (user_id, name, source_type, dns, xc_username, xc_password, m3u_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text
	`, userID, data.Name, data.SourceType, util.NullIfEmpty(data.DNS), util.NullIfEmpty(data.Username), util.NullIfEmpty(data.Password), util.NullIfEmpty(data.M3uURL)).Scan(&id)
	return id, err
}

// playlistSourceType function to fetch the source type of an owned playlist
func (h *Handler) playlistSourceType(ctx context.Context, userID, playlistID string) (string, error) {
	var sourceType string
	err := h.DB.QueryRow(ctx, `
		SELECT source_type FROM user_playlists WHERE id = $1 AND user_id = $2
	`, playlistID, userID).Scan(&sourceType)
	return sourceType, err
}

// updateSavedPlaylist function to update playlist fields based on its source type
func (h *Handler) updateSavedPlaylist(ctx context.Context, userID, playlistID, sourceType string, data *updatePlaylistData) error {
	if sourceType == "xc" {
		if data.Password != "" {
			_, err := h.DB.Exec(ctx, `
				UPDATE user_playlists
				SET name = $1, dns = $2, xc_username = $3, xc_password = $4, updated_at = NOW()
				WHERE id = $5 AND user_id = $6
			`, data.Name, data.DNS, data.Username, data.Password, playlistID, userID)
			return err
		}

		_, err := h.DB.Exec(ctx, `
			UPDATE user_playlists
			SET name = $1, dns = $2, xc_username = $3, updated_at = NOW()
			WHERE id = $4 AND user_id = $5
		`, data.Name, data.DNS, data.Username, playlistID, userID)
		return err
	}

	_, err := h.DB.Exec(ctx, `
		UPDATE user_playlists
		SET name = $1, m3u_url = $2, updated_at = NOW()
		WHERE id = $3 AND user_id = $4
	`, data.Name, data.M3uURL, playlistID, userID)
	return err
}

// fetchOwnedPlaylist function to load a saved playlist owned by the authenticated user
func (h *Handler) fetchOwnedPlaylist(ctx context.Context, userID, playlistID string) (*savedPlaylistRecord, error) {
	var record savedPlaylistRecord
	err := h.DB.QueryRow(ctx, `
		SELECT name, source_type, COALESCE(dns, ''), COALESCE(xc_username, ''), COALESCE(xc_password, ''), COALESCE(m3u_url, '')
		FROM user_playlists
		WHERE id = $1 AND user_id = $2
	`, playlistID, userID).Scan(&record.Name, &record.SourceType, &record.DNS, &record.Username, &record.Password, &record.M3uURL)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// buildPlaylistConnection function to derive URL and label from a saved playlist record
func buildPlaylistConnection(record *savedPlaylistRecord) (string, string, error) {
	switch record.SourceType {
	case "xc":
		return playlist.BuildXCPlaylistURL(record.DNS, record.Username, record.Password), "Xtream Codes API", nil
	case "m3u":
		return record.M3uURL, "Playlist M3U", nil
	default:
		return "", "", fiber.NewError(fiber.StatusBadRequest, "Tipo de playlist inválido")
	}
}

// connectAndRespond function to fetch a remote playlist and return the session response
func (h *Handler) connectAndRespond(c *fiber.Ctx, playlistURL, playlistType string, extra fiber.Map) error {
	response, err := playlist.ConnectResponse(h.Store, playlistURL, playlistType, playlist.ConnectParamsFromCtx(c), extra)
	if err != nil {
		return respondConnectError(c, err)
	}
	return c.JSON(response)
}

// getSessionEntry function to load an in-memory playlist session by session ID
func (h *Handler) getSessionEntry(c *fiber.Ctx) (model.Entry, error) {
	entry, ok := h.Store.Get(c.Params("sessionId"))
	if !ok {
		return model.Entry{}, c.Status(404).JSON(fiber.Map{"error": "Sessão expirada ou inválida. Carregue a playlist novamente."})
	}
	return entry, nil
}

// scanPlaylistResponse function to map a database row into a playlist API response item
func scanPlaylistResponse(rows interface {
	Scan(dest ...any) error
}) (playlistResponse, error) {
	var item playlistResponse
	var createdAt, updatedAt time.Time
	if err := rows.Scan(&item.ID, &item.Name, &item.SourceType, &item.DNS, &item.Username, &item.M3uURL, &createdAt, &updatedAt); err != nil {
		return playlistResponse{}, err
	}
	item.CreatedAt = createdAt.Format(time.RFC3339)
	item.UpdatedAt = updatedAt.Format(time.RFC3339)
	return item, nil
}

// respondPlaylistError function to map database errors to HTTP responses on playlist save/update
func respondPlaylistError(c *fiber.Ctx, err error) error {
	return c.Status(500).JSON(fiber.Map{"error": "Erro ao salvar playlist"})
}

// respondConnectError function to map connection errors to HTTP responses
func respondConnectError(c *fiber.Ctx, err error) error {
	if fe, ok := err.(*fiber.Error); ok {
		return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
	}
	return c.Status(502).JSON(fiber.Map{"error": "Falha ao buscar playlist: " + err.Error()})
}
