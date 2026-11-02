package vcs

import "sync"

// Config holds the VCS settings shared by the portal and the operator.
type Config struct {
	AuthMode        string
	PrivKeyPath     string
	PrivKeyPassword string
	Token           string
	User            string
	Vendor          string
	BaseURL         string
	PublicBaseURL   string
	FQDN            string
}

var (
	mu      sync.RWMutex
	current Config
)

// Configure sets the process-wide VCS configuration.
func Configure(cfg Config) {
	mu.Lock()
	defer mu.Unlock()
	current = cfg
}

// Current returns the process-wide VCS configuration.
func Current() Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}
