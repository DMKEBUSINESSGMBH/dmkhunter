package lister

import (
	"path/filepath"
	"reflect"
	"testing"

	"go.etcd.io/bbolt"
)

func TestReadHashes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hunter.db")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucket([]byte("file_hashes"))
		if err != nil {
			return err
		}
		for _, p := range []string{"/a/1", "/a/2", "/a/3", "/ab/1", "/b/1"} {
			if err := b.Put([]byte(p), []byte{1}); err != nil {
				return err
			}
		}
		return nil
	})
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	collect := func(prefix string, max int) []string {
		var paths []string
		err := ReadHashes(dbPath, prefix, func(path string, hash []byte) bool {
			paths = append(paths, path)
			return max == 0 || len(paths) < max
		})
		if err != nil {
			t.Fatal(err)
		}
		return paths
	}

	if got := collect("", 0); len(got) != 5 {
		t.Errorf("expected all 5 records, got %v", got)
	}
	if got, exp := collect("/a/", 0), []string{"/a/1", "/a/2", "/a/3"}; !reflect.DeepEqual(got, exp) {
		t.Errorf("prefix /a/: expected %v, got %v", exp, got)
	}
	if got, exp := collect("/a", 2), []string{"/a/1", "/a/2"}; !reflect.DeepEqual(got, exp) {
		t.Errorf("prefix /a with limit 2: expected %v, got %v", exp, got)
	}
	if got := collect("/c", 0); len(got) != 0 {
		t.Errorf("prefix /c: expected no records, got %v", got)
	}
}

func TestReadHashes_MissingBucket(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := ReadHashes(dbPath, "", func(string, []byte) bool { return true }); err == nil {
		t.Error("expected an error for a database without file hashes")
	}
}
