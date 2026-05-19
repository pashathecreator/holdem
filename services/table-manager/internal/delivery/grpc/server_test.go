package grpc

import (
	"context"
	"testing"

	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	applicationpkg "github.com/pashathecreator/holdem/services/table-manager/internal/application"
	"github.com/pashathecreator/holdem/services/table-manager/internal/domain"
	tablemanagerv1 "github.com/pashathecreator/holdem/services/table-manager/pkg/gen/go/table_manager/v1"
)

type repoStub struct {
	tables map[string]*domain.Table
}

func (r *repoStub) CreateTable(_ context.Context, table *domain.Table) error {
	r.tables[table.ID] = cloneDomainTable(table)
	return nil
}

func (r *repoStub) SaveTable(_ context.Context, table *domain.Table) error {
	r.tables[table.ID] = cloneDomainTable(table)
	return nil
}

func (r *repoStub) FindTable(_ context.Context, tableID string) (*domain.Table, error) {
	table, ok := r.tables[tableID]
	if !ok {
		return nil, domain.ErrTableNotFound
	}
	return cloneDomainTable(table), nil
}

func (r *repoStub) ListTables(_ context.Context) ([]*domain.Table, error) {
	result := make([]*domain.Table, 0, len(r.tables))
	for _, table := range r.tables {
		result = append(result, cloneDomainTable(table))
	}
	return result, nil
}

type engineStub struct {
	startResp *enginev1.StartHandResponse
	applyResp *enginev1.ApplyActionResponse
	getResp   *enginev1.GetGameStateResponse
}

func (e *engineStub) StartHand(_ context.Context, _ *enginev1.StartHandRequest, _ ...grpc.CallOption) (*enginev1.StartHandResponse, error) {
	if e.startResp != nil {
		return e.startResp, nil
	}
	return &enginev1.StartHandResponse{
		State: &enginev1.GameState{Id: "hand-1", Street: enginev1.Street_STREET_PREFLOP},
	}, nil
}

func (e *engineStub) ApplyAction(_ context.Context, _ *enginev1.ApplyActionRequest, _ ...grpc.CallOption) (*enginev1.ApplyActionResponse, error) {
	return e.applyResp, nil
}

func (e *engineStub) GetGameState(_ context.Context, _ *enginev1.GetGameStateRequest, _ ...grpc.CallOption) (*enginev1.GetGameStateResponse, error) {
	return e.getResp, nil
}

