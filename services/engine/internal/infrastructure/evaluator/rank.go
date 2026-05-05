package evaluator

import (
	"sort"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
)

func rankHand(cards [5]domain.Card) domain.HandRank {
	if isRoyalFlush(cards) {
		return domain.RoyalFlush
	}
	if isStraightFlush(cards) {
		return domain.StraightFlush
	}
	if isFourOfAKind(cards) {
		return domain.FourOfAKind
	}
	if isFullHouse(cards) {
		return domain.FullHouse
	}
	if isFlush(cards) {
		return domain.Flush
	}
	if isStraight(cards) {
		return domain.Straight
	}
	if isThreeOfAKind(cards) {
		return domain.ThreeOfAKind
	}
	if isTwoPair(cards) {
		return domain.TwoPair
	}
	if isOnePair(cards) {
		return domain.OnePair
	}
	return domain.HighCard
}

func isFlush(cards [5]domain.Card) bool {
	suit := cards[0].Suit
	for _, c := range cards[1:] {
		if c.Suit != suit {
			return false
		}
	}
	return true
}

func isStraight(cards [5]domain.Card) bool {
	ranks := sortedRanks(cards)

	if ranks[4]-ranks[0] == 4 && uniqueCount(ranks) == 5 {
		return true
	}

	if ranks[4] == int(domain.Ace) &&
		ranks[0] == int(domain.Two) &&
		ranks[1] == int(domain.Three) &&
		ranks[2] == int(domain.Four) &&
		ranks[3] == int(domain.Five) {
		return true
	}

	return false
}

func isRoyalFlush(cards [5]domain.Card) bool {
	return isFlush(cards) && isStraight(cards) && sortedRanks(cards)[4] == int(domain.Ace) && sortedRanks(cards)[0] == int(domain.Ten)
}

func isStraightFlush(cards [5]domain.Card) bool {
	return isFlush(cards) && isStraight(cards)
}

func isFourOfAKind(cards [5]domain.Card) bool {
	counts := rankCounts(cards)
	for _, c := range counts {
		if c == 4 {
			return true
		}
	}
	return false
}

func isFullHouse(cards [5]domain.Card) bool {
	counts := rankCounts(cards)
	hasThree, hasTwo := false, false
	for _, c := range counts {
		if c == 3 {
			hasThree = true
		}
		if c == 2 {
			hasTwo = true
		}
	}
	return hasThree && hasTwo
}

func isThreeOfAKind(cards [5]domain.Card) bool {
	counts := rankCounts(cards)
	for _, c := range counts {
		if c == 3 {
			return true
		}
	}
	return false
}

func isTwoPair(cards [5]domain.Card) bool {
	counts := rankCounts(cards)
	pairs := 0
	for _, c := range counts {
		if c == 2 {
			pairs++
		}
	}
	return pairs == 2
}

func isOnePair(cards [5]domain.Card) bool {
	counts := rankCounts(cards)
	for _, c := range counts {
		if c == 2 {
			return true
		}
	}
	return false
}

func rankCounts(cards [5]domain.Card) map[domain.Rank]int {
	counts := make(map[domain.Rank]int)
	for _, c := range cards {
		counts[c.Rank]++
	}
	return counts
}

func sortedRanks(cards [5]domain.Card) []int {
	ranks := make([]int, 5)
	for i, c := range cards {
		ranks[i] = int(c.Rank)
	}
	sort.Ints(ranks)
	return ranks
}

func uniqueCount(ranks []int) int {
	seen := make(map[int]bool)
	for _, r := range ranks {
		seen[r] = true
	}
	return len(seen)
}

func compareCards(a, b [5]domain.Card) int {
	aRanks := sortedRanksDesc(a)
	bRanks := sortedRanksDesc(b)
	for i := range aRanks {
		if aRanks[i] > bRanks[i] {
			return 1
		}
		if aRanks[i] < bRanks[i] {
			return -1
		}
	}
	return 0
}

func sortedRanksDesc(cards [5]domain.Card) []int {
	ranks := sortedRanks(cards)
	for i, j := 0, len(ranks)-1; i < j; i, j = i+1, j-1 {
		ranks[i], ranks[j] = ranks[j], ranks[i]
	}
	return ranks
}
