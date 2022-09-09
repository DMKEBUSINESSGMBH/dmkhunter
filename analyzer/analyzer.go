package analyzer

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
)

type Analyzer interface {
	Analyze(file model.File, stack model.ViolationStack) error
}
