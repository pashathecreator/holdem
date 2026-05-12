//go:build integration

package integration_test

import (
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
)

func deterministicShuffle(deck []domain.Card) domain.ShuffleFunc {
	return func(_ []domain.Card) []domain.Card {
		result := make([]domain.Card, len(deck))
		copy(result, deck)
		return result
	}
}

func orderedDeck(prefix []domain.Card) []domain.Card {
	used := make(map[string]struct{}, len(prefix))
	result := make([]domain.Card, 0, 52)

	for _, card := range prefix {
		result = append(result, card)
		used[card.String()] = struct{}{}
	}

	for _, card := range allCards() {
		if _, ok := used[card.String()]; ok {
			continue
		}
		result = append(result, card)
	}

	return result
}

func allCards() []domain.Card {
	suits := []domain.Suit{domain.Spades, domain.Hearts, domain.Diamonds, domain.Clubs}
	ranks := []domain.Rank{
		domain.Two,
		domain.Three,
		domain.Four,
		domain.Five,
		domain.Six,
		domain.Seven,
		domain.Eight,
		domain.Nine,
		domain.Ten,
		domain.Jack,
		domain.Queen,
		domain.King,
		domain.Ace,
	}

	cards := make([]domain.Card, 0, 52)
	for _, suit := range suits {
		for _, rank := range ranks {
			cards = append(cards, domain.NewCard(rank, suit))
		}
	}

	return cards
}

func stateStacks(state *enginev1.GameState) map[string]int64 {
	stacks := make(map[string]int64, len(state.Players))
	for _, player := range state.Players {
		stacks[player.Id] = player.Stack
	}
	return stacks
}

func skipIfDockerUnavailable(t *testing.T) {
	t.Helper()

	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = "unix:///var/run/docker.sock"
	}

	u, err := url.Parse(dockerHost)
	if err != nil {
		t.Skipf("skipping integration test: invalid DOCKER_HOST %q: %v", dockerHost, err)
	}

	switch u.Scheme {
	case "unix":
		if _, err := os.Stat(u.Path); err != nil {
			t.Skipf("skipping integration test: docker socket unavailable: %v", err)
		}

		conn, err := net.DialTimeout("unix", u.Path, time.Second)
		if err != nil {
			t.Skipf("skipping integration test: cannot connect to docker socket: %v", err)
		}
		_ = conn.Close()
	case "tcp", "http", "https":
		address := u.Host
		if !strings.Contains(address, ":") {
			address = net.JoinHostPort(address, "2375")
		}

		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			t.Skipf("skipping integration test: cannot connect to docker host %s: %v", address, err)
		}
		_ = conn.Close()
	default:
		t.Skipf("skipping integration test: unsupported docker host scheme %q", u.Scheme)
	}
}
