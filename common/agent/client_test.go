package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientLegacySessionAndMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "opencode" || pass != "secret" {
			t.Fatalf("unexpected basic auth: %q %q %v", user, pass, ok)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/session":
			writeJSON(t, w, map[string]any{"id": "ses_1", "title": "demo"})
		case r.Method == http.MethodPost && r.URL.Path == "/session/ses_1/message":
			var body promptBody
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if len(body.Parts) != 1 || body.Parts[0].Text != "build it" {
				t.Fatalf("unexpected prompt: %#v", body)
			}
			writeJSON(t, w, Message{
				Info:  MessageInfo{ID: "msg_1", SessionID: "ses_1", Role: "assistant"},
				Parts: []Part{{Type: "step-start"}, {Type: "text", Text: "done"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, Username: "opencode", Password: "secret"})
	ctx := context.Background()

	session, err := client.CreateSession(ctx, "demo")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.ID != "ses_1" {
		t.Fatalf("session id = %q", session.ID)
	}

	msg, err := client.SendMessage(ctx, session.ID, "build it")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if got := msg.Text(); got != "done" {
		t.Fatalf("text = %q", got)
	}
}

func TestClientFallsBackToV2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/session":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			writeJSON(t, w, map[string]any{"id": "ses_2", "title": "demo"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/ses_2/prompt":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/api/experimental/session/ses_2/wait":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/api/session/ses_2/message":
			writeJSON(t, w, map[string]any{
				"data": []map[string]any{{
					"id":      "msg_2",
					"type":    "assistant",
					"role":    "assistant",
					"content": []map[string]string{{"type": "text", "text": "v2 reply"}},
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	ctx := context.Background()

	session, err := client.CreateSession(ctx, "demo")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	msg, err := client.SendMessage(ctx, session.ID, "hello")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if got := msg.Text(); got != "v2 reply" {
		t.Fatalf("text = %q", got)
	}
}

func TestServicePromptIncludesProjectRules(t *testing.T) {
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/session":
			writeJSON(t, w, map[string]any{"id": "ses_3", "title": "demo"})
		case r.Method == http.MethodPost && r.URL.Path == "/session/ses_3/message":
			var body promptBody
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			prompt = body.Parts[0].Text
			writeJSON(t, w, Message{
				Info:  MessageInfo{ID: "msg_3", SessionID: "ses_3", Role: "assistant"},
				Parts: []Part{{Type: "text", Text: "ok"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(Config{BaseURL: server.URL})
	result, err := service.Prompt(context.Background(), PromptRequest{
		Project:   "orders",
		ProjectID: "abc",
		Text:      "make a health endpoint",
	})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if result.SessionID != "ses_3" || result.Text != "ok" {
		t.Fatalf("result = %#v", result)
	}
	for _, want := range []string{"orders", "not supported yet", "litefunctions", "health endpoint"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
