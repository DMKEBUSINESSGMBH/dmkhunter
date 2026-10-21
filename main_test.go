package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	quiet := newProgress(verbosityQuiet, io.Discard)

	if err := saveHashes(dbPath, scanner, quiet); err != nil {
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
	if err := analyzeFiles(scanner, []analyzer.Analyzer{dbanalyzer}, &stack, quiet); err != nil {
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

func TestConfirmDeleteDatabases(t *testing.T) {
	dir := t.TempDir()
	deleted := filepath.Join(dir, "deleted.db")
	kept := filepath.Join(dir, "kept.db")
	missing := filepath.Join(dir, "missing.db")
	noAnswer := filepath.Join(dir, "no-answer.db")

	for _, p := range []string{deleted, kept, noAnswer} {
		if err := os.WriteFile(p, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	// missing.db gets no prompt, so "n" answers kept.db, and no-answer.db hits EOF
	in := strings.NewReader("Yes\nn\n")
	var out strings.Builder

	if err := confirmDeleteDatabases([]string{deleted, missing, kept, noAnswer}, in, &out); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(deleted); !os.IsNotExist(err) {
		t.Errorf("%s should be deleted", deleted)
	}
	for _, p := range []string{kept, noAnswer} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should be kept: %v", p, err)
		}
	}
	if strings.Contains(out.String(), missing) {
		t.Errorf("no prompt expected for missing database, got:\n%s", out.String())
	}
}

func TestSplitGlobalFlags(t *testing.T) {
	tests := []struct {
		args        string
		flags, rest string
		verbosity   int
		configFile  string
	}{
		{"analyze -vv", "-vv", "analyze", verbosityFiles, ".hunter.conf"},
		{"-v analyze", "-v", "analyze", verbositySpinner, ".hunter.conf"},
		{"-v analyze -v", "-v -v", "analyze", verbosityFiles, ".hunter.conf"},
		{"--vv rescan", "--vv", "rescan", verbosityFiles, ".hunter.conf"},
		{"list -prefix /srv -config x.conf -limit 5 -v", "-config x.conf -v", "list -prefix /srv -limit 5", verbositySpinner, "x.conf"},
		{"list -config=x.conf -contains foo", "-config=x.conf", "list -contains foo", verbosityQuiet, "x.conf"},
		{"analyze -- -v", "", "analyze -v", verbosityQuiet, ".hunter.conf"},
		{"-x analyze", "", "-x analyze", verbosityQuiet, ".hunter.conf"},
	}

	for _, test := range tests {
		verbosity := verbosityQuiet
		var configFile string
		globals := flag.NewFlagSet("test", flag.ContinueOnError)
		globals.StringVar(&configFile, "config", ".hunter.conf", "")
		globals.Var(verbosityFlag{&verbosity, 1}, "v", "")
		globals.Var(verbosityFlag{&verbosity, 2}, "vv", "")

		flags, rest := splitGlobalFlags(strings.Fields(test.args), globals)

		if strings.Join(flags, " ") != test.flags || strings.Join(rest, " ") != test.rest {
			t.Errorf("%q: expected flags %q and rest %q, got %q and %q", test.args, test.flags, test.rest, flags, rest)
			continue
		}
		if err := globals.Parse(flags); err != nil {
			t.Fatalf("%q: %v", test.args, err)
		}
		if verbosity != test.verbosity || configFile != test.configFile {
			t.Errorf("%q: expected verbosity %d and config %q, got %d and %q", test.args, test.verbosity, test.configFile, verbosity, configFile)
		}
	}
}
