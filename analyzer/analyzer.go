package analyzer

import (
	"dmkhunter/model"
)

type Analyzer interface {
	Analyze(file model.File, stack model.ViolationStack) error
}
