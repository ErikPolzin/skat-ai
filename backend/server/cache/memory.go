package cache

import (
	"context"
	"sync"
	"time"

	"skat/game"
)

type MemoryBackend struct {
	mu          sync.RWMutex
	items       map[string]memoryItem
	revisions   map[string]int64
	presence    map[string]memoryPresence
	queue       chan game.GameState
	subscribers map[int]chan []byte
	nextSubID   int
	closed      chan struct{}
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

type memoryPresence struct {
	nodeID    string
	expiresAt time.Time
}

func NewMemoryBackend(queueSize int) *MemoryBackend {
	if queueSize <= 0 {
		queueSize = 1024
	}
	return &MemoryBackend{
		items:       make(map[string]memoryItem),
		revisions:   make(map[string]int64),
		presence:    make(map[string]memoryPresence),
		queue:       make(chan game.GameState, queueSize),
		subscribers: make(map[int]chan []byte),
		closed:      make(chan struct{}),
	}
}

func (m *MemoryBackend) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	item, ok := m.items[key]
	m.mu.RUnlock()
	if !ok || (!item.expiresAt.IsZero() && time.Now().After(item.expiresAt)) {
		if ok {
			m.mu.Lock()
			delete(m.items, key)
			m.mu.Unlock()
		}
		return nil, ErrMiss
	}
	value := make([]byte, len(item.value))
	copy(value, item.value)
	return value, nil
}

func (m *MemoryBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	item := memoryItem{value: append([]byte(nil), value...)}
	if ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.items[key] = item
	m.mu.Unlock()
	return nil
}

func (m *MemoryBackend) NextRevision(ctx context.Context, gameID string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revisions[gameID]++
	return m.revisions[gameID], nil
}

func (m *MemoryBackend) EnqueueGameSave(ctx context.Context, gs game.GameState) error {
	select {
	case m.queue <- gs:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-m.closed:
		return ctx.Err()
	}
}

func (m *MemoryBackend) DequeueGameSave(ctx context.Context) (*game.GameState, error) {
	select {
	case gs := <-m.queue:
		return &gs, nil
	case <-time.After(500 * time.Millisecond):
		return nil, ErrQueueEmpty
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.closed:
		return nil, ctx.Err()
	}
}

func (m *MemoryBackend) MarkOnline(ctx context.Context, profileID, nodeID string, ttl time.Duration) error {
	presence := memoryPresence{nodeID: nodeID}
	if ttl > 0 {
		presence.expiresAt = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.presence[profileID] = presence
	m.mu.Unlock()
	return nil
}

func (m *MemoryBackend) MarkOffline(ctx context.Context, profileID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.presence[profileID]
	if ok && current.nodeID != "" && current.nodeID != nodeID {
		return nil
	}
	delete(m.presence, profileID)
	return nil
}

func (m *MemoryBackend) IsOnline(ctx context.Context, profileID string) (bool, error) {
	m.mu.RLock()
	presence, ok := m.presence[profileID]
	m.mu.RUnlock()
	if !ok {
		return false, nil
	}
	if !presence.expiresAt.IsZero() && time.Now().After(presence.expiresAt) {
		m.mu.Lock()
		delete(m.presence, profileID)
		m.mu.Unlock()
		return false, nil
	}
	return true, nil
}

func (m *MemoryBackend) OnlineIDs(ctx context.Context) ([]string, error) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.presence))
	for profileID, presence := range m.presence {
		if !presence.expiresAt.IsZero() && now.After(presence.expiresAt) {
			delete(m.presence, profileID)
			continue
		}
		ids = append(ids, profileID)
	}
	return ids, nil
}

func (m *MemoryBackend) PublishClientMessage(ctx context.Context, payload []byte) error {
	m.mu.RLock()
	subscribers := make([]chan []byte, 0, len(m.subscribers))
	for _, subscriber := range m.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	m.mu.RUnlock()

	for _, subscriber := range subscribers {
		message := append([]byte(nil), payload...)
		select {
		case subscriber <- message:
		default:
		}
	}
	return nil
}

func (m *MemoryBackend) SubscribeClientMessages(ctx context.Context) (<-chan []byte, error) {
	ch := make(chan []byte, 64)
	m.mu.Lock()
	id := m.nextSubID
	m.nextSubID++
	m.subscribers[id] = ch
	m.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
		case <-m.closed:
		}
		m.mu.Lock()
		if subscriber, ok := m.subscribers[id]; ok {
			delete(m.subscribers, id)
			close(subscriber)
		}
		m.mu.Unlock()
	}()
	return ch, nil
}

func (m *MemoryBackend) Close() error {
	select {
	case <-m.closed:
	default:
		m.mu.Lock()
		for id, subscriber := range m.subscribers {
			delete(m.subscribers, id)
			close(subscriber)
		}
		m.mu.Unlock()
		close(m.closed)
	}
	return nil
}
