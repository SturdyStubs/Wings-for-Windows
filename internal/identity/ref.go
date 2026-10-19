// Package identity resolves locally approved Windows execution identities.
package identity

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type Reference struct {
	SID              string `json:"sid"`
	CredentialTarget string `json:"credential_target"`
}

type PathRule struct {
	Path      string
	Forbidden uint32
	Recursive bool
}

// File/directory access bits used for a read-only protected-path DACL audit.
const WriteAccess uint32 = 0x2 | 0x4 | 0x10 | 0x100 | 0x10000 | 0x40000 | 0x80000
const ReadWriteAccess uint32 = WriteAccess | 0x1 | 0x40
const ReplaceChildAccess uint32 = WriteAccess | 0x40
const AncestorAccess uint32 = 0x40 | 0x10000 | 0x40000 | 0x80000

var localAccount = regexp.MustCompile(`^S-1-5-21-([0-9]+)-([0-9]+)-([0-9]+)-([0-9]+)$`)
var target = regexp.MustCompile(`^WingsWindows/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (r Reference) Validate() error {
	m := localAccount.FindStringSubmatch(r.SID)
	if m == nil {
		return errors.New("run_as.sid must be a canonical local account SID")
	}
	for i, part := range m[1:] {
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil || strconv.FormatUint(n, 10) != part || (i == 3 && n < 1000) {
			return errors.New("run_as.sid must identify a non-built-in local account")
		}
	}
	if !target.MatchString(r.CredentialTarget) {
		return errors.New("run_as.credential_target must be WingsWindows/<lowercase-server-uuid>")
	}
	return nil
}

func (r Reference) MatchesServer(id string) bool {
	return r.CredentialTarget == "WingsWindows/"+strings.ToLower(id)
}
