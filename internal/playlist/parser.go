package playlist

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"turbo-iptv-api/internal/model"
)

var (
	groupTitleRegex = regexp.MustCompile(`group-title="([^"]*)"`)
	tvgNameRegex    = regexp.MustCompile(`tvg-name="([^"]*)"`)
	tvgLogoRegex    = regexp.MustCompile(`tvg-logo="([^"]*)"`)
)

func ClassifyContentType(streamURL string) string {
	lower := strings.ToLower(streamURL)
	if strings.Contains(lower, "/movie/") {
		return model.ContentMovie
	}
	if strings.Contains(lower, "/series/") {
		return model.ContentSeries
	}
	return model.ContentLive
}

func FetchM3U(playlistURL string) (string, error) {
	client := &http.Client{Timeout: 120 * time.Second}

	resp, err := client.Get(playlistURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("playlist retornou status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func ParseM3U(content string) []model.Channel {
	lines := strings.Split(content, "\n")
	channels := make([]model.Channel, 0)

	var pendingName, pendingCategory, pendingLogo string

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			pendingCategory = extractGroupTitle(line)
			pendingName = extractChannelName(line)
			pendingLogo = extractLogo(line)
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		channels = append(channels, model.Channel{
			Name:     pendingName,
			Category: pendingCategory,
			Logo:     pendingLogo,
			URL:      line,
			Type:     ClassifyContentType(line),
		})
		pendingName = ""
		pendingCategory = ""
		pendingLogo = ""
	}

	return channels
}

func extractGroupTitle(line string) string {
	match := groupTitleRegex.FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func extractChannelName(line string) string {
	if match := tvgNameRegex.FindStringSubmatch(line); len(match) > 1 && match[1] != "" {
		return match[1]
	}

	if idx := strings.LastIndex(line, ","); idx != -1 && idx < len(line)-1 {
		return strings.TrimSpace(line[idx+1:])
	}

	return "Canal sem nome"
}

func extractLogo(line string) string {
	match := tvgLogoRegex.FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func BuildXCPlaylistURL(dns, username, password string) string {
	base := strings.TrimRight(dns, "/")
	params := url.Values{}
	params.Set("username", username)
	params.Set("password", password)
	params.Set("type", "m3u_plus")
	params.Set("output", "mpegts")
	return base + "/get.php?" + params.Encode()
}

func ExtractCategories(channels []model.Channel) []string {
	seen := make(map[string]struct{})
	categories := make([]string, 0)

	for _, channel := range channels {
		if channel.Category == "" {
			continue
		}
		if _, exists := seen[channel.Category]; exists {
			continue
		}
		seen[channel.Category] = struct{}{}
		categories = append(categories, channel.Category)
	}

	return categories
}

func GenerateSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
