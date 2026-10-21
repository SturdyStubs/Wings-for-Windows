package eggconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

var xmlNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,255}$`)
var xmlAttributePattern = regexp.MustCompile(`^\[([A-Za-z_][A-Za-z0-9_-]{0,255})='([^\r\n]*)'\]$`)

type xmlRule struct {
	path             []string
	value, attribute string
}
type xmlElement struct {
	name       string
	attributes []xml.Attr
	children   []*xmlElement
	text       []byte
}

func xmlCharacters(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if r != 9 && r != 10 && r != 13 && !(r >= 0x20 && r <= 0xd7ff) && !(r >= 0xe000 && r <= 0xfffd) && !(r >= 0x10000 && r <= 0x10ffff) {
			return false
		}
	}
	return true
}
func compileXMLRules(items []map[string]json.RawMessage) ([]xmlRule, error) {
	var rules []xmlRule
	for _, item := range items {
		for field := range item {
			if field != "match" && field != "if_value" && field != "replace_with" && field != "value" {
				return nil, errors.New("unknown XML replacement field")
			}
		}
		match, err := scalar(item["match"])
		if err != nil || len(match) > 8192 {
			return nil, errors.New("invalid XML match")
		}
		path := strings.Split(match, ".")
		if len(path) > 32 {
			return nil, errors.New("XML path exceeds bounds")
		}
		wild := 0
		for i, part := range path {
			if part == "*" {
				wild++
				if i == 0 || wild > 1 {
					return nil, errors.New("unsupported XML wildcard")
				}
			} else if !xmlNamePattern.MatchString(part) {
				return nil, errors.New("unsupported XML path name")
			}
		}
		if raw, ok := item["if_value"]; ok && string(raw) != "null" {
			condition, err := scalar(raw)
			if err != nil || condition != "" {
				return nil, errors.New("conditional XML rules are unsupported")
			}
		}
		raw, ok := item["replace_with"]
		if !ok {
			raw = item["value"]
		}
		value, err := scalar(raw)
		if err != nil || len(value) > 8192 || !xmlCharacters(value) || strings.Contains(value, "{{") {
			return nil, errors.New("invalid or unresolved XML value")
		}
		rule := xmlRule{path: path, value: value}
		if attr := xmlAttributePattern.FindStringSubmatch(value); attr != nil {
			rule.attribute = attr[1]
			rule.value = attr[2]
			if rule.attribute == "xmlns" {
				return nil, errors.New("XML namespaces are unsupported")
			}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}
func readXML(ctx context.Context, input []byte) (*xmlElement, error) {
	if len(input) > MaxFileBytes || !utf8.Valid(input) || bytes.HasPrefix(input, []byte{239, 187, 191}) {
		return nil, errors.New("XML input exceeds bounds or encoding")
	}
	decoder := xml.NewDecoder(bytes.NewReader(input))
	var root *xmlElement
	var stack []*xmlElement
	count := 0
	declaration := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("invalid XML input")
		}
		switch token := token.(type) {
		case xml.StartElement:
			count += 1 + len(token.Attr)
			if len(stack) >= 64 || count > 100000 || len(token.Attr) > 128 {
				return nil, errors.New("XML structure exceeds bounds")
			}
			if token.Name.Space != "" || !xmlNamePattern.MatchString(token.Name.Local) {
				return nil, errors.New("unsupported XML element name or namespace")
			}
			seen := map[string]bool{}
			for _, attr := range token.Attr {
				if attr.Name.Space != "" || attr.Name.Local == "xmlns" || !xmlNamePattern.MatchString(attr.Name.Local) || seen[attr.Name.Local] || len(attr.Value) > 64<<10 {
					return nil, errors.New("unsupported XML attribute")
				}
				seen[attr.Name.Local] = true
			}
			node := &xmlElement{name: token.Name.Local, attributes: append([]xml.Attr(nil), token.Attr...)}
			if len(stack) == 0 {
				if root != nil {
					return nil, errors.New("multiple XML roots")
				}
				root = node
			} else {
				parent := stack[len(stack)-1]
				if len(bytes.TrimSpace(parent.text)) != 0 {
					return nil, errors.New("mixed XML content is unsupported")
				}
				parent.text = nil
				parent.children = append(parent.children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, errors.New("invalid XML end")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if len(bytes.TrimSpace(token)) != 0 {
					return nil, errors.New("text outside XML root")
				}
				continue
			}
			node := stack[len(stack)-1]
			if len(node.children) > 0 {
				if len(bytes.TrimSpace(token)) != 0 {
					return nil, errors.New("mixed XML content is unsupported")
				}
			} else {
				node.text = append(node.text, token...)
			}
		case xml.ProcInst:
			if token.Target != "xml" || root != nil || declaration {
				return nil, errors.New("XML processing instructions are unsupported")
			}
			declaration = true
		case xml.Directive:
			return nil, errors.New("XML directives and DTDs are unsupported")
		case xml.Comment: // Comments are intentionally normalized away.
		default:
			return nil, errors.New("unsupported XML token")
		}
	}
	if len(stack) != 0 {
		return nil, errors.New("incomplete XML document")
	}
	return root, nil
}
func xmlTargets(ctx context.Context, root *xmlElement, path []string) ([]*xmlElement, error) {
	if root.name != path[0] {
		return nil, errors.New("XML root does not match rule")
	}
	current := []*xmlElement{root}
	for _, part := range path[1:] {
		var next []*xmlElement
		for _, parent := range current {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			for _, child := range parent.children {
				if part == "*" || child.name == part {
					next = append(next, child)
					if len(next) > 4096 {
						return nil, errors.New("XML match count exceeds bounds")
					}
				}
			}
		}
		current = next
	}
	return current, nil
}

type xmlBuffer struct {
	ctx context.Context
	bytes.Buffer
	err error
}

func (b *xmlBuffer) Write(data []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	b.err = b.ctx.Err()
	if b.err != nil {
		return 0, b.err
	}
	if b.Len()+len(data) > MaxFileBytes {
		b.err = errors.New("XML output exceeds 1 MiB")
		return 0, b.err
	}
	return b.Buffer.Write(data)
}
func writeXML(ctx context.Context, encoder *xml.Encoder, node *xmlElement, depth int, count *int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	*count += 1 + len(node.attributes)
	if depth > 64 || *count > 100000 || len(node.attributes) > 128 {
		return errors.New("XML result structure exceeds bounds")
	}
	start := xml.StartElement{Name: xml.Name{Local: node.name}, Attr: node.attributes}
	if err := encoder.EncodeToken(start); err != nil {
		return errors.New("XML encoding failed")
	}
	if len(node.children) > 0 {
		for _, child := range node.children {
			if err := writeXML(ctx, encoder, child, depth+1, count); err != nil {
				return err
			}
		}
	} else if err := encoder.EncodeToken(xml.CharData(node.text)); err != nil {
		return errors.New("XML text encoding failed")
	}
	if err := encoder.EncodeToken(start.End()); err != nil {
		return errors.New("XML end encoding failed")
	}
	return nil
}
func sameXML(a, b *xmlElement) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.name != b.name || string(a.text) != string(b.text) || len(a.attributes) != len(b.attributes) || len(a.children) != len(b.children) {
		return false
	}
	for i := range a.attributes {
		if a.attributes[i] != b.attributes[i] {
			return false
		}
	}
	for i := range a.children {
		if !sameXML(a.children[i], b.children[i]) {
			return false
		}
	}
	return true
}
func renderXML(ctx context.Context, input []byte, rules []xmlRule) ([]byte, error) {
	root, err := readXML(ctx, input)
	if err != nil {
		return nil, err
	}
	budget := len(input)
	reserve := func(growth int) error {
		if budget+growth > MaxFileBytes {
			return errors.New("XML mutation exceeds size budget")
		}
		budget += growth
		return nil
	}
	for _, rule := range rules {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if root == nil {
			if err := reserve(2*len(rule.path[0]) + 5); err != nil {
				return nil, err
			}
			root = &xmlElement{name: rule.path[0]}
		}
		if root.name != rule.path[0] {
			return nil, errors.New("XML root does not match rule")
		}
		wildcard := false
		for _, part := range rule.path {
			if part == "*" {
				wildcard = true
			}
		}
		if !wildcard {
			node := root
			for _, part := range rule.path[1:] {
				var found *xmlElement
				for _, child := range node.children {
					if child.name == part {
						found = child
						break
					}
				}
				if found == nil {
					if len(bytes.TrimSpace(node.text)) != 0 {
						return nil, errors.New("XML path would replace existing text")
					}
					node.text = nil
					if err := reserve(2*len(part) + 5); err != nil {
						return nil, err
					}
					found = &xmlElement{name: part}
					node.children = append(node.children, found)
				}
				node = found
			}
		}
		targets, err := xmlTargets(ctx, root, rule.path)
		if err != nil {
			return nil, err
		}
		for _, target := range targets {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if rule.attribute != "" {
				updated := false
				for i := range target.attributes {
					if target.attributes[i].Name.Local == rule.attribute {
						if err := reserve(len(rule.value) - len(target.attributes[i].Value)); err != nil {
							return nil, err
						}
						target.attributes[i].Value = rule.value
						updated = true
						break
					}
				}
				if !updated {
					if err := reserve(len(rule.attribute) + len(rule.value) + 4); err != nil {
						return nil, err
					}
					target.attributes = append(target.attributes, xml.Attr{Name: xml.Name{Local: rule.attribute}, Value: rule.value})
				}
			} else {
				if len(target.children) > 0 {
					return nil, errors.New("XML text replacement requires a leaf")
				}
				if err := reserve(len(rule.value) - len(target.text)); err != nil {
					return nil, err
				}
				target.text = []byte(rule.value)
			}
		}
	}
	if root == nil {
		return nil, errors.New("XML needs a root or creation rule")
	}
	output := &xmlBuffer{ctx: ctx}
	encoder := xml.NewEncoder(output)
	encoder.Indent("", "  ")
	count := 0
	err = writeXML(ctx, encoder, root, 1, &count)
	if err == nil {
		err = encoder.Flush()
	}
	if output.err != nil {
		return nil, output.err
	}
	if err != nil {
		return nil, err
	}
	check, err := readXML(ctx, output.Bytes())
	if err != nil {
		return nil, err
	}
	if !sameXML(root, check) {
		return nil, errors.New("XML output cannot preserve data")
	}
	return output.Bytes(), ctx.Err()
}
