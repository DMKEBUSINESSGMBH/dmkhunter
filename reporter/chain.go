package reporter

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
)

type ChainReporter struct {
	reporters []Reporter
}

func (c *ChainReporter) Send(stack model.ViolationStack) error {
	for _, reporter := range c.reporters {
		err := reporter.Send(stack)

		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ChainReporter) Add(reporter Reporter) {
	c.reporters = append(c.reporters, reporter)
}
