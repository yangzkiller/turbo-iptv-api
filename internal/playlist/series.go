package playlist

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"turbo-iptv-api/internal/model"
)

var episodeSuffixRegex = regexp.MustCompile(`(?i)\s+S(\d+)\s*E(\d+)\s*$`)

func ParseEpisodeInfo(name string) (seriesName string, season, episode int) {
	match := episodeSuffixRegex.FindStringSubmatch(name)
	if len(match) == 3 {
		seriesName = strings.TrimSpace(episodeSuffixRegex.ReplaceAllString(name, ""))
		season, _ = strconv.Atoi(match[1])
		episode, _ = strconv.Atoi(match[2])
		return seriesName, season, episode
	}
	return name, 0, 0
}

func GroupIntoSeries(channels []model.Channel) []model.SeriesItem {
	groups := make(map[string]*model.SeriesItem)
	order := make([]string, 0)

	for _, channel := range channels {
		if channel.Type != model.ContentSeries {
			continue
		}

		seriesName, season, episodeNum := ParseEpisodeInfo(channel.Name)
		key := strings.ToLower(seriesName)

		item, exists := groups[key]
		if !exists {
			item = &model.SeriesItem{
				Name:     seriesName,
				Category: channel.Category,
				Logo:     channel.Logo,
				Type:     model.ContentSeries,
				Episodes: make([]model.Episode, 0),
			}
			groups[key] = item
			order = append(order, key)
		}

		if item.Logo == "" && channel.Logo != "" {
			item.Logo = channel.Logo
		}

		item.Episodes = append(item.Episodes, model.Episode{
			Name:    channel.Name,
			Season:  season,
			Episode: episodeNum,
			Logo:    channel.Logo,
			URL:     channel.URL,
		})
	}

	result := make([]model.SeriesItem, 0, len(order))
	for _, key := range order {
		item := groups[key]
		sort.Slice(item.Episodes, func(i, j int) bool {
			if item.Episodes[i].Season != item.Episodes[j].Season {
				return item.Episodes[i].Season < item.Episodes[j].Season
			}
			return item.Episodes[i].Episode < item.Episodes[j].Episode
		})
		item.EpisodeCount = len(item.Episodes)
		result = append(result, *item)
	}

	return result
}

func ExtractSeriesCategories(series []model.SeriesItem) []string {
	seen := make(map[string]struct{})
	categories := make([]string, 0)

	for _, item := range series {
		if item.Category == "" {
			continue
		}
		if _, exists := seen[item.Category]; exists {
			continue
		}
		seen[item.Category] = struct{}{}
		categories = append(categories, item.Category)
	}

	return categories
}

func FilterSeries(series []model.SeriesItem, search, category string) []model.SeriesItem {
	filtered := series

	if category != "" {
		next := make([]model.SeriesItem, 0)
		for _, item := range filtered {
			if item.Category == category {
				next = append(next, item)
			}
		}
		filtered = next
	}

	if search != "" {
		needle := strings.ToLower(strings.TrimSpace(search))
		next := make([]model.SeriesItem, 0)
		for _, item := range filtered {
			if strings.Contains(strings.ToLower(item.Name), needle) ||
				strings.Contains(strings.ToLower(item.Category), needle) {
				next = append(next, item)
			}
		}
		filtered = next
	}

	return filtered
}

func PaginateSeries(series []model.SeriesItem, page, limit int, search, category string) ([]model.SeriesItem, int) {
	if limit <= 0 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}

	filtered := FilterSeries(series, search, category)
	total := len(filtered)
	start := (page - 1) * limit
	if start >= total {
		return []model.SeriesItem{}, total
	}

	end := start + limit
	if end > total {
		end = total
	}

	return filtered[start:end], total
}

func FilterChannels(channels []model.Channel, search, category, contentType string) []model.Channel {
	filtered := channels

	if contentType != "" {
		next := make([]model.Channel, 0)
		for _, channel := range channels {
			if channel.Type == contentType {
				next = append(next, channel)
			}
		}
		filtered = next
	}

	if category != "" {
		next := make([]model.Channel, 0)
		for _, channel := range filtered {
			if channel.Category == category {
				next = append(next, channel)
			}
		}
		filtered = next
	}

	if search != "" {
		needle := strings.ToLower(strings.TrimSpace(search))
		next := make([]model.Channel, 0)
		for _, channel := range filtered {
			if strings.Contains(strings.ToLower(channel.Name), needle) ||
				strings.Contains(strings.ToLower(channel.Category), needle) {
				next = append(next, channel)
			}
		}
		filtered = next
	}

	return filtered
}

func PaginateChannels(channels []model.Channel, page, limit int, search, category, contentType string) ([]model.Channel, int) {
	if limit <= 0 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}

	filtered := FilterChannels(channels, search, category, contentType)
	total := len(filtered)
	start := (page - 1) * limit
	if start >= total {
		return []model.Channel{}, total
	}

	end := start + limit
	if end > total {
		end = total
	}

	return filtered[start:end], total
}

func CountByType(channels []model.Channel, series []model.SeriesItem) fiber.Map {
	counts := map[string]int{
		model.ContentLive:  0,
		model.ContentMovie: 0,
	}
	for _, channel := range channels {
		counts[channel.Type]++
	}
	return fiber.Map{
		"live":   counts[model.ContentLive],
		"movies": counts[model.ContentMovie],
		"series": len(series),
	}
}
