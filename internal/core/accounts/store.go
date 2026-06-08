package accounts

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	vaultKeySize = 32
	storeVersion = "UTILSACCT1"
)

var ErrStoreNotFound = errors.New("account store not found")

type KeyStore interface {
	LoadOrCreateKey() ([]byte, error)
	DeleteKey() error
}

type Store struct {
	path     string
	keyStore KeyStore
}

func NewDefaultStore() (Store, error) {
	path, err := DefaultStorePath()
	if err != nil {
		return Store{}, err
	}
	return NewStore(path, NewDefaultKeyStore()), nil
}

func NewStore(path string, keyStore KeyStore) Store {
	return Store{path: path, keyStore: keyStore}
}

func (s Store) Path() string {
	return s.path
}

func (s Store) Load() ([]Account, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrStoreNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(data) <= len(storeVersion) || string(data[:len(storeVersion)]) != storeVersion {
		return nil, fmt.Errorf("invalid account store format")
	}

	key, err := s.keyStore.LoadOrCreateKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	payload := data[len(storeVersion):]
	if len(payload) < gcm.NonceSize() {
		return nil, fmt.Errorf("invalid account store payload")
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	var accounts []Account
	if err := json.Unmarshal(plaintext, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s Store) Save(accounts []Account) error {
	key, err := s.keyStore.LoadOrCreateKey()
	if err != nil {
		return err
	}
	if len(key) != vaultKeySize {
		return fmt.Errorf("invalid account key length")
	}

	plaintext, err := json.Marshal(accounts)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	data := append([]byte(storeVersion), nonce...)
	data = append(data, ciphertext...)

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s Store) Purge() error {
	err := os.Remove(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.keyStore.DeleteKey()
}

func randomVaultKey() ([]byte, error) {
	key := make([]byte, vaultKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}
