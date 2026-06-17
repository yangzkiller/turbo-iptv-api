package playlist

import (
	"sync"

	"turbo-iptv-api/internal/model"
)

type Store struct {
	mu    sync.RWMutex
	items map[string]model.Entry
}

func NewStore() *Store {
	return &Store{items: make(map[string]model.Entry)}
}

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

func (s *Store) Get(id string) (model.Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.items[id]
	return entry, ok
}
