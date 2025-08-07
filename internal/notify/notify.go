// Package notify sends operational email. MAIL_PROVIDER=outbox (the default)
// writes messages to the outbox_messages table, shown at /_dev/outbox;
// MAIL_PROVIDER=log only logs them.
package notify

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/remitlane/remitlane/internal/db"
)

// Message is an email.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender sends messages.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// New returns the configured sender.
func New(provider string, q *db.Queries, log *slog.Logger) (Sender, error) {
	switch provider {
	case "outbox", "":
		return &Outbox{q: q}, nil
	case "log":
		return &Log{log: log}, nil
	}
	return nil, fmt.Errorf("mail: unknown MAIL_PROVIDER %q", provider)
}

// Outbox stores messages for the dev outbox page.
type Outbox struct {
	q *db.Queries
}

// Send implements Sender.
func (o *Outbox) Send(ctx context.Context, m Message) error {
	return o.q.InsertOutbox(ctx, db.InsertOutboxParams{Channel: "email", Recipient: m.To, Subject: m.Subject, Body: m.Body})
}

// Log writes messages to the log.
type Log struct {
	log *slog.Logger
}

// Send implements Sender.
func (l *Log) Send(_ context.Context, m Message) error {
	l.log.Info("email", "to", m.To, "subject", m.Subject)
	return nil
}
