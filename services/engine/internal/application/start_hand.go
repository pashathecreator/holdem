package application

import (
	"context"
	"fmt"
	"time"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/metrics"
)

type startHandGameRepository interface {
	Save(ctx context.Context, state *domain.GameState) error
}

type startHandEventPublisher interface {
	PublishHandStarted(ctx context.Context, event domain.HandStartedEvent) error
}

type StartHandInput struct {
	TableID    domain.TableID
	Players    []*domain.Player
	Button     int
	SmallBlind int
	BigBlind   int
}

type StartHand struct {
	repo      startHandGameRepository
	publisher startHandEventPublisher
	shuffle   domain.ShuffleFunc
}

func NewStartHand(repo startHandGameRepository, publisher startHandEventPublisher, shuffle domain.ShuffleFunc) *StartHand {
	return &StartHand{repo: repo, publisher: publisher, shuffle: shuffle}
}

func (uc *StartHand) Execute(ctx context.Context, input StartHandInput) (*domain.GameState, error) {
	if err := validateStartHandInput(input); err != nil {
		return nil, err
	}

	deck := domain.NewDeck(uc.shuffle)
	deck.Shuffle()

	if err := dealHoleCards(deck, input.Players); err != nil {
		return nil, err
	}

	sbIndex := (input.Button + 1) % len(input.Players)
	bbIndex := (input.Button + 2) % len(input.Players)

	if err := postBlinds(input.Players, sbIndex, bbIndex, input.SmallBlind, input.BigBlind); err != nil {
		return nil, err
	}

	state := &domain.GameState{
		ID:           domain.HandID(fmt.Sprintf("%d", time.Now().UnixNano())),
		TableID:      input.TableID,
		Players:      input.Players,
		Board:        make([]domain.Card, 0, 5),
		Pots:         []domain.Pot{},
		Street:       domain.StreetPreflop,
		CurrentBet:   input.BigBlind,
		ActivePlayer: nextActiveIndex(bbIndex, input.Players),
		Button:       input.Button,
		SmallBlind:   input.SmallBlind,
		BigBlind:     input.BigBlind,
		Deck:         deck,
	}

	if err := uc.repo.Save(ctx, state); err != nil {
		return nil, fmt.Errorf("start hand: save state: %w", err)
	}

	playerIDs := make([]domain.PlayerID, len(input.Players))
	for i, p := range input.Players {
		playerIDs[i] = p.ID
	}

	if err := uc.publisher.PublishHandStarted(ctx, domain.HandStartedEvent{
		HandID:     state.ID,
		TableID:    state.TableID,
		Players:    playerIDs,
		Button:     input.Button,
		SmallBlind: input.SmallBlind,
		BigBlind:   input.BigBlind,
		OccurredAt: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("start hand: publish event: %w", err)
	}

	metrics.HandsStarted.Inc()
	metrics.ActiveGames.Inc()
	metrics.PlayersPerHand.Observe(float64(len(input.Players)))

	return state, nil
}

func validateStartHandInput(input StartHandInput) error {
	if len(input.Players) < 2 {
		return domain.ErrNotEnoughPlayers
	}
	if input.SmallBlind <= 0 || input.BigBlind <= 0 {
		return fmt.Errorf("blinds must be positive")
	}
	if input.BigBlind != input.SmallBlind*2 {
		return fmt.Errorf("big blind must be twice the small blind")
	}
	if input.Button < 0 || input.Button >= len(input.Players) {
		return fmt.Errorf("invalid button position")
	}
	return nil
}

func dealHoleCards(deck *domain.Deck, players []*domain.Player) error {
	for _, p := range players {
		cards, err := deck.Deal(2)
		if err != nil {
			return fmt.Errorf("deal hole cards: %w", err)
		}
		p.HoleCards = [2]domain.Card{cards[0], cards[1]}
		p.Status = domain.PlayerStatusActive
	}
	return nil
}

func postBlinds(players []*domain.Player, sbIndex, bbIndex, sb, bb int) error {
	sbPlayer := players[sbIndex]
	if sbPlayer.Stack < sb {
		return domain.ErrInsufficientStack
	}
	sbPlayer.Stack -= sb
	sbPlayer.CurrentBet = sb

	bbPlayer := players[bbIndex]
	if bbPlayer.Stack < bb {
		return domain.ErrInsufficientStack
	}
	bbPlayer.Stack -= bb
	bbPlayer.CurrentBet = bb

	return nil
}

func nextActiveIndex(bbIndex int, players []*domain.Player) int {
	n := len(players)
	for i := 1; i <= n; i++ {
		idx := (bbIndex + i) % n
		if players[idx].CanAct() {
			return idx
		}
	}
	return bbIndex
}