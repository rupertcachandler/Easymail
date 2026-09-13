package client

import (
	"context"
	"sync"
)

// PolicyStore persists the current EAS provisioning policy key. The empty
// string means no policy has been negotiated yet; callers should treat that
// as the "0" policy key on the wire.
type PolicyStore interface {
	Get(ctx context.Context) (string, error)
	Set(ctx context.Context, policyKey string) error
}

// SyncStateStore persists the per-collection SyncKey returned by the server.
// The default value for an unknown collection is "0", which the EAS server
// recognises as a request for an initial sync.
type SyncStateStore interface {
	Get(ctx context.Context, collectionID string) (string, error)
	Set(ctx context.Context, collectionID, syncKey string) error
}

// InMemoryPolicyStore is a process-local PolicyStore. It is safe for
// concurrent use and resets when the process terminates.
type InMemoryPolicyStore struct {
	mu  sync.RWMutex
	key string
}

// NewInMemoryPolicyStore returns a fresh in-memory PolicyStore.
func NewInMemoryPolicyStore() *InMemoryPolicyStore { return &InMemoryPolicyStore{} }

// Get returns the cached policy key.
func (s *InMemoryPolicyStore) Get(_ context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.key, nil
}

// Set replaces the cached policy key.
func (s *InMemoryPolicyStore) Set(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
	return nil
}

// InMemorySyncStateStore is a process-local SyncStateStore.
type InMemorySyncStateStore struct {
	mu   sync.RWMutex
	keys map[string]string
}

// NewInMemorySyncStateStore returns a fresh in-memory SyncStateStore.
func NewInMemorySyncStateStore() *InMemorySyncStateStore {
	return &InMemorySyncStateStore{keys: map[string]string{}}
}

// Get returns the SyncKey for the collection, or "0" if unknown.
func (s *InMemorySyncStateStore) Get(_ context.Context, collectionID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if k, ok := s.keys[collectionID]; ok {
		return k, nil
	}
	return "0", nil
}

// Set stores the SyncKey for the collection.
func (s *InMemorySyncStateStore) Set(_ context.Context, collectionID, syncKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[collectionID] = syncKey
	return nil
}
