package integrations

import (
	"crypto/tls"
	"fmt"

	"github.com/go-mail/mail/v2"
)

type Mailer struct {
	host                 string
	port                 int
	user, password, from string
}

func NewMailer(host string, port int, user, password, from string) *Mailer {
	return &Mailer{host: host, port: port, user: user, password: password, from: from}
}

func (m *Mailer) Send(to, subject, html string) error {
	if m.host == "" {
		return nil
	}

	msg := mail.NewMessage()
	msg.SetHeader("From", m.from)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/html", html)

	d := mail.NewDialer(m.host, m.port, m.user, m.password)
	d.TLSConfig = &tls.Config{ServerName: m.host, InsecureSkipVerify: false}
	if err := d.DialAndSend(msg); err != nil {
		return fmt.Errorf("email sending failed: %w", err)
	}

	return nil
}
