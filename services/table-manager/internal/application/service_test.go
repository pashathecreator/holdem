package application

import (
	"context"
	"errors"
	"testing"

	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
	"google.golang.org/grpc"

	"github.com/pashathecreator/holdem/services/table-manager/internal/domain"
)

type stubRepo struct {
	tables map[string]*domain.Table
}

func (r *stubRepo) CreateTable(_ context.Context, table *domain.Table) error {
	r.tables[table.ID] = cloneTable(table)
	return nil
}

func (r *stubRepo) SaveTable(_ context.Context, table *domain.Table) error {
	r.tables[table.ID] = cloneTable(table)
	return nil
}

func (r *stubRepo) FindTable(_ context.Context, tableID string) (*domain.Table, error) {
	table, ok := r.tables[tableID]
	if !ok {
		return nil, domain.ErrTableNotFound
	}
	return cloneTable(table), nil
}

func (r *stubRepo) ListTables(_ context.Context) ([]*domain.Table, error) {
	result := make([]*domain.Table, 0, len(r.tables))
	for _, table := range r.tables {
		result = append(result, cloneTable(table))
	}
	return result, nil
}

type stubEngine struct {
	started      int
	lastStartReq *enginev1.StartHandRequest
	lastApplyReq *enginev1.ApplyActionRequest
	lastGetReq   *enginev1.GetGameStateRequest
	startResp    *enginev1.StartHandResponse
	startErr     error
	applyResp    *enginev1.ApplyActionResponse
	applyErr     error
	getResp      *enginev1.GetGameStateResponse
	getErr       error
}

func (e *stubEngine) StartHand(_ context.Context, in *enginev1.StartHandRequest, _ ...grpc.CallOption) (*enginev1.StartHandResponse, error) {
	e.started++
	e.lastStartReq = in
	if e.startErr != nil {
		return nil, e.startErr
	}
	if e.startResp != nil {
		return e.startResp, nil
	}
	players := make([]*enginev1.Player, len(in.Players))
	for i, player := range in.Players {
		players[i] = &enginev1.Player{
			Id:       player.Id,
			Stack:    player.Stack,
			Position: player.Position,
			Status:   enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE,
		}
	}
	return &enginev1.StartHandResponse{
		State: &enginev1.GameState{
			Id:         "hand-1",
			TableId:    in.TableId,
			Players:    players,
			Street:     enginev1.Street_STREET_PREFLOP,
			SmallBlind: in.SmallBlind,
			BigBlind:   in.BigBlind,
			Button:     in.Button,
		},
	}, nil
}

func (e *stubEngine) ApplyAction(_ context.Context, in *enginev1.ApplyActionRequest, _ ...grpc.CallOption) (*enginev1.ApplyActionResponse, error) {
	e.lastApplyReq = in
	if e.applyErr != nil {
		return nil, e.applyErr
	}
	return e.applyResp, nil
}

func (e *stubEngine) GetGameState(_ context.Context, in *enginev1.GetGameStateRequest, _ ...grpc.CallOption) (*enginev1.GetGameStateResponse, error) {
	e.lastGetReq = in
	if e.getErr != nil {
		return nil, e.getErr
	}
	return e.getResp, nil
}

type stubWallet struct {
	lastDebit  *walletCall
	lastCredit *walletCall
	debitErr   error
	creditErr  error
}

type walletCall struct {
	userID         string
	tableID        string
	amountGwei     int64
	idempotencyKey string
}

