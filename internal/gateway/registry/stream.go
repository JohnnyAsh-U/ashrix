package registry

import (
	"context"
	"sync"
)

type ActiveStreamRegistry struct {
	mu sync.Mutex
	streams map[string]map[string]context.CancelFunc
}


func NewActiveStreamRegistry() *ActiveStreamRegistry {
	return &ActiveStreamRegistry{
		streams: make(map[string]map[string]context.CancelFunc),
	}
}

func (r *ActiveStreamRegistry) Register(sessionID, streamID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.streams[sessionID]; !exists {
		r.streams[sessionID] = make(map[string]context.CancelFunc)
	}
	r.streams[sessionID][streamID] = cancel
}

func (r *ActiveStreamRegistry) Unregister(sessionID, streamID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if streams, exists := r.streams[sessionID]; exists {
		delete(streams, streamID)
		if len(streams) == 0 {
			delete(r.streams, sessionID)
		}
	}
}

func (r *ActiveStreamRegistry) RevokeSession(sessionID string) {
	r.mu.Lock()
	streams := r.streams[sessionID]

	delete(r.streams, sessionID)
	r.mu.Unlock()

	for _, cancel := range streams {
		cancel()
	}
}