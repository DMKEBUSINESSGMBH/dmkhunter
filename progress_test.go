package main

import (
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestVerbosityFlag(t *testing.T) {
	tests := map[string]int{
		"":      verbosityQuiet,
		"-v":    verbositySpinner,
		"-vv":   verbosityFiles,
		"-v -v": verbosityFiles,
	}

	for args, expected := range tests {
		level := verbosityQuiet
		flags := flag.NewFlagSet("test", flag.ContinueOnError)
		flags.Var(verbosityFlag{&level, 1}, "v", "")
		flags.Var(verbosityFlag{&level, 2}, "vv", "")

		if err := flags.Parse(append(strings.Fields(args), "analyze")); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		if level != expected {
			t.Errorf("%q: expected level %d, got %d", args, expected, level)
		}
		if flags.Arg(0) != "analyze" {
			t.Errorf("%q: the command after the flags got lost: %v", args, flags.Args())
		}
	}
}

func TestProgress(t *testing.T) {
	run := func(level int) string {
		var out strings.Builder
		p := newProgress(level, &out)
		p.File("/srv/first.php")
		p.File("/srv/second.php")
		// Start draws at once, so the output does not depend on timing
		p.Start("Analyzing")
		p.Error(errors.New("permission denied"))
		p.Stop()
		return out.String()
	}

	if out := run(verbosityQuiet); out != "permission denied\n" {
		t.Errorf("quiet: expected only the error, got %q", out)
	}

	out := run(verbositySpinner)
	if !strings.Contains(out, "· Analyzing… (0s)") || strings.Contains(out, "second.php") || !strings.Contains(out, "permission denied\n") {
		t.Errorf("spinner: expected spinner, label and error without files, got %q", out)
	}

	out = run(verbosityFiles)
	if !strings.Contains(out, "Analyzing… (0s) · 2 files · /srv/second.php") || !strings.Contains(out, "permission denied\n") {
		t.Errorf("files: expected label, count, current file and error, got %q", out)
	}
	if !strings.HasSuffix(out, clearLine) {
		t.Errorf("files: Stop should clear the progress line, got %q", out)
	}
	// a strings.Builder is no terminal
	if strings.Contains(out, colorReset) {
		t.Errorf("files: expected no color codes when not writing to a terminal, got %q", out)
	}
}

func TestProgress_Color(t *testing.T) {
	var out strings.Builder
	p := newProgress(verbositySpinner, &out)
	p.color = true
	p.Start("Rescanning")
	p.Stop()

	if !strings.Contains(out.String(), colorSpinner+"·"+colorReset+" Rescanning…") {
		t.Errorf("expected colored spinner, got %q", out.String())
	}
}

func TestProgress_StopWithoutStart(t *testing.T) {
	p := newProgress(verbosityFiles, io.Discard)
	p.Stop()
}

func TestShortenPath(t *testing.T) {
	if got := shortenPath("/short/path.php", 100); got != "/short/path.php" {
		t.Errorf("short path changed: %q", got)
	}

	long := "/" + strings.Repeat("ä", 80) + "/file.php"
	got := shortenPath(long, 40)

	if len(got) > 40 || !strings.HasPrefix(got, "...") || !strings.HasSuffix(got, "/file.php") {
		t.Errorf("unexpected shortened path %q (%d bytes)", got, len(got))
	}
	if !utf8.ValidString(got) {
		t.Errorf("shortened path is not valid UTF-8: %q", got)
	}
}
