package identity

import (
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"strings"
	"unsafe"
)

var advapi = windows.NewLazySystemDLL("advapi32.dll")
var credRead = advapi.NewProc("CredReadW")
var credFree = advapi.NewProc("CredFree")
var logonUser = advapi.NewProc("LogonUserW")
var restrictToken = advapi.NewProc("CreateRestrictedToken")

// credential matches CREDENTIALW on Windows; returned memory belongs to CredFree.
type credential struct {
	Flags, Type             uint32
	TargetName, Comment     *uint16
	LastWritten             windows.Filetime
	BlobSize                uint32
	Blob                    *byte
	Persist, AttributeCount uint32
	Attributes              uintptr
	TargetAlias, UserName   *uint16
}

// Open resolves only the explicitly referenced generic credential in the
// daemon account's credential set. Passwords never enter config or environment.
func Open(ref Reference) (windows.Token, error) {
	if err := ref.Validate(); err != nil {
		return 0, err
	}
	user, password, err := readCredential(ref.CredentialTarget)
	if err != nil {
		return 0, err
	}
	defer clear(password)
	domain, _ := windows.UTF16PtrFromString(".")
	var token windows.Token
	// Batch logon; no interactive session or user profile is loaded.
	ok, _, logonErr := logonUser.Call(uintptr(unsafe.Pointer(user)), uintptr(unsafe.Pointer(domain)), uintptr(unsafe.Pointer(&password[0])), 4, 0, uintptr(unsafe.Pointer(&token)))
	runtime.KeepAlive(password)
	if ok == 0 {
		return 0, fmt.Errorf("game batch logon: %w", logonErr)
	}
	defer token.Close()
	if err := validateToken(token, ref.SID); err != nil {
		return 0, err
	}
	return Restrict(token)
}

func readCredential(target string) (*uint16, []uint16, error) {
	name, _ := windows.UTF16PtrFromString(target)
	var stored *credential
	if ok, _, err := credRead.Call(uintptr(unsafe.Pointer(name)), 1, 0, uintptr(unsafe.Pointer(&stored))); ok == 0 {
		return nil, nil, fmt.Errorf("read game credential: %w", err)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(stored)))
	if stored == nil {
		return nil, nil, errors.New("game credential was empty")
	}
	if stored.BlobSize > 5120 || stored.BlobSize == 0 || stored.BlobSize%2 != 0 || stored.Blob == nil {
		return nil, nil, errors.New("game credential must contain a UTF-16 password")
	}
	blob := unsafe.Slice(stored.Blob, int(stored.BlobSize))
	defer clear(blob)
	username := windows.UTF16PtrToString(stored.UserName)
	if !strings.HasPrefix(username, `.\`) || len(username) <= 2 || strings.ContainsAny(username[2:], `\/@`) {
		return nil, nil, errors.New("game credential username must use .\\local-account")
	}
	user, err := windows.UTF16PtrFromString(username[2:])
	if err != nil {
		return nil, nil, errors.New("invalid game account name")
	}
	password := make([]uint16, len(blob)/2+1)
	for i := 0; i < len(password)-1; i++ {
		password[i] = binary.LittleEndian.Uint16(blob[i*2:])
		if password[i] == 0 {
			clear(password)
			return nil, nil, errors.New("game credential password contains NUL")
		}
	}
	return user, password, nil
}

func validateToken(token windows.Token, pinned string) error {
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("query game token: %w", err)
	}
	if user.User.Sid.String() != pinned {
		return errors.New("game token SID differs from pinned run_as.sid")
	}
	current, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("query daemon token: %w", err)
	}
	if current.User.Sid.Equals(user.User.Sid) {
		return errors.New("game identity must differ from the daemon account")
	}
	groups, err := token.GetTokenGroups()
	if err != nil {
		return fmt.Errorf("query game groups: %w", err)
	}
	for _, g := range groups.AllGroups() {
		for _, kind := range []windows.WELL_KNOWN_SID_TYPE{windows.WinBuiltinAdministratorsSid, windows.WinBuiltinPowerUsersSid, windows.WinBuiltinBackupOperatorsSid, windows.WinBuiltinAccountOperatorsSid, windows.WinBuiltinSystemOperatorsSid, windows.WinBuiltinPrintOperatorsSid, windows.WinBuiltinNetworkConfigurationOperatorsSid, windows.WinBuiltinHyperVAdminsSid} {
			// Include deny-only/disabled groups: do not accept filtered admin accounts.
			if g.Sid.IsWellKnown(kind) {
				return errors.New("game account belongs to a privileged local group")
			}
		}
	}
	return nil
}

// Restrict removes token privileges except Windows' retained traverse privilege.
// It does not impose filesystem/network sandboxing or change the user SID.
func Restrict(token windows.Token) (windows.Token, error) {
	var restricted windows.Token
	if ok, _, err := restrictToken.Call(uintptr(token), 1, 0, 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&restricted))); ok == 0 {
		return 0, fmt.Errorf("restrict game token: %w", err)
	}
	return restricted, nil
}
