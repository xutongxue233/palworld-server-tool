package database

import (
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

func TestConfigValuesRoundTrip(t *testing.T) {
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "pst.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := EnsureBuckets(db); err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"web.port":                   9090,
		"palworld.control.mode":      "process",
		"palworld.control.arguments": []string{"-useperfthreads", "-NoAsyncLoadingThread"},
	}
	if err := PutConfigValues(db, want); err != nil {
		t.Fatal(err)
	}
	got, err := ListConfigValues(db)
	if err != nil {
		t.Fatal(err)
	}
	if got["web.port"] != float64(9090) || got["palworld.control.mode"] != "process" {
		t.Fatalf("unexpected config values: %#v", got)
	}
	arguments, ok := got["palworld.control.arguments"].([]any)
	if !ok || len(arguments) != 2 || arguments[0] != "-useperfthreads" {
		t.Fatalf("unexpected arguments: %#v", got["palworld.control.arguments"])
	}
}
