package pkg

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureRepo(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "creates missing repository", statusCode: http.StatusCreated},
		{name: "accepts existing repository", statusCode: http.StatusConflict},
		{name: "returns unexpected API errors", statusCode: http.StatusInternalServerError, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/user/repos" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}

				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if request["name"] != "project-a" || request["auto_init"] != true || request["default_branch"] != "main" {
					t.Errorf("unexpected create repository request: %#v", request)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusCreated {
					_ = json.NewEncoder(w).Encode(map[string]any{
						"name":           "project-a",
						"full_name":      "owner/project-a",
						"default_branch": "main",
					})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "repository unavailable"})
			}))
			defer server.Close()

			cfg := Config{
				Vendor:  "gitea",
				BaseUrl: server.URL,
				Token:   "test-token",
			}
			err := ensureRepo(cfg, "project-a")
			if (err != nil) != tt.wantErr {
				t.Fatalf("ensureRepo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
