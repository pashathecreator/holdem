package repository

import (
	"fmt"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
)

func ParseCard(s string) (domain.Card, error) {
	if len(s) < 2 {
		return domain.Card{}, fmt.Errorf("invalid card string: %q", s)
	}

	rankStr := string(s[0])
	suitStr := s[1:]

	rank, err := ParseRank(rankStr)
	if err != nil {
		return domain.Card{}, err
	}

	suit, err := ParseSuit(suitStr)
	if err != nil {
		return domain.Card{}, err
	}

	return domain.NewCard(rank, suit), nil
}

func ParseRank(s string) (domain.Rank, error) {
	switch s {
	case "2":
		return domain.Two, nil
	case "3":
		return domain.Three, nil
	case "4":
		return domain.Four, nil
	case "5":
		return domain.Five, nil
	case "6":
		return domain.Six, nil
	case "7":
		return domain.Seven, nil
	case "8":
		return domain.Eight, nil
	case "9":
		return domain.Nine, nil
	case "T":
		return domain.Ten, nil
	case "J":
		return domain.Jack, nil
	case "Q":
		return domain.Queen, nil
	case "K":
		return domain.King, nil
	case "A":
		return domain.Ace, nil
	default:
		return 0, fmt.Errorf("invalid rank: %q", s)
	}
}

func ParseSuit(s string) (domain.Suit, error) {
	switch s {
	case "♠":
		return domain.Spades, nil
	case "♥":
		return domain.Hearts, nil
	case "♦":
		return domain.Diamonds, nil
	case "♣":
		return domain.Clubs, nil
	default:
		return 0, fmt.Errorf("invalid suit: %q", s)
	}
}
