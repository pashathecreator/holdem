package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/pashathecreator/holdem/services/auth-service/internal/domain"
)

type Publisher struct {
	writer *kafka.Writer
	topic  string
}

type userCreatedMessage struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

func NewPublisher(brokers []string, topic string) *Publisher {
	if len(brokers) == 0 || topic == "" {
		return &Publisher{}
	}
	return &Publisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
		},
		topic: topic,
	}
}

func (p *Publisher) PublishUserCreated(ctx context.Context, user *domain.User) error {
	if p == nil || p.writer == nil || user == nil {
		return nil
	}

	payload, err := json.Marshal(userCreatedMessage{
		UserID:    user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: p.topic,
		Key:   []byte(user.ID),
		Value: payload,
		Time:  time.Now().UTC(),
	})
}

func (p *Publisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
