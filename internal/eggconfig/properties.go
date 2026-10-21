package eggconfig

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/magiconair/properties"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type propertyRule struct{ key, value, condition string }

// Join physical continuations before decoding escapes. An escape (including
// a Unicode escape) may itself be split across continued physical lines.
func joinPropertyLines(ctx context.Context, input []byte) ([]byte, error) {
	var out bytes.Buffer
	lineStart := true
	for i := 0; i < len(input); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if input[i] == '\n' {
			out.WriteByte('\n')
			i++
			lineStart = true
			continue
		}
		if lineStart {
			if input[i] == ' ' || input[i] == '\t' || input[i] == '\f' {
				out.WriteByte(input[i])
				i++
				continue
			}
			if input[i] == '#' || input[i] == '!' {
				for i < len(input) && input[i] != '\n' {
					out.WriteByte(input[i])
					i++
				}
				continue
			}
			lineStart = false
		}
		if input[i] == '\\' && i+1 < len(input) {
			if input[i+1] == '\n' {
				i += 2
				for i < len(input) && (input[i] == ' ' || input[i] == '\t' || input[i] == '\f') {
					i++
				}
				continue
			}
			out.Write(input[i : i+2])
			i += 2
			continue
		}
		out.WriteByte(input[i])
		i++
	}
	return out.Bytes(), nil
}

