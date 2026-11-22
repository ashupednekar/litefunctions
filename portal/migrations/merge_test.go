package migrations_test

import (
	"io/fs"
	"testing"

	agentmigrations "github.com/ashupednekar/litefunctions/agent/migrations"
	"github.com/ashupednekar/litefunctions/portal/migrations"
)

func TestMergeIncludesModuleMigrations(t *testing.T) {
	merged := migrations.Merge(migrations.Files, agentmigrations.Files)
	entries, err := fs.ReadDir(merged, ".")
	if err != nil {
		t.Fatalf("read merged migrations: %v", err)
	}

	got := map[string]bool{}
	for _, entry := range entries {
		got[entry.Name()] = true
	}
	for _, name := range []string{"00004_agent_history.sql", "00005_function_ui.sql"} {
		if !got[name] {
			t.Errorf("merged migrations missing %s (got %v)", name, got)
		}
		body, err := fs.ReadFile(merged, name)
		if err != nil || len(body) == 0 {
			t.Errorf("read %s: len=%d err=%v", name, len(body), err)
		}
	}
}
