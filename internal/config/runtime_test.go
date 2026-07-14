package config

import (
	"slices"
	"testing"

	"github.com/spf13/viper"
)

func TestRuntimeValuesRedactsSecrets(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	db := openConfigTestDB(t)
	var current Config
	Init(db, &current)
	if _, err := ApplyRuntimeValues(map[string]any{
		"rest.password": "server-secret",
		"web.port":      float64(9090),
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := RuntimeValues()
	if err != nil {
		t.Fatal(err)
	}
	if _, exposed := snapshot.Values["rest.password"]; exposed {
		t.Fatal("REST password was exposed")
	}
	if !slices.Contains(snapshot.ConfiguredSecrets, "rest.password") {
		t.Fatalf("configured secrets = %#v", snapshot.ConfiguredSecrets)
	}
	if snapshot.Values["web.port"] != float64(9090) {
		t.Fatalf("web port = %#v", snapshot.Values["web.port"])
	}
}

func TestApplyRuntimeValuesValidatesKeysAndRestart(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	db := openConfigTestDB(t)
	var current Config
	Init(db, &current)
	restart, err := ApplyRuntimeValues(map[string]any{
		"web.port":                   float64(9191),
		"palworld.control.arguments": []any{"-useperfthreads"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(restart, []string{"palworld.control.arguments", "web.port"}) {
		t.Fatalf("restart keys = %#v", restart)
	}
	if _, err := ApplyRuntimeValues(map[string]any{"unknown.key": true}); err == nil {
		t.Fatal("unknown configuration key was accepted")
	}
	if _, err := ApplyRuntimeValues(map[string]any{"web.port": float64(70000)}); err == nil {
		t.Fatal("invalid web port was accepted")
	}
}
