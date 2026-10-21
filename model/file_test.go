package model

import (
	"bytes"
	"crypto/md5"
	"os"
	"path/filepath"
	"testing"
)

func TestFile_Hash(t *testing.T) {
	dir := t.TempDir()

	tests := map[string][]byte{
		"empty":        {},
		"small":        []byte("hello dmkhunter"),
		"multi-buffer": bytes.Repeat([]byte("0123456789"), hashBufferSize/3),
	}

	for name, content := range tests {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, content, 0600); err != nil {
			t.Fatal(err)
		}

		hash, err := NewFile(p).Hash()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		expected := md5.Sum(content)
		if !bytes.Equal(hash, expected[:]) {
			t.Errorf("%s: hash mismatch\nexp: %x\ngot: %x", name, expected, hash)
		}
	}
}

func TestFile_HashMissingFile(t *testing.T) {
	if _, err := NewFile(filepath.Join(t.TempDir(), "missing")).Hash(); err == nil {
		t.Error("expected an error for a missing file")
	}
}
