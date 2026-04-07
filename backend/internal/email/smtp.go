package email

import (
	"context"
	"fmt"

	"github.com/wneessen/go-mail"
)

// SMTPConfig holds the configuration for the SMTP sender.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SMTPSender sends emails via any standard SMTP server.
// For local development, point it at Mailpit (port 1025, no auth, no TLS).
// For production, point it at SendGrid, Postmark, AWS SES, etc.
type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	m := mail.NewMsg()
	if err := m.From(s.cfg.From); err != nil {
		return fmt.Errorf("set from: %w", err)
	}
	if err := m.To(msg.To); err != nil {
		return fmt.Errorf("set to: %w", err)
	}
	m.Subject(msg.Subject)
	m.SetBodyString(mail.TypeTextHTML, msg.HTML)
	m.AddAlternativeString(mail.TypeTextPlain, msg.Text)

	opts := []mail.Option{
		mail.WithPort(s.cfg.Port),
		mail.WithTLSPolicy(mail.TLSOpportunistic),
	}
	if s.cfg.Username != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(s.cfg.Username),
			mail.WithPassword(s.cfg.Password),
		)
	} else {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthNoAuth))
	}

	c, err := mail.NewClient(s.cfg.Host, opts...)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	if err := c.DialAndSendWithContext(ctx, m); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
