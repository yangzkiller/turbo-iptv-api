package playlist

import (
	// External packages
	"github.com/gofiber/fiber/v2" // For HTTP context and JSON response maps

	// Internal packages
	"turbo-iptv-api/internal/model" // For content type constants and session entry model
)

// ConnectParams struct for pagination and filtering options when browsing playlist content
type ConnectParams struct {
	Page        int    // Page number (1-based)
	Limit       int    // Items per page
	Search      string // Optional search term for name/category
	Category    string // Optional exact category filter
	ContentType string // Content type: live, movie or series
}

// ConnectResponse function to download, parse, store and return the first page of a playlist connection
func ConnectResponse(
	store *Store,
	playlistURL, playlistType string,
	params ConnectParams,
	extra fiber.Map,
) (fiber.Map, error) {
	// Download remote M3U content
	content, err := FetchM3U(playlistURL)
	if err != nil {
		return nil, err
	}

	// Parse M3U text into flat channel list
	channels := ParseM3U(content)
	if len(channels) == 0 {
		return nil, fiber.NewError(fiber.StatusNotFound, "Nenhum canal encontrado na playlist")
	}

	// Persist parsed content in memory and obtain a session ID
	sessionID, entry := store.Save(channels)
	contentType := params.ContentType
	if contentType == "" {
		contentType = model.ContentLive
	}

	// Build base connect response shared by all content types
	response := fiber.Map{
		"type":        playlistType,
		"status":      "conectado",
		"sessionId":   sessionID,
		"contentType": contentType,
		"totals":      CountByType(channels, entry.Series),
		"page":        params.Page,
		"limit":       params.Limit,
	}

	// Attach paginated content and categories for the requested type
	if contentType == model.ContentSeries {
		slice, total := PaginateSeries(entry.Series, params.Page, params.Limit, params.Search, params.Category)
		response["total"] = total
		response["categories"] = ExtractSeriesCategories(entry.Series)
		response["series"] = slice
	} else {
		slice, total := PaginateChannels(channels, params.Page, params.Limit, params.Search, params.Category, contentType)
		typeChannels := FilterChannels(channels, "", "", contentType)
		response["total"] = total
		response["categories"] = ExtractCategories(typeChannels)
		response["channels"] = slice
	}

	// Merge optional extra fields (saved playlist info, username, etc.)
	for key, value := range extra {
		response[key] = value
	}

	return response, nil
}

// PageResponse function to paginate content from an existing in-memory playlist session
func PageResponse(entry model.Entry, params ConnectParams) fiber.Map {
	contentType := params.ContentType
	if contentType == "" {
		contentType = model.ContentLive
	}

	if contentType == model.ContentSeries {
		slice, total := PaginateSeries(entry.Series, params.Page, params.Limit, params.Search, params.Category)
		return fiber.Map{
			"type":   contentType,
			"total":  total,
			"page":   params.Page,
			"limit":  params.Limit,
			"series": slice,
		}
	}

	slice, total := PaginateChannels(entry.Channels, params.Page, params.Limit, params.Search, params.Category, contentType)
	return fiber.Map{
		"type":     contentType,
		"total":    total,
		"page":     params.Page,
		"limit":    params.Limit,
		"channels": slice,
	}
}

// ConnectParamsFromCtx function to read pagination and filter query params from a Fiber request
func ConnectParamsFromCtx(c *fiber.Ctx) ConnectParams {
	contentType := c.Query("type", model.ContentLive)
	return ConnectParams{
		Page:        c.QueryInt("page", 1),
		Limit:       c.QueryInt("limit", 100),
		Search:      c.Query("search", ""),
		Category:    c.Query("category", ""),
		ContentType: contentType,
	}
}
