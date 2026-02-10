package pkg

import (
	"net/http"
)

// Handle is the user-facing HTTP handler for sync execution.
func Handle(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(""))
}
