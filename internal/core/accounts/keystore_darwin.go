//go:build darwin

package accounts

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	keychainService = "UTILS ChatGPT Codex Account Vault"
	keychainAccount = "UTILS"
)

type darwinKeyStore struct{}

func NewDefaultKeyStore() KeyStore {
	return darwinKeyStore{}
}

func (darwinKeyStore) LoadOrCreateKey() ([]byte, error) {
	key, err := readKeychainKey()
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, errKeychainItemNotFound) {
		return nil, err
	}

	key, err = randomVaultKey()
	if err != nil {
		return nil, err
	}
	if err := writeKeychainKey(key); err != nil {
		return nil, err
	}
	return key, nil
}

func (darwinKeyStore) DeleteKey() error {
	args := append([]string{"delete-generic-password", "-s", keychainService, "-a", keychainAccount}, smokeKeychainPathArg()...)
	err := runSecurityCommand(args...)
	if errors.Is(err, errKeychainItemNotFound) {
		return nil
	}
	return err
}

var errKeychainItemNotFound = errors.New("keychain item not found")

func readKeychainKey() ([]byte, error) {
	args := append([]string{"find-generic-password", "-s", keychainService, "-a", keychainAccount, "-w"}, smokeKeychainPathArg()...)
	out, err := securityCommandOutput(args...)
	if err != nil {
		return nil, err
	}
	encoded := strings.TrimSpace(string(out))
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != vaultKeySize {
		return nil, fmt.Errorf("invalid keychain key payload")
	}
	return key, nil
}

func writeKeychainKey(key []byte) error {
	if len(key) != vaultKeySize {
		return fmt.Errorf("invalid key length")
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	args := append([]string{"add-generic-password", "-U", "-s", keychainService, "-a", keychainAccount, "-w", encoded}, smokeKeychainPathArg()...)
	return runSecurityCommand(args...)
}

func runSecurityCommand(args ...string) error {
	_, err := securityCommandOutput(args...)
	return err
}

func securityCommandOutput(args ...string) ([]byte, error) {
	out, err := exec.Command("/usr/bin/security", args...).CombinedOutput()
	if err == nil {
		return out, nil
	}
	if isKeychainItemNotFound(string(out)) {
		return nil, errKeychainItemNotFound
	}
	return nil, fmt.Errorf("macOS Keychain operation failed")
}

func smokeKeychainPathArg() []string {
	if path := os.Getenv("UTILS_KEYCHAIN_PATH"); path != "" {
		return []string{path}
	}
	return nil
}

func isKeychainItemNotFound(output string) bool {
	output = strings.ToLower(output)
	return strings.Contains(output, "could not be found") ||
		strings.Contains(output, "specified item could not be found") ||
		strings.Contains(output, "item not found")
}
