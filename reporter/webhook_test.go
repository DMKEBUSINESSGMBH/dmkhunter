package reporter

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
)

func TestWebhookReporter_Send(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type %q", r.Header.Get("Content-Type"))
		}
		received, _ = io.ReadAll(r.Body)
	}))
	defer server.Close()

	violation := model.Violation{
		Severity: model.LEVEL_ERROR,
		Message:  "The file index.php has been changed!",
		Filepath: "/srv/www/index.php",
	}
	stack := model.ViolationStack{}
	stack.Add(violation)

	if err := NewWebhookReporter(server.URL).Send(stack); err != nil {
		t.Fatal(err)
	}

	var payload webhookPayload
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("invalid JSON %q: %v", received, err)
	}
	if payload.Count != 1 || !reflect.DeepEqual(payload.Violations, []model.Violation{violation}) {
		t.Errorf("unexpected payload %s", received)
	}

	var raw map[string][]map[string]string
	json.Unmarshal(received, &raw)
	if raw["violations"][0]["filepath"] != "/srv/www/index.php" {
		t.Errorf("expected lowercase JSON keys, got %s", received)
	}
}

func TestWebhookReporter_SendErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	stack := model.ViolationStack{}
	stack.Add(model.Violation{Message: "x"})

	if err := NewWebhookReporter(server.URL).Send(stack); err == nil {
		t.Error("expected an error for status 500")
	}
}
