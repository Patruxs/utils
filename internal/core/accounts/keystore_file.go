//go:build !windows && !darwin

package accounts

import (
	"os"
	"path/filepath"
)

type fileKeyStore struct {
	path string
}

func NewDefaultKeyStore() KeyStore {
	base, err := configBaseDir()
	if err != nil {
		base = "."
	}
	return fileKeyStore{path: filepath.Join(base, "utils", "accounts.key")}
}

func (s fileKeyStore) LoadOrCreateKey() ([]byte, error) {
	key, err := os.ReadFile(s.path)
	if err == nil && len(key) == vaultKeySize {
		return key, nil
	}

	key, err = randomVaultKey()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(s.path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (s fileKeyStore) DeleteKey() error {
	err := os.Remove(s.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