func TestServerJoinTableMapsNotFoundToGRPCCode(t *testing.T) {
	service := applicationpkg.NewService(&repoStub{tables: map[string]*domain.Table{}}, &engineStub{}, nil)
	server := NewServer(service, NewHub(), NewAuthenticator(nil, true))

	_, err := server.JoinTable(withUserID("p1"), &tablemanagerv1.JoinTableRequest{
		TableId:   "missing",
		SeatIndex: 0,
		BuyIn:     1000,
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status.Code(err) = %v, want %v", status.Code(err), codes.NotFound)
	}
}

func TestServerCreateTableRequiresAuthentication(t *testing.T) {
	service := applicationpkg.NewService(&repoStub{tables: map[string]*domain.Table{}}, &engineStub{}, nil)
	server := NewServer(service, NewHub(), NewAuthenticator(nil, false))

	_, err := server.CreateTable(context.Background(), &tablemanagerv1.CreateTableRequest{
		Name:       "Main",
		SeatCount:  2,
		SmallBlind: 50,
		BigBlind:   100,
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("status.Code(err) = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestServerActMapsSpectatorErrorToInvalidArgument(t *testing.T) {
	repo := &repoStub{tables: map[string]*domain.Table{
		"table-1": {
			ID:           "table-1",
			Name:         "Main",
			SeatCount:    2,
			Status:       domain.TableStatusInHand,
			ActiveHandID: "hand-1",
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	service := applicationpkg.NewService(repo, &engineStub{}, nil)
	server := NewServer(service, NewHub(), NewAuthenticator(nil, true))

	_, err := server.Act(withUserID("spectator"), &tablemanagerv1.ActRequest{
		TableId: "table-1",
		Action:  &tablemanagerv1.Action{Type: tablemanagerv1.ActionType_ACTION_TYPE_CHECK},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status.Code(err) = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}

func TestServerGetTableAllowsAnonymousViewer(t *testing.T) {
	repo := &repoStub{tables: map[string]*domain.Table{
		"table-1": {
			ID:        "table-1",
			Name:      "Main",
			SeatCount: 2,
			Status:    domain.TableStatusIdle,
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1},
			},
		},
	}}
	service := applicationpkg.NewService(repo, &engineStub{}, nil)
	server := NewServer(service, NewHub(), NewAuthenticator(nil, false))

	resp, err := server.GetTable(context.Background(), &tablemanagerv1.GetTableRequest{TableId: "table-1"})
	if err != nil {
		t.Fatalf("GetTable() error = %v", err)
	}
	if resp.Table.Seats[0].IsViewer {
		t.Fatalf("anonymous viewer unexpectedly marked as viewer")
	}
}

func TestServerJoinTablePublishesViewerUpdate(t *testing.T) {
	repo := &repoStub{tables: map[string]*domain.Table{
		"table-1": {
			ID:         "table-1",
			Name:       "Main",
			SeatCount:  2,
			Status:     domain.TableStatusIdle,
			SmallBlind: 50,
			BigBlind:   100,
			Seats: []domain.Seat{
				{Index: 0},
				{Index: 1},
			},
		},
	}}
	service := applicationpkg.NewService(repo, &engineStub{}, nil)
	hub := NewHub()
	server := NewServer(service, hub, NewAuthenticator(nil, true))
	updates := hub.Subscribe("table-1", "p1")
	defer hub.Unsubscribe("table-1", updates)

	resp, err := server.JoinTable(withUserID("p1"), &tablemanagerv1.JoinTableRequest{
		TableId:   "table-1",
		SeatIndex: 0,
		BuyIn:     1000,
	})
	if err != nil {
		t.Fatalf("JoinTable() error = %v", err)
	}
	if resp.Table == nil || resp.Table.Seats[0].PlayerId != "p1" {
		t.Fatalf("response table = %+v, want p1 seated", resp.Table)
	}

	update := <-updates
	if !update.Seats[0].IsViewer {
		t.Fatalf("published update IsViewer = false, want true")
	}
}

func TestServerPublishTableFetchesStateWhenMissing(t *testing.T) {
	repo := &repoStub{tables: map[string]*domain.Table{
		"table-1": {
			ID:           "table-1",
			Name:         "Main",
			SeatCount:    2,
			Status:       domain.TableStatusInHand,
			ActiveHandID: "hand-1",
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	engine := &engineStub{
		getResp: &enginev1.GetGameStateResponse{
			State: &enginev1.GameState{
				Id: "hand-1",
				Players: []*enginev1.Player{
					{Id: "p1", HoleCards: []*enginev1.Card{{Value: "A♠"}, {Value: "A♥"}}},
					{Id: "p2", HoleCards: []*enginev1.Card{{Value: "K♠"}, {Value: "K♥"}}},
				},
			},
		},
	}
	service := applicationpkg.NewService(repo, engine, nil)
	hub := NewHub()
	server := NewServer(service, hub, NewAuthenticator(nil, true))
	updates := hub.Subscribe("table-1", "p1")
	defer hub.Unsubscribe("table-1", updates)

	server.publishTable(context.Background(), repo.tables["table-1"], nil)

	update := <-updates
	if update.Hand == nil || update.Hand.HandId != "hand-1" {
		t.Fatalf("published hand = %+v, want hand-1", update.Hand)
	}
	if len(update.Seats[0].HoleCards) != 2 {
		t.Fatalf("viewer hole cards = %d, want 2", len(update.Seats[0].HoleCards))
	}
}

func withUserID(userID string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(userIDMetadataKey, userID))
}

func cloneDomainTable(table *domain.Table) *domain.Table {
	seats := make([]domain.Seat, len(table.Seats))
	copy(seats, table.Seats)
	return &domain.Table{
		ID:           table.ID,
		Name:         table.Name,
		SeatCount:    table.SeatCount,
		Status:       table.Status,
		SmallBlind:   table.SmallBlind,
		BigBlind:     table.BigBlind,
		Button:       table.Button,
		ActiveHandID: table.ActiveHandID,
		Seats:        seats,
	}
}
