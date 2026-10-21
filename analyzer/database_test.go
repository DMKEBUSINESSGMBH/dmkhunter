package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"go.etcd.io/bbolt"
)

func writeFile(t *testing.T, p, content string) *model.File {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return model.NewFile(p)
}

func TestDatabaseAnalyzer_Analyze(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hunter.db")

	unchanged := writeFile(t, filepath.Join(dir, "unchanged"), "same")
	changed := writeFile(t, filepath.Join(dir, "changed"), "before")
	added := writeFile(t, filepath.Join(dir, "added"), "new")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
		for _, f := range []*model.File{unchanged, changed} {
			hash, err := f.Hash()
			if err != nil {
				return err
			}
			if err := b.Put([]byte(f.Path), hash); err != nil {
				return err
			}
		}
		return nil
	})
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, changed.Path, "after")

	a, err := NewDatabaseAnalyzer(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	// the analyzer is shared between workers, so run it concurrently (go test -race)
	stacks := make([]model.ViolationStack, 3)
	var wg sync.WaitGroup
	for i, f := range []*model.File{unchanged, changed, added} {
		wg.Go(func() {
			if err := a.Analyze(*f, &stacks[i]); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()

	if stacks[0].Count() != 0 {
		t.Errorf("unchanged file reported: %v", stacks[0].All())
	}
	if stacks[1].Count() != 1 || !strings.Contains(stacks[1].All()[0].Message, "has been changed") {
		t.Errorf("changed file not reported: %v", stacks[1].All())
	}
	if stacks[2].Count() != 1 || !strings.Contains(stacks[2].All()[0].Message, "New file detected") {
		t.Errorf("new file not reported: %v", stacks[2].All())
	}
}

func TestNewDatabaseAnalyzer_MissingBucket(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	if _, err := NewDatabaseAnalyzer(dbPath); err == nil {
		t.Error("expected an error for a database without file hashes")
	}
}

func TestNewDatabaseAnalyzer_MissingDatabase(t *testing.T) {
	if _, err := NewDatabaseAnalyzer(filepath.Join(t.TempDir(), "missing.db")); err == nil {
		t.Error("expected an error for a missing database")
	}
}
