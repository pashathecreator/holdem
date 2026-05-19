package grpc

import (
	"sync"

	tablemanagerv1 "github.com/pashathecreator/holdem/services/table-manager/pkg/gen/go/table_manager/v1"
)

type subscriber struct {
	userID string
	ch     chan *tablemanagerv1.TableView
}

type Hub struct {
	mu   sync.RWMutex
	subs map[string][]subscriber
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string][]subscriber)}
}

func (h *Hub) Subscribe(tableID, userID string) chan *tablemanagerv1.TableView {
	ch := make(chan *tablemanagerv1.TableView, 16)
	h.mu.Lock()
	h.subs[tableID] = append(h.subs[tableID], subscriber{userID: userID, ch: ch})
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(tableID string, ch chan *tablemanagerv1.TableView) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.subs[tableID]
	for i, sub := range subs {
		if sub.ch == ch {
			h.subs[tableID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
}

func (h *Hub) Publish(tableID string, fn func(userID string) *tablemanagerv1.TableView) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, sub := range h.subs[tableID] {
		view := fn(sub.userID)
		select {
		case sub.ch <- view:
		default:
		}
	}
}
