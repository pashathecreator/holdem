package shuffle

import (
	"crypto/rand"
	"math/big"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
)

func Shuffle(cards []domain.Card) []domain.Card {
	result := make([]domain.Card, len(cards))
	copy(result, cards)

	for i := len(result) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			panic("rng: crypto/rand failed: " + err.Error())
		}
		result[i], result[j.Int64()] = result[j.Int64()], result[i]
	}

	return result
}
