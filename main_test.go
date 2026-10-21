package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/DMKEBUSINESSGMBH/dmkhunter/analyzer"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	scanner2 "github.com/DMKEBUSINESSGMBH/dmkhunter/scanner"
)

func TestRescanThenAnalyze(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "hunter.db")

	// more files than writeBatchSize, so rescan writes several transactions
	count := writeBatchSize + 500
	for i := range count {
		p := filepath.Join(root, fmt.Sprintf("file%05d", i))
		if err := os.WriteFile(p, []byte(p), 0600); err != nil {
			t.Fatal(err)
		}
	}

	scanner := scanner2.NewScanner([]string{root}, &[]string{`\.ignored$`})

	if err := saveHashes(dbPath, scanner); err != nil {
		t.Fatal(err)
	}

	changed := filepath.Join(root, "file00042")
	if err := os.WriteFile(changed, []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	added := filepath.Join(root, "added")
	if err := os.WriteFile(added, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.ignored"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	dbanalyzer, err := analyzer.NewDatabaseAnalyzer(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dbanalyzer.Close()

	stack := model.ViolationStack{}
	if err := analyzeFiles(scanner, []analyzer.Analyzer{dbanalyzer}, &stack); err != nil {
		t.Fatal(err)
	}

	reported := map[string]bool{}
	for _, v := range stack.All() {
		reported[v.Filepath] = true
	}

	if len(reported) != 2 || !reported[changed] || !reported[added] {
		t.Errorf("expected violations for %s and %s, got %v", changed, added, stack.All())
	}
}
