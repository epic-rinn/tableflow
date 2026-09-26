package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbe(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/health/ready" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
	}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	if got := probe(addr); got != 0 {
		t.Fatalf("ready: %d", got)
	}
	status = http.StatusServiceUnavailable
	if got := probe(addr); got != 1 {
		t.Fatalf("not ready: %d", got)
	}
	if got := probe("127.0.0.1:1"); got != 1 {
		t.Fatalf("closed port: %d", got)
	}
}
