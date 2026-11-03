package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
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

func TestAnalyzeOutputFormats(t *testing.T) {
	root := t.TempDir()
	database := filepath.Join(t.TempDir(), "hunter.db")
	confPath := filepath.Join(t.TempDir(), "hunter.conf")
	quiet := newProgress(verbosityQuiet, io.Discard)
	scanner := scanner2.NewScanner([]string{root}, &[]string{})
	if err := saveHashes(database, scanner, quiet); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "new.php")
	if err := os.WriteFile(file, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	configuration := fmt.Sprintf("[[Presets]]\npaths = [%q]\ndatabase = %q\n", root, database)
	if err := os.WriteFile(confPath, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		args   []string
		pretty bool
	}{
		{name: "default text", args: []string{"analyze"}},
		{name: "pretty before command", args: []string{"--pretty", "analyze"}, pretty: true},
		{name: "pretty after command", args: []string{"analyze", "--pretty"}, pretty: true},
		{name: "pretty disabled", args: []string{"analyze", "--pretty=false"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestCLIHelperProcess$", "--", "-config", confPath}, tt.args...)
			cmd := exec.Command(executable, args...)
			cmd.Env = append(os.Environ(), "DMKHUNTER_CLI_HELPER=1")
			output, err := cmd.Output()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != exitViolations {
				t.Fatalf("analyze error = %v, output = %q", err, output)
			}

			if tt.pretty {
				path := file
				if chars := []rune(path); len(chars) > 50 {
					path = "…" + string(chars[len(chars)-49:])
				}
				for _, part := range []string{"┌", "SEVERITY", "MESSAGE", "PATH", "New file detected: new.php", path} {
					if !strings.Contains(string(output), part) {
						t.Errorf("table output is missing %q: %s", part, output)
					}
				}
			} else {
				want := "Severity: error\nMessage: New file detected: new.php\nPath: " + file + "\n"
				if string(output) != want {
					t.Errorf("text output = %q, want %q", output, want)
				}
			}
		})
	}
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("DMKHUNTER_CLI_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"dmkhunter"}, os.Args[i+1:]...)
			flag.CommandLine = flag.NewFlagSet("dmkhunter", flag.ExitOnError)
			main()
			return
		}
	}
	t.Fatal("missing CLI arguments")
}
