package reporter

import (
	"fmt"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"net/smtp"
)

type MessageFormatter struct {
	topic string
}

func (f MessageFormatter) Format(stack model.ViolationStack) []byte {
	msgBody := ""
	header := "Subject: Alert from %s (%d changes)\r\n\r\n"
	header = fmt.Sprintf(header, f.topic, len(stack.All()))

	for _, violation := range stack.All() {
		msgBody += violation.Message + "\r\n"
	}

	return []byte(header + msgBody)
}

func NewSmtpReporter(username string, passwords string, host string, recipients []string, fromAddress string, topic string) SmtpReporter {
	return SmtpReporter{
		username:      username,
		password:      passwords,
		host:          host,
		recipientList: recipients,
		from:          fromAddress,
		topic:         topic,
	}
}

type SmtpReporter struct {
	username      string
	password      string
	host          string
	recipientList []string
	from          string
	topic         string
}

func (s SmtpReporter) Send(stack model.ViolationStack) error {
	formatter := MessageFormatter{topic: s.topic}

	fmt.Printf("mail %#v", s)
	auth := smtp.CRAMMD5Auth(s.username, s.password)
	//auth := smtp.PlainAuth("", s.username, s.password, s.host)
	err := smtp.SendMail(s.host, auth, s.from, s.recipientList, formatter.Format(stack))

	return err
}
