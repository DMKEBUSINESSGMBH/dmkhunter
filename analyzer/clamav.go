package analyzer

import (
	"dmkhunter/model"
	"github.com/dutchcoders/go-clamd"
	"log"
)

type ClamAVAnalyzer struct {
	clamd *clamd.Clamd
}

func NewClamAVAnalyzer(address string) ClamAVAnalyzer{
	cl := clamd.NewClamd(address)

	return ClamAVAnalyzer{
		clamd: cl,
	}
}

func (a ClamAVAnalyzer) Analyze(f model.File, stack model.ViolationStack) {
	ch, err := a.clamd.ScanFile(f.Path)

	if err != nil {
		log.Fatal(err)
	}

	response := <- ch

	if clamd.RES_FOUND == response.Status {
		stack.Add(model.Violation{
			Message:  response.Description,
			Filepath: f.Path,
			Severity: model.LEVEL_ERROR,
		})
	}
}