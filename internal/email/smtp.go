package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

var ErrDisabled = errors.New("email backend disabled")

type Message struct {
	To      string
	Subject string
	Body    string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

type NoneSender struct{}

func (NoneSender) Send(context.Context, Message) error {
	return ErrDisabled
}

type SMTPStartTLSMode string

const (
	SMTPStartTLSRequired      SMTPStartTLSMode = "required"
	SMTPStartTLSOpportunistic SMTPStartTLSMode = "opportunistic"
	SMTPStartTLSDisabled      SMTPStartTLSMode = "disabled"
)

type SMTPOptions struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	StartTLS SMTPStartTLSMode
	Timeout  time.Duration
}

type SMTPSender struct {
	opts SMTPOptions
}

func NewSMTPSender(opts SMTPOptions) *SMTPSender {
	return &SMTPSender{opts: opts}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if err := validateMessage(s.opts.From, msg); err != nil {
		return err
	}
	address := net.JoinHostPort(s.opts.Host, fmt.Sprintf("%d", s.opts.Port))
	dialer := net.Dialer{Timeout: s.opts.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial smtp server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.opts.Host)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer client.Close()

	if err := s.startTLS(client); err != nil {
		return err
	}
	if s.opts.Username != "" {
		auth := smtp.PlainAuth("", s.opts.Username, s.opts.Password, s.opts.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authenticate smtp client: %w", err)
		}
	}
	if err := client.Mail(s.opts.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := io.WriteString(writer, formatMessage(s.opts.From, msg)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write smtp message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close smtp message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit smtp client: %w", err)
	}
	return nil
}

func (s *SMTPSender) startTLS(client *smtp.Client) error {
	if s.opts.StartTLS == SMTPStartTLSDisabled {
		return nil
	}
	ok, _ := client.Extension("STARTTLS")
	if !ok {
		if s.opts.StartTLS == SMTPStartTLSRequired {
			return fmt.Errorf("smtp starttls required")
		}
		return nil
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: s.opts.Host,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("start smtp tls: %w", err)
	}
	return nil
}

func validateMessage(from string, msg Message) error {
	if _, err := mail.ParseAddress(from); err != nil {
		return fmt.Errorf("invalid sender address")
	}
	if _, err := mail.ParseAddress(msg.To); err != nil {
		return fmt.Errorf("invalid recipient address")
	}
	if hasHeaderNewline(from) || hasHeaderNewline(msg.To) || hasHeaderNewline(msg.Subject) {
		return fmt.Errorf("email header contains newline")
	}
	if strings.TrimSpace(msg.Subject) == "" {
		return fmt.Errorf("email subject is required")
	}
	return nil
}

func formatMessage(from string, msg Message) string {
	return "From: " + from + "\r\n" +
		"To: " + msg.To + "\r\n" +
		"Subject: " + msg.Subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		normalizeBody(msg.Body) + "\r\n"
}

func normalizeBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.ReplaceAll(body, "\n", "\r\n")
}

func hasHeaderNewline(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}
