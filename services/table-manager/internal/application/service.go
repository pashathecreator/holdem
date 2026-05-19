package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
	"google.golang.org/grpc"

	"github.com/pashathecreator/holdem/services/table-manager/internal/domain"
)

type Repository interface {
	CreateTable(ctx context.Context, table *domain.Table) error
	SaveTable(ctx context.Context, table *domain.Table) error
	FindTable(ctx context.Context, tableID string) (*domain.Table, error)
	ListTables(ctx context.Context) ([]*domain.Table, error)
}

type EngineClient interface {
	StartHand(ctx context.Context, in *enginev1.StartHandRequest, opts ...grpc.CallOption) (*enginev1.StartHandResponse, error)
	ApplyAction(ctx context.Context, in *enginev1.ApplyActionRequest, opts ...grpc.CallOption) (*enginev1.ApplyActionResponse, error)
	GetGameState(ctx context.Context, in *enginev1.GetGameStateRequest, opts ...grpc.CallOption) (*enginev1.GetGameStateResponse, error)
}

type WalletClient interface {
	DebitForJoin(ctx context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error
	CreditForCashout(ctx context.Context, userID, tableID string, amountGwei int64, idempotencyKey string) error
}

type Service struct {
	repo   Repository
	engine EngineClient
	wallet WalletClient
}

func NewService(repo Repository, engine EngineClient, wallet WalletClient) *Service {
	return &Service{repo: repo, engine: engine, wallet: wallet}
}

func (s *Service) CreateTable(ctx context.Context, name string, seatCount int, smallBlind, bigBlind int64) (*domain.Table, error) {
	if seatCount < 2 || smallBlind <= 0 || bigBlind <= 0 || bigBlind != smallBlind*2 {
		return nil, domain.ErrInvalidTableConfig
	}

	seats := make([]domain.Seat, seatCount)
	for i := 0; i < seatCount; i++ {
		seats[i] = domain.Seat{Index: i}
	}

	table := &domain.Table{
		ID:         fmt.Sprintf("table-%d", time.Now().UnixNano()),
		Name:       name,
		SeatCount:  seatCount,
		Status:     domain.TableStatusIdle,
		SmallBlind: smallBlind,
		BigBlind:   bigBlind,
		Seats:      seats,
	}

	if err := s.repo.CreateTable(ctx, table); err != nil {
		return nil, err
	}
	return table, nil
}

func (s *Service) ListTables(ctx context.Context) ([]*domain.Table, error) {
	return s.repo.ListTables(ctx)
}

func (s *Service) GetTable(ctx context.Context, tableID string) (*domain.Table, *enginev1.GameState, error) {
	table, err := s.repo.FindTable(ctx, tableID)
	if err != nil {
		return nil, nil, err
	}
	state, err := s.getGameState(ctx, table)
	if err != nil {
		return nil, nil, err
	}
	return table, state, nil
}

func (s *Service) JoinTable(ctx context.Context, tableID, playerID string, seatIndex int, buyIn int64) (*domain.Table, *enginev1.GameState, error) {
	table, err := s.repo.FindTable(ctx, tableID)
	if err != nil {
		return nil, nil, err
	}

	if buyIn <= 0 {
		return nil, nil, domain.ErrInvalidTableConfig
	}
	if seatIndex < 0 || seatIndex >= table.SeatCount {
		return nil, nil, domain.ErrSeatOutOfRange
	}
	if table.SeatByPlayerID(playerID) != nil {
		return nil, nil, domain.ErrPlayerAlreadySeated
	}

	seat := table.SeatByIndex(seatIndex)
	if seat == nil {
		return nil, nil, domain.ErrSeatOutOfRange
	}
	if seat.Occupied() {
		return nil, nil, domain.ErrSeatOccupied
	}

	if s.wallet != nil {
		if err := s.wallet.DebitForJoin(ctx, playerID, table.ID, buyIn, joinIdempotencyKey(table, playerID, seatIndex, buyIn)); err != nil {
			return nil, nil, err
		}
	}

	seat.PlayerID = playerID
	seat.Stack = buyIn

	var state *enginev1.GameState
	if table.CanAutoStart() {
		state, err = s.startHand(ctx, table)
		if err != nil {
			return nil, nil, err
		}
	}

	if err := s.repo.SaveTable(ctx, table); err != nil {
		return nil, nil, err
	}
	return table, state, nil
}

func (s *Service) LeaveTable(ctx context.Context, tableID, playerID string) (*domain.Table, *enginev1.GameState, error) {
	table, err := s.repo.FindTable(ctx, tableID)
	if err != nil {
		return nil, nil, err
	}
	if table.Status == domain.TableStatusInHand {
		return nil, nil, domain.ErrLeaveDuringActiveHand
	}

	seat := table.SeatByPlayerID(playerID)
	if seat == nil {
		return nil, nil, domain.ErrPlayerNotSeated
	}

	if s.wallet != nil {
		if err := s.wallet.CreditForCashout(ctx, playerID, table.ID, seat.Stack, cashoutIdempotencyKey(table, playerID, seat.Index, seat.Stack)); err != nil {
			return nil, nil, err
		}
	}

	seat.PlayerID = ""
	seat.Stack = 0
	if err := s.repo.SaveTable(ctx, table); err != nil {
		return nil, nil, err
	}
	return table, nil, nil
}

func (s *Service) Act(ctx context.Context, tableID, playerID string, actionType enginev1.ActionType, amount int64) (*domain.Table, *enginev1.GameState, error) {
	table, err := s.repo.FindTable(ctx, tableID)
	if err != nil {
		return nil, nil, err
	}
	if table.ActiveHandID == "" {
		return nil, nil, domain.ErrActiveHandRequired
	}
	if table.SeatByPlayerID(playerID) == nil {
		return nil, nil, domain.ErrSpectatorCannotAct
	}

	resp, err := s.engine.ApplyAction(ctx, &enginev1.ApplyActionRequest{
		HandId: table.ActiveHandID,
		Action: &enginev1.Action{
			PlayerId: playerID,
			Type:     actionType,
			Amount:   amount,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("apply action via engine: %w", err)
	}
	state := resp.GetState()
	if state == nil {
		return nil, nil, fmt.Errorf("apply action via engine: empty state")
	}

	if state.Street == enginev1.Street_STREET_SHOWDOWN {
		s.applyStacksFromState(table, state)
		s.unseatBustedPlayers(table)
		table.ActiveHandID = ""
		table.Status = domain.TableStatusIdle
		table.AdvanceButton()
		if table.CanAutoStart() {
			state, err = s.startHand(ctx, table)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	if err := s.repo.SaveTable(ctx, table); err != nil {
		return nil, nil, err
	}
	return table, state, nil
}

func (s *Service) getGameState(ctx context.Context, table *domain.Table) (*enginev1.GameState, error) {
	if table.ActiveHandID == "" {
		return nil, nil
	}
	resp, err := s.engine.GetGameState(ctx, &enginev1.GetGameStateRequest{HandId: table.ActiveHandID})
	if err != nil {
		return nil, fmt.Errorf("get game state via engine: %w", err)
	}
	return resp.GetState(), nil
}

func (s *Service) startHand(ctx context.Context, table *domain.Table) (*enginev1.GameState, error) {
	occupied := table.PlayablePlayers()
	sort.Slice(occupied, func(i, j int) bool { return occupied[i].Index < occupied[j].Index })

	buttonSeat := table.NormalizedButtonSeat()
	button := 0
	players := make([]*enginev1.Player, 0, len(occupied))
	for i, seat := range occupied {
		players = append(players, &enginev1.Player{
			Id:       seat.PlayerID,
			Stack:    seat.Stack,
			Position: int32(seat.Index),
		})
		if seat.Index == buttonSeat {
			button = i
		}
	}

	resp, err := s.engine.StartHand(ctx, &enginev1.StartHandRequest{
		TableId:    table.ID,
		Players:    players,
		Button:     int32(button),
		SmallBlind: table.SmallBlind,
		BigBlind:   table.BigBlind,
	})
	if err != nil {
		return nil, fmt.Errorf("start hand via engine: %w", err)
	}
	state := resp.GetState()
	if state == nil {
		return nil, fmt.Errorf("start hand via engine: empty state")
	}

	table.ActiveHandID = state.Id
	table.Status = domain.TableStatusInHand
	return state, nil
}

func (s *Service) applyStacksFromState(table *domain.Table, state *enginev1.GameState) {
	for _, player := range state.Players {
		seat := table.SeatByPlayerID(player.Id)
		if seat == nil {
			continue
		}
		seat.Stack = player.Stack
	}
}

func (s *Service) unseatBustedPlayers(table *domain.Table) {
	for i := range table.Seats {
		if table.Seats[i].Occupied() && table.Seats[i].Stack == 0 {
			table.Seats[i].PlayerID = ""
		}
	}
}

func joinIdempotencyKey(table *domain.Table, playerID string, seatIndex int, buyIn int64) string {
	return strings.Join([]string{
		"join",
		table.ID,
		playerID,
		fmt.Sprintf("%d", seatIndex),
		fmt.Sprintf("%d", buyIn),
		string(table.Status),
		table.ActiveHandID,
		fmt.Sprintf("%d", table.Button),
		fmt.Sprintf("%d", len(table.PlayablePlayers())),
	}, ":")
}

func cashoutIdempotencyKey(table *domain.Table, playerID string, seatIndex int, stack int64) string {
	return strings.Join([]string{
		"cashout",
		table.ID,
		playerID,
		fmt.Sprintf("%d", seatIndex),
		fmt.Sprintf("%d", stack),
		string(table.Status),
		table.ActiveHandID,
		fmt.Sprintf("%d", table.Button),
	}, ":")
}