func compilePropertyRules(items []map[string]json.RawMessage) ([]propertyRule, error) {
	var rules []propertyRule
	for _, item := range items {
		for key := range item {
			if key != "match" && key != "if_value" && key != "replace_with" && key != "value" {
				return nil, errors.New("unknown properties replacement field")
			}
		}
		key, err := scalar(item["match"])
		if err != nil || key == "" || key == "<nil>" || len(key) > 8192 || strings.ContainsRune(key, 0) {
			return nil, errors.New("invalid properties key")
		}
		raw, ok := item["replace_with"]
		if !ok {
			raw = item["value"]
		}
		value, err := scalar(raw)
		if err != nil || len(value) > 8192 || strings.ContainsRune(value, 0) || strings.Contains(value, "{{") {
			return nil, errors.New("invalid or unresolved properties replacement")
		}
		rule := propertyRule{key: key, value: value}
		if raw, ok := item["if_value"]; ok && string(raw) != "null" {
			rule.condition, err = scalar(raw)
			if err != nil || len(rule.condition) > 8192 {
				return nil, errors.New("invalid properties condition")
			}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// The library decodes BMP escapes but treats UTF-16 surrogate units as invalid
// standalone runes. Normalize only valid escaped pairs into literal UTF-8;
// preserve all other escapes so escaped separators cannot become syntax.
func normalizeSurrogates(ctx context.Context, input []byte) ([]byte, error) {
	var out bytes.Buffer
	lineStart := true
	for i := 0; i < len(input); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if input[i] == '\r' || input[i] == '\n' {
			out.WriteByte(input[i])
			i++
			lineStart = true
			continue
		}
		if lineStart {
			if input[i] == ' ' || input[i] == '\t' || input[i] == '\f' {
				out.WriteByte(input[i])
				i++
				continue
			}
			if input[i] == '#' || input[i] == '!' {
				for i < len(input) && input[i] != '\r' && input[i] != '\n' {
					out.WriteByte(input[i])
					i++
				}
				continue
			}
			lineStart = false
		}
		if input[i] != '\\' || i+1 >= len(input) {
			out.WriteByte(input[i])
			i++
			continue
		}
		if input[i+1] != 'u' {
			if input[i+1] == '\r' || input[i+1] == '\n' {
				if input[i+1] == '\r' && i+2 < len(input) && input[i+2] == '\n' {
					i++
				}
				i += 2
				for i < len(input) && (input[i] == ' ' || input[i] == '\t' || input[i] == '\f') {
					i++
				}
				lineStart = false
				continue
			}
			out.Write(input[i : i+2])
			i += 2
			continue
		}
		if i+6 > len(input) {
			return nil, errors.New("invalid properties Unicode escape")
		}
		unit, err := strconv.ParseUint(string(input[i+2:i+6]), 16, 16)
		if err != nil {
			return nil, errors.New("invalid properties Unicode escape")
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			if i+12 > len(input) || string(input[i+6:i+8]) != `\u` {
				return nil, errors.New("unpaired properties surrogate")
			}
			low, err := strconv.ParseUint(string(input[i+8:i+12]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return nil, errors.New("unpaired properties surrogate")
			}
			out.WriteRune(utf16.DecodeRune(rune(unit), rune(low)))
			i += 12
			continue
		}
		if unit >= 0xdc00 && unit <= 0xdfff {
			return nil, errors.New("unpaired properties surrogate")
		}
		out.Write(input[i : i+6])
		i += 6
	}
	return out.Bytes(), nil
}
func writePropertyString(ctx context.Context, emit func(string) error, value string, key bool) error {
	leading := true
	for _, r := range value {
		if err := ctx.Err(); err != nil {
			return err
		}
		var encoded string
		switch r {
		case '\\':
			encoded = `\\`
		case '\n':
			encoded = `\n`
		case '\r':
			encoded = `\r`
		case '\t':
			encoded = `\t`
		case '\f':
			encoded = `\f`
		default:
			switch {
			case r < 32 || r > 126:
				if r <= 0xffff {
					encoded = fmt.Sprintf(`\u%04x`, r)
				} else {
					high, low := utf16.EncodeRune(r)
					encoded = fmt.Sprintf(`\u%04x\u%04x`, high, low)
				}
			case (key && strings.ContainsRune(" =:#!", r)) || (leading && r == ' '):
				encoded = "\\" + string(r)
			default:
				encoded = string(r)
			}
		}
		if err := emit(encoded); err != nil {
			return err
		}
		if r != ' ' && r != '\t' && r != '\f' {
			leading = false
		}
	}
	return nil
}
func renderProperties(ctx context.Context, input []byte, rules []propertyRule) ([]byte, error) {
	if len(input) > MaxFileBytes || !utf8.Valid(input) || bytes.ContainsRune(input, 0) || bytes.HasPrefix(input, []byte{0xef, 0xbb, 0xbf}) {
		return nil, errors.New("properties input exceeds bounds or encoding")
	}
	input = bytes.ReplaceAll(bytes.ReplaceAll(input, []byte("\r\n"), []byte("\n")), []byte("\r"), []byte("\n"))
	if bytes.Count(input, []byte{'\n'}) > 65536 {
		return nil, errors.New("properties line count exceeds bounds")
	}
	joined, err := joinPropertyLines(ctx, input)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeSurrogates(ctx, joined)
	if err != nil {
		return nil, err
	}
	loader := properties.Loader{Encoding: properties.UTF8, DisableExpansion: true}
	values, err := loader.LoadBytes(normalized)
	if err != nil {
		return nil, errors.New("invalid properties input")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(values.Keys()) > 16384 {
		return nil, errors.New("properties key count exceeds bounds")
	}
	// Validate decoded data too: escaped NULs must not survive the raw-input check.
	for _, key := range values.Keys() {
		value, _ := values.Get(key)
		if len(key) > 8192 || len(value) > 64<<10 || strings.ContainsRune(key, 0) || strings.ContainsRune(value, 0) {
			return nil, errors.New("decoded properties data exceeds bounds")
		}
	}
	for _, rule := range rules {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		current, exists := values.Get(rule.key)
		if rule.condition != "" && (!exists || current != rule.condition) {
			continue
		}
		if _, _, err := values.Set(rule.key, rule.value); err != nil {
			return nil, errors.New("properties replacement failed")
		}
	}
	if len(values.Keys()) > 16384 {
		return nil, errors.New("properties key count exceeds bounds")
	}
	var output bytes.Buffer
	emit := func(value string) error {
		if output.Len()+len(value) > MaxFileBytes {
			return errors.New("properties output exceeds 1 MiB")
		}
		output.WriteString(value)
		return nil
	}
	// Retain initial comments/blank lines only. Inline comments/formatting are
	// intentionally normalized; comments are never treated as configuration data.
	scanner := bufio.NewScanner(bytes.NewReader(input))
	scanner.Buffer(make([]byte, 4096), MaxFileBytes+1)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := scanner.Text()
		trimmed := strings.TrimLeft(line, " \t\f")
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "!") {
			break
		}
		if err := emit(line + "\n"); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("invalid properties header")
	}
	for _, key := range values.Keys() {
		value, _ := values.Get(key)
		if err := writePropertyString(ctx, emit, key, true); err != nil {
			return nil, err
		}
		if err := emit("="); err != nil {
			return nil, err
		}
		if err := writePropertyString(ctx, emit, value, false); err != nil {
			return nil, err
		}
		if err := emit("\n"); err != nil {
			return nil, err
		}
	}
	return output.Bytes(), ctx.Err()
}
