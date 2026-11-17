package logs

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSelectorFiltersNamespaceAndApps(t *testing.T) {
	query, err := selector("litefunctions", []string{" runtimefunctions ", "litefunctions-ingestor", "runtimefunctions"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{namespace="litefunctions",app=~"^(?:runtimefunctions|litefunctions-ingestor)$"}`
	if query != want {
		t.Fatalf("unexpected selector\nwant: %s\n got: %s", want, query)
	}
}

func TestSelectorEscapesLogQLValues(t *testing.T) {
	query, err := selector(`lite"functions`, []string{"runtime.functions"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{namespace="lite\"functions",app=~"^(?:runtime\\.functions)$"}`
	if query != want {
		t.Fatalf("unexpected selector\nwant: %s\n got: %s", want, query)
	}
}

func TestNewClientValidation(t *testing.T) {
	tests := []Config{
		{URL: "localhost:3100", Namespace: "litefunctions", Apps: []string{"runtimefunctions"}},
		{URL: "http://localhost:3100", Namespace: "", Apps: []string{"runtimefunctions"}},
		{URL: "http://localhost:3100", Namespace: "litefunctions"},
		{URL: "http://localhost:3100", Namespace: "litefunctions", Apps: []string{"runtimefunctions"}, Limit: 1001},
		{URL: "http://localhost:3100", Namespace: "litefunctions", Apps: []string{"runtimefunctions"}, PollInterval: -time.Second},
		{URL: "http://localhost:3100", Namespace: "litefunctions", Apps: []string{"runtimefunctions"}, Lookback: -time.Second},
	}
	for _, cfg := range tests {
		if _, err := NewClient(cfg); err == nil {
			t.Fatalf("expected config to fail: %#v", cfg)
		}
	}
}

type streamCapture struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	count  int
	cancel context.CancelFunc
}

func (w *streamCapture) Header() http.Header { return http.Header{} }
func (w *streamCapture) WriteHeader(int)     {}
func (w *streamCapture) Flush()              {}
func (w *streamCapture) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buffer.Write(p)
	w.count += bytes.Count(p, []byte("event: log\n"))
	if w.count >= 2 {
		w.cancel()
	}
	return n, err
}

func TestStreamQueriesLokiAndEmitsRecentLogs(t *testing.T) {
	requests := make(chan urlValues, 4)
	loki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- urlValues(r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"result":[{"stream":{"app":"runtimefunctions","function":"hello"},"values":[["2000000000","second"],["1000000000","first"]]}]}}`)
	}))
	defer loki.Close()

	client, err := NewClient(Config{
		URL:          loki.URL,
		Namespace:    "litefunctions",
		Apps:         []string{"runtimefunctions", "litefunctions-ingestor"},
		Limit:        2,
		PollInterval: time.Hour,
		Lookback:     time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	output := &streamCapture{cancel: cancel}
	if err := client.Stream(ctx, output, output); err != nil {
		t.Fatal(err)
	}

	body := output.buffer.String()
	first := strings.Index(body, `"line":"first"`)
	second := strings.Index(body, `"line":"second"`)
	if first < 0 || second < 0 || first >= second {
		t.Fatalf("expected recent logs in chronological order, got:\n%s", body)
	}
	if !strings.Contains(body, `"app":"runtimefunctions"`) || !strings.Contains(body, `"function":"hello"`) {
		t.Fatalf("expected stream labels, got:\n%s", body)
	}

	select {
	case query := <-requests:
		if got := query.Get("query"); got != `{namespace="litefunctions",app=~"^(?:runtimefunctions|litefunctions-ingestor)$"}` {
			t.Fatalf("unexpected Loki query: %s", got)
		}
		if got := query.Get("direction"); got != "backward" {
			t.Fatalf("expected initial backward query, got %q", got)
		}
		if got := query.Get("limit"); got != "2" {
			t.Fatalf("unexpected limit %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("no Loki request received")
	}
}

type urlValues map[string][]string

func (v urlValues) Get(key string) string { return v[key][0] }
