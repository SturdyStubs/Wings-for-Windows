package identity

import (
	"errors"
	"golang.org/x/sys/windows"
	"strings"
	"unicode/utf16"
)

func RCONCredentialStatus(serverID string) (CredentialMetadata, error) {
	target, err := RCONCredentialTarget(serverID)
	if err != nil {
		return CredentialMetadata{}, err
	}
	result, err := credentialStatusTarget(target)
	result.Username = ""
	return result, err
}
func RemoveRCONCredential(serverID string) (bool, error) {
	target, err := RCONCredentialTarget(serverID)
	if err != nil {
		return false, err
	}
	return removeCredentialTarget(target)
}
func RCONPassword(serverID string) (string, error) {
	target, err := RCONCredentialTarget(serverID)
	if err != nil {
		return "", err
	}
	user, password, err := readCredential(target)
	if err != nil {
		return "", errors.New("RCON credential is unavailable or invalid")
	}
	defer clear(password)
	if windows.UTF16PtrToString(user) != "WingsRCONSecret-v1" {
		return "", errors.New("RCON credential format is invalid")
	}
	units := password[:len(password)-1]
	for i := 0; i < len(units); i++ {
		if utf16.IsSurrogate(rune(units[i])) {
			if i+1 >= len(units) || utf16.DecodeRune(rune(units[i]), rune(units[i+1])) == '\ufffd' {
				return "", errors.New("RCON credential format is invalid")
			}
			i++
		}
	}
	value := string(utf16.Decode(units))
	if value == "" || len(value) > 4086 || strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("RCON credential format is invalid")
	}
	return value, nil
}
