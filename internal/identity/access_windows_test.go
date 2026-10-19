package identity

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccessCheckUsesEffectiveDACLGrants(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	var original windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	var token windows.Token
	if err := windows.DuplicateTokenEx(original, windows.TOKEN_QUERY, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &token); err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	for _, test := range []struct {
		name, sddl  string
		read, write bool
	}{
		{"read only", "O:SYG:SYD:P(A;;FR;;;" + sid + ")", true, false},
		{"deny read allow write", "O:SYG:SYD:P(D;;FR;;;" + sid + ")(A;;FW;;;" + sid + ")", false, true},
		{"world full access", "O:SYG:SYD:P(A;;FA;;;WD)", true, true},
		{"no grants", "O:SYG:SYD:P", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			sd, err := windows.SecurityDescriptorFromString(test.sddl)
			if err != nil {
				t.Fatal(err)
			}
			grant, err := grantedAccess(token, sd)
			if err != nil {
				t.Fatal(err)
			}
			if (grant&windows.FILE_READ_DATA != 0) != test.read || (grant&windows.FILE_WRITE_DATA != 0) != test.write {
				t.Fatalf("incorrect effective grant 0x%x", grant)
			}
		})
	}
}

func TestProtectedPathAuditFailsOnExposureAndMissingPaths(t *testing.T) {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &token); err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	path := filepath.Join(t.TempDir(), "secret.json")
	if err := os.WriteFile(path, []byte("test-only"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckAccess(token, []PathRule{{Path: path, Forbidden: ReadWriteAccess}}); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatal("readable secret was accepted", err)
	}
	if err := CheckAccess(token, []PathRule{{Path: path + "-missing", Forbidden: ReadWriteAccess}}); err == nil {
		t.Fatal("missing path ACL was ignored")
	}
	if err := CheckAccess(token, nil); err == nil {
		t.Fatal("empty path policy accepted")
	}
}

func TestRawGenericFilePermissionIsRejected(t *testing.T) {
	sd, err := windows.SecurityDescriptorFromString("O:SYG:SYD:P(A;;GR;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	var original, token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	if err := windows.DuplicateTokenEx(original, windows.TOKEN_QUERY, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &token); err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	if _, err := grantedAccess(token, sd); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatal("raw generic permission was silently ignored", err)
	}
}
