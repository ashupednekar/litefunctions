package common

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FunctionsManifestPath is the repo-local metadata file the MCP server writes.
// The portal merges it during its normal repo sync so agent-authored functions
// keep async flags and custom endpoints.
const FunctionsManifestPath = ".litefunctions/functions.json"

// FunctionsManifest is the machine-readable list of agent-managed functions.
type FunctionsManifest struct {
	Version   int                `json:"version"`
	Functions []ManifestFunction `json:"functions"`
}

// ManifestFunction describes one function package in the project repository.
type ManifestFunction struct {
	Name     string `json:"name"`
	Language string `json:"language"`
	Path     string `json:"path"`
	IsAsync  bool   `json:"is_async"`
	// UI marks a server-side rendered web handler. The portal previews these in
	// an iframe and exposes a live link instead of the API tester.
	UI        bool               `json:"ui,omitempty"`
	Endpoints []ManifestEndpoint `json:"endpoints,omitempty"`
}

// ManifestEndpoint describes an HTTP route the portal should create.
type ManifestEndpoint struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Scope  string `json:"scope"`
}

// ParseFunctionsManifest decodes manifest bytes. An empty document is treated
// as an empty manifest.
func ParseFunctionsManifest(body []byte) (*FunctionsManifest, error) {
	manifest := &FunctionsManifest{Version: 1}
	if len(strings.TrimSpace(string(body))) == 0 {
		return manifest, nil
	}
	if err := json.Unmarshal(body, manifest); err != nil {
		return nil, fmt.Errorf("decode functions manifest: %w", err)
	}
	if manifest.Version == 0 {
		manifest.Version = 1
	}
	return manifest, nil
}

// Marshal encodes a manifest with a stable function order.
func (m *FunctionsManifest) Marshal() ([]byte, error) {
	m.Version = 1
	sort.SliceStable(m.Functions, func(i, j int) bool {
		if m.Functions[i].Language != m.Functions[j].Language {
			return m.Functions[i].Language < m.Functions[j].Language
		}
		return m.Functions[i].Name < m.Functions[j].Name
	})
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

// Upsert inserts or replaces a function entry, preserving endpoint order.
func (m *FunctionsManifest) Upsert(fn ManifestFunction) {
	for i := range m.Functions {
		if sameFunction(m.Functions[i], fn) {
			m.Functions[i] = fn
			return
		}
	}
	m.Functions = append(m.Functions, fn)
}

// Remove deletes a function entry by language and name.
func (m *FunctionsManifest) Remove(language, name string) bool {
	for i := range m.Functions {
		if m.Functions[i].Language == language && m.Functions[i].Name == name {
			m.Functions = append(m.Functions[:i], m.Functions[i+1:]...)
			return true
		}
	}
	return false
}

// Find returns the manifest entry for a function, if present.
func (m *FunctionsManifest) Find(language, name string) (ManifestFunction, bool) {
	for _, fn := range m.Functions {
		if fn.Language == language && fn.Name == name {
			return fn, true
		}
	}
	return ManifestFunction{}, false
}

func sameFunction(a, b ManifestFunction) bool {
	if a.Language == b.Language && a.Name == b.Name {
		return true
	}
	return a.Path != "" && a.Path == b.Path
}
