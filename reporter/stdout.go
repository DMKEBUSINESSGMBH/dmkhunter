package reporter

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"github.com/olekukonko/tablewriter"
	"os"
)

type StdOut struct {
}

func (s StdOut) Send(stack model.ViolationStack) error {
	violations := stack.All()

	if 0 == len(violations) {
		return nil
	}

	writer := tablewriter.NewWriter(os.Stdout)
	writer.SetHeader([]string{"Severity", "Message", "Path"})

	for _, v := range violations {
		writer.Append([]string{
			v.Severity,
			v.Message,
			v.Filepath,
		})
	}

	writer.Render()

	return nil
}
