package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/segmentio/kafka-go"

	"github.com/pashathecreator/holdem/services/wallet-service/internal/application"
)

type Consumer struct {
	reader  *kafka.Reader
	service *application.Service
}

type userCreatedMessage struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

func NewConsumer(brokers []string, topic, group string, service *application.Service) *Consumer {
	if len(brokers) == 0 || topic == "" || group == "" {
		return &Consumer{service: service}
	}
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   topic,
			GroupID: group,
		}),
		service: service,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	if c == nil || c.reader == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("fetch kafka message: %w", err)
		}

		var payload userCreatedMessage
		if err := json.Unmarshal(msg.Value, &payload); err == nil {
			_ = c.service.ProvisionUser(ctx, payload.UserID, payload.Email)
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("commit kafka message: %w", err)
		}
	}
}

func (c *Consumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}
	return c.reader.Close()
}
