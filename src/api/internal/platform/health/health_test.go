package health

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

type blockingPinger struct {
	calls  atomic.Int32
	active atomic.Int32
	peak   atomic.Int32
	err    error
	delay  time.Duration
}

func (p *blockingPinger) Ping(ctx context.Context) error {
	p.calls.Add(1)
	n := p.active.Add(1)
	defer p.active.Add(-1)
	for {
		old := p.peak.Load()
		if n <= old || p.peak.CompareAndSwap(old, n) {
			break
		}
	}
	select {
	case <-time.After(p.delay):
		return p.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func get(h http.HandlerFunc, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	httpx.Middleware(discard, h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// ReadinessUnavailableDatabase: liveness stays 200 while readiness returns a
// sanitized 503 when PostgreSQL refuses connections.
func TestReadinessUnavailableDatabase(t *testing.T) {
	// Port 1 on loopback is closed; pgxpool connects lazily, like production.
	pool, err := pgxpool.New(context.Background(), "postgres://tableflow:pw@127.0.0.1:1/tableflow?connect_timeout=1")
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	h := New(pool, time.Second, discard)

	if rec := get(h.Live, "/api/v1/health/live"); rec.Code != http.StatusOK {
		t.Fatalf("live status = %d", rec.Code)
	}

	start := time.Now()
	rec := get(h.Ready, "/api/v1/health/ready")
	elapsed := time.Since(start)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want 503", rec.Code)
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("readiness took %v, exceeding its 1s bound", elapsed)
	}
	body := rec.Body.String()
	if strings.Contains(body, "127.0.0.1") || strings.Contains(body, "tableflow:pw") {
		t.Fatalf("readiness leaked connection details: %s", body)
	}
	var eb httpx.ErrorBody
	if err := json.Unmarshal([]byte(body), &eb); err != nil || eb.Error.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("unexpected body %s (%v)", body, err)
	}
	t.Logf("refused-connection readiness: status=%d elapsed=%v", rec.Code, elapsed)
}

func TestReadinessTimeoutIsBounded(t *testing.T) {
	p := &blockingPinger{delay: time.Hour}
	h := New(p, 200*time.Millisecond, discard)
	start := time.Now()
	rec := get(h.Ready, "/api/v1/health/ready")
	elapsed := time.Since(start)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	if elapsed < 200*time.Millisecond || elapsed > 700*time.Millisecond {
		t.Fatalf("elapsed %v outside timeout bound", elapsed)
	}
	t.Logf("blackholed readiness: elapsed=%v (timeout 200ms)", elapsed)
}

func TestConcurrentReadinessSharesOneProbe(t *testing.T) {
	p := &blockingPinger{delay: 100 * time.Millisecond}
	h := New(p, time.Second, discard)
	var wg sync.WaitGroup
	codes := make([]int, 50)
	for i := range codes {
		wg.Go(func() { codes[i] = get(h.Ready, "/api/v1/health/ready").Code })
	}
	wg.Wait()
	for i, c := range codes {
		if c != http.StatusOK {
			t.Fatalf("request %d status %d", i, c)
		}
	}
	if p.peak.Load() != 1 {
		t.Fatalf("peak concurrent probes = %d, want 1", p.peak.Load())
	}
	t.Logf("50 concurrent readiness requests used %d probe(s), peak concurrency %d", p.calls.Load(), p.peak.Load())
}

func TestReadinessProbeSurvivesCallerCancellation(t *testing.T) {
	p := &blockingPinger{delay: 100 * time.Millisecond}
	h := New(p, time.Second, discard)

	ctx, cancel := context.WithCancel(context.Background())
	first := httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		httpx.Middleware(discard, http.HandlerFunc(h.Ready)).ServeHTTP(httptest.NewRecorder(), first)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	second := make(chan int)
	go func() { second <- get(h.Ready, "/api/v1/health/ready").Code }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	if code := <-second; code != http.StatusOK {
		t.Fatalf("second caller got %d after first caller cancelled", code)
	}
}
