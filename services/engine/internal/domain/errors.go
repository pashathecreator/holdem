package domain

import "errors"

var (
	ErrEmptyDeck      = errors.New("deck is empty")
	ErrNotEnoughCards = errors.New("not enough cards in deck")

	ErrInvalidAction      = errors.New("invalid action")
	ErrInvalidRaiseAmount = errors.New("raise amount is invalid")
	ErrPlayerNotActive    = errors.New("player is not active")
	ErrNotPlayerTurn      = errors.New("not player's turn")
	ErrInsufficientStack  = errors.New("insufficient stack")

	ErrGameNotFound   = errors.New("game not found")
	ErrTableNotFound  = errors.New("table not found")
	ErrPlayerNotFound = errors.New("player not found")

	ErrHandAlreadyStarted = errors.New("hand already started")
	ErrHandNotStarted     = errors.New("hand not started")
	ErrNotEnoughPlayers   = errors.New("not enough players to start hand")
)
