package grpc

import (
	"net/http"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/protojson"
)

type wsSnapshotReader interface {
	GetTable(r *http.Request, tableID, viewerID string) ([]byte, error)
}

type WSHandler struct {
	hub    *Hub
	reader wsSnapshotReader
}

func NewWSHandler(hub *Hub, reader wsSnapshotReader) *WSHandler {
	return &WSHandler{hub: hub, reader: reader}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("table_id")
	viewerID := r.Header.Get(userIDMetadataKey)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	initial, err := h.reader.GetTable(r, tableID, viewerID)
	if err == nil {
		if err := conn.WriteMessage(websocket.TextMessage, initial); err != nil {
			return
		}
	}

	updates := h.hub.Subscribe(tableID, viewerID)
	defer h.hub.Unsubscribe(tableID, updates)

	for {
		select {
		case <-r.Context().Done():
			return
		case view, ok := <-updates:
			if !ok {
				return
			}
			data, err := protojson.Marshal(view)
			if err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}
}
