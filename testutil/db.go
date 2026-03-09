package testutil

import (
	"path/filepath"
	"testing"

	"github.com/seanmeyer/opportunity-hunter/storage"
)

// NewTestDB creates an in-memory SQLite database for testing.
func NewTestDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
