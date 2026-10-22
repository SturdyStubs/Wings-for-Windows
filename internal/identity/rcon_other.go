//go:build !windows

package identity

import "errors"

func RCONCredentialStatus(string) (CredentialMetadata, error) {
	return CredentialMetadata{}, errors.New("RCON credentials require Windows")
}
func RemoveRCONCredential(string) (bool, error) {
	return false, errors.New("RCON credentials require Windows")
}
func RCONPassword(string) (string, error) { return "", errors.New("RCON credentials require Windows") }