func (w *stubWallet) DebitForJoin(_ context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error {
	w.lastDebit = &walletCall{userID: userID, tableID: tableID, amountGwei: amountGwei, idempotencyKey: idempotencyKey}
	return w.debitErr
}

func (w *stubWallet) CreditForCashout(_ context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error {
	w.lastCredit = &walletCall{userID: userID, tableID: tableID, amountGwei: amountGwei, idempotencyKey: idempotencyKey}
	return w.creditErr
}

func TestCreateTableRejectsInvalidConfig(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{}}
	service := NewService(repo, &stubEngine{}, &stubWallet{})

	if _, err := service.CreateTable(context.Background(), "bad", 1, 50, 100); err != domain.ErrInvalidTableConfig {
		t.Fatalf("CreateTable() error = %v, want %v", err, domain.ErrInvalidTableConfig)
	}
	if len(repo.tables) != 0 {
		t.Fatalf("expected no tables to be created, got %d", len(repo.tables))
	}
}

func TestCreateTableInitializesSeats(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{}}
	service := NewService(repo, &stubEngine{}, &stubWallet{})

	table, err := service.CreateTable(context.Background(), "Main", 4, 50, 100)
	if err != nil {
		t.Fatalf("CreateTable() error = %v", err)
	}
	if table.Status != domain.TableStatusIdle {
		t.Fatalf("table status = %s, want idle", table.Status)
	}
	if len(table.Seats) != 4 {
		t.Fatalf("seat count = %d, want 4", len(table.Seats))
	}
	for i, seat := range table.Seats {
		if seat.Index != i {
			t.Fatalf("seat[%d].Index = %d, want %d", i, seat.Index, i)
		}
		if seat.PlayerID != "" || seat.Stack != 0 {
			t.Fatalf("seat[%d] = %+v, want empty seat", i, seat)
		}
	}
}

func TestGetTableFetchesGameStateForActiveHand(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:           "table-1",
			Name:         "Test",
			SeatCount:    2,
			Status:       domain.TableStatusInHand,
			ActiveHandID: "hand-1",
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	engine := &stubEngine{
		getResp: &enginev1.GetGameStateResponse{
			State: &enginev1.GameState{Id: "hand-1", Street: enginev1.Street_STREET_TURN},
		},
	}
	service := NewService(repo, engine, &stubWallet{})

	table, state, err := service.GetTable(context.Background(), "table-1")
	if err != nil {
		t.Fatalf("GetTable() error = %v", err)
	}
	if table.ActiveHandID != "hand-1" {
		t.Fatalf("table.ActiveHandID = %q, want hand-1", table.ActiveHandID)
	}
	if state == nil || state.Id != "hand-1" {
		t.Fatalf("state = %+v, want hand-1", state)
	}
	if engine.lastGetReq == nil || engine.lastGetReq.HandId != "hand-1" {
		t.Fatalf("GetGameState request = %+v, want hand-1", engine.lastGetReq)
	}
}

func TestJoinTableAutoStartsHandAtSecondPlayer(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{}}
	engine := &stubEngine{}
	wallet := &stubWallet{}
	service := NewService(repo, engine, wallet)

	table := &domain.Table{
		ID:         "table-1",
		Name:       "Test",
		SeatCount:  6,
		Status:     domain.TableStatusIdle,
		SmallBlind: 50,
		BigBlind:   100,
		Seats: []domain.Seat{
			{Index: 0},
			{Index: 1},
			{Index: 2},
			{Index: 3},
			{Index: 4},
			{Index: 5},
		},
	}
	repo.tables[table.ID] = cloneTable(table)

	if _, _, err := service.JoinTable(context.Background(), table.ID, "p1", 0, 1000); err != nil {
		t.Fatalf("join first player: %v", err)
	}
	if wallet.lastDebit == nil || wallet.lastDebit.amountGwei != 1000 {
		t.Fatalf("wallet debit = %+v, want 1000", wallet.lastDebit)
	}
	if engine.started != 0 {
		t.Fatalf("engine started = %d, want 0 after first player", engine.started)
	}

	joinedTable, state, err := service.JoinTable(context.Background(), table.ID, "p2", 1, 1000)
	if err != nil {
		t.Fatalf("join second player: %v", err)
	}
	if engine.started != 1 {
		t.Fatalf("engine started = %d, want 1", engine.started)
	}
	if joinedTable.Status != domain.TableStatusInHand {
		t.Fatalf("table status = %s, want in_hand", joinedTable.Status)
	}
	if joinedTable.ActiveHandID == "" {
		t.Fatalf("active hand id is empty")
	}
	if state == nil || state.Id == "" {
		t.Fatalf("expected engine state after autostart")
	}
}

