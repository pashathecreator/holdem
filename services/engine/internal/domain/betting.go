package domain

type BettingStructure byte

const (
	BettingFixedLimit BettingStructure = iota
)

type BettingConfig struct {
	Structure          BettingStructure
	SmallBlind         int
	BigBlind           int
	SmallBet           int
	BigBet             int
	MaxRaisesPerStreet int
}

func ValidateAction(state *GameState, action Action) error {
	player := state.Players[state.ActivePlayer]

	if player.ID != action.PlayerID {
		return ErrNotPlayerTurn
	}

	if !player.CanAct() {
		return ErrPlayerNotActive
	}

	switch action.Type {
	case ActionCheck:
		if state.CurrentBet > player.CurrentBet {
			return ErrInvalidAction
		}

	case ActionCall:
		if state.CurrentBet == player.CurrentBet {
			return ErrInvalidAction
		}

	case ActionRaise:
		if state.RaisesThisStreet >= state.MaxRaisesPerStreet {
			return ErrRaiseCapReached
		}

		expectedRaiseTo := state.CurrentBet + state.BetSizeForStreet()
		if action.Amount != expectedRaiseTo {
			return ErrInvalidRaiseAmount
		}

		if action.Amount > player.Stack+player.CurrentBet {
			return ErrInsufficientStack
		}

	case ActionFold:

	case ActionAllIn:
		return ErrInvalidAction

	default:
		return ErrInvalidAction
	}

	return nil
}

func ApplyAction(state *GameState, action Action) {
	player := state.Players[state.ActivePlayer]

	switch action.Type {
	case ActionFold:
		player.Status = PlayerStatusFolded

	case ActionCheck:

	case ActionCall:
		amount := state.CurrentBet - player.CurrentBet
		if amount >= player.Stack {
			amount = player.Stack
			player.Status = PlayerStatusAllIn
		}
		player.Stack -= amount
		player.CurrentBet += amount

	case ActionRaise:
		amount := action.Amount - player.CurrentBet
		player.Stack -= amount
		player.CurrentBet = action.Amount
		state.CurrentBet = action.Amount
		state.RaisesThisStreet++

		if player.Stack == 0 {
			player.Status = PlayerStatusAllIn
		}

	case ActionAllIn:
	}
}

func IsBettingRoundOver(state *GameState) bool {
	for _, p := range state.Players {
		if !p.CanAct() {
			continue
		}
		if p.CurrentBet < state.CurrentBet {
			return false
		}
	}
	return true
}

func NextActivePlayer(state *GameState) int {
	n := len(state.Players)
	for i := 1; i <= n; i++ {
		idx := (state.ActivePlayer + i) % n
		if state.Players[idx].CanAct() {
			return idx
		}
	}
	return state.ActivePlayer
}

func FirstActiveAfterButton(state *GameState) int {
	n := len(state.Players)
	for i := 1; i <= n; i++ {
		idx := (state.Button + i) % n
		if state.Players[idx].IsActive() || state.Players[idx].IsAllIn() {
			return idx
		}
	}
	return state.Button
}

func CollectBets(state *GameState) {
	contributions := make(map[PlayerID]int, len(state.Players))
	for _, p := range state.Players {
		if p.CurrentBet > 0 {
			contributions[p.ID] = p.CurrentBet
			p.CurrentBet = 0
		}
	}

	newPots := Calculate(contributions)
	if len(state.Pots) == 0 {
		state.Pots = newPots
		return
	}

	state.Pots = mergePots(state.Pots, newPots)
}

func mergePots(existing, newPots []Pot) []Pot {
	if len(existing) == 0 {
		return newPots
	}
	if len(newPots) == 0 {
		return existing
	}

	existing[len(existing)-1].Amount += newPots[0].Amount
	return append(existing, newPots[1:]...)
}