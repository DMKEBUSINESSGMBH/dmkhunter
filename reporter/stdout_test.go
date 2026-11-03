package reporter

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
)

func TestStdOut_SendText(t *testing.T) {
	longMessage := strings.Repeat("long message ", 20)
	longPath := "/" + strings.Repeat("nested/", 20) + "file.php"

	tests := []struct {
		name       string
		pretty     bool
		violations []model.Violation
		want       string
	}{
		{
			name: "empty stack",
		},
		{
			name:   "empty stack with pretty output",
			pretty: true,
		},
		{
			name: "single violation",
			violations: []model.Violation{{
				Severity: model.LEVEL_ERROR,
				Message:  "file has changed",
				Filepath: "/path/to/file",
			}},
			want: "Severity: error\nMessage: file has changed\nPath: /path/to/file\n",
		},
		{
			name:   "pretty output",
			pretty: true,
			violations: []model.Violation{{
				Severity: model.LEVEL_ERROR,
				Message:  "file has changed",
				Filepath: "/path/to/file",
			}},
			want: "┌──────────┬──────────────────┬───────────────┐\n" +
				"│ SEVERITY │     MESSAGE      │     PATH      │\n" +
				"├──────────┼──────────────────┼───────────────┤\n" +
				"│ error    │ file has changed │ /path/to/file │\n" +
				"└──────────┴──────────────────┴───────────────┘\n",
		},
		{
			name: "multiple violations with long fields",
			violations: []model.Violation{
				{Severity: model.LEVEL_ERROR, Message: longMessage, Filepath: longPath},
				{Severity: model.LEVEL_WARN, Message: "file is unreadable", Filepath: "/var/www/other.php"},
			},
			want: "Severity: error\nMessage: " + longMessage + "\nPath: " + longPath + "\n\n" +
				"Severity: warn\nMessage: file is unreadable\nPath: /var/www/other.php\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stack model.ViolationStack
			for _, violation := range tt.violations {
				stack.Add(violation)
			}

			got, err := captureStdout(t, func() error {
				return (StdOut{Pretty: tt.pretty}).Send(stack)
			})
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Send() output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStdOut_PrettyShortensMessageFromRight(t *testing.T) {
	for _, message := range []string{
		strings.Repeat("a", 50),
		strings.Repeat("a", 51),
		strings.Repeat("ä", 51),
	} {
		var stack model.ViolationStack
		stack.Add(model.Violation{Severity: model.LEVEL_ERROR, Message: message, Filepath: "/file"})
		output, err := captureStdout(t, func() error { return (StdOut{Pretty: true}).Send(stack) })
		if err != nil {
			t.Fatal(err)
		}
		want := message
		if chars := []rune(message); len(chars) > 50 {
			want = string(chars[:49]) + "…"
			if strings.Contains(output, message) {
				t.Errorf("unshortened message in output: %q", output)
			}
		}
		if !strings.Contains(output, want) {
			t.Errorf("expected message %q in output: %q", want, output)
		}
		if stack.All()[0].Message != message {
			t.Error("Send modified the original message")
		}
	}
}

func TestStdOut_PrettyShortensPathFromLeft(t *testing.T) {
	for _, path := range []string{
		strings.Repeat("a", 50),
		strings.Repeat("a", 51),
		strings.Repeat("ä", 51),
	} {
		var stack model.ViolationStack
		stack.Add(model.Violation{Severity: model.LEVEL_ERROR, Message: "changed", Filepath: path})
		output, err := captureStdout(t, func() error { return (StdOut{Pretty: true}).Send(stack) })
		if err != nil {
			t.Fatal(err)
		}
		want := path
		if chars := []rune(path); len(chars) > 50 {
			want = "…" + string(chars[len(chars)-49:])
			if strings.Contains(output, path) {
				t.Errorf("unshortened path in output: %q", output)
			}
		}
		if !strings.Contains(output, want) {
			t.Errorf("expected path %q in output: %q", want, output)
		}
		if stack.All()[0].Filepath != path {
			t.Error("Send modified the original path")
		}
	}
}

func TestStdOut_GroupsBySeverity(t *testing.T) {
	violations := []model.Violation{
		{Severity: model.LEVEL_WARN, Message: "warn first", Filepath: "/warn-first"},
		{Severity: model.LEVEL_INFO, Message: "info first", Filepath: "/info-first"},
		{Severity: model.LEVEL_ERROR, Message: "error first", Filepath: "/error-first"},
		{Severity: "zeta", Message: "zeta", Filepath: "/zeta"},
		{Severity: model.LEVEL_WARN, Message: "warn second", Filepath: "/warn-second"},
		{Severity: model.LEVEL_ERROR, Message: "error second", Filepath: "/error-second"},
		{Severity: "beta", Message: "beta first", Filepath: "/beta-first"},
		{Severity: model.LEVEL_INFO, Message: "info second", Filepath: "/info-second"},
		{Severity: "", Message: "severity missing", Filepath: "/missing-severity"},
		{Severity: "beta", Message: "beta second", Filepath: "/beta-second"},
	}

	var stack model.ViolationStack
	for _, violation := range violations {
		stack.Add(violation)
	}

	for _, pretty := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "pretty"}[pretty], func(t *testing.T) {
			output, err := captureStdout(t, func() error {
				return (StdOut{Pretty: pretty}).Send(stack)
			})
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			if pretty {
				if !strings.Contains(output, "SEVERITY") || !strings.Contains(output, "┌") {
					t.Fatalf("pretty output is not a table: %s", output)
				}
			} else {
				want := "Severity: error\nMessage: error first\nPath: /error-first\n\n" +
					"Message: error second\nPath: /error-second\n\n" +
					"Severity: warn\nMessage: warn first\nPath: /warn-first\n\n" +
					"Message: warn second\nPath: /warn-second\n\n" +
					"Severity: info\nMessage: info first\nPath: /info-first\n\n" +
					"Message: info second\nPath: /info-second\n\n" +
					"Severity: \nMessage: severity missing\nPath: /missing-severity\n\n" +
					"Severity: beta\nMessage: beta first\nPath: /beta-first\n\n" +
					"Message: beta second\nPath: /beta-second\n\n" +
					"Severity: zeta\nMessage: zeta\nPath: /zeta\n"
				if output != want {
					t.Errorf("text output = %q, want %q", output, want)
				}
			}

			orderedPaths := []string{
				"/error-first", "/error-second", "/warn-first", "/warn-second",
				"/info-first", "/info-second", "/missing-severity", "/beta-first",
				"/beta-second", "/zeta",
			}
			previous := -1
			for _, path := range orderedPaths {
				index := strings.Index(output, path)
				if index <= previous {
					t.Errorf("output does not group severities and preserve group order at %q: %s", path, output)
				}
				previous = index
			}

			if got := stack.All(); !reflect.DeepEqual(got, violations) {
				t.Errorf("Send() mutated violation order: got %v, want %v", got, violations)
			}
		})
	}
}

func TestStdOut_SendWriteError(t *testing.T) {
	var stack model.ViolationStack
	stack.Add(model.Violation{Severity: model.LEVEL_ERROR, Message: "changed", Filepath: "/file"})

	for _, pretty := range []bool{false, true} {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}

		previousStdout := os.Stdout
		os.Stdout = writer
		err = (StdOut{Pretty: pretty}).Send(stack)
		os.Stdout = previousStdout
		reader.Close()
		if err == nil {
			t.Errorf("Send() with pretty=%t did not report a write error", pretty)
		}
	}
}

func captureStdout(t *testing.T, send func() error) (string, error) {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	previousStdout := os.Stdout
	os.Stdout = writer
	sendErr := send()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	os.Stdout = previousStdout

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	return string(output), sendErr
}
