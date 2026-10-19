//go:build !windows

package identity

import "errors"

func CredentialStatus(string) (CredentialMetadata, error) {
	return CredentialMetadata{}, errors.New("credential management requires Windows")
}
func RemoveCredential(string) (bool, error) {
	return false, errors.New("credential management requires Windows")
}
