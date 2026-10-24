package node

import (
	"errors"
	ignore "github.com/sabhiram/go-gitignore"
	"path"
	"strings"
)

// FilePolicy is an immutable, case-folded egg denylist for the Windows file API.
// It enforces path policy, not an OS sandbox against the game executable.
type FilePolicy struct{ matcher *ignore.GitIgnore }

func NewFilePolicy(patterns []string) (*FilePolicy, error) {
	if len(patterns) > 256 {
		return nil, errors.New("egg file denylist exceeds 256 patterns")
	}
	folded := make([]string, len(patterns))
	size := 0
	for i, pattern := range patterns {
		size += len(pattern)
		if size > 32*1024 || strings.ContainsAny(pattern, "\x00\r\n") || strings.Count(pattern, "*") > 16 {
			return nil, errors.New("egg file denylist exceeds supported bounds")
		}
		glob := strings.TrimPrefix(pattern, "!")
		if !strings.HasPrefix(strings.TrimSpace(glob), "#") {
			if _, err := path.Match(glob, ""); err != nil {
				return nil, errors.New("invalid egg file denylist glob")
			}
		}
		// The upstream library leaves regex-only operators unescaped and treats
		// '?' literally. Preserve glob intent for Windows-valid filenames.
		var safe strings.Builder
		for _, r := range strings.ToLower(pattern) {
			if strings.ContainsRune(`+()|^${}`, r) {
				safe.WriteRune('\\')
			}
			if r == '?' {
				safe.WriteString("[^/]")
			} else {
				safe.WriteRune(r)
			}
		}
		folded[i] = safe.String()
	}
	return &FilePolicy{ignore.CompileIgnoreLines(folded...)}, nil
}
func (p *FilePolicy) Denied(name string, directory bool) bool {
	if p == nil {
		return true
	}
	clean := strings.ToLower(strings.TrimPrefix(path.Clean("/"+name), "/"))
	if clean == "" {
		clean = "."
	}
	if directory && clean != "." {
		clean += "/"
	}
	return p.matcher.MatchesPath(clean)
}
func (s *Server) FileDenied(name string, directory bool) bool {
	s.mu.Lock()
	policy := s.filePolicy
	s.mu.Unlock()
	return policy.Denied(name, directory)
}

// FilePolicySnapshot returns an immutable policy identity for bounded multi-step
// operations. Sync replaces the policy; callers may then refuse publication.
func (s *Server) FilePolicySnapshot() *FilePolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filePolicy
}