func TestJoinTableRejectsOccupiedSeatAndDuplicatePlayer(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:         "table-1",
			Name:       "Test",
			SeatCount:  2,
			Status:     domain.TableStatusIdle,
			SmallBlind: 50,
			BigBlind:   100,
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1},
			},
		},
	}}
	service := NewService(repo, &stubEngine{}, &stubWallet{})

	if _, _, err := service.JoinTable(context.Background(), "table-1", "p2", 0, 1000); err != domain.ErrSeatOccupied {
		t.Fatalf("JoinTable() occupied seat error = %v, want %v", err, domain.ErrSeatOccupied)
	}
	if _, _, err := service.JoinTable(context.Background(), "table-1", "p1", 1, 1000); err != domain.ErrPlayerAlreadySeated {
		t.Fatalf("JoinTable() duplicate player error = %v, want %v", err, domain.ErrPlayerAlreadySeated)
	}
}

func TestJoinTableRejectsWhenWalletDebitFails(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:         "table-1",
			Name:       "Test",
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
	wallet := &stubWallet{debitErr: domain.ErrInsufficientFunds}
	service := NewService(repo, &stubEngine{}, wallet)

	_, _, err := service.JoinTable(context.Background(), "table-1", "p1", 0, 1000)
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("JoinTable() error = %v, want %v", err, domain.ErrInsufficientFunds)
	}
	if repo.tables["table-1"].SeatByIndex(0).Occupied() {
		t.Fatalf("seat was occupied despite failed debit: %+v", repo.tables["table-1"].SeatByIndex(0))
	}
}

func TestLeaveTableClearsSeatWhenIdle(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:        "table-1",
			Name:      "Test",
			SeatCount: 2,
			Status:    domain.TableStatusIdle,
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	wallet := &stubWallet{}
	service := NewService(repo, &stubEngine{}, wallet)

	table, state, err := service.LeaveTable(context.Background(), "table-1", "p1")
	if err != nil {
		t.Fatalf("LeaveTable() error = %v", err)
	}
	if state != nil {
		t.Fatalf("LeaveTable() state = %+v, want nil", state)
	}
	seat := table.SeatByIndex(0)
	if seat == nil || seat.PlayerID != "" || seat.Stack != 0 {
		t.Fatalf("seat after leave = %+v, want empty seat", seat)
	}
	if wallet.lastCredit == nil || wallet.lastCredit.amountGwei != 1000 {
		t.Fatalf("wallet credit = %+v, want 1000", wallet.lastCredit)
	}
}

func TestLeaveTableRejectedDuringActiveHand(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:           "table-1",
			Name:         "Test",
			SeatCount:    2,
			Status:       domain.TableStatusInHand,
			ActiveHandID: "hand-1",
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	service := NewService(repo, &stubEngine{}, &stubWallet{})

	_, _, err := service.LeaveTable(context.Background(), "table-1", "p1")
	if err != domain.ErrLeaveDuringActiveHand {
		t.Fatalf("LeaveTable() error = %v, want %v", err, domain.ErrLeaveDuringActiveHand)
	}
}

func TestActRejectsWithoutActiveHand(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:        "table-1",
			Name:      "Test",
			SeatCount: 2,
			Status:    domain.TableStatusIdle,
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	service := NewService(repo, &stubEngine{}, &stubWallet{})

	_, _, err := service.Act(context.Background(), "table-1", "p1", enginev1.ActionType_ACTION_TYPE_CHECK, 0)
	if err != domain.ErrActiveHandRequired {
		t.Fatalf("Act() error = %v, want %v", err, domain.ErrActiveHandRequired)
	}
}

