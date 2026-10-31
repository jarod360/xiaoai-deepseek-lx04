package miservice

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenStoreAtomicSaveAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mi.token")
	store := NewTokenStore(path)
	token := NewTokens()
	token.UserName = "first"
	if err := store.SaveToken(token); err != nil {
		t.Fatal(err)
	}
	token.UserName = "second"
	if err := store.SaveToken(token); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadToken()
	if err != nil || loaded.UserName != "second" {
		t.Fatalf("token not replaced: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0600 {
		t.Fatalf("token file is not private: %v", err)
	}
}
