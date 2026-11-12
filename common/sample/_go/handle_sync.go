package pkg

import (
	"encoding/json"
	"log"
	"net/http"

	"runtimes/go/functions/{{name}}/internal/types"
)

func Handle(w http.ResponseWriter, r *http.Request) {
	var input types.Input
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	name := "stranger"
	if input.Name != nil && *input.Name != "" {
		name = *input.Name
	}

	log.Printf("[INFO] handling request name=%s", name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(types.Output{
		Message: "Hello " + name + " from Go!",
	})
}
