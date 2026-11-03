package pkg

import (
	"encoding/json"
	"net/http"
)

type Input struct {
	Name *string `json:"name"`
}

type Output struct {
	Message string `json:"message"`
}

func Handle(w http.ResponseWriter, r *http.Request) {
	var input Input
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	name := "stranger"
	if input.Name != nil && *input.Name != "" {
		name = *input.Name
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Output{
		Message: "Hello " + name + " from Go!",
	})
}
