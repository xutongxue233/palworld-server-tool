package service

import (
	"path/filepath"
	"testing"

	"github.com/zaigie/palworld-server-tool/internal/database"
	"go.etcd.io/bbolt"
)

func TestWhitelistSupportsCrossPlatformUserID(t *testing.T) {
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "test.db"), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	player := database.PlayerW{Name: "CrossPlatform", UserID: "xbox_123"}
	if err := AddWhitelist(db, player); err != nil {
		t.Fatal(err)
	}
	players, err := ListWhitelist(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 1 || players[0].UserID != player.UserID {
		t.Fatalf("unexpected whitelist: %#v", players)
	}
	if err := RemoveWhitelist(db, database.PlayerW{UserID: player.UserID}); err != nil {
		t.Fatal(err)
	}
	if err := PutWhitelist(db, []database.PlayerW{{Name: "missing identifiers"}}); err == nil {
		t.Fatal("expected identifier-less whitelist entry to be rejected")
	}
}
