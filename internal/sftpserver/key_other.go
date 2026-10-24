//go:build !windows

package sftpserver

import (
	"errors"
	"golang.org/x/crypto/ssh"
)

func loadSigner(string) (ssh.Signer, error) { return nil, errors.New("Windows host required") }
