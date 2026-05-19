package grpc

import (
	"context"
	"errors"

	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pashathecreator/holdem/services/table-manager/internal/application"
	"github.com/pashathecreator/holdem/services/table-manager/internal/domain"
	tablemanagerv1 "github.com/pashathecreator/holdem/services/table-manager/pkg/gen/go/table_manager/v1"
)

type Server struct {
	service *application.Service
	hub     *Hub
	auth    *Authenticator
}

func NewServer(service *application.Service, hub *Hub, auth *Authenticator) *Server {
	return &Server{service: service, hub: hub, auth: auth}
}

func (s *Server) CreateTable(ctx context.Context, req *tablemanagerv1.CreateTableRequest) (*tablemanagerv1.CreateTableResponse, error) {
	viewerID, err := s.auth.RequiredUserID(ctx)
	if err != nil {
		return nil, authErr(err)
	}
	table, err := s.service.CreateTable(ctx, req.Name, int(req.SeatCount), req.SmallBlind, req.BigBlind)
	if err != nil {
		return nil, domainErr(err)
	}
	return &tablemanagerv1.CreateTableResponse{Table: domainTableToProto(table, nil, viewerID)}, nil
}

func (s *Server) ListTables(ctx context.Context, _ *tablemanagerv1.ListTablesRequest) (*tablemanagerv1.ListTablesResponse, error) {
	viewerID, err := s.auth.OptionalUserID(ctx)
	if err != nil {
		return nil, authErr(err)
	}
	tables, err := s.service.ListTables(ctx)
	if err != nil {
		return nil, domainErr(err)
	}
	views := make([]*tablemanagerv1.TableView, 0, len(tables))
	for _, table := range tables {
		views = append(views, domainTableToProto(table, nil, viewerID))
	}
	return &tablemanagerv1.ListTablesResponse{Tables: views}, nil
}

func (s *Server) GetTable(ctx context.Context, req *tablemanagerv1.GetTableRequest) (*tablemanagerv1.GetTableResponse, error) {
	viewerID, err := s.auth.OptionalUserID(ctx)
	if err != nil {
		return nil, authErr(err)
	}
	table, state, err := s.service.GetTable(ctx, req.TableId)
	if err != nil {
		return nil, domainErr(err)
	}
	return &tablemanagerv1.GetTableResponse{Table: domainTableToProto(table, state, viewerID)}, nil
}

func (s *Server) JoinTable(ctx context.Context, req *tablemanagerv1.JoinTableRequest) (*tablemanagerv1.JoinTableResponse, error) {
	userID, err := s.auth.RequiredUserID(ctx)
	if err != nil {
		return nil, authErr(err)
	}
	table, state, err := s.service.JoinTable(ctx, req.TableId, userID, int(req.SeatIndex), req.BuyIn)
	if err != nil {
		return nil, domainErr(err)
	}
	s.publishTable(ctx, table, state)
	return &tablemanagerv1.JoinTableResponse{Table: domainTableToProto(table, state, userID)}, nil
}

func (s *Server) LeaveTable(ctx context.Context, req *tablemanagerv1.LeaveTableRequest) (*tablemanagerv1.LeaveTableResponse, error) {
	userID, err := s.auth.RequiredUserID(ctx)
	if err != nil {
		return nil, authErr(err)
	}
	table, state, err := s.service.LeaveTable(ctx, req.TableId, userID)
	if err != nil {
		return nil, domainErr(err)
	}
	s.publishTable(ctx, table, state)
	return &tablemanagerv1.LeaveTableResponse{Table: domainTableToProto(table, state, userID)}, nil
}

func (s *Server) Act(ctx context.Context, req *tablemanagerv1.ActRequest) (*tablemanagerv1.ActResponse, error) {
	userID, err := s.auth.RequiredUserID(ctx)
	if err != nil {
		return nil, authErr(err)
	}
	engineActionType := tableActionTypeToEngine(req.Action.GetType())
	table, state, err := s.service.Act(ctx, req.TableId, userID, engineActionType, req.Action.GetAmount())
	if err != nil {
		return nil, domainErr(err)
	}
	s.publishTable(ctx, table, state)
	return &tablemanagerv1.ActResponse{Table: domainTableToProto(table, state, userID)}, nil
}

func (s *Server) publishTable(ctx context.Context, table *domain.Table, state *enginev1.GameState) {
	s.hub.Publish(table.ID, func(viewerID string) *tablemanagerv1.TableView {
		if state == nil && table.ActiveHandID != "" {
			currentTable, currentState, err := s.service.GetTable(context.Background(), table.ID)
			if err == nil {
				return domainTableToProto(currentTable, currentState, viewerID)
			}
		}
		return domainTableToProto(table, state, viewerID)
	})
}

func tableActionTypeToEngine(actionType tablemanagerv1.ActionType) enginev1.ActionType {
	switch actionType {
	case tablemanagerv1.ActionType_ACTION_TYPE_FOLD:
		return enginev1.ActionType_ACTION_TYPE_FOLD
	case tablemanagerv1.ActionType_ACTION_TYPE_CHECK:
		return enginev1.ActionType_ACTION_TYPE_CHECK
	case tablemanagerv1.ActionType_ACTION_TYPE_CALL:
		return enginev1.ActionType_ACTION_TYPE_CALL
	case tablemanagerv1.ActionType_ACTION_TYPE_RAISE:
		return enginev1.ActionType_ACTION_TYPE_RAISE
	case tablemanagerv1.ActionType_ACTION_TYPE_ALL_IN:
		return enginev1.ActionType_ACTION_TYPE_ALL_IN
	default:
		return enginev1.ActionType_ACTION_TYPE_UNSPECIFIED
	}
}

func domainErr(err error) error {
	switch {
	case errors.Is(err, domain.ErrTableNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrSeatOutOfRange),
		errors.Is(err, domain.ErrSeatOccupied),
		errors.Is(err, domain.ErrPlayerAlreadySeated),
		errors.Is(err, domain.ErrPlayerNotSeated),
		errors.Is(err, domain.ErrActiveHandRequired),
		errors.Is(err, domain.ErrLeaveDuringActiveHand),
		errors.Is(err, domain.ErrSpectatorCannotAct),
		errors.Is(err, domain.ErrInvalidTableConfig),
		errors.Is(err, domain.ErrInsufficientFunds):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func authErr(err error) error {
	return status.Error(codes.Unauthenticated, err.Error())
}
