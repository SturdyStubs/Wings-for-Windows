package eggconfig

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func xmlPlan(t *testing.T, rules string) File {
	t.Helper()
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.xml","parser":"xml","replace":` + rules + `}`)}, []string{"server.xml"})
	if err != nil {
		t.Fatal(err)
	}
	return plans[0]
}
func TestXMLPathsAttributesWildcardsAndEscapes(t *testing.T) {
	plan := xmlPlan(t, `[{"match":"server.port","replace_with":28015},{"match":"server.name","value":"Café 😀 & <safe>"},{"match":"server.worlds.world.*","replace_with":"new"},{"match":"server.worlds.world","replace_with":"[enabled='true']"},{"match":"server.new.leaf","replace_with":null}]`)
	input := []byte(`<?xml version="1.0" encoding="UTF-8"?><server mode="keep"><!-- remove --><port>1</port><name>old</name><worlds><world id="1"><mode>old</mode></world><world id="2"><mode>stay</mode></world></worlds></server>`)
	output, err := Render(context.Background(), input, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "<name>Café 😀 &amp; &lt;safe&gt;</name>") || strings.Count(string(output), `enabled="true"`) != 2 || strings.Count(string(output), "<mode>new</mode>") != 2 || !strings.Contains(string(output), "<leaf>&lt;nil&gt;</leaf>") || strings.Contains(string(output), "remove") {
		t.Fatal(string(output))
	}
	again, err := Render(context.Background(), output, plan)
	if err != nil || string(again) != string(output) {
		t.Fatal("unstable XML", err)
	}
}
func TestXMLRootCreationAndLiteralValues(t *testing.T) {
	t.Setenv("XML_SECRET", "never-copy-this")
	plan := xmlPlan(t, `[{"match":"server.literal","replace_with":"${XML_SECRET}"},{"match":"server.enabled","replace_with":true},{"match":"server.literal","replace_with":"[label='Café & <safe>']"}]`)
	output, err := Render(context.Background(), nil, plan)
	if err != nil {
		t.Fatal(err)
	}
	root, err := readXML(context.Background(), output)
	if err != nil || root.name != "server" || string(root.children[0].text) != "${XML_SECRET}" || root.children[0].attributes[0].Value != "Café & <safe>" || string(root.children[1].text) != "true" || strings.Contains(string(output), "never-copy-this") {
		t.Fatal(string(output), err)
	}
}
func TestXMLRejectsUnsafeInput(t *testing.T) {
	cases := map[string][]byte{
		"malformed": []byte("<server>"), "multi-root": []byte("<a/><b/>"), "mixed": []byte("<a>before<b/>after</a>"), "duplicate-attr": []byte(`<a x="1" x="2"/>`), "namespace": []byte(`<a xmlns="urn:test"/>`), "prefix": []byte(`<x:a/>`), "dtd": []byte(`<!DOCTYPE a SYSTEM "file:///C:/secret"><a/>`), "entity": []byte(`<a>&external;</a>`), "pi": []byte(`<?include file?><a/>`), "bad-name": []byte(`<a.b/>`), "nul": []byte("<a>\x00</a>"), "bad-utf8": {255}, "bom": {239, 187, 191}, "depth": []byte(strings.Repeat("<a>", 65) + strings.Repeat("</a>", 65)), "nodes": []byte("<a>" + strings.Repeat("<b/>", 100000) + "</a>"), "attributes": []byte("<a " + strings.Repeat(`x="1" `, 129) + "/>"), "size": []byte(strings.Repeat("x", MaxFileBytes+1)),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Render(context.Background(), input, xmlPlan(t, `[]`)); err == nil {
				t.Fatal("unsafe XML accepted")
			}
		})
	}
}
func TestXMLRejectsUnsupportedRules(t *testing.T) {
	cases := map[string]string{"xpath": `[{"match":"a.b[1]","replace_with":"x"}]`, "root-wildcard": `[{"match":"*.a","replace_with":"x"}]`, "multi-wildcard": `[{"match":"a.*.*","replace_with":"x"}]`, "condition": `[{"match":"a","if_value":"old","replace_with":"x"}]`, "control": `[{"match":"a","replace_with":"\u0001"}]`, "template": `[{"match":"a","replace_with":"{{config.x}}"}]`, "namespace": `[{"match":"a","replace_with":"[xmlns='urn:test']"}]`}
	for name, rules := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.xml","parser":"xml","replace":` + rules + `}`)}, []string{"server.xml"}); err == nil {
				t.Fatal("unsupported rule accepted")
			}
		})
	}
}
func TestXMLMutationOutputAndShapeFailures(t *testing.T) {
	for name, input := range map[string]string{"root": "<other/>", "nonleaf": "<a><b/></a>"} {
		t.Run(name, func(t *testing.T) {
			if _, err := Render(context.Background(), []byte(input), xmlPlan(t, `[{"match":"a","replace_with":"text"}]`)); err == nil {
				t.Fatal("shape mismatch accepted")
			}
		})
	}
	input := []byte("<a>" + strings.Repeat("<b/>", 4097) + "</a>")
	if _, err := Render(context.Background(), input, xmlPlan(t, `[{"match":"a.*","replace_with":"x"}]`)); err == nil {
		t.Fatal("match limit missing")
	}
	input = []byte("<a>" + strings.Repeat("<b/>", 200) + "</a>")
	if _, err := Render(context.Background(), input, File{Parser: "xml", XMLRules: []xmlRule{{path: []string{"a", "b"}, value: strings.Repeat("v", 8192)}}}); err == nil || !strings.Contains(err.Error(), "mutation") {
		t.Fatal("mutation budget missing", err)
	}
	input = []byte("<a>" + strings.Repeat("<b/>", 120) + "</a>")
	if _, err := Render(context.Background(), input, File{Parser: "xml", XMLRules: []xmlRule{{path: []string{"a", "b"}, value: strings.Repeat("&", 8192)}}}); err == nil || !strings.Contains(err.Error(), "output exceeds") {
		t.Fatal("escaped output cap missing", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Render(ctx, []byte("<a/>"), xmlPlan(t, `[]`)); err != context.Canceled {
		t.Fatal(err)
	}
}
