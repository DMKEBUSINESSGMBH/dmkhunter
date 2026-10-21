package reporter

import (
	"strings"
	"testing"

	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
)

func TestMessageFormatter_Format(t *testing.T) {
	stack := model.ViolationStack{}
	stack.Add(model.Violation{
		Severity: model.LEVEL_ERROR,
		Message:  "The file index.php has been changed!",
		Filepath: "/srv/www/index.php",
	})

	body := string(MessageFormatter{topic: "host"}.Format(stack))

	if !strings.Contains(body, "Subject: Alert from host (1 changes)") {
		t.Errorf("missing subject in %q", body)
	}
	if !strings.Contains(body, "The file index.php has been changed! (/srv/www/index.php)\r\n") {
		t.Errorf("expected message with full path in %q", body)
	}
}
