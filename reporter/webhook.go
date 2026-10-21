package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"net/http"
)

type WebhookReporter struct {
	url    string
	client http.Client
}

func NewWebhookReporter(url string) WebhookReporter {
	client := http.Client{}

	return WebhookReporter{
		client: client,
		url:    url,
	}
}

// webhookPayload is the JSON body posted to the webhook, ViolationStack itself has no exported fields.
type webhookPayload struct {
	Count      int               `json:"count"`
	Violations []model.Violation `json:"violations"`
}

func (w WebhookReporter) Send(s model.ViolationStack) error {
	body, err := json.Marshal(webhookPayload{
		Count:      s.Count(),
		Violations: s.All(),
	})

	if err != nil {
		return err
	}

	resp, err := w.client.Post(w.url, "application/json", bytes.NewBuffer(body))

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("webhook %s answered with %s", w.url, resp.Status)
	}

	return nil
}