func TestActRejectsSpectator(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:           "table-1",
			Name:         "Test",
			SeatCount:    2,
			Status:       domain.TableStatusInHand,
			ActiveHandID: "hand-1",
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	service := NewService(repo, &stubEngine{}, &stubWallet{})

	_, _, err := service.Act(context.Background(), "table-1", "spectator", enginev1.ActionType_ACTION_TYPE_CHECK, 0)
	if err != domain.ErrSpectatorCannotAct {
		t.Fatalf("Act() error = %v, want %v", err, domain.ErrSpectatorCannotAct)
	}
}

func TestActShowdownSyncsStacksAdvancesButtonAndAutoStartsNextHand(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:           "table-1",
			Name:         "Test",
			SeatCount:    2,
			Status:       domain.TableStatusInHand,
			Button:       0,
			ActiveHandID: "hand-1",
			SmallBlind:   50,
			BigBlind:     100,
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	engine := &stubEngine{
		applyResp: &enginev1.ApplyActionResponse{
			State: &enginev1.GameState{
				Id:     "hand-1",
				Street: enginev1.Street_STREET_SHOWDOWN,
				Players: []*enginev1.Player{
					{Id: "p1", Stack: 1200, Position: 0, Status: enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE},
					{Id: "p2", Stack: 800, Position: 1, Status: enginev1.PlayerStatus_PLAYER_STATUS_FOLDED},
				},
			},
		},
		startResp: &enginev1.StartHandResponse{
			State: &enginev1.GameState{
				Id:         "hand-2",
				TableId:    "table-1",
				Street:     enginev1.Street_STREET_PREFLOP,
				Button:     1,
				SmallBlind: 50,
				BigBlind:   100,
				Players: []*enginev1.Player{
					{Id: "p1", Stack: 1200, Position: 0, Status: enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE},
					{Id: "p2", Stack: 800, Position: 1, Status: enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE},
				},
			},
		},
	}
	service := NewService(repo, engine, &stubWallet{})

	table, state, err := service.Act(context.Background(), "table-1", "p1", enginev1.ActionType_ACTION_TYPE_CHECK, 0)
	if err != nil {
		t.Fatalf("Act() error = %v", err)
	}
	if engine.lastApplyReq == nil || engine.lastApplyReq.HandId != "hand-1" {
		t.Fatalf("ApplyAction request = %+v, want hand-1", engine.lastApplyReq)
	}
	if engine.started != 1 {
		t.Fatalf("engine.started = %d, want 1", engine.started)
	}
	if table.ActiveHandID != "hand-2" {
		t.Fatalf("table.ActiveHandID = %q, want hand-2", table.ActiveHandID)
	}
	if table.Status != domain.TableStatusInHand {
		t.Fatalf("table.Status = %s, want in_hand", table.Status)
	}
	if table.Button != 1 {
		t.Fatalf("table.Button = %d, want 1", table.Button)
	}
	if table.SeatByPlayerID("p1").Stack != 1200 || table.SeatByPlayerID("p2").Stack != 800 {
		t.Fatalf("seat stacks = (%d,%d), want (1200,800)", table.SeatByPlayerID("p1").Stack, table.SeatByPlayerID("p2").Stack)
	}
	if state == nil || state.Id != "hand-2" {
		t.Fatalf("returned state = %+v, want hand-2", state)
	}
	if engine.lastStartReq == nil || engine.lastStartReq.Button != 1 {
		t.Fatalf("StartHand request = %+v, want button 1", engine.lastStartReq)
	}
}

