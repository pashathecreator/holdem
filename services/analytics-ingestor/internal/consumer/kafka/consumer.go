package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/config"
	decoderavro "github.com/pashathecreator/holdem/services/analytics-ingestor/internal/decoder/avro"
	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/model"
)

type decoder interface {
	Decode(ctx context.Context, value []byte, meta decoderavro.MessageMeta) (model.DecodedEvent, error)
}

type storage interface {
	InsertEvent(ctx context.Context, event model.DecodedEvent) error
}

type Consumer struct {
	readers []*kafka.Reader
	decoder decoder
	storage storage
}

func NewConsumer(cfg config.KafkaConfig, decoder decoder, storage storage) (*Consumer, error) {
	if len(cfg.Topics) == 0 {
		return nil, fmt.Errorf("kafka consumer: no topics configured")
	}

	readers := make([]*kafka.Reader, 0, len(cfg.Topics))
	for _, topic := range cfg.Topics {
		readers = append(readers, kafka.NewReader(kafka.ReaderConfig{
			Brokers:                cfg.Brokers,
			GroupID:                cfg.ConsumerGroup,
			Topic:                  topic,
			MinBytes:               1,
			MaxBytes:               10e6,
			CommitInterval:         0,
			MaxWait:                time.Second,
			ReadLagInterval:        -1,
			WatchPartitionChanges:  true,
			PartitionWatchInterval: 5 * time.Second,
		}))
	}

	return &Consumer{
		readers: readers,
		decoder: decoder,
		storage: storage,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	errCh := make(chan error, len(c.readers))
	var wg sync.WaitGroup

	for _, reader := range c.readers {
		wg.Add(1)
		go func(reader *kafka.Reader) {
			defer wg.Done()
			if err := c.consumeTopic(ctx, reader); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}
		}(reader)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		return ctx.Err()
	case err := <-errCh:
		return err
	case <-done:
		return nil
	}
}

func (c *Consumer) consumeTopic(ctx context.Context, reader *kafka.Reader) error {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			return err
		}

		event, err := c.decoder.Decode(ctx, msg.Value, decoderavro.MessageMeta{
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
		})
		if err != nil {
			return fmt.Errorf("decode kafka message %s[%d]@%d: %w", msg.Topic, msg.Partition, msg.Offset, err)
		}

		if err := c.storage.InsertEvent(ctx, event); err != nil {
			return fmt.Errorf("persist kafka message %s[%d]@%d: %w", msg.Topic, msg.Partition, msg.Offset, err)
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			return fmt.Errorf("commit kafka message %s[%d]@%d: %w", msg.Topic, msg.Partition, msg.Offset, err)
		}
	}
}

func (c *Consumer) Close() error {
	var combined error
	for _, reader := range c.readers {
		if err := reader.Close(); err != nil {
			combined = errors.Join(combined, err)
		}
	}
	return combined
}
