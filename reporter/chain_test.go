package reporter

import (
	"errors"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"testing"
)

type StdOutMock struct {
}

// StdOut Mock returns violation stack as error message
func (s StdOutMock) Send(stack model.ViolationStack) error {
	var errText string
	for _, k := range stack.All() {
		errText += k.Severity + " " + k.Filepath + " " + k.Message + "\n"
	}

	return errors.New(errText)
}

func TestChainReporter_Add(t *testing.T) {
	chain := ChainReporter{}
	chain.Add(StdOut{})

	if len(chain.reporters) != 1 {
		t.Errorf("chain length does not matter: %v", chain.reporters)
	}
}

func TestStdOut_Send(t *testing.T) {
	chain := ChainReporter{}
	stdOutMock := StdOutMock{}
	chain.Add(stdOutMock)

	violationStack := model.ViolationStack{}
	violationStack.Add(model.Violation{
		Severity: model.LEVEL_ERROR,
		Message:  "file has changed",
		Filepath: "/path/to/file",
	})
	violationStack.Add(model.Violation{
		Severity: model.LEVEL_INFO,
		Message:  "test end",
		Filepath: "/",
	})
	errorResult := chain.Send(violationStack)

	if len(errorResult.Error()) == 0 {
		t.Errorf("send result to output is empty: %v", errorResult)
	}

	if errorResult.Error() != "error /path/to/file file has changed\ninfo / test end\n" {
		t.Errorf("output error text is wrong: %s", errorResult.Error())
	}
}