func TestActShowdownAutoUnseatsBustedPlayer(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:           "table-1",
			Name:         "Test",
			SeatCount:    2,
			Status:       domain.TableStatusInHand,
			Button:       0,
			ActiveHandID: "hand-1",
			SmallBlind:   50,
			BigBlind:     100,
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	engine := &stubEngine{
		applyResp: &enginev1.ApplyActionResponse{
			State: &enginev1.GameState{
				Id:     "hand-1",
				Street: enginev1.Street_STREET_SHOWDOWN,
				Players: []*enginev1.Player{
					{Id: "p1", Stack: 2000, Position: 0, Status: enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE},
					{Id: "p2", Stack: 0, Position: 1, Status: enginev1.PlayerStatus_PLAYER_STATUS_FOLDED},
				},
			},
		},
	}
	service := NewService(repo, engine, &stubWallet{})

	table, state, err := service.Act(context.Background(), "table-1", "p1", enginev1.ActionType_ACTION_TYPE_CHECK, 0)
	if err != nil {
		t.Fatalf("Act() error = %v", err)
	}
	if state == nil || state.Street != enginev1.Street_STREET_SHOWDOWN {
		t.Fatalf("state = %+v, want showdown state", state)
	}
	if table.SeatByIndex(1).PlayerID != "" || table.SeatByIndex(1).Stack != 0 {
		t.Fatalf("busted player seat = %+v, want empty zero-stack seat", table.SeatByIndex(1))
	}
	if table.Status != domain.TableStatusIdle {
		t.Fatalf("table.Status = %s, want idle", table.Status)
	}
}

