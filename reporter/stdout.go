package reporter

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"github.com/olekukonko/tablewriter"
)

type StdOut struct {
	Pretty bool
}

func (s StdOut) Send(stack model.ViolationStack) error {
	violations := append([]model.Violation(nil), stack.All()...)

	if 0 == len(violations) {
		return nil
	}
	sort.SliceStable(violations, func(i, j int) bool {
		left, right := severityOrder(violations[i].Severity), severityOrder(violations[j].Severity)
		if left != right {
			return left < right
		}
		if left == 3 && violations[i].Severity != violations[j].Severity {
			return violations[i].Severity < violations[j].Severity
		}
		return false
	})

	if s.Pretty {
		var output bytes.Buffer
		writer := tablewriter.NewWriter(&output)
		writer.Header("Severity", "Message", "Path")

		for _, v := range violations {
			message := v.Message
			if chars := []rune(message); len(chars) > 50 {
				message = string(chars[:49]) + "…"
			}
			path := v.Filepath
			if chars := []rune(path); len(chars) > 50 {
				path = "…" + string(chars[len(chars)-49:])
			}
			if err := writer.Append([]string{v.Severity, message, path}); err != nil {
				return err
			}
		}

		if err := writer.Render(); err != nil {
			return err
		}
		_, err := output.WriteTo(os.Stdout)
		return err
	}

	for i, v := range violations {
		if i > 0 {
			if _, err := fmt.Fprintln(os.Stdout); err != nil {
				return err
			}
		}

		if i == 0 || v.Severity != violations[i-1].Severity {
			if _, err := fmt.Fprintf(os.Stdout, "Severity: %s\n", v.Severity); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(os.Stdout, "Message: %s\nPath: %s\n", v.Message, v.Filepath); err != nil {
			return err
		}
	}

	return nil
}

func severityOrder(severity string) int {
	switch severity {
	case model.LEVEL_ERROR:
		return 0
	case model.LEVEL_WARN:
		return 1
	case model.LEVEL_INFO:
		return 2
	default:
		return 3
	}
}
