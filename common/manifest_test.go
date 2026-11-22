package common

import (
	"testing"
)

func TestFunctionsManifestUpsertAndRemove(t *testing.T) {
	manifest, err := ParseFunctionsManifest(nil)
	if err != nil {
		t.Fatalf("ParseFunctionsManifest: %v", err)
	}
	manifest.Upsert(ManifestFunction{Name: "a", Language: "go", Path: "functions/go/a/handle.go"})
	manifest.Upsert(ManifestFunction{Name: "a", Language: "go", Path: "functions/go/a/handle.go", IsAsync: true})
	if len(manifest.Functions) != 1 || !manifest.Functions[0].IsAsync {
		t.Fatalf("upsert = %#v", manifest.Functions)
	}
	body, err := manifest.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	roundTrip, err := ParseFunctionsManifest(body)
	if err != nil {
		t.Fatalf("ParseFunctionsManifest round trip: %v", err)
	}
	if _, ok := roundTrip.Find("go", "a"); !ok {
		t.Fatalf("round trip lost function: %#v", roundTrip)
	}
	if !roundTrip.Remove("go", "a") || len(roundTrip.Functions) != 0 {
		t.Fatalf("Remove did not delete entry: %#v", roundTrip.Functions)
	}
}
