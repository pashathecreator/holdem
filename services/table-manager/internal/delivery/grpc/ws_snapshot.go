package grpc

import (
	"context"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"

	tablemanagerv1 "github.com/pashathecreator/holdem/services/table-manager/pkg/gen/go/table_manager/v1"
)

type snapshotServer interface {
	GetTable(context.Context, *tablemanagerv1.GetTableRequest) (*tablemanagerv1.GetTableResponse, error)
}

type wsSnapshotAdapter struct {
	server snapshotServer
}

func NewWSSnapshotAdapter(server snapshotServer) *wsSnapshotAdapter {
	return &wsSnapshotAdapter{server: server}
}

func (a *wsSnapshotAdapter) GetTable(r *http.Request, tableID, _ string) ([]byte, error) {
	resp, err := a.server.GetTable(r.Context(), &tablemanagerv1.GetTableRequest{TableId: tableID})
	if err != nil {
		return nil, err
	}
	return protojson.Marshal(resp.GetTable())
}
