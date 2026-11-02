package repo

import (
	"log"
	"os"
	"testing"

	"github.com/ashupednekar/litefunctions/common/vcs"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestVcs(t *testing.T) {
	cfg := vcs.Config{
		AuthMode:        envOr("VCS_AUTH_MODE", "ssh"),
		PrivKeyPath:     os.Getenv("VCS_PRIVATE_KEY_PATH"),
		PrivKeyPassword: os.Getenv("VCS_PRIVATE_KEY_PASSWORD"),
		Token:           os.Getenv("VCS_TOKEN"),
		User:            os.Getenv("VCS_USER"),
		Vendor:          os.Getenv("VCS_VENDOR"),
		BaseURL:         os.Getenv("VCS_BASE_URL"),
	}
	if cfg.AuthMode == "token" && cfg.Token == "" {
		t.Skip("VCS_TOKEN not set")
	}
	if cfg.BaseURL == "" || cfg.User == "" {
		t.Skip("VCS_BASE_URL or VCS_USER not set")
	}
	r, err := New(cfg, "projone", nil)
	if err != nil {
		t.Fatalf("error in new repo call: %s", err)
	}
	err = r.Clone()
	if err != nil {
		t.Errorf("error cloning repo: %s", err)
	}
	readme := "README.md"
	if _, err := r.Fs.Stat(readme); err != nil {
		f, err := r.Fs.Create(readme)
		if err != nil {
			t.Fatalf("cannot create README.md: %s", err)
		}
		_, _ = f.Write([]byte("# " + r.Project + "\n"))
		f.Close()
	} else {
		f, err := r.Fs.OpenFile(readme, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			t.Fatalf("cannot append to README.md: %s", err)
		}
		_, _ = f.Write([]byte("\n"))
		f.Close()
	}
	if err := r.Commit(readme); err != nil {
		t.Fatalf("commit error: %s", err)
	}

	if err := r.Push(); err != nil {
		t.Fatalf("push error: %s", err)
	}
	log.Println("opening existing repo")
	rOpened, err := New(cfg, "projone", nil)
	if err != nil {
		t.Errorf("error in new repo call: %s", err)
	}
	PrintFS(rOpened.Fs)
}
