package eggconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type jsonStep struct {
	key         string
	index       int
	array, wild bool
}
type jsonRule struct {
	path      []jsonStep
	value     any
	condition string
}

func jsonPath(value string) ([]jsonStep, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 32 {
		return nil, errors.New("JSON path exceeds depth limit")
	}
	var steps []jsonStep
	wildcards := 0
	for i, part := range parts {
		if part == "*" {
			wildcards++
			if wildcards > 1 || i == 0 || i == len(parts)-1 {
				return nil, errors.New("unsupported JSON wildcard path")
			}
			steps = append(steps, jsonStep{wild: true})
			continue
		}
		if part == "" || strings.ContainsAny(part, "*\x00\r\n") {
			return nil, errors.New("invalid JSON key path")
		}
		bracket := strings.IndexByte(part, '[')
		if bracket < 0 {
			if strings.ContainsRune(part, ']') {
				return nil, errors.New("invalid JSON array path")
			}
			steps = append(steps, jsonStep{key: part})
			continue
		}
		if bracket == 0 || !strings.HasSuffix(part, "]") || strings.ContainsAny(part[bracket+1:len(part)-1], "[]") {
			return nil, errors.New("invalid JSON array path")
		}
		index, err := strconv.Atoi(part[bracket+1 : len(part)-1])
		if err != nil || index < 0 || index > 1023 || strconv.Itoa(index) != part[bracket+1:len(part)-1] {
			return nil, errors.New("JSON array index must be 0..1023")
		}
		steps = append(steps, jsonStep{key: part[:bracket]}, jsonStep{array: true, index: index})
	}
	if len(steps) > 64 {
		return nil, errors.New("JSON path exceeds depth limit")
	}
	return steps, nil
}
func compileJSONRules(raw []map[string]json.RawMessage) ([]jsonRule, error) {
	var rules []jsonRule
	for _, item := range raw {
		for key := range item {
			if key != "match" && key != "if_value" && key != "replace_with" && key != "value" {
				return nil, errors.New("unknown JSON replacement field")
			}
		}
		match, err := scalar(item["match"])
		if err != nil || len(match) > 8192 || match == "<nil>" {
			return nil, errors.New("invalid JSON match")
		}
		path, err := jsonPath(match)
		if err != nil {
			return nil, err
		}
		value, ok := item["replace_with"]
		if !ok {
			value = item["value"]
		}
		text, err := scalar(value)
		if err != nil || len(text) > 8192 || strings.ContainsRune(text, 0) || strings.Contains(text, "{{") {
			return nil, errors.New("invalid or unresolved JSON replacement")
		}
		// Match upstream scalar coercion: booleans retain bool type; integral
		// spellings become host-width integers, other scalar spellings are strings.
		var replacement any = text
		if string(value) == "true" || string(value) == "false" {
			replacement = string(value) == "true"
		} else if number, e := strconv.ParseInt(text, 10, 64); e == nil {
			replacement = json.Number(strconv.FormatInt(number, 10))
		}
		rule := jsonRule{path: path, value: replacement}
		if condition, ok := item["if_value"]; ok && string(condition) != "null" {
			rule.condition, err = scalar(condition)
			if err != nil || len(rule.condition) > 8192 {
				return nil, errors.New("invalid JSON condition")
			}
			if strings.HasPrefix(rule.condition, "regex:") {
				return nil, errors.New("regex JSON conditions are unsupported")
			}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// A token walk rejects duplicate keys and bounds depth/node count before making
// a mutable document. UseNumber preserves untouched large integers exactly.
func readJSON(ctx context.Context, d *json.Decoder, depth int, nodes *int) (any, error) {
	*nodes++
	if depth > 64 || *nodes > 100000 {
		return nil, errors.New("JSON structure exceeds bounds")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	token, err := d.Token()
	if err != nil {
		return nil, errors.New("invalid JSON input")
	}
	switch token {
	case json.Delim('{'):
		object := map[string]any{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, errors.New("invalid JSON key")
			}
			name, ok := key.(string)
			if !ok {
				return nil, errors.New("invalid JSON key")
			}
			if _, exists := object[name]; exists {
				return nil, errors.New("duplicate JSON key")
			}
			value, err := readJSON(ctx, d, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		if end, err := d.Token(); err != nil || end != json.Delim('}') {
			return nil, errors.New("invalid JSON object")
		}
		return object, nil
	case json.Delim('['):
		array := []any{}
		for d.More() {
			value, err := readJSON(ctx, d, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if end, err := d.Token(); err != nil || end != json.Delim(']') {
			return nil, errors.New("invalid JSON array")
		}
		return array, nil
	default:
		switch token.(type) {
		case string, json.Number, bool, nil:
			return token, nil
		default:
			return nil, errors.New("invalid JSON value")
		}
	}
}
func atJSON(node any, path []jsonStep) (any, bool, error) {
	for _, step := range path {
		if step.array {
			array, ok := node.([]any)
			if !ok {
				return nil, false, errors.New("JSON path requires an array")
			}
			if step.index >= len(array) {
				return nil, false, nil
			}
			node = array[step.index]
		} else {
			object, ok := node.(map[string]any)
			if !ok {
				return nil, false, errors.New("JSON path requires an object")
			}
			value, exists := object[step.key]
			if !exists {
				return nil, false, nil
			}
			node = value
		}
	}
	return node, true, nil
}
func putJSON(node any, path []jsonStep, value any) (any, error) {
	if len(path) == 0 {
		return value, nil
	}
	step := path[0]
	if step.array {
		var array []any
		if node == nil && step.index == 0 {
			array = make([]any, 1)
		} else {
			var ok bool
			array, ok = node.([]any)
			if !ok {
				return nil, errors.New("JSON path requires an array")
			}
		}
		if step.index >= len(array) {
			return nil, errors.New("JSON array index missing")
		}
		next, err := putJSON(array[step.index], path[1:], value)
		if err != nil {
			return nil, err
		}
		array[step.index] = next
		return array, nil
	}
	var object map[string]any
	if node == nil {
		object = map[string]any{}
	} else {
		var ok bool
		object, ok = node.(map[string]any)
		if !ok {
			return nil, errors.New("JSON path requires an object")
		}
	}
	next, err := putJSON(object[step.key], path[1:], value)
	if err != nil {
		return nil, err
	}
	object[step.key] = next
	return object, nil
}
func jsonText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	raw, _ := json.Marshal(value)
	return string(raw)
}
func applyJSONRule(ctx context.Context, root any, path []jsonStep, rule jsonRule, budget *int) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i, step := range path {
		if step.wild {
			parent, exists, err := atJSON(root, path[:i])
			if err != nil || !exists {
				return root, err
			}
			var children []jsonStep
			switch parent := parent.(type) {
			case map[string]any:
				if len(parent) > 4096 {
					return nil, errors.New("JSON wildcard exceeds 4096 children")
				}
				keys := make([]string, 0, len(parent))
				for key := range parent {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					children = append(children, jsonStep{key: key})
				}
			case []any:
				if len(parent) > 4096 {
					return nil, errors.New("JSON wildcard exceeds 4096 children")
				}
				for index := range parent {
					children = append(children, jsonStep{array: true, index: index})
				}
			default:
				return nil, errors.New("JSON wildcard requires a container")
			}
			if len(children) > 4096 {
				return nil, errors.New("JSON wildcard exceeds 4096 children")
			}
			for _, child := range children {
				concrete := append([]jsonStep(nil), path[:i]...)
				concrete = append(concrete, child)
				concrete = append(concrete, path[i+1:]...)
				root, err = applyJSONRule(ctx, root, concrete, rule, budget)
				if err != nil {
					return nil, err
				}
			}
			return root, nil
		}
	}
	current, exists, err := atJSON(root, path)
	if err != nil {
		return nil, err
	}
	value := rule.value
	if rule.condition != "" && exists && jsonText(current) != rule.condition {
		return root, nil
	}
	incoming, _ := json.Marshal(value)
	growth := len(incoming)
	if exists {
		old, _ := json.Marshal(current)
		growth -= len(old)
	} else {
		for _, step := range path {
			key, _ := json.Marshal(step.key)
			growth += len(key) + 4
		}
	}
	if *budget+growth > MaxFileBytes {
		return nil, errors.New("JSON mutation exceeds size budget")
	}
	*budget += growth

	return putJSON(root, path, value)
}
func renderJSON(ctx context.Context, input []byte, rules []jsonRule) ([]byte, error) {
	if len(input) > MaxFileBytes || !utf8.Valid(input) {
		return nil, errors.New("JSON file exceeds bounds or encoding")
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	nodes := 0
	root, err := readJSON(ctx, decoder, 0, &nodes)
	if err != nil {
		return nil, err
	}
	if _, ok := root.(map[string]any); !ok {
		return nil, errors.New("JSON root must be an object")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("trailing JSON data")
	}
	compact, _ := json.Marshal(root)
	budget := len(compact)
	if budget > MaxFileBytes {
		return nil, errors.New("JSON encoding exceeds bounds")
	}
	for _, rule := range rules {
		root, err = applyJSONRule(ctx, root, rule.path, rule, &budget)
		if err != nil {
			return nil, err
		}
	}
	return boundedJSON(ctx, root)
}

func boundedJSON(ctx context.Context, root any) ([]byte, error) {
	var output bytes.Buffer
	emit := func(value string) error {
		if output.Len()+len(value) > MaxFileBytes {
			return errors.New("JSON output exceeds bounds")
		}
		output.WriteString(value)
		return nil
	}
	nodes := 0
	var write func(any, int) error
	write = func(value any, depth int) error {
		nodes++
		if depth > 64 || nodes > 100000 {
			return errors.New("JSON output structure exceeds bounds")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		indent := func(level int) error { return emit(strings.Repeat("    ", level)) }
		switch value := value.(type) {
		case map[string]any:
			if len(value) == 0 {
				return emit("{}")
			}
			if err := emit("{\n"); err != nil {
				return err
			}
			keys := make([]string, 0, len(value))
			for key := range value {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for i, key := range keys {
				if err := indent(depth + 1); err != nil {
					return err
				}
				encoded, _ := json.Marshal(key)
				if err := emit(string(encoded) + ": "); err != nil {
					return err
				}
				if err := write(value[key], depth+1); err != nil {
					return err
				}
				ending := "\n"
				if i < len(keys)-1 {
					ending = ",\n"
				}
				if err := emit(ending); err != nil {
					return err
				}
			}
			if err := indent(depth); err != nil {
				return err
			}
			return emit("}")
		case []any:
			if len(value) == 0 {
				return emit("[]")
			}
			if err := emit("[\n"); err != nil {
				return err
			}
			for i, item := range value {
				if err := indent(depth + 1); err != nil {
					return err
				}
				if err := write(item, depth+1); err != nil {
					return err
				}
				ending := "\n"
				if i < len(value)-1 {
					ending = ",\n"
				}
				if err := emit(ending); err != nil {
					return err
				}
			}
			if err := indent(depth); err != nil {
				return err
			}
			return emit("]")
		default:
			raw, err := json.Marshal(value)
			if err != nil {
				return errors.New("invalid JSON output")
			}
			return emit(string(raw))
		}
	}
	if err := write(root, 0); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
