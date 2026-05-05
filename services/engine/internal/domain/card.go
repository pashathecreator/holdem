package domain

import "fmt"

type Suit byte

const (
	Spades Suit = iota
	Hearts
	Diamonds
	Clubs
)

func (s Suit) String() string {
	switch s {
	case Spades:
		return "♠"
	case Hearts:
		return "♥"
	case Diamonds:
		return "♦"
	case Clubs:
		return "♣"
	default:
		return "?"
	}
}

type Rank byte

const (
	Two Rank = iota + 2
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

func (r Rank) String() string {
	switch r {
	case Two:
		return "2"
	case Three:
		return "3"
	case Four:
		return "4"
	case Five:
		return "5"
	case Six:
		return "6"
	case Seven:
		return "7"
	case Eight:
		return "8"
	case Nine:
		return "9"
	case Ten:
		return "T"
	case Jack:
		return "J"
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	default:
		return "?"
	}
}

type Card struct {
	Rank Rank
	Suit Suit
}

func NewCard(rank Rank, suit Suit) Card {
	return Card{Rank: rank, Suit: suit}
}

func (c Card) String() string {
	return fmt.Sprintf("%s%s", c.Rank, c.Suit)
}

func (c Card) IsValid() bool {
	return c.Rank >= Two && c.Rank <= Ace &&
		c.Suit >= Spades && c.Suit <= Clubs
}
