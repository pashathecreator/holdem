package kafka

import (
	"testing"

	"github.com/linkedin/goavro/v2"
)

func TestPublisherSchemasContainExpectedTopics(t *testing.T) {
	t.Parallel()

	schemas := publisherSchemas()

	expectedTopics := []string{
		topicHandStarted,
		topicPlayerActed,
		topicHandEnded,
	}

	if len(schemas) != len(expectedTopics) {
		t.Fatalf("unexpected schemas count: got %d want %d", len(schemas), len(expectedTopics))
	}

	for _, topic := range expectedTopics {
		if _, ok := schemas[topic]; !ok {
			t.Fatalf("missing schema for topic %s", topic)
		}
	}
}

func TestPublisherSchemasCompileAsAvro(t *testing.T) {
	t.Parallel()

	for topic, schema := range publisherSchemas() {
		if _, err := goavro.NewCodec(schema); err != nil {
			t.Fatalf("compile schema for topic %s: %v", topic, err)
		}
	}
}
