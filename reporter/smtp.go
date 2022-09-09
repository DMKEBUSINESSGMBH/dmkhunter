package reporter

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"net/smtp"
)

type MessageFormatter struct {
}

func (f MessageFormatter) Format(stack model.ViolationStack) []byte {
	msgBody := ""

	for _, violation := range stack.All() {
		msgBody += violation.Message + "\r\n"
	}

	return []byte("Subject: Summary\r\n\r\n" + msgBody)
}

func NewSmtpReporter(username string, passwords string, host string, recipients []string) SmtpReporter {
	return SmtpReporter{
		username:      username,
		password:      passwords,
		host:          host,
		recipientList: recipients,
	}
}

type SmtpReporter struct {
	username      string
	password      string
	host          string
	recipientList []string
}

func (s SmtpReporter) Send(stack model.ViolationStack) error {
	formatter := MessageFormatter{}

	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	err := smtp.SendMail(s.host, auth, "dmhunter", s.recipientList, formatter.Format(stack))

	return err
}
