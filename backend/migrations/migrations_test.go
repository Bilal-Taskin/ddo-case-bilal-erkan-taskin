package migrations_test

import (
	"sort"
	"strings"
	"testing"

	"fitgiris/migrations"
)

func TestEmbeddedMigrations(t *testing.T) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("Embedded migrations okunamadi: %v", err)
	}

	var sqlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			sqlFiles = append(sqlFiles, entry.Name())
		}
	}

	expectedCount := 6
	if len(sqlFiles) != expectedCount {
		t.Errorf("Beklenen sql dosya sayisi %d, bulunan %d", expectedCount, len(sqlFiles))
	}

	sort.Strings(sqlFiles)

	for _, file := range sqlFiles {
		content, err := migrations.FS.ReadFile(file)
		if err != nil {
			t.Errorf("Dosya okunamadi %s: %v", file, err)
		}
		if len(content) == 0 {
			t.Errorf("Migrasyon dosyasi bos olamaz: %s", file)
		}
	}
}
