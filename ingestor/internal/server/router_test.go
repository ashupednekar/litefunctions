package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutesMatchSubpaths(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutes(mux, &IngestHandler{})

	cases := []struct {
		path string
		want string
	}{
		{"/lambda/aaa/forex", "/lambda/{project}/{name}"},
		{"/lambda/aaa/forex/", "/lambda/{project}/{name}/"},
		{"/lambda/aaa/forex/quotes", "/lambda/{project}/{name}/"},
		{"/lambda/aaa/forex/quotes/USD", "/lambda/{project}/{name}/"},
		{"/lambda/sse/aaa/forex", "/lambda/sse/{project}/{name}"},
		{"/lambda/sse/aaa/forex/", "/lambda/sse/{project}/{name}/"},
		{"/lambda/ws/aaa/forex", "/lambda/ws/{project}/{name}"},
		{"/lambda/ws/aaa/forex/", "/lambda/ws/{project}/{name}/"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		_, pattern := mux.Handler(req)
		if pattern != tc.want {
			t.Errorf("%s matched %q, want %q", tc.path, pattern, tc.want)
		}
	}
}
