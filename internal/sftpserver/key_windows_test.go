package sftpserver

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
	"os"
	"path/filepath"
	"testing"
)

func TestSFTPHostKeyGuards(t *testing.T) {
	for _, kind := range []string{"valid", "missing", "invalid", "oversize", "hardlink", "locked"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			keyfile := filepath.Join(dir, "host-key")
			_, key, e := ed25519.GenerateKey(rand.Reader)
			if e != nil {
				t.Fatal(e)
			}
			block, e := ssh.MarshalPrivateKey(key, "")
			if e != nil {
				t.Fatal(e)
			}
			data := pem.EncodeToMemory(block)
			if kind == "invalid" {
				data = []byte("invalid")
			}
			if kind == "oversize" {
				data = make([]byte, 16385)
			}
			if kind != "missing" {
				if e = os.WriteFile(keyfile, data, 0600); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "hardlink" {
				if e = os.Link(keyfile, filepath.Join(dir, "linked")); e != nil {
					t.Fatal(e)
				}
			}
			var writer *os.File
			if kind == "locked" {
				writer, e = os.OpenFile(keyfile, os.O_WRONLY, 0)
				if e != nil {
					t.Fatal(e)
				}
				defer writer.Close()
			}
			signer, e := loadSigner(keyfile)
			if kind == "valid" {
				if e != nil || signer.PublicKey().Type() != ssh.KeyAlgoED25519 {
					t.Fatal(e)
				}
			} else if e == nil {
				t.Fatal("unsafe key accepted")
			}
		})
	}
}
