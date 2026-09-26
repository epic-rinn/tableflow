// Package mail sends transactional email through a pluggable adapter. Local
// development uses SMTP to Mailpit; the production provider is a deployment
// decision. Message bodies may contain single-use links and are never logged.
package mail

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// Message is a plain-text email.
type Message struct {
	To      string
	Subject string
	Text    string
	// Kind labels the message in logs (e.g. "member.verify"); never the body.
	Kind string
}

// Sender delivers one message.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// Outbox accepts messages for delivery after the caller's transaction.
type Outbox interface {
	Enqueue(m Message) bool
}

// SMTP delivers without authentication (local Mailpit). Header values are
// rejected if they contain line breaks.
type SMTP struct {
	Addr    string
	From    string
	Timeout time.Duration
}

func headerSafe(v string) bool { return !strings.ContainsAny(v, "\r\n") }

// Send implements Sender.
func (s SMTP) Send(ctx context.Context, m Message) error {
	if !headerSafe(m.To) || !headerSafe(m.Subject) || !headerSafe(s.From) {
		return errors.New("mail: header contains a line break")
	}
	deadline := time.Now().Add(s.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn, err := (&net.Dialer{Deadline: deadline}).DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("mail: dial: %w", err)
	}
	_ = conn.SetDeadline(deadline)
	host, _, _ := net.SplitHostPort(s.Addr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("mail: hello: %w", err)
	}
	defer c.Close()
	from := s.From
	if i := strings.LastIndex(from, "<"); i >= 0 {
		from = strings.TrimSuffix(from[i+1:], ">")
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := c.Rcpt(m.To); err != nil {
		return fmt.Errorf("mail: rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	msg := "From: " + s.From + "\r\nTo: " + m.To + "\r\nSubject: " + m.Subject +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" +
		strings.ReplaceAll(m.Text, "\n", "\r\n")
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: close: %w", err)
	}
	return c.Quit()
}

// Queue delivers asynchronously with bounded capacity so request latency
// never depends on whether (or how quickly) a message was sent.
type Queue struct {
	ch      chan Message
	sender  Sender
	logger  *slog.Logger
	timeout time.Duration
}

// NewQueue buffers up to capacity messages.
func NewQueue(sender Sender, capacity int, timeout time.Duration, logger *slog.Logger) *Queue {
	return &Queue{ch: make(chan Message, capacity), sender: sender, logger: logger, timeout: timeout}
}

// Enqueue implements Outbox; it drops (and logs) when the queue is full.
func (q *Queue) Enqueue(m Message) bool {
	select {
	case q.ch <- m:
		return true
	default:
		q.logger.Warn("mail queue full; message dropped", "kind", m.Kind)
		return false
	}
}

// Run delivers with workers until ctx is cancelled. Undelivered messages are
// lost on shutdown; users can request them again.
func (q *Queue) Run(ctx context.Context, workers int) {
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case m := <-q.ch:
					sctx, cancel := context.WithTimeout(ctx, q.timeout)
					if err := q.sender.Send(sctx, m); err != nil {
						q.logger.Warn("mail send failed", "kind", m.Kind, "error", err.Error())
					}
					cancel()
				}
			}
		})
	}
	wg.Wait()
}

// Recorder is a synchronous Outbox for tests.
type Recorder struct {
	mu   sync.Mutex
	msgs []Message
}

// Enqueue implements Outbox.
func (r *Recorder) Enqueue(m Message) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, m)
	return true
}

// Messages returns messages sent to addr (all when addr is "").
func (r *Recorder) Messages(addr string) []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Message
	for _, m := range r.msgs {
		if addr == "" || m.To == addr {
			out = append(out, m)
		}
	}
	return out
}
