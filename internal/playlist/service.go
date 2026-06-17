package playlist

import (
	"github.com/gofiber/fiber/v2"

	"turbo-iptv-api/internal/model"
)

type ConnectParams struct {
	Page        int
	Limit       int
	Search      string
	Category    string
	ContentType string
}

func ConnectResponse(
	store *Store,
	playlistURL, playlistType string,
	params ConnectParams,
	extra fiber.Map,
) (fiber.Map, error) {
	content, err := FetchM3U(playlistURL)
	if err != nil {
		return nil, err
	}

	channels := ParseM3U(content)
	if len(channels) == 0 {
		return nil, fiber.NewError(fiber.StatusNotFound, "Nenhum canal encontrado na playlist")
	}

	sessionID, entry := store.Save(channels)
	contentType := params.ContentType
	if contentType == "" {
		contentType = model.ContentLive
	}

	response := fiber.Map{
		"type":        playlistType,
		"status":      "conectado",
		"sessionId":   sessionID,
		"contentType": contentType,
		"totals":      CountByType(channels, entry.Series),
		"page":        params.Page,
		"limit":       params.Limit,
	}

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

	for key, value := range extra {
		response[key] = value
	}

	return response, nil
}

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
