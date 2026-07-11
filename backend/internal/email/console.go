package email

import (
	"context"
	"fmt"
	"os"
)

// ConsoleSender prints emails to stdout. Used in development and tests.
type ConsoleSender struct{}

func NewConsoleSender() *ConsoleSender { return &ConsoleSender{} }

func (c *ConsoleSender) Send(_ context.Context, msg Message) error {
	_, _ = fmt.Fprintf(os.Stdout, "[EMAIL] To: %s | Subject: %s\n%s\n", msg.To, msg.Subject, msg.Text)
	return nil
}
