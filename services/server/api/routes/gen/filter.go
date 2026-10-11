// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

// filterSpec returns a copy of the spec in which paths keeps only the methods of the operations
// keep accepts (the enabled ones), path items left without a method are dropped, webhooks is
// removed and everything else (components above all) stays as it is (plan decision P-04). The copy is made from the
// node tree, so key order and comments stay those of the original; only the YAML layout
// (indentation, line breaks of long scalars) may differ, which oapi-codegen does not see.
//
// The pre-filtered copy covers every enable list with one mechanism, including the empty one:
// oapi-codegen's include-operation-ids applies only to a non-empty list and would generate every
// operation for an empty one (docs/DECISIONS.md, M1-17).
func filterSpec(data []byte, keep func(operationID string) bool) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return nil, fmt.Errorf("filter: not a YAML document")
	}
	root := doc.Content[0]
	removeKey(root, "webhooks")
	if paths := value(root, "paths"); paths != nil && paths.Kind == yaml.MappingNode {
		filterPaths(paths, keep)
	}
	out, err := encodeYAML(&doc)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}
	return out, nil
}

// filterPaths keeps in the paths mapping only the methods of the operations keep accepts and
// drops every path item left without a method; an empty mapping is written as {}.
func filterPaths(paths *yaml.Node, keep func(operationID string) bool) {
	var kept []*yaml.Node
	for i := 0; i+1 < len(paths.Content); i += 2 {
		item := deref(paths.Content[i+1])
		removeRefusedMethods(item, keep)
		if hasMethod(item) {
			kept = append(kept, paths.Content[i], paths.Content[i+1])
		}
	}
	paths.Content = kept
	if len(kept) == 0 {
		paths.Style = yaml.FlowStyle
	}
}

// removeRefusedMethods deletes from the path item every method whose operation keep refuses.
func removeRefusedMethods(item *yaml.Node, keep func(operationID string) bool) {
	for _, m := range methods {
		key := strings.ToLower(m)
		if op := value(item, key); op != nil {
			if id := scalar(value(op, "operationId")); !keep(id) {
				removeKey(item, key)
			}
		}
	}
}

// encodeYAML writes the document with an indentation of two spaces.
func encodeYAML(doc *yaml.Node) ([]byte, error) {
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// removeKey deletes key and its value from the mapping m.
func removeKey(m *yaml.Node, key string) {
	m = deref(m)
	if m == nil || m.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = slices.Delete(m.Content, i, i+2)
			return
		}
	}
}

// hasMethod reports whether the path item still has an operation.
func hasMethod(item *yaml.Node) bool {
	for _, k := range keys(item) {
		if slices.Contains(methods, strings.ToUpper(k)) {
			return true
		}
	}
	return false
}

// mergeKeys returns the places of YAML merge keys (<<) below n, as dotted paths. The filter edits
// the node tree, in which a merged operation or method is invisible, so the generator refuses
// merge keys in the spec instead of filtering past them. An alias is a leaf here: its anchor is
// walked where it is defined, and following aliases would never end on a self-referencing one.
func mergeKeys(n *yaml.Node, at string) []string {
	var out []string
	switch {
	case n == nil:
	case n.Kind == yaml.DocumentNode:
		for _, c := range n.Content {
			out = append(out, mergeKeys(c, at)...)
		}
	case n.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Tag == "!!merge" || key.Value == "<<" {
				out = append(out, at)
			}
			out = append(out, mergeKeys(n.Content[i+1], at+"."+key.Value)...)
		}
	case n.Kind == yaml.SequenceNode:
		for i, c := range n.Content {
			out = append(out, mergeKeys(c, fmt.Sprintf("%s[%d]", at, i))...)
		}
	}
	return out
}

// checkFiltered loads the copy for oapi-codegen with the same loader and requires its operations
// to be exactly the enabled operations want (operation ID, method and path). Whatever the
// filter could miss (a method key it does not read, a path item it cannot see into) would else
// reach the generated server without an entry in the route table.
func checkFiltered(filtered []byte, want []opMeta) error {
	doc, err := openapi3.NewLoader().LoadFromData(filtered)
	if err != nil {
		return fmt.Errorf("filtered copy: cannot be loaded: %w", err)
	}
	expected := map[string]bool{}
	for _, op := range want {
		expected[op.Method+" "+strings.TrimPrefix(op.Path, baseURL)+" "+op.ID] = true
	}
	got := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			got[method+" "+path+" "+op.OperationID] = true
		}
	}
	var problems []string
	for _, k := range slices.Sorted(maps.Keys(got)) {
		if !expected[k] {
			problems = append(problems, "filtered copy holds an operation that is not enabled: "+k)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(expected)) {
		if !got[k] {
			problems = append(problems, "filtered copy lacks the enabled operation "+k)
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}
