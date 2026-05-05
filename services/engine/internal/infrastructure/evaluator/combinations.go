package evaluator

import "github.com/pashathecreator/holdem/services/engine/internal/domain"

func combinations(cards []domain.Card) [][5]domain.Card {
	// C(5, 7) = 21
	result := make([][5]domain.Card, 0, 21)
	n := len(cards)

	for i := 0; i < n-4; i++ {
		for j := i + 1; j < n-3; j++ {
			for k := j + 1; k < n-2; k++ {
				for l := k + 1; l < n-1; l++ {
					for m := l + 1; m < n; m++ {
						result = append(result, [5]domain.Card{
							cards[i], cards[j], cards[k], cards[l], cards[m],
						})
					}
				}
			}
		}
	}

	return result
}
