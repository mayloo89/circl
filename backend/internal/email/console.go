package email

import (
	"context"
	"log"
)

// ConsoleSender logs emails to stdout. Used in development and tests.
type ConsoleSender struct{}

func NewConsoleSender() *ConsoleSender { return &ConsoleSender{} }

func (c *ConsoleSender) Send(_ context.Context, msg Message) error {
	log.Printf("[EMAIL] To: %s | Subject: %s\n%s\n", msg.To, msg.Subject, msg.Text)
	return nil
}
