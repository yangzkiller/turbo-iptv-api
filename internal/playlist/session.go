package playlist

import (
	// Standard packages
	"sync" // For thread-safe concurrent access to in-memory sessions

	// Internal packages
	"turbo-iptv-api/internal/model" // For channel list and session entry models
)

// Store struct for in-memory playlist sessions keyed by session ID
type Store struct {
	mu    sync.RWMutex           // Protects concurrent reads and writes to items
	items map[string]model.Entry // Session ID → parsed playlist content
}

// NewStore function to create an empty in-memory playlist session store
func NewStore() *Store {
	return &Store{items: make(map[string]model.Entry)}
}

// Save function to store parsed channels and return a new session ID and entry snapshot
func (s *Store) Save(channels []model.Channel) (string, model.Entry) {
	id := GenerateSessionID()
	series := GroupIntoSeries(channels)
	entry := model.Entry{
		Channels:   channels,
		Categories: ExtractCategories(channels),
		Series:     series,
	}
	s.mu.Lock()
	s.items[id] = entry
	s.mu.Unlock()
	return id, entry
}

// Get function to retrieve a playlist session entry by session ID
func (s *Store) Get(id string) (model.Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.items[id]
	return entry, ok
}
