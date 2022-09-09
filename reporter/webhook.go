package reporter

import (
	"bytes"
	"encoding/json"
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

func (w WebhookReporter) Send(s model.ViolationStack) error {
	body, err := json.Marshal(s)

	if err != nil {
		return err
	}

	if _, err = w.client.Post(w.url, "application/json", bytes.NewBuffer(body)); err != nil {
		return err
	}

	return nil
}
