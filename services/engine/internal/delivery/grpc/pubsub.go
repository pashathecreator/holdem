package grpc

import (
	"sync"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
)

type PubSub struct {
	mu          sync.RWMutex
	subscribers map[domain.HandID][]chan *domain.GameState
}

func NewPubSub() *PubSub {
	return &PubSub{
		subscribers: make(map[domain.HandID][]chan *domain.GameState),
	}
}

func (ps *PubSub) Subscribe(handID domain.HandID) chan *domain.GameState {
	ch := make(chan *domain.GameState, 16)

	ps.mu.Lock()
	ps.subscribers[handID] = append(ps.subscribers[handID], ch)
	ps.mu.Unlock()

	return ch
}

func (ps *PubSub) Unsubscribe(handID domain.HandID, ch chan *domain.GameState) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	subs := ps.subscribers[handID]
	for i, sub := range subs {
		if sub == ch {
			ps.subscribers[handID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
}

func (ps *PubSub) Publish(handID domain.HandID, state *domain.GameState) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	for _, ch := range ps.subscribers[handID] {
		select {
		case ch <- state:
		default:
		}
	}
}