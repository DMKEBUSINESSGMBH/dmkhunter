package reporter

import "github.com/DMKEBUSINESSGMBH/dmkhunter/model"

type Reporter interface {
	Send(stack model.ViolationStack) error
}
