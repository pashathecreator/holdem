package grpc

import (
	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
)

func domainStateToProto(state *domain.GameState) *enginev1.GameState {
	return &enginev1.GameState{
		Id:           string(state.ID),
		TableId:      string(state.TableID),
		Players:      domainPlayersToProto(state.Players),
		Board:        domainBoardToProto(state.Board),
		Pots:         domainPotsToProto(state.Pots),
		Street:       domainStreetToProto(state.Street),
		CurrentBet:   int64(state.CurrentBet),
		ActivePlayer: int32(state.ActivePlayer),
		Button:       int32(state.Button),
		SmallBlind:   int64(state.SmallBlind),
		BigBlind:     int64(state.BigBlind),
	}
}

func domainPlayersToProto(players []*domain.Player) []*enginev1.Player {
	result := make([]*enginev1.Player, len(players))
	for i, p := range players {
		result[i] = &enginev1.Player{
			Id:         string(p.ID),
			Stack:      int64(p.Stack),
			HoleCards:  domainHoleCardsToProto(p.HoleCards),
			Status:     domainStatusToProto(p.Status),
			CurrentBet: int64(p.CurrentBet),
			Position:   int32(p.Position),
		}
	}
	return result
}

func domainHoleCardsToProto(cards [2]domain.Card) []*enginev1.Card {
	return []*enginev1.Card{
		{Value: cards[0].String()},
		{Value: cards[1].String()},
	}
}

func domainBoardToProto(cards []domain.Card) []*enginev1.Card {
	result := make([]*enginev1.Card, len(cards))
	for i, c := range cards {
		result[i] = &enginev1.Card{Value: c.String()}
	}
	return result
}

func domainPotsToProto(pots []domain.Pot) []*enginev1.Pot {
	result := make([]*enginev1.Pot, len(pots))
	for i, p := range pots {
		eligible := make([]string, len(p.Eligible))
		for j, id := range p.Eligible {
			eligible[j] = string(id)
		}
		result[i] = &enginev1.Pot{
			Amount:   int64(p.Amount),
			Eligible: eligible,
		}
	}
	return result
}

func domainStreetToProto(s domain.Street) enginev1.Street {
	switch s {
	case domain.StreetPreflop:
		return enginev1.Street_STREET_PREFLOP
	case domain.StreetFlop:
		return enginev1.Street_STREET_FLOP
	case domain.StreetTurn:
		return enginev1.Street_STREET_TURN
	case domain.StreetRiver:
		return enginev1.Street_STREET_RIVER
	case domain.StreetShowdown:
		return enginev1.Street_STREET_SHOWDOWN
	default:
		return enginev1.Street_STREET_UNSPECIFIED
	}
}

func domainStatusToProto(s domain.PlayerStatus) enginev1.PlayerStatus {
	switch s {
	case domain.PlayerStatusActive:
		return enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE
	case domain.PlayerStatusFolded:
		return enginev1.PlayerStatus_PLAYER_STATUS_FOLDED
	case domain.PlayerStatusAllIn:
		return enginev1.PlayerStatus_PLAYER_STATUS_ALL_IN
	case domain.PlayerStatusSittingOut:
		return enginev1.PlayerStatus_PLAYER_STATUS_SITTING_OUT
	default:
		return enginev1.PlayerStatus_PLAYER_STATUS_UNSPECIFIED
	}
}

func protoActionTypeToDomain(t enginev1.ActionType) domain.ActionType {
	switch t {
	case enginev1.ActionType_ACTION_TYPE_FOLD:
		return domain.ActionFold
	case enginev1.ActionType_ACTION_TYPE_CHECK:
		return domain.ActionCheck
	case enginev1.ActionType_ACTION_TYPE_CALL:
		return domain.ActionCall
	case enginev1.ActionType_ACTION_TYPE_RAISE:
		return domain.ActionRaise
	case enginev1.ActionType_ACTION_TYPE_ALL_IN:
		return domain.ActionAllIn
	default:
		return domain.ActionFold
	}
}

func protoPlayersToDoomain(players []*enginev1.Player) []*domain.Player {
	result := make([]*domain.Player, len(players))
	for i, p := range players {
		result[i] = &domain.Player{
			ID:       domain.PlayerID(p.Id),
			Stack:    int(p.Stack),
			Position: int(p.Position),
		}
	}
	return result
}