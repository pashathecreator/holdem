package evaluator_test

import (
	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/infrastructure/evaluator"
	"testing"
)

var e = evaluator.New()

func card(rank domain.Rank, suit domain.Suit) domain.Card {
	return domain.NewCard(rank, suit)
}
func TestBest_RoyalFlush(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Ace, domain.Spades),
		card(domain.King, domain.Spades),
	}
	board := []domain.Card{
		card(domain.Queen, domain.Spades),
		card(domain.Jack, domain.Spades),
		card(domain.Ten, domain.Spades),
		card(domain.Two, domain.Hearts),
		card(domain.Three, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.RoyalFlush {
		t.Errorf("expected RoyalFlush, got %s", result.Rank)
	}
}
func TestBest_StraightFlush(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Nine, domain.Hearts),
		card(domain.Eight, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.Seven, domain.Hearts),
		card(domain.Six, domain.Hearts),
		card(domain.Five, domain.Hearts),
		card(domain.Two, domain.Spades),
		card(domain.Ace, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.StraightFlush {
		t.Errorf("expected StraightFlush, got %s", result.Rank)
	}
}
func TestBest_FourOfAKind(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Ace, domain.Spades),
		card(domain.Ace, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.Ace, domain.Diamonds),
		card(domain.Ace, domain.Clubs),
		card(domain.King, domain.Spades),
		card(domain.Two, domain.Hearts),
		card(domain.Three, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.FourOfAKind {
		t.Errorf("expected FourOfAKind, got %s", result.Rank)
	}
}
func TestBest_FullHouse(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.King, domain.Spades),
		card(domain.King, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.King, domain.Diamonds),
		card(domain.Ace, domain.Clubs),
		card(domain.Ace, domain.Spades),
		card(domain.Two, domain.Hearts),
		card(domain.Three, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.FullHouse {
		t.Errorf("expected FullHouse, got %s", result.Rank)
	}
}
func TestBest_Flush(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Ace, domain.Hearts),
		card(domain.Jack, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.Nine, domain.Hearts),
		card(domain.Seven, domain.Hearts),
		card(domain.Two, domain.Hearts),
		card(domain.King, domain.Spades),
		card(domain.Three, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.Flush {
		t.Errorf("expected Flush, got %s", result.Rank)
	}
}
func TestBest_Straight(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Nine, domain.Spades),
		card(domain.Eight, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.Seven, domain.Diamonds),
		card(domain.Six, domain.Clubs),
		card(domain.Five, domain.Spades),
		card(domain.Ace, domain.Hearts),
		card(domain.King, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.Straight {
		t.Errorf("expected Straight, got %s", result.Rank)
	}
}
func TestBest_Straight_Wheel(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Ace, domain.Spades),
		card(domain.Two, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.Three, domain.Diamonds),
		card(domain.Four, domain.Clubs),
		card(domain.Five, domain.Spades),
		card(domain.King, domain.Hearts),
		card(domain.Queen, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.Straight {
		t.Errorf("expected Straight (wheel), got %s", result.Rank)
	}
}
func TestBest_ThreeOfAKind(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Queen, domain.Spades),
		card(domain.Queen, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.Queen, domain.Diamonds),
		card(domain.Ace, domain.Clubs),
		card(domain.King, domain.Spades),
		card(domain.Two, domain.Hearts),
		card(domain.Three, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.ThreeOfAKind {
		t.Errorf("expected ThreeOfAKind, got %s", result.Rank)
	}
}
func TestBest_TwoPair(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Ace, domain.Spades),
		card(domain.Ace, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.King, domain.Diamonds),
		card(domain.King, domain.Clubs),
		card(domain.Two, domain.Spades),
		card(domain.Three, domain.Hearts),
		card(domain.Four, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.TwoPair {
		t.Errorf("expected TwoPair, got %s", result.Rank)
	}
}
func TestBest_OnePair(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Ace, domain.Spades),
		card(domain.Ace, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.King, domain.Diamonds),
		card(domain.Queen, domain.Clubs),
		card(domain.Two, domain.Spades),
		card(domain.Three, domain.Hearts),
		card(domain.Four, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.OnePair {
		t.Errorf("expected OnePair, got %s", result.Rank)
	}
}
func TestBest_HighCard(t *testing.T) {
	hole := [2]domain.Card{
		card(domain.Ace, domain.Spades),
		card(domain.King, domain.Hearts),
	}
	board := []domain.Card{
		card(domain.Queen, domain.Diamonds),
		card(domain.Jack, domain.Clubs),
		card(domain.Nine, domain.Spades),
		card(domain.Three, domain.Hearts),
		card(domain.Two, domain.Clubs),
	}
	result := e.Best("A", hole, board)
	if result.Rank != domain.HighCard {
		t.Errorf("expected HighCard, got %s", result.Rank)
	}
}
func TestCompare_HigherRankWins(t *testing.T) {
	flush := e.Best("A", [2]domain.Card{
		card(domain.Ace, domain.Hearts),
		card(domain.Jack, domain.Hearts),
	}, []domain.Card{
		card(domain.Nine, domain.Hearts),
		card(domain.Seven, domain.Hearts),
		card(domain.Two, domain.Hearts),
		card(domain.King, domain.Spades),
		card(domain.Three, domain.Clubs),
	})
	pair := e.Best("B", [2]domain.Card{
		card(domain.Ace, domain.Spades),
		card(domain.Ace, domain.Diamonds),
	}, []domain.Card{
		card(domain.King, domain.Clubs),
		card(domain.Queen, domain.Diamonds),
		card(domain.Two, domain.Spades),
		card(domain.Three, domain.Hearts),
		card(domain.Four, domain.Clubs),
	})
	if e.Compare(flush, pair) != 1 {
		t.Error("expected flush to beat one pair")
	}
	if e.Compare(pair, flush) != -1 {
		t.Error("expected one pair to lose to flush")
	}
}
func TestCompare_SameRankKicker(t *testing.T) {
	aceKing := e.Best("A", [2]domain.Card{
		card(domain.Ace, domain.Spades),
		card(domain.Ace, domain.Hearts),
	}, []domain.Card{
		card(domain.King, domain.Diamonds),
		card(domain.Queen, domain.Clubs),
		card(domain.Two, domain.Spades),
		card(domain.Three, domain.Hearts),
		card(domain.Four, domain.Clubs),
	})
	aceJack := e.Best("B", [2]domain.Card{
		card(domain.Ace, domain.Diamonds),
		card(domain.Ace, domain.Clubs),
	}, []domain.Card{
		card(domain.Jack, domain.Spades),
		card(domain.Queen, domain.Hearts),
		card(domain.Two, domain.Diamonds),
		card(domain.Three, domain.Clubs),
		card(domain.Four, domain.Hearts),
	})
	if e.Compare(aceKing, aceJack) != 1 {
		t.Error("expected ace-king kicker to beat ace-jack kicker")
	}
}
func TestCompare_Tie(t *testing.T) {
	board := []domain.Card{
		card(domain.Ace, domain.Spades),
		card(domain.King, domain.Hearts),
		card(domain.Queen, domain.Diamonds),
		card(domain.Jack, domain.Clubs),
		card(domain.Ten, domain.Spades),
	}
	a := e.Best("A", [2]domain.Card{
		card(domain.Two, domain.Hearts),
		card(domain.Three, domain.Clubs),
	}, board)
	b := e.Best("B", [2]domain.Card{
		card(domain.Four, domain.Hearts),
		card(domain.Five, domain.Diamonds),
	}, board)
	if e.Compare(a, b) != 0 {
		t.Errorf("expected tie, got %d", e.Compare(a, b))
	}
}
