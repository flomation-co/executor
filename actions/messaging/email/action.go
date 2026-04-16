package email

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Email"
	Description  = "Send an email via SMTP with optional HTML body"
	Website      = "https://www.flomation.co"
	Icon         = "envelope"
	Date         = "03/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "smtp_host",
		Type:        core.ConnectionTypeString,
		Label:       "SMTP Host",
		Placeholder: "smtp.example.com",
		Required:    true,
	},
	{
		Name:        "smtp_port",
		Type:        core.ConnectionTypeInteger,
		Label:       "SMTP Port",
		Placeholder: "587",
		Required:    true,
	},
	{
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Username",
		Placeholder: "user@example.com",
	},
	{
		Name:        "password",
		Type:        core.ConnectionTypeString,
		Label:       "Password",
		Placeholder: "",
	},
	{
		Name:  "use_tls",
		Type:  core.ConnectionTypeBoolean,
		Label: "Use TLS",
	},
	{
		Name:        "from",
		Type:        core.ConnectionTypeString,
		Label:       "From Address",
		Placeholder: "noreply@example.com",
		Required:    true,
	},
	{
		Name:        "to",
		Type:        core.ConnectionTypeString,
		Label:       "To Addresses",
		Placeholder: "user@example.com, admin@example.com",
		Required:    true,
	},
	{
		Name:        "subject",
		Type:        core.ConnectionTypeString,
		Label:       "Subject",
		Placeholder: "Hello from Flomation",
		Required:    true,
	},
	{
		Name:        "body",
		Type:        core.ConnectionTypeText,
		Label:       "Body",
		Placeholder: "Email body content",
		Required:    true,
	},
	{
		Name:  "content_type",
		Type:  core.ConnectionTypeString,
		Label: "Content Type",
		Options: []core.ConnectionOption{
			{Name: "Plain Text", Value: "text/plain"},
			{Name: "HTML", Value: "text/html"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	hostConn := core.FindConnection("smtp_host", inputs)
	if hostConn == nil || hostConn.String() == nil || *hostConn.String() == "" {
		return nil, fmt.Errorf("smtp_host is required")
	}
	host := *hostConn.String()

	portConn := core.FindConnection("smtp_port", inputs)
	if portConn == nil || portConn.Number() == nil {
		return nil, fmt.Errorf("smtp_port is required")
	}
	port := fmt.Sprintf("%d", *portConn.Number())

	fromConn := core.FindConnection("from", inputs)
	if fromConn == nil || fromConn.String() == nil || *fromConn.String() == "" {
		return nil, fmt.Errorf("from address is required")
	}
	from := *fromConn.String()

	toConn := core.FindConnection("to", inputs)
	if toConn == nil || toConn.String() == nil || *toConn.String() == "" {
		return nil, fmt.Errorf("to address is required")
	}
	toAddresses := strings.Split(*toConn.String(), ",")
	for i := range toAddresses {
		toAddresses[i] = strings.TrimSpace(toAddresses[i])
	}

	subjectConn := core.FindConnection("subject", inputs)
	if subjectConn == nil || subjectConn.String() == nil || *subjectConn.String() == "" {
		return nil, fmt.Errorf("subject is required")
	}
	subject := *subjectConn.String()

	bodyConn := core.FindConnection("body", inputs)
	if bodyConn == nil || bodyConn.String() == nil || *bodyConn.String() == "" {
		return nil, fmt.Errorf("body is required")
	}
	body := *bodyConn.String()

	contentType := "text/plain"
	ctConn := core.FindConnection("content_type", inputs)
	if ctConn != nil && ctConn.String() != nil && *ctConn.String() != "" {
		contentType = *ctConn.String()
	}

	username := ""
	usernameConn := core.FindConnection("username", inputs)
	if usernameConn != nil && usernameConn.String() != nil {
		username = *usernameConn.String()
	}

	password := ""
	passwordConn := core.FindConnection("password", inputs)
	if passwordConn != nil && passwordConn.String() != nil {
		password = *passwordConn.String()
	}

	useTLS := false
	tlsConn := core.FindConnection("use_tls", inputs)
	if tlsConn != nil && tlsConn.Boolean() != nil {
		useTLS = *tlsConn.Boolean()
	}

	// Build message
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s; charset=\"UTF-8\"\r\n\r\n%s",
		from,
		strings.Join(toAddresses, ", "),
		subject,
		contentType,
		body,
	)

	addr := net.JoinHostPort(host, port)

	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	var sendErr error
	if useTLS {
		sendErr = sendWithTLS(addr, host, auth, from, toAddresses, []byte(msg))
	} else {
		sendErr = smtp.SendMail(addr, auth, from, toAddresses, []byte(msg))
	}

	if sendErr != nil {
		return map[string]interface{}{
			"success": false,
			"error":   sendErr.Error(),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"error":   "",
	}, nil
}

func sendWithTLS(addr string, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{ServerName: host}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("RCPT TO failed for %s: %w", addr, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	return client.Quit()
}
