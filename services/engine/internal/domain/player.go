package domain

type PlayerStatus byte

const (
	PlayerStatusActive PlayerStatus = iota
	PlayerStatusFolded
	PlayerStatusAllIn
	PlayerStatusSittingOut
)

type Player struct {
	ID         PlayerID
	Stack      int
	HoleCards  [2]Card
	Status     PlayerStatus
	CurrentBet int
	Position   int
}

func (p *Player) IsActive() bool {
	return p.Status == PlayerStatusActive
}

func (p *Player) IsAllIn() bool {
	return p.Status == PlayerStatusAllIn
}

func (p *Player) IsFolded() bool {
	return p.Status == PlayerStatusFolded
}

func (p *Player) CanAct() bool {
	return p.Status == PlayerStatusActive && p.Stack > 0
}
