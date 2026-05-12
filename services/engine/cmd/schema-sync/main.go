package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root, err := filepath.Abs(".")
	if err != nil {
		fail("resolve engine root", err)
	}

	sourceDir := filepath.Join(root, "schemas")
	targetDir := filepath.Join(root, "internal", "delivery", "kafka", "avroschema", "assets")

	files := []string{
		"hand_started.avsc",
		"player_acted.avsc",
		"hand_ended.avsc",
	}

	for _, fileName := range files {
		sourcePath := filepath.Join(sourceDir, fileName)
		targetPath := filepath.Join(targetDir, fileName)

		data, err := os.ReadFile(sourcePath)
		if err != nil {
			fail(fmt.Sprintf("read %s", sourcePath), err)
		}

		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			fail(fmt.Sprintf("write %s", targetPath), err)
		}
	}
}

func fail(step string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", step, err)
	os.Exit(1)
}
