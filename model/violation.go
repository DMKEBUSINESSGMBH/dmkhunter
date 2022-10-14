package model

const (
	LEVEL_ERROR = "error"
	LEVEL_WARN  = "warn"
	LEVEL_INFO  = "info"
)

type Violation struct {
	Severity string
	Message  string
	Filepath string
}

type ViolationStack struct {
	violations []Violation
}

func (s *ViolationStack) Add(violation Violation) {
	s.violations = append(s.violations, violation)
}

func (s *ViolationStack) All() []Violation {
	return s.violations
}
