package evaluator

import "github.com/pashathecreator/holdem/services/engine/internal/domain"

type Evaluator struct{}

func New() *Evaluator {
	return &Evaluator{}
}

func (e *Evaluator) Best(playerID domain.PlayerID, hole [2]domain.Card, board []domain.Card) domain.HandResult {
	all := make([]domain.Card, 0, 7)
	all = append(all, hole[0], hole[1])
	all = append(all, board...)

	combos := combinations(all)

	var bestCards [5]domain.Card
	var bestRank domain.HandRank = 0

	for _, combo := range combos {
		rank := rankHand(combo)
		if rank > bestRank {
			bestRank = rank
			bestCards = combo
		} else if rank == bestRank {
			if compareCards(combo, bestCards) > 0 {
				bestCards = combo
			}
		}
	}

	return domain.HandResult{
		PlayerID: playerID,
		Rank:     bestRank,
		Cards:    bestCards,
	}
}

func (e *Evaluator) Compare(a, b domain.HandResult) int {
	if a.Rank > b.Rank {
		return 1
	}
	if a.Rank < b.Rank {
		return -1
	}
	return compareCards(a.Cards, b.Cards)
}
