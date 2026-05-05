package grpc

import (
	"context"
	"io"

	"github.com/pashathecreator/holdem/services/engine/internal/application"
	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
)

type gameStateReader interface {
	FindByID(ctx context.Context, id domain.HandID) (*domain.GameState, error)
}

type Server struct {
	startHand   *application.StartHand
	applyAction *application.ApplyAction
	finishHand  *application.FinishHand
	repo        gameStateReader
	pubsub      *PubSub
}

func NewServer(
	startHand *application.StartHand,
	applyAction *application.ApplyAction,
	finishHand *application.FinishHand,
	repo gameStateReader,
	pubsub *PubSub,
) *Server {
	return &Server{
		startHand:   startHand,
		applyAction: applyAction,
		finishHand:  finishHand,
		repo:        repo,
		pubsub:      pubsub,
	}
}

func (s *Server) StartHand(ctx context.Context, req *enginev1.StartHandRequest) (*enginev1.StartHandResponse, error) {
	input := application.StartHandInput{
		TableID: domain.TableID(req.TableId),
		Players: protoPlayersToDoomain(req.Players),
		Button:  int(req.Button),
		BettingConfig: domain.BettingConfig{
			Structure:          domain.BettingFixedLimit,
			SmallBlind:         int(req.SmallBlind),
			BigBlind:           int(req.BigBlind),
			SmallBet:           int(req.BigBlind),
			BigBet:             int(req.BigBlind) * 2,
			MaxRaisesPerStreet: 4,
		},
	}

	state, err := s.startHand.Execute(ctx, input)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	s.pubsub.Publish(state.ID, state)

	return &enginev1.StartHandResponse{
		State: domainStateToProto(state),
	}, nil
}

func (s *Server) ApplyAction(ctx context.Context, req *enginev1.ApplyActionRequest) (*enginev1.ApplyActionResponse, error) {
	input := application.ApplyActionInput{
		HandID: domain.HandID(req.HandId),
		Action: domain.Action{
			PlayerID: domain.PlayerID(req.Action.PlayerId),
			Type:     protoActionTypeToDomain(req.Action.Type),
			Amount:   int(req.Action.Amount),
		},
	}

	state, err := s.applyAction.Execute(ctx, input)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	s.pubsub.Publish(state.ID, state)

	return &enginev1.ApplyActionResponse{
		State: domainStateToProto(state),
	}, nil
}

func (s *Server) GetGameState(ctx context.Context, req *enginev1.GetGameStateRequest) (*enginev1.GetGameStateResponse, error) {
	state, err := s.repo.FindByID(ctx, domain.HandID(req.HandId))
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	return &enginev1.GetGameStateResponse{
		State: domainStateToProto(state),
	}, nil
}

func (s *Server) StreamGameState(req *enginev1.StreamGameStateRequest, stream enginev1.GameEngine_StreamGameStateServer) error {
	handID := domain.HandID(req.HandId)

	state, err := s.repo.FindByID(stream.Context(), handID)
	if err != nil {
		return domainErrorToGRPC(err)
	}

	if err := stream.Send(&enginev1.StreamGameStateResponse{
		State: domainStateToProto(state),
	}); err != nil {
		return err
	}

	updates := s.pubsub.Subscribe(handID)
	defer s.pubsub.Unsubscribe(handID, updates)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case state, ok := <-updates:
			if !ok {
				return nil
			}
			if err := stream.Send(&enginev1.StreamGameStateResponse{
				State: domainStateToProto(state),
			}); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
		}
	}
}
