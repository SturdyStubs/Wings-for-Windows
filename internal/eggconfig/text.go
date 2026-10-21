// Package eggconfig implements an explicitly authorized subset of egg rewriting.
package eggconfig

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
	"wings-windows/internal/config"
)

const MaxFileBytes = 1 << 20

type Replacement struct {
	Match string
	Value string
}
type File struct {
	Name            string
	Replace         []Replacement
	Parser          string
	JSONRules       []jsonRule
	PropertiesRules []propertyRule
}

func scalar(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("missing scalar")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", errors.New("invalid scalar")
	}
	switch value := value.(type) {
	case string:
		return value, nil
	case json.Number:
		return string(value), nil
	case bool:
		if value {
			return "true", nil
		}
		return "false", nil
	case nil:
		return "<nil>", nil
	default:
		return "", errors.New("replacement must be a scalar")
	}
}

// Compile validates every remote entry before any target is opened or changed.
func Compile(raw []json.RawMessage, allowed []string) ([]File, error) {
	if len(raw) > 16 {
		return nil, errors.New("egg configuration exceeds 16 files")
	}
	allow := map[string]bool{}
	for _, name := range allowed {
		if err := config.EggFileName(name); err != nil {
			return nil, err
		}
		allow[name] = true
	}
	seen := map[string]bool{}
	result := make([]File, 0, len(raw))
	total := 0
	for _, entry := range raw {
		total += len(entry)
		if total > 128<<10 {
			return nil, errors.New("egg configuration exceeds 128 KiB")
		}
		var item struct {
			Name    string                       `json:"file"`
			Parser  string                       `json:"parser"`
			Replace []map[string]json.RawMessage `json:"replace"`
		}
		decoder := json.NewDecoder(bytes.NewReader(entry))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&item) != nil || decoder.Decode(new(any)) != io.EOF {
			return nil, errors.New("invalid egg configuration entry")
		}
		if config.EggFileName(item.Name) != nil || !allow[item.Name] || seen[strings.ToLower(item.Name)] {
			return nil, errors.New("egg configuration target is invalid, duplicated or not locally approved")
		}
		seen[strings.ToLower(item.Name)] = true
		if item.Parser != "file" && item.Parser != "json" && item.Parser != "properties" {
			return nil, errors.New("only file, json and properties egg parsers are supported")
		}
		if item.Replace == nil || len(item.Replace) > 128 {
			return nil, errors.New("egg file requires at most 128 replacements")
		}
		file := File{Name: item.Name, Parser: item.Parser}
		if item.Parser == "properties" {
			var err error
			file.PropertiesRules, err = compilePropertyRules(item.Replace)
			if err != nil {
				return nil, err
			}
			result = append(result, file)
			continue
		}
		if item.Parser == "json" {
			var err error
			file.JSONRules, err = compileJSONRules(item.Replace)
			if err != nil {
				return nil, err
			}
			result = append(result, file)
			continue
		}
		for _, rule := range item.Replace {
			for key := range rule {
				if key != "match" && key != "if_value" && key != "replace_with" && key != "value" {
					return nil, errors.New("unknown replacement field")
				}
			}
			match, err := scalar(rule["match"])
			if err != nil || match == "" || match == "<nil>" || len(match) > 8192 || strings.ContainsAny(match, "\x00\r\n") {
				return nil, errors.New("invalid text prefix")
			}
			if value, ok := rule["if_value"]; ok && string(value) != "null" {
				condition, err := scalar(value)
				if err != nil || condition != "" {
					return nil, errors.New("conditional text replacements are unsupported")
				}
			}
			value, ok := rule["replace_with"]
			if !ok {
				value = rule["value"]
			}
			text, err := scalar(value)
			if err != nil || len(text) > 8192 || strings.ContainsRune(text, 0) || strings.Contains(text, "{{") {
				return nil, errors.New("invalid or unresolved replacement value")
			}
			file.Replace = append(file.Replace, Replacement{match, text})
		}
		result = append(result, file)
	}
	return result, nil
}

// Render matches upstream file-parser prefix behavior, including ordered
// concatenation when multiple rules match a line and LF output normalization.
func Render(ctx context.Context, input []byte, file File) ([]byte, error) {
	if file.Parser == "properties" {
		return renderProperties(ctx, input, file.PropertiesRules)
	}
	if file.Parser == "json" {
		return renderJSON(ctx, input, file.JSONRules)
	}
	if file.Parser != "" && file.Parser != "file" {
		return nil, errors.New("unsupported egg parser")
	}
	if len(input) > MaxFileBytes || !utf8.Valid(input) || bytes.ContainsRune(input, 0) || bytes.HasPrefix(input, []byte{0xef, 0xbb, 0xbf}) {
		return nil, errors.New("egg text file must be bounded UTF-8 without BOM/NUL")
	}
	scanner := bufio.NewScanner(bytes.NewReader(input))
	scanner.Buffer(make([]byte, 4096), MaxFileBytes+1)
	var output bytes.Buffer
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := scanner.Text()
		matched := false
		for _, rule := range file.Replace {
			if strings.HasPrefix(line, rule.Match) {
				if output.Len()+len(rule.Value)+1 > MaxFileBytes {
					return nil, errors.New("egg text result exceeds 1 MiB")
				}
				output.WriteString(rule.Value)
				matched = true
			}
		}
		if !matched {
			if output.Len()+len(line)+1 > MaxFileBytes {
				return nil, errors.New("egg text result exceeds 1 MiB")
			}
			output.WriteString(line)
		}
		output.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("cannot scan egg text file")
	}
	return output.Bytes(), ctx.Err()
}
