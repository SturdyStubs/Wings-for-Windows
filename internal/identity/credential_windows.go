package identity

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"time"
	"unsafe"
)

// CredentialStatus reports only metadata. It never decodes the password or
// attempts account logon; a present entry is not proof of working credentials.
func CredentialStatus(serverID string) (CredentialMetadata, error) {
	target, err := CredentialTarget(serverID)
	if err != nil {
		return CredentialMetadata{}, err
	}
	result := CredentialMetadata{Target: target}
	name, _ := windows.UTF16PtrFromString(target)
	var stored *credential
	if ok, _, err := credRead.Call(uintptr(unsafe.Pointer(name)), 1, 0, uintptr(unsafe.Pointer(&stored))); ok == 0 {
		if errors.Is(err, windows.ERROR_NOT_FOUND) {
			return result, nil
		}
		return result, fmt.Errorf("read game credential metadata: %w", err)
	}
	if stored == nil {
		return result, errors.New("credential metadata was empty")
	}
	defer releaseCredential(stored)
	result.Present = true
	result.Username = windows.UTF16PtrToString(stored.UserName)
	result.LastWritten = time.Unix(0, stored.LastWritten.Nanoseconds()).UTC().Format(time.RFC3339Nano)
	switch stored.Persist {
	case 1:
		result.Persistence = "session"
	case 2:
		result.Persistence = "local_machine"
	case 3:
		result.Persistence = "enterprise"
	default:
		result.Persistence = "unknown"
	}
	return result, nil
}

// RemoveCredential deletes only the exact UUID target in the current account.
// Missing entries succeed without changing other credentials or running games.
func RemoveCredential(serverID string) (bool, error) {
	target, err := CredentialTarget(serverID)
	if err != nil {
		return false, err
	}
	name, _ := windows.UTF16PtrFromString(target)
	if ok, _, err := advapi.NewProc("CredDeleteW").Call(uintptr(unsafe.Pointer(name)), 1, 0); ok == 0 {
		if errors.Is(err, windows.ERROR_NOT_FOUND) {
			return false, nil
		}
		return false, fmt.Errorf("remove game credential: %w", err)
	}
	return true, nil
}

func releaseCredential(stored *credential) {
	if stored.Blob != nil && stored.BlobSize <= 5120 {
		clear(unsafe.Slice(stored.Blob, int(stored.BlobSize)))
	}
	credFree.Call(uintptr(unsafe.Pointer(stored)))
}
