package grpc

import (
	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"

	"github.com/pashathecreator/holdem/services/table-manager/internal/domain"
	tablemanagerv1 "github.com/pashathecreator/holdem/services/table-manager/pkg/gen/go/table_manager/v1"
)

func domainTableToProto(table *domain.Table, state *enginev1.GameState, viewerID string) *tablemanagerv1.TableView {
	view := &tablemanagerv1.TableView{
		TableId:      table.ID,
		Name:         table.Name,
		Status:       domainTableStatusToProto(table.Status),
		SeatCount:    int32(table.SeatCount),
		SmallBlind:   table.SmallBlind,
		BigBlind:     table.BigBlind,
		Button:       int32(table.Button),
		ActiveHandId: table.ActiveHandID,
	}

	playerByID := make(map[string]*enginev1.Player)
	if state != nil {
		view.Hand = engineStateToProto(state)
		for _, player := range state.Players {
			playerByID[player.Id] = player
		}
	}

	view.Seats = make([]*tablemanagerv1.SeatView, 0, len(table.Seats))
	for _, seat := range table.Seats {
		seatView := &tablemanagerv1.SeatView{
			SeatIndex: int32(seat.Index),
			Status:    tablemanagerv1.SeatStatus_SEAT_STATUS_EMPTY,
			Stack:     seat.Stack,
		}
		if seat.Occupied() {
			seatView.Status = tablemanagerv1.SeatStatus_SEAT_STATUS_OCCUPIED
			seatView.PlayerId = seat.PlayerID
			if seat.PlayerID == viewerID {
				seatView.IsViewer = true
			}
		}

		if player, ok := playerByID[seat.PlayerID]; ok {
			seatView.Stack = player.Stack
			seatView.CurrentBet = player.CurrentBet
			seatView.PlayerStatus = enginePlayerStatusToProto(player.Status)
			if seat.PlayerID == viewerID {
				seatView.HoleCards = engineCardsToProto(player.HoleCards)
			}
		}

		view.Seats = append(view.Seats, seatView)
	}

	return view
}

func engineStateToProto(state *enginev1.GameState) *tablemanagerv1.HandView {
	if state == nil {
		return nil
	}
	return &tablemanagerv1.HandView{
		HandId:       state.Id,
		Board:        engineCardsToProto(state.Board),
		Pots:         enginePotsToProto(state.Pots),
		Street:       engineStreetToProto(state.Street),
		CurrentBet:   state.CurrentBet,
		ActivePlayer: state.ActivePlayer,
		Button:       state.Button,
		SmallBlind:   state.SmallBlind,
		BigBlind:     state.BigBlind,
	}
}

func engineCardsToProto(cards []*enginev1.Card) []*tablemanagerv1.Card {
	result := make([]*tablemanagerv1.Card, len(cards))
	for i, card := range cards {
		result[i] = &tablemanagerv1.Card{Value: card.Value}
	}
	return result
}

func enginePotsToProto(pots []*enginev1.Pot) []*tablemanagerv1.Pot {
	result := make([]*tablemanagerv1.Pot, len(pots))
	for i, pot := range pots {
		result[i] = &tablemanagerv1.Pot{
			Amount:   pot.Amount,
			Eligible: append([]string(nil), pot.Eligible...),
		}
	}
	return result
}

func domainTableStatusToProto(status domain.TableStatus) tablemanagerv1.TableStatus {
	switch status {
	case domain.TableStatusIdle:
		return tablemanagerv1.TableStatus_TABLE_STATUS_IDLE
	case domain.TableStatusInHand:
		return tablemanagerv1.TableStatus_TABLE_STATUS_IN_HAND
	default:
		return tablemanagerv1.TableStatus_TABLE_STATUS_UNSPECIFIED
	}
}

func engineStreetToProto(street enginev1.Street) tablemanagerv1.Street {
	switch street {
	case enginev1.Street_STREET_PREFLOP:
		return tablemanagerv1.Street_STREET_PREFLOP
	case enginev1.Street_STREET_FLOP:
		return tablemanagerv1.Street_STREET_FLOP
	case enginev1.Street_STREET_TURN:
		return tablemanagerv1.Street_STREET_TURN
	case enginev1.Street_STREET_RIVER:
		return tablemanagerv1.Street_STREET_RIVER
	case enginev1.Street_STREET_SHOWDOWN:
		return tablemanagerv1.Street_STREET_SHOWDOWN
	default:
		return tablemanagerv1.Street_STREET_UNSPECIFIED
	}
}

func enginePlayerStatusToProto(status enginev1.PlayerStatus) tablemanagerv1.PlayerStatus {
	switch status {
	case enginev1.PlayerStatus_PLAYER_STATUS_ACTIVE:
		return tablemanagerv1.PlayerStatus_PLAYER_STATUS_ACTIVE
	case enginev1.PlayerStatus_PLAYER_STATUS_FOLDED:
		return tablemanagerv1.PlayerStatus_PLAYER_STATUS_FOLDED
	case enginev1.PlayerStatus_PLAYER_STATUS_ALL_IN:
		return tablemanagerv1.PlayerStatus_PLAYER_STATUS_ALL_IN
	case enginev1.PlayerStatus_PLAYER_STATUS_SITTING_OUT:
		return tablemanagerv1.PlayerStatus_PLAYER_STATUS_SITTING_OUT
	default:
		return tablemanagerv1.PlayerStatus_PLAYER_STATUS_UNSPECIFIED
	}
}
