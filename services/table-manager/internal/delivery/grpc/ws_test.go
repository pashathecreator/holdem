package grpc

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc/metadata"

	tablemanagerv1 "github.com/pashathecreator/holdem/services/table-manager/pkg/gen/go/table_manager/v1"
)

type snapshotServerStub struct {
	resp *tablemanagerv1.GetTableResponse
	ctx  context.Context
}

func (s *snapshotServerStub) GetTable(ctx context.Context, _ *tablemanagerv1.GetTableRequest) (*tablemanagerv1.GetTableResponse, error) {
	s.ctx = ctx
	return s.resp, nil
}

type snapshotReaderStub struct {
	data []byte
}

func (s *snapshotReaderStub) GetTable(_ *http.Request, _, _ string) ([]byte, error) {
	return s.data, nil
}

func TestHubPublishAndUnsubscribe(t *testing.T) {
	hub := NewHub()
	updates := hub.Subscribe("table-1", "viewer")
	hub.Publish("table-1", func(userID string) *tablemanagerv1.TableView {
		return &tablemanagerv1.TableView{Name: userID}
	})

	select {
	case view := <-updates:
		if view.Name != "viewer" {
			t.Fatalf("view.Name = %q, want viewer", view.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published update")
	}

	hub.Unsubscribe("table-1", updates)
	if _, ok := <-updates; ok {
		t.Fatal("expected closed channel after unsubscribe")
	}
}

func TestWSSnapshotAdapterUsesRequestContext(t *testing.T) {
	server := &snapshotServerStub{
		resp: &tablemanagerv1.GetTableResponse{
			Table: &tablemanagerv1.TableView{TableId: "table-1", Name: "Main"},
		},
	}
	adapter := NewWSSnapshotAdapter(server)
	req := httptest.NewRequest(http.MethodGet, "/v1/tables/table-1/ws", nil)
	req = req.WithContext(metadata.NewIncomingContext(req.Context(), metadata.Pairs(userIDMetadataKey, "p1")))

	data, err := adapter.GetTable(req, "table-1", "p1")
	if err != nil {
		t.Fatalf("GetTable() error = %v", err)
	}
	if !strings.Contains(string(data), `"tableId":"table-1"`) {
		t.Fatalf("snapshot = %s, want table id", data)
	}

	md, ok := metadata.FromIncomingContext(server.ctx)
	if !ok || len(md.Get(userIDMetadataKey)) == 0 || md.Get(userIDMetadataKey)[0] != "p1" {
		t.Fatalf("metadata = %+v, want x-user-id p1", md)
	}
}

func TestWSHandlerSendsSnapshotAndUpdates(t *testing.T) {
	hub := NewHub()
	handler := NewWSHandler(hub, &snapshotReaderStub{
		data: []byte(`{"tableId":"table-1","name":"snapshot"}`),
	})

	srv, ok := newWSTestServer(t, handler)
	if !ok {
		return
	}
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"X-User-Id": []string{"viewer"}})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = conn.Close() }()

	_, initial, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(initial) error = %v", err)
	}
	if !strings.Contains(string(initial), `"name":"snapshot"`) {
		t.Fatalf("initial message = %s, want snapshot", initial)
	}

	hub.Publish("table-1", func(userID string) *tablemanagerv1.TableView {
		return &tablemanagerv1.TableView{Name: userID}
	})

	_, update, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(update) error = %v", err)
	}
	if !strings.Contains(string(update), `"name":"viewer"`) {
		t.Fatalf("update message = %s, want viewer", update)
	}
}

func newWSTestServer(t *testing.T, handler *WSHandler) (_ *httptest.Server, ok bool) {
	t.Helper()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("websocket server test skipped: local listener unavailable in sandbox: %v", err)
		return nil, false
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("table_id", "table-1")
		handler.ServeHTTP(w, r)
	}))
	srv.Listener = ln
	srv.Start()
	return srv, true
}
