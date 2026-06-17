package model

// Content type constants used across playlist parsing, filtering and API queries.
const (
	ContentLive   = "live"   // Live TV channels
	ContentMovie  = "movie"  // Movies (VOD)
	ContentSeries = "series" // Series with episodes
)

// XCRequest struct for the JSON body of the Xtream Codes connect endpoint
type XCRequest struct {
	DNS      string `json:"dns"`      // Xtream Codes server URL
	Username string `json:"username"` // Xtream Codes username
	Password string `json:"password"` // Xtream Codes password
}

// Channel struct for a single playlist item (live, movie or series episode)
type Channel struct {
	Name     string `json:"name"`     // Display name
	Category string `json:"category"` // Group or genre label
	Logo     string `json:"logo"`     // Optional logo URL
	URL      string `json:"url"`      // Stream URL
	Type     string `json:"type"`     // Content type: live, movie or series
}

// Episode struct for a single episode inside a series grouping
type Episode struct {
	Name    string `json:"name"`    // Episode display name
	Season  int    `json:"season"`  // Season number
	Episode int    `json:"episode"` // Episode number within the season
	Logo    string `json:"logo"`    // Optional episode logo URL
	URL     string `json:"url"`     // Stream URL
}

// SeriesItem struct for a series grouped from individual series-type channels
type SeriesItem struct {
	Name         string    `json:"name"`         // Series display name
	Category     string    `json:"category"`     // Group or genre label
	Logo         string    `json:"logo"`         // Optional series logo URL
	Type         string    `json:"type"`         // Always series for grouped items
	EpisodeCount int       `json:"episodeCount"` // Total number of episodes
	Episodes     []Episode `json:"episodes"`     // Episodes belonging to this series
}

// Entry struct for an in-memory playlist session stored in the playlist Store
type Entry struct {
	Channels   []Channel    // Flat list of all parsed channels
	Categories []string     // Unique categories extracted from channels
	Series     []SeriesItem // Series grouped from series-type channels
}
