package mail

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeSMTP accepts one session and never advertises STARTTLS.
func fakeSMTP(t *testing.T) (string, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	seen := make(chan string, 16)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		c.Write([]byte("220 fake ESMTP\r\n"))
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			seen <- strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "EHLO"):
				c.Write([]byte("250-fake\r\n250 AUTH PLAIN\r\n"))
			case strings.HasPrefix(line, "QUIT"):
				c.Write([]byte("221 bye\r\n"))
				return
			default:
				c.Write([]byte("250 ok\r\n"))
			}
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String(), seen
}

// TestStartTLSNeverFallsBackToPlaintext: when TLS is required but the
// server does not offer it, nothing (no credentials, no message) is sent.
func TestStartTLSNeverFallsBackToPlaintext(t *testing.T) {
	addr, seen := fakeSMTP(t)
	s := SMTP{Addr: addr, From: "a@b.test", Timeout: 2 * time.Second, TLS: TLSStartTLS, Username: "resend", Password: "secret-api-key"}
	err := s.Send(context.Background(), Message{To: "x@y.test", Subject: "s", Text: "t"})
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("expected refusal, got %v", err)
	}
	close := time.After(200 * time.Millisecond)
	for {
		select {
		case line := <-seen:
			if strings.HasPrefix(line, "AUTH") || strings.HasPrefix(line, "MAIL") || strings.Contains(line, "secret-api-key") {
				t.Fatalf("sent %q over plaintext", line)
			}
		case <-close:
			return
		}
	}
}

func TestHeaderInjectionRejected(t *testing.T) {
	s := SMTP{Addr: "127.0.0.1:1", From: "a@b.test", Timeout: time.Second, TLS: TLSNone}
	if err := s.Send(context.Background(), Message{To: "x@y.test\r\nBcc: z@z.test", Subject: "s", Text: "t"}); err == nil || !strings.Contains(err.Error(), "line break") {
		t.Fatalf("header injection not rejected: %v", err)
	}
}
