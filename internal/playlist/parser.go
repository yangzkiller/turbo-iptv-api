package playlist

import (
	// Standard packages
	"crypto/rand"     // For secure random session ID generation
	"encoding/hex"    // For hexadecimal encoding
	"fmt"             // For formatted error strings
	"io"              // For reading HTTP response bodies
	"net/http"        // For downloading remote M3U playlists
	"net/url"         // For building Xtream Codes query strings
	"regexp"          // For parsing M3U EXTINF metadata
	"strings"         // For string manipulation
	"time"            // For HTTP client timeout and session ID fallback

	// Internal packages
	"turbo-iptv-api/internal/model" // For content type constants and channel models
)

// Compiled regex patterns for extracting metadata from M3U EXTINF lines
var (
	groupTitleRegex = regexp.MustCompile(`group-title="([^"]*)"`) // Category/group label
	tvgNameRegex    = regexp.MustCompile(`tvg-name="([^"]*)"`)    // Channel display name
	tvgLogoRegex    = regexp.MustCompile(`tvg-logo="([^"]*)"`)    // Channel logo URL
)

// ClassifyContentType function to infer content type from a stream URL path
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

// FetchM3U function to download a remote M3U playlist as plain text
func FetchM3U(playlistURL string) (string, error) {
	client := &http.Client{Timeout: 120 * time.Second}

	// Download playlist from the remote URL
	resp, err := client.Get(playlistURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("playlist retornou status %d", resp.StatusCode)
	}

	// Read full response body into memory
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// ParseM3U function to convert raw M3U text into a flat list of channels
func ParseM3U(content string) []model.Channel {
	lines := strings.Split(content, "\n")
	channels := make([]model.Channel, 0)

	// Metadata from the most recent EXTINF line, applied to the next stream URL
	var pendingName, pendingCategory, pendingLogo string

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		// Capture channel metadata from EXTINF header lines
		if strings.HasPrefix(line, "#EXTINF:") {
			pendingCategory = extractGroupTitle(line)
			pendingName = extractChannelName(line)
			pendingLogo = extractLogo(line)
			continue
		}

		// Skip other M3U comment/directive lines
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Non-comment line is treated as the stream URL for the pending EXTINF block
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

// extractGroupTitle function to read group-title from an EXTINF line
func extractGroupTitle(line string) string {
	match := groupTitleRegex.FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// extractChannelName function to read channel name from tvg-name or text after the comma
func extractChannelName(line string) string {
	if match := tvgNameRegex.FindStringSubmatch(line); len(match) > 1 && match[1] != "" {
		return match[1]
	}

	if idx := strings.LastIndex(line, ","); idx != -1 && idx < len(line)-1 {
		return strings.TrimSpace(line[idx+1:])
	}

	return "Canal sem nome"
}

// extractLogo function to read tvg-logo from an EXTINF line
func extractLogo(line string) string {
	match := tvgLogoRegex.FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// BuildXCPlaylistURL function to build an M3U download URL from Xtream Codes credentials
func BuildXCPlaylistURL(dns, username, password string) string {
	base := strings.TrimRight(dns, "/")
	params := url.Values{}
	params.Set("username", username)
	params.Set("password", password)
	params.Set("type", "m3u_plus")
	params.Set("output", "mpegts")
	return base + "/get.php?" + params.Encode()
}

// ExtractCategories function to collect unique non-empty categories from a channel list
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

// GenerateSessionID function to create a random identifier for an in-memory playlist session
func GenerateSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
