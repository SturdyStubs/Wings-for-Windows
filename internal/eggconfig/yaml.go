package eggconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Decode a node tree, never Node.Decode/interface unmarshalling: aliases and
// merges must be refused before any expansion or key coercion can occur.
func yamlData(ctx context.Context, node *yaml.Node, depth int, count *int) (any, error) {
	*count++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if depth > 64 || *count > 100000 {
		return nil, errors.New("YAML structure exceeds bounds")
	}
	if node.Anchor != "" || node.Kind == yaml.AliasNode {
		return nil, errors.New("YAML anchors and aliases are unsupported")
	}
	switch node.Kind {
	case yaml.MappingNode:
		if node.Tag != "!!map" || len(node.Content)%2 != 0 {
			return nil, errors.New("unsupported YAML mapping")
		}
		object := map[string]any{}
		for i := 0; i < len(node.Content); i += 2 {
			raw, err := yamlData(ctx, node.Content[i], depth+1, count)
			if err != nil {
				return nil, err
			}
			key, ok := raw.(string)
			if !ok || len(key) > 8192 {
				return nil, errors.New("YAML mapping keys must be bounded strings")
			}
			if _, exists := object[key]; exists {
				return nil, errors.New("duplicate YAML key")
			}
			value, err := yamlData(ctx, node.Content[i+1], depth+1, count)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		return object, nil
	case yaml.SequenceNode:
		if node.Tag != "!!seq" {
			return nil, errors.New("unsupported YAML sequence")
		}
		array := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := yamlData(ctx, child, depth+1, count)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		return array, nil
	case yaml.ScalarNode:
		if strings.ContainsRune(node.Value, 0) {
			return nil, errors.New("YAML scalar contains NUL")
		}
		switch node.Tag {
		case "!!str":
			return node.Value, nil
		case "!!null":
			if node.Value == "" || node.Value == "~" || strings.EqualFold(node.Value, "null") {
				return nil, nil
			}
		case "!!bool":
			if strings.EqualFold(node.Value, "true") {
				return true, nil
			}
			if strings.EqualFold(node.Value, "false") {
				return false, nil
			}
		case "!!int", "!!float":
			if node.Tag == "!!int" && strings.ContainsAny(node.Value, ".eE") {
				return nil, errors.New("invalid YAML integer")
			}
			decoder := json.NewDecoder(strings.NewReader(node.Value))
			decoder.UseNumber()
			var value any
			if decoder.Decode(&value) == nil {
				if number, ok := value.(json.Number); ok {
					if decoder.Decode(new(any)) == io.EOF {
						return number, nil
					}
				}
			}
		}
	}
	return nil, errors.New("unsupported YAML tag, number or node")
}
func readYAML(ctx context.Context, input []byte) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(input) > MaxFileBytes || !utf8.Valid(input) || bytes.ContainsRune(input, 0) || bytes.HasPrefix(input, []byte{0xef, 0xbb, 0xbf}) {
		return nil, errors.New("YAML input exceeds bounds or encoding")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(input))
	var doc yaml.Node
	if decoder.Decode(&doc) != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return nil, errors.New("invalid YAML document")
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return nil, errors.New("multiple or trailing YAML documents are unsupported")
	}
	count := 0
	value, err := yamlData(ctx, doc.Content[0], 0, &count)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("YAML root must be a mapping")
	}
	return object, nil
}
func yamlOutputNode(ctx context.Context, value any, depth int, count *int) (*yaml.Node, error) {
	*count++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if depth > 64 || *count > 100000 {
		return nil, errors.New("YAML result structure exceeds bounds")
	}
	node := &yaml.Node{}
	switch value := value.(type) {
	case map[string]any:
		node.Kind = yaml.MappingNode
		node.Tag = "!!map"
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			name, err := yamlOutputNode(ctx, key, depth+1, count)
			if err != nil {
				return nil, err
			}
			child, err := yamlOutputNode(ctx, value[key], depth+1, count)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, name, child)
		}
	case []any:
		node.Kind = yaml.SequenceNode
		node.Tag = "!!seq"
		for _, value := range value {
			child, err := yamlOutputNode(ctx, value, depth+1, count)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, child)
		}
	case string:
		node.Kind = yaml.ScalarNode
		node.Tag = "!!str"
		node.Style = yaml.DoubleQuotedStyle
		node.Value = value
	case json.Number:
		node.Kind = yaml.ScalarNode
		node.Tag = "!!int"
		if strings.ContainsAny(string(value), ".eE") {
			node.Tag = "!!float"
		}
		node.Value = string(value)
	case bool:
		node.Kind = yaml.ScalarNode
		node.Tag = "!!bool"
		node.Value = "false"
		if value {
			node.Value = "true"
		}
	case nil:
		node.Kind = yaml.ScalarNode
		node.Tag = "!!null"
		node.Value = "null"
	default:
		return nil, errors.New("unsupported YAML result")
	}
	return node, nil
}

type yamlBuffer struct {
	ctx context.Context
	bytes.Buffer
	err error
}

func (b *yamlBuffer) Write(data []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if b.err = b.ctx.Err(); b.err != nil {
		return 0, b.err
	}
	if b.Len()+len(data) > MaxFileBytes {
		b.err = errors.New("YAML output exceeds 1 MiB")
		return 0, b.err
	}
	return b.Buffer.Write(data)
}
func renderYAML(ctx context.Context, input []byte, rules []jsonRule) ([]byte, error) {
	root, err := readYAML(ctx, input)
	if err != nil {
		return nil, err
	}
	compact, err := json.Marshal(root)
	if err != nil || len(compact) > MaxFileBytes {
		return nil, errors.New("YAML JSON representation exceeds bounds")
	}
	budget := len(compact)
	var updated any = root
	for _, rule := range rules {
		updated, err = applyJSONRule(ctx, updated, rule.path, rule, &budget)
		if err != nil {
			return nil, err
		}
	}
	count := 0
	node, err := yamlOutputNode(ctx, updated, 0, &count)
	if err != nil {
		return nil, err
	}
	output := &yamlBuffer{ctx: ctx}
	encoder := yaml.NewEncoder(output)
	encoder.SetIndent(2)
	err = encoder.Encode(node)
	if err == nil {
		err = encoder.Close()
	}
	if output.err != nil {
		return nil, output.err
	}
	if err != nil {
		return nil, errors.New("YAML encoding failed")
	}
	// Compare canonical raw data, never floating-point conversions. This also
	// guards against emitter spelling/tag changes before guarded writes begin.
	check, err := readYAML(ctx, output.Bytes())
	if err != nil {
		return nil, err
	}
	before, _ := json.Marshal(updated)
	after, _ := json.Marshal(check)
	if !bytes.Equal(before, after) {
		return nil, errors.New("YAML output cannot preserve parsed data")
	}
	return output.Bytes(), ctx.Err()
}
