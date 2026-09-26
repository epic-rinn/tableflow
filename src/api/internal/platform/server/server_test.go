package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

var timeouts = Timeouts{
	ReadHeader: time.Second, Read: 5 * time.Second, Write: 5 * time.Second,
	Idle: time.Second, Shutdown: 2 * time.Second,
}

// GracefulShutdownSmoke: cancellation lets an in-flight request finish, then
// the listener stops accepting connections and Serve returns nil.
func TestGracefulShutdownSmoke(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, "finished")
	})
	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- Serve(ctx, ln, h, timeouts) }()

	type result struct {
		body string
		err  error
	}
	inflight := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			inflight <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		inflight <- result{string(b), err}
	}()
	<-started
	cancel()

	r := <-inflight
	if r.err != nil || r.body != "finished" {
		t.Fatalf("in-flight request: body=%q err=%v", r.body, r.err)
	}
	if err := <-serveErr; err != nil {
		t.Fatalf("Serve returned %v", err)
	}
	if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		c.Close()
		t.Fatal("listener still accepting after shutdown")
	}
}

func TestShutdownTimeoutForcesClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})
	ctx, cancel := context.WithCancel(context.Background())
	short := timeouts
	short.Shutdown = 100 * time.Millisecond
	serveErr := make(chan error, 1)
	go func() { serveErr <- Serve(ctx, ln, h, short) }()
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err == nil {
			resp.Body.Close()
		}
	}()
	<-started
	cancel()
	select {
	case err := <-serveErr:
		if err == nil {
			t.Fatal("expected shutdown deadline error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after shutdown timeout")
	}
}
