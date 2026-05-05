package domain

type Street byte

const (
	StreetPreflop Street = iota
	StreetFlop
	StreetTurn
	StreetRiver
	StreetShowdown
)

type GameState struct {
	ID           HandID
	TableID      TableID
	Players      []*Player
	Board        []Card
	Pots         []Pot
	Street       Street
	CurrentBet   int
	ActivePlayer int
	Button       int

	BettingConfig
	RaisesThisStreet int

	Deck *Deck
}

func (g *GameState) BetSizeForStreet() int {
	switch g.Street {
	case StreetPreflop, StreetFlop:
		return g.SmallBet
	case StreetTurn, StreetRiver:
		return g.BigBet
	default:
		return 0
	}
}

func (g *GameState) ActivePlayers() []*Player {
	result := make([]*Player, 0, len(g.Players))
	for _, p := range g.Players {
		if p.IsActive() || p.IsAllIn() {
			result = append(result, p)
		}
	}
	return result
}

func (g *GameState) PlayersWhoCanAct() []*Player {
	result := make([]*Player, 0, len(g.Players))
	for _, p := range g.Players {
		if p.CanAct() {
			result = append(result, p)
		}
	}
	return result
}

func (g *GameState) PlayerByID(id PlayerID) *Player {
	for _, p := range g.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (g *GameState) IsHandOver() bool {
	return len(g.PlayersWhoCanAct()) <= 1
}