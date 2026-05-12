package domain

import "errors"

type ShuffleFunc func([]Card) []Card

type Deck struct {
	cards   []Card
	shuffle ShuffleFunc
}

func NewDeck(shuffle ShuffleFunc) *Deck {
	cards := make([]Card, 0, 52)
	for _, suit := range []Suit{Spades, Hearts, Diamonds, Clubs} {
		for rank := Two; rank <= Ace; rank++ {
			cards = append(cards, NewCard(rank, suit))
		}
	}
	return &Deck{cards: cards, shuffle: shuffle}
}

func NewDeckFromCards(cards []Card) *Deck {
	deckCards := make([]Card, len(cards))
	copy(deckCards, cards)
	return &Deck{cards: deckCards}
}

func (d *Deck) Shuffle() {
	d.cards = d.shuffle(d.cards)
}

func (d *Deck) Deal(n int) ([]Card, error) {
	if n <= 0 {
		return nil, errors.New("n must be positive")
	}
	if len(d.cards) == 0 {
		return nil, ErrEmptyDeck
	}
	if len(d.cards) < n {
		return nil, ErrNotEnoughCards
	}
	dealt := make([]Card, n)
	copy(dealt, d.cards[:n])
	d.cards = d.cards[n:]
	return dealt, nil
}

func (d *Deck) Remaining() int {
	return len(d.cards)
}

func (d *Deck) Cards() []Card {
	cards := make([]Card, len(d.cards))
	copy(cards, d.cards)
	return cards
}
