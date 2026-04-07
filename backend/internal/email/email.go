package email

import "context"

// Message holds the content of an outbound email.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// Sender is the interface implemented by all email providers.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
