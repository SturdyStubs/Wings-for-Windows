package identity

import (
	"errors"
	"strings"
)

type CredentialMetadata struct {
	Target      string `json:"target"`
	Present     bool   `json:"present"`
	Username    string `json:"username,omitempty"`
	LastWritten string `json:"last_written,omitempty"`
	Persistence string `json:"persistence,omitempty"`
}

// CredentialTarget limits management operations to one exact project target.
func CredentialTarget(serverID string) (string, error) {
	name := "WingsWindows/" + strings.ToLower(serverID)
	if !target.MatchString(name) {
		return "", errors.New("credential operation requires a full server UUID")
	}
	return name, nil
}