func TestPersistenceSequenceCreateJoinGetLeaveGetListPreservesTableState(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{}}
	engine := &stubEngine{
		getResp: &enginev1.GetGameStateResponse{
			State: &enginev1.GameState{
				Id:     "hand-1",
				Street: enginev1.Street_STREET_PREFLOP,
			},
		},
	}
	service := NewService(repo, engine, &stubWallet{})

	created, err := service.CreateTable(context.Background(), "Main", 3, 50, 100)
	if err != nil {
		t.Fatalf("CreateTable() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("created table id is empty")
	}

	storedAfterCreate, state, err := service.GetTable(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetTable(after create) error = %v", err)
	}
	if state != nil {
		t.Fatalf("GetTable(after create) state = %+v, want nil", state)
	}
	if len(storedAfterCreate.Seats) != 3 || storedAfterCreate.SeatByIndex(2) == nil {
		t.Fatalf("stored seats after create = %+v, want 3 indexed seats", storedAfterCreate.Seats)
	}

	if _, _, err := service.JoinTable(context.Background(), created.ID, "p1", 2, 1500); err != nil {
		t.Fatalf("JoinTable(p1) error = %v", err)
	}
	if _, _, err := service.JoinTable(context.Background(), created.ID, "p2", 0, 2000); err != nil {
		t.Fatalf("JoinTable(p2) error = %v", err)
	}

	storedAfterJoin, joinedState, err := service.GetTable(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetTable(after join) error = %v", err)
	}
	if joinedState == nil || joinedState.Id == "" {
		t.Fatalf("GetTable(after join) state = %+v, want active state", joinedState)
	}
	if storedAfterJoin.Status != domain.TableStatusInHand {
		t.Fatalf("stored status after join = %s, want in_hand", storedAfterJoin.Status)
	}
	if storedAfterJoin.ActiveHandID == "" {
		t.Fatal("stored active hand id after join is empty")
	}
	if storedAfterJoin.SeatByIndex(0).PlayerID != "p2" || storedAfterJoin.SeatByIndex(2).PlayerID != "p1" {
		t.Fatalf("stored seats after join = %+v, want p2 at 0 and p1 at 2", storedAfterJoin.Seats)
	}

	storedAfterJoin.Status = domain.TableStatusIdle
	storedAfterJoin.ActiveHandID = ""
	repo.tables[created.ID] = cloneTable(storedAfterJoin)

	left, _, err := service.LeaveTable(context.Background(), created.ID, "p1")
	if err != nil {
		t.Fatalf("LeaveTable() error = %v", err)
	}
	if left.SeatByIndex(2).PlayerID != "" || left.SeatByIndex(2).Stack != 0 {
		t.Fatalf("seat after leave = %+v, want empty seat", left.SeatByIndex(2))
	}

	storedAfterLeave, finalState, err := service.GetTable(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetTable(after leave) error = %v", err)
	}
	if finalState != nil {
		t.Fatalf("GetTable(after leave) state = %+v, want nil", finalState)
	}
	if storedAfterLeave.SeatByIndex(2).PlayerID != "" {
		t.Fatalf("stored seat after leave = %+v, want empty seat", storedAfterLeave.SeatByIndex(2))
	}
	if storedAfterLeave.SeatByIndex(0).PlayerID != "p2" {
		t.Fatalf("stored seat 0 after leave = %+v, want p2", storedAfterLeave.SeatByIndex(0))
	}

	listed, err := service.ListTables(context.Background())
	if err != nil {
		t.Fatalf("ListTables() error = %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("ListTables() len = %d, want 1", len(listed))
	}
	if listed[0].ID != created.ID {
		t.Fatalf("listed[0].ID = %q, want %q", listed[0].ID, created.ID)
	}
}

func TestPersistenceSequenceShowdownThenGetUsesUpdatedHandAndStacks(t *testing.T) {
	repo := &stubRepo{tables: map[string]*domain.Table{
		"table-1": {
			ID:           "table-1",
			Name:         "Main",
			SeatCount:    3,
			Status:       domain.TableStatusInHand,
			Button:       2,
			ActiveHandID: "hand-1",
			SmallBlind:   50,
			BigBlind:     100,
			Seats: []domain.Seat{
				{Index: 0, PlayerID: "p1", Stack: 1000},
				{Index: 1},
				{Index: 2, PlayerID: "p2", Stack: 1000},
			},
		},
	}}
	engine := &stubEngine{
		applyResp: &enginev1.ApplyActionResponse{
			State: &enginev1.GameState{
				Id:     "hand-1",
				Street: enginev1.Street_STREET_SHOWDOWN,
				Players: []*enginev1.Player{
					{Id: "p1", Stack: 1300, Position: 0, Status: enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE},
					{Id: "p2", Stack: 700, Position: 2, Status: enginev1.PlayerStatus_PLAYER_STATUS_FOLDED},
				},
			},
		},
		startResp: &enginev1.StartHandResponse{
			State: &enginev1.GameState{
				Id:         "hand-2",
				TableId:    "table-1",
				Street:     enginev1.Street_STREET_PREFLOP,
				Button:     0,
				SmallBlind: 50,
				BigBlind:   100,
				Players: []*enginev1.Player{
					{Id: "p1", Stack: 1300, Position: 0, Status: enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE},
					{Id: "p2", Stack: 700, Position: 2, Status: enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE},
				},
			},
		},
		getResp: &enginev1.GetGameStateResponse{
			State: &enginev1.GameState{
				Id:     "hand-2",
				Street: enginev1.Street_STREET_PREFLOP,
			},
		},
	}
	service := NewService(repo, engine, &stubWallet{})

	if _, _, err := service.Act(context.Background(), "table-1", "p1", enginev1.ActionType_ACTION_TYPE_CHECK, 0); err != nil {
		t.Fatalf("Act() error = %v", err)
	}

	stored, state, err := service.GetTable(context.Background(), "table-1")
	if err != nil {
		t.Fatalf("GetTable() error = %v", err)
	}
	if state == nil || state.Id != "hand-2" {
		t.Fatalf("GetTable() state = %+v, want hand-2", state)
	}
	if stored.ActiveHandID != "hand-2" {
		t.Fatalf("stored.ActiveHandID = %q, want hand-2", stored.ActiveHandID)
	}
	if stored.Button != 0 {
		t.Fatalf("stored.Button = %d, want 0", stored.Button)
	}
	if stored.SeatByPlayerID("p1").Stack != 1300 || stored.SeatByPlayerID("p2").Stack != 700 {
		t.Fatalf("stored stacks = (%d,%d), want (1300,700)", stored.SeatByPlayerID("p1").Stack, stored.SeatByPlayerID("p2").Stack)
	}
}

func cloneTable(table *domain.Table) *domain.Table {
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
