package domain

type ActionType byte

const (
	ActionFold ActionType = iota 
	ActionCheck 
	ActionCall 
	ActionRaise
	ActionAllIn
)

func (a ActionType) String() string {
	switch a {
	case ActionFold:
		return "fold"
	case ActionCheck:
		return "check"
	case ActionCall:
		return "call"
	case ActionRaise:
		return "raise"
	case ActionAllIn:
		return "all-in"
	default:
		return "unknown"
	}
}

type Action struct {
	PlayerID PlayerID
	Type     ActionType
	Amount   int
}


func NewFold(playerID PlayerID) Action {
	return Action{PlayerID: playerID, Type: ActionFold}
}

func NewCheck(playerID PlayerID) Action {
	return Action{PlayerID: playerID, Type: ActionCheck}
}

func NewCall(playerID PlayerID) Action {
	return Action{PlayerID: playerID, Type: ActionCall}
}

func NewRaise(playerID PlayerID, amount int) Action {
	return Action{PlayerID: playerID, Type: ActionRaise, Amount: amount}
}

func NewAllIn(playerID PlayerID) Action {
	return Action{PlayerID: playerID, Type: ActionAllIn}
}
