package pkg

import (
	"bytes"
	"net/http"
	"net/http/httptest"
)

// Handle is the user-facing HTTP handler for sync execution.
func Handle(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(""))
}

// Handler is the internal adapter used by the broker path.
func Handler(state *AppState, req_id *string, body []byte) ([]byte, error) {
	req := httptest.NewRequest(http.MethodPost, "http://runtime/function", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	Handle(rec, req)
	return rec.Body.Bytes(), nil
}
