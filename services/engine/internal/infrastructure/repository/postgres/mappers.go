package postgres

import (
	"github.com/pashathecreator/holdem/services/engine/internal/domain"
)

func streetToString(street domain.Street) string {
	switch street {
	case domain.StreetPreflop:
		return "preflop"
	case domain.StreetFlop:
		return "flop"
	case domain.StreetTurn:
		return "turn"
	case domain.StreetRiver:
		return "river"
	case domain.StreetShowdown:
		return "showdown"
	default:
		return "unknown"
	}
}

func streetFromString(street string) domain.Street {
	switch street {
	case "preflop":
		return domain.StreetPreflop
	case "flop":
		return domain.StreetFlop
	case "turn":
		return domain.StreetTurn
	case "river":
		return domain.StreetRiver
	case "showdown":
		return domain.StreetShowdown
	default:
		return domain.StreetPreflop
	}
}

func playerStatusToString(status domain.PlayerStatus) string {
	switch status {
	case domain.PlayerStatusActive:
		return "active"
	case domain.PlayerStatusFolded:
		return "folded"
	case domain.PlayerStatusAllIn:
		return "all_in"
	case domain.PlayerStatusSittingOut:
		return "sitting_out"
	default:
		return "active"
	}
}

func playerStatusFromString(status string) domain.PlayerStatus {
	switch status {
	case "active":
		return domain.PlayerStatusActive
	case "folded":
		return domain.PlayerStatusFolded
	case "all_in":
		return domain.PlayerStatusAllIn
	case "sitting_out":
		return domain.PlayerStatusSittingOut
	default:
		return domain.PlayerStatusActive
	}
}

func bettingStructureToString(structure domain.BettingStructure) string {
	switch structure {
	case domain.BettingFixedLimit:
		return "fixed_limit"
	default:
		return "fixed_limit"
	}
}

func bettingStructureFromString(structure string) domain.BettingStructure {
	switch structure {
	case "fixed_limit":
		return domain.BettingFixedLimit
	default:
		return domain.BettingFixedLimit
	}
}

func cardToString(card domain.Card) string {
	return card.String()
}