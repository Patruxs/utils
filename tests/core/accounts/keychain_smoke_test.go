//go:build darwin && keychain_smoke

package accounts_test

import (
	"errors"
	"os"
	"testing"

	"utils/internal/core/accounts"
)

func TestMacOSKeychainSmoke(t *testing.T) {
	if os.Getenv("UTILS_KEYCHAIN_SMOKE") != "1" {
		t.Fatal("set UTILS_KEYCHAIN_SMOKE=1 and run through scripts/smoke-macos-keychain.sh")
	}

	store, err := accounts.NewDefaultStore()
	if err != nil {
		t.Fatalf("NewDefaultStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Purge()
	})

	want := []accounts.Account{{
		Email:      "keychain-smoke@example.invalid",
		Password:   "password-smoke",
		TOTPSecret: "JBSWY3DPEHPK3PXP",
	}}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("unexpected loaded accounts: %#v", got)
	}

	if err := store.Purge(); err != nil {
		t.Fatalf("Purge returned error: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, accounts.ErrStoreNotFound) {
		t.Fatalf("expected missing store after purge, got %v", err)
	}
}
