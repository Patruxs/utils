//go:build windows

package accounts

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	credentialTarget        = "UTILS ChatGPT Codex Account Vault"
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

type windowsCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type windowsKeyStore struct{}

func NewDefaultKeyStore() KeyStore {
	return windowsKeyStore{}
}

func (windowsKeyStore) LoadOrCreateKey() ([]byte, error) {
	key, err := readCredentialKey()
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, syscall.ERROR_NOT_FOUND) {
		return nil, err
	}

	key, err = randomVaultKey()
	if err != nil {
		return nil, err
	}
	if err := writeCredentialKey(key); err != nil {
		return nil, err
	}
	return key, nil
}

func (windowsKeyStore) DeleteKey() error {
	target, err := syscall.UTF16PtrFromString(credentialTarget)
	if err != nil {
		return err
	}
	ret, _, callErr := procCredDelete.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(credTypeGeneric),
		0,
	)
	if ret == 0 {
		if errors.Is(callErr, syscall.ERROR_NOT_FOUND) {
			return nil
		}
		return callErr
	}
	return nil
}

func readCredentialKey() ([]byte, error) {
	target, err := syscall.UTF16PtrFromString(credentialTarget)
	if err != nil {
		return nil, err
	}
	var credentialPtr uintptr
	ret, _, callErr := procCredRead.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&credentialPtr)),
	)
	if ret == 0 {
		return nil, callErr
	}
	defer procCredFree.Call(credentialPtr)

	credential := (*windowsCredential)(unsafe.Pointer(credentialPtr))
	if credential.CredentialBlobSize != vaultKeySize {
		return nil, fmt.Errorf("invalid credential key length")
	}
	key := unsafe.Slice(credential.CredentialBlob, credential.CredentialBlobSize)
	return append([]byte(nil), key...), nil
}

func writeCredentialKey(key []byte) error {
	if len(key) != vaultKeySize {
		return fmt.Errorf("invalid credential key length")
	}
	target, err := syscall.UTF16PtrFromString(credentialTarget)
	if err != nil {
		return err
	}
	userName, err := syscall.UTF16PtrFromString("UTILS")
	if err != nil {
		return err
	}

	credential := windowsCredential{
		Type:               credTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(key)),
		CredentialBlob:     &key[0],
		Persist:            credPersistLocalMachine,
		UserName:           userName,
	}
	ret, _, callErr := procCredWrite.Call(
		uintptr(unsafe.Pointer(&credential)),
		0,
	)
	if ret == 0 {
		return callErr
	}
	return nil
}
