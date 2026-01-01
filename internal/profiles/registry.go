package profiles

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

const HandlePrefix = "handle:"

type Metadata struct {
	ID           string `json:"id"`
	Service      string `json:"service,omitempty"`
	Env          string `json:"env,omitempty"`
	Type         string `json:"type,omitempty"`
	Timestamp    string `json:"timestamp,omitempty"`
	ProfileID    string `json:"profile_id,omitempty"`
	EventID      string `json:"event_id,omitempty"`
	Path         string `json:"path"`
	Bytes        int64  `json:"bytes,omitempty"`
	RegisteredAt string `json:"registered_at"`
}

type Registry struct {
	mu    sync.RWMutex
	items map[string]Metadata
}

func NewRegistry() *Registry {
	return &Registry{items: make(map[string]Metadata)}
}

func (r *Registry) Register(meta Metadata) (string, error) {
	if meta.Path == "" {
		return "", errors.New("profile path required")
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	meta.ID = id
	meta.RegisteredAt = time.Now().UTC().Format(time.RFC3339)

	r.mu.Lock()
	r.items[id] = meta
	r.mu.Unlock()

	return HandlePrefix + id, nil
}

func (r *Registry) Resolve(handle string) (Metadata, bool) {
	id := strings.TrimPrefix(handle, HandlePrefix)
	r.mu.RLock()
	meta, ok := r.items[id]
	r.mu.RUnlock()
	return meta, ok
}

func IsHandle(value string) bool {
	return strings.HasPrefix(value, HandlePrefix)
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
