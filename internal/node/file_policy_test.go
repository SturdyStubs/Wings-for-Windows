package node

import (
	"strings"
	"testing"
)

func TestFilePolicyPatterns(t *testing.T) {
	policy, err := NewFilePolicy([]string{"/root.cfg", "private/", "**/*.secret", "!public.secret", "name(1).cfg", "file?.ini"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name              string
		directory, denied bool
	}{{"root.cfg", false, true}, {"ROOT.CFG", false, true}, {"nested/root.cfg", false, false}, {"private", true, true}, {"private/deep/config.cfg", false, true}, {"nested/value.secret", false, true}, {"public.secret", false, false}, {"server.cfg", false, false}, {"name(1).cfg", false, true}, {"name1.cfg", false, false}, {"file1.ini", false, true}} {
		if got := policy.Denied(test.name, test.directory); got != test.denied {
			t.Errorf("%s: got denied=%v", test.name, got)
		}
	}
}
func TestFilePolicyBounds(t *testing.T) {
	for _, patterns := range [][]string{make([]string, 257), {strings.Repeat("x", 32769)}, {"bad\npattern"}, {strings.Repeat("*", 17)}, {"[invalid"}} {
		if _, err := NewFilePolicy(patterns); err == nil {
			t.Fatal("unbounded file policy accepted")
		}
	}
}
