package menu_test

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

// TestMenuGzip: the public menu is compressed when accepted and decodes to
// the same document; other routes are not compressed.
func TestMenuGzip(t *testing.T) {
	e := testenv.New(t)
	put(e, e.Manager, 1, sampleTree())
	req, _ := http.NewRequest("GET", e.Srv.URL+"/api/v1/branches/"+e.Branch+"/menu", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res, err := http.DefaultTransport.RoundTrip(req) // no transparent decompression
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.Header.Get("Content-Encoding") != "gzip" || res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Request-ID") == "" {
		t.Fatalf("headers: %v", res.Header)
	}
	gz, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(gz)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil || m["currency"] != "THB" {
		t.Fatalf("decoded body: %v %s", err, b)
	}
	req2, _ := http.NewRequest("GET", e.Srv.URL+"/api/v1/health/live", nil)
	req2.Header.Set("Accept-Encoding", "gzip")
	res2, err := http.DefaultTransport.RoundTrip(req2)
	if err != nil {
		t.Fatal(err)
	}
	res2.Body.Close()
	if res2.Header.Get("Content-Encoding") != "" {
		t.Fatal("non-menu route compressed")
	}
}
