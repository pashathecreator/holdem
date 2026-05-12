package avroschema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSchemasMatchCanonicalFiles(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		fileName string
		embedded string
	}{
		{name: "hand_started", fileName: "hand_started.avsc", embedded: HandStarted()},
		{name: "player_acted", fileName: "player_acted.avsc", embedded: PlayerActed()},
		{name: "hand_ended", fileName: "hand_ended.avsc", embedded: HandEnded()},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			canonicalPath := filepath.Join("..", "..", "..", "..", "schemas", tc.fileName)
			canonical, err := os.ReadFile(canonicalPath)
			if err != nil {
				t.Fatalf("read canonical schema: %v", err)
			}

			if got, want := tc.embedded, string(canonical); got != want {
				t.Fatalf("embedded schema %s does not match canonical file %s", tc.name, tc.fileName)
			}
		})
	}
}
