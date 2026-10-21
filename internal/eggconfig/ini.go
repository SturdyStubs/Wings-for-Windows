package eggconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/ini.v1"
)

type iniRule struct{ section, key, value string }
type iniPair struct{ key, value string }
type iniSection struct {
	name  string
	pairs []iniPair
}

func iniName(name string, key bool) bool {
	if name == "" || len(name) > 8192 || strings.TrimSpace(name) != name || strings.ContainsAny(name, "[]\x00\r\n") {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	if key && (name == "-" || strings.ContainsAny(name, "=:;#\"'`")) {
		return false
	}
	return true
}
func iniValue(value string, limit int) bool {
	return len(value) <= limit && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n") && !(strings.Contains(value, "`") && strings.Contains(value, `"""`))
}

// A bracketed section can contain dots. Otherwise only the first dot splits
// section and key, following the upstream INI path convention.
func iniPath(match string) (string, string, error) {
	section, key := "DEFAULT", match
	if strings.HasPrefix(match, "[") {
		end := strings.Index(match, "].")
		if end < 0 {
			return "", "", errors.New("invalid INI section path")
		}
		section, key = match[1:end], match[end+2:]
	} else if dot := strings.IndexByte(match, '.'); dot >= 0 {
		section, key = match[:dot], match[dot+1:]
	}
	if !iniName(section, false) || !iniName(key, true) {
		return "", "", errors.New("invalid INI section or key")
	}
	return section, key, nil
}
func compileINIRules(items []map[string]json.RawMessage) ([]iniRule, error) {
	var rules []iniRule
	for _, item := range items {
		for field := range item {
			if field != "match" && field != "if_value" && field != "replace_with" && field != "value" {
				return nil, errors.New("unknown INI replacement field")
			}
		}
		match, err := scalar(item["match"])
		if err != nil || match == "<nil>" || len(match) > 8192 {
			return nil, errors.New("invalid INI match")
		}
		section, key, err := iniPath(match)
		if err != nil {
			return nil, err
		}
		if raw, ok := item["if_value"]; ok && string(raw) != "null" {
			condition, err := scalar(raw)
			if err != nil || condition != "" {
				return nil, errors.New("conditional INI rules are unsupported")
			}
		}
		raw, ok := item["replace_with"]
		if !ok {
			raw = item["value"]
		}
		value, err := scalar(raw)
		if err != nil || !iniValue(value, 8192) || strings.Contains(value, "{{") {
			return nil, errors.New("invalid or unresolved INI value")
		}
		rules = append(rules, iniRule{section, key, value})
	}
	return rules, nil
}
func loadINI(ctx context.Context, input []byte) ([]iniSection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(input) > MaxFileBytes || !utf8.Valid(input) || bytes.ContainsRune(input, 0) || bytes.HasPrefix(input, []byte{0xef, 0xbb, 0xbf}) {
		return nil, errors.New("INI input exceeds bounds or encoding")
	}
	input = bytes.ReplaceAll(bytes.ReplaceAll(input, []byte("\r\n"), []byte("\n")), []byte("\r"), []byte("\n"))
	if bytes.Count(input, []byte{'\n'}) > 8192 {
		return nil, errors.New("INI line count exceeds bounds")
	}
	// Refuse multiline quoted values before the library can concatenate lines.
	// This subset treats backslashes literally, including Windows path endings.
	for _, line := range bytes.Split(input, []byte{'\n'}) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		text := strings.TrimSpace(string(line))
		if text == "" || strings.HasPrefix(text, "#") || strings.HasPrefix(text, ";") || strings.HasPrefix(text, "[") {
			continue
		}
		split := strings.IndexAny(text, "=:")
		if split < 0 || !iniName(strings.TrimSpace(text[:split]), true) {
			return nil, errors.New("unsupported INI key syntax")
		}
		value := strings.TrimSpace(text[split+1:])
		quote := ""
		if strings.HasPrefix(value, `"""`) {
			quote = `"""`
		} else if strings.HasPrefix(value, "`") {
			quote = "`"
		}
		if quote != "" && !strings.Contains(value[len(quote):], quote) {
			return nil, errors.New("multiline INI values are unsupported")
		}
	}
	// Byte input only: no filesystem loading, environment mapping or interpolated
	// String()/GetKey() calls. Keys() enumerates the exact section, not ancestors.
	cfg, err := ini.LoadSources(ini.LoadOptions{IgnoreContinuation: true}, input)
	if err != nil {
		return nil, errors.New("invalid INI input")
	}
	sections := cfg.Sections()
	if len(sections) > 512 {
		return nil, errors.New("INI section count exceeds bounds")
	}
	result := make([]iniSection, 0, len(sections))
	total := 0
	for _, section := range sections {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !iniName(section.Name(), false) {
			return nil, errors.New("unsupported INI section")
		}
		target := iniSection{name: section.Name()}
		for _, key := range section.Keys() {
			total++
			if total > 4096 || !iniName(key.Name(), true) || !iniValue(key.Value(), 64<<10) {
				return nil, errors.New("INI data exceeds supported shape or bounds")
			}
			target.pairs = append(target.pairs, iniPair{key.Name(), key.Value()})
		}
		result = append(result, target)
	}
	return result, ctx.Err()
}
func renderINI(ctx context.Context, input []byte, rules []iniRule) ([]byte, error) {
	sections, err := loadINI(ctx, input)
	if err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		index := -1
		for i := range sections {
			if sections[i].name == rule.section {
				index = i
				break
			}
		}
		if index < 0 {
			sections = append(sections, iniSection{name: rule.section})
			index = len(sections) - 1
		}
		found := false
		for i := range sections[index].pairs {
			if sections[index].pairs[i].key == rule.key {
				sections[index].pairs[i].value = rule.value
				found = true
				break
			}
		}
		if !found {
			sections[index].pairs = append(sections[index].pairs, iniPair{rule.key, rule.value})
		}
	}
	if len(sections) > 512 {
		return nil, errors.New("INI result section count exceeds bounds")
	}
	var output bytes.Buffer
	emit := func(value string) error {
		if output.Len()+len(value) > MaxFileBytes {
			return errors.New("INI output exceeds 1 MiB")
		}
		output.WriteString(value)
		return nil
	}
	count := 0
	for _, section := range sections {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if section.name != "DEFAULT" {
			if err := emit("[" + section.name + "]\n"); err != nil {
				return nil, err
			}
		}
		for _, pair := range section.pairs {
			count++
			if count > 4096 {
				return nil, errors.New("INI result key count exceeds bounds")
			}
			value := pair.value
			if strings.TrimSpace(value) != value || strings.ContainsAny(value, "`\"'#;") || strings.HasSuffix(value, "\\") {
				if strings.Contains(value, "`") {
					value = `"""` + value + `"""`
				} else {
					value = "`" + value + "`"
				}
			}
			if err := emit(pair.key + "=" + value + "\n"); err != nil {
				return nil, err
			}
		}
	}
	// Reject any non-representable serialization instead of injecting another
	// key/section or silently changing data. Both parses remain bounded.
	check, err := loadINI(ctx, output.Bytes())
	if err != nil {
		return nil, err
	}
	if !sameINI(sections, check) {
		return nil, errors.New("INI output cannot preserve parsed data")
	}
	return output.Bytes(), ctx.Err()
}
func sameINI(a, b []iniSection) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].name != b[i].name || len(a[i].pairs) != len(b[i].pairs) {
			return false
		}
		for j := range a[i].pairs {
			if a[i].pairs[j] != b[i].pairs[j] {
				return false
			}
		}
	}
	return true
}
