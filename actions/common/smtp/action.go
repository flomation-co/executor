package smtp

import (
	"crypto/tls"
	"fmt"
	"net/smtp"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send SMTP Email"
	Description  = "Send an HTML email via SMTP"
	Website      = "https://www.flomation.co"
	Icon         = "fa-solid fa-envelope"
	Date         = "27/11/2025"
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing SMTP Send Email action")

	to := core.FindConnection("to", inputs)
	from := core.FindConnection("from", inputs)
	subject := core.FindConnection("subject", inputs)
	message := core.FindConnection("message", inputs)

	host := core.FindConnection("smtp_host", inputs)
	user := core.FindConnection("smtp_username", inputs)
	password := core.FindConnection("smtp_password", inputs)
	port := core.FindConnection("smtp_port", inputs)
	smtpHost := fmt.Sprintf("%v:%v", host.String(), port.Number())

	auth := smtp.PlainAuth("", user.String(), password.String(), host.String())
	msg := fmt.Sprintf("From: %v\nTo: %v\nSubject: %v\nMIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n%v\n\n", from.String(), to.String(), subject.String(), message.String())

	c, err := smtp.Dial(smtpHost)
	if err != nil {
		return nil, err
	}

	if err = c.Hello("flomation.app"); err != nil {
		return nil, err
	}

	if ok, _ := c.Extension("STARTTLS"); ok {
		cfg := &tls.Config{
			ServerName: host.String(),
			MinVersion: tls.VersionTLS12,
		}
		if err = c.StartTLS(cfg); err != nil {
			return nil, err
		}
	}

	if err = c.Auth(auth); err != nil {
		return nil, err
	}

	if err = c.Mail(from.String()); err != nil {
		return nil, err
	}

	if err = c.Rcpt(to.String()); err != nil {
		return nil, err
	}

	wc, err := c.Data()
	if err != nil {
		return nil, err
	}

	if _, err = wc.Write([]byte(msg)); err != nil {
		return nil, err
	}

	if err = wc.Close(); err != nil {
		return nil, err
	}

	if err = c.Quit(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"count":  len(msg),
		"result": 0,
	}, nil
}
