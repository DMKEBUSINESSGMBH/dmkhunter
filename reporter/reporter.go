package reporter

import "dmkhunter/model"

type Reporter interface {
	Send(stack model.ViolationStack) error
}


