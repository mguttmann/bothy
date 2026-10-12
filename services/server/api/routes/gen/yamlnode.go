// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// readYAML reads the file at path and parses it into a node tree; it returns the tree and the
// bytes it read. Checks work on the tree, not on a decoded struct, so they can tell a missing
// key from an empty one.
func readYAML(path string) (*yaml.Node, []byte, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("not valid YAML: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("not a YAML mapping")
	}
	return &doc, data, nil
}

// deref follows YAML aliases, so an anchor reused elsewhere reads like the original node.
func deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// value returns the value node of key in the mapping m, or nil when m is no mapping or lacks
// the key.
func value(m *yaml.Node, key string) *yaml.Node {
	m = deref(m)
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return deref(m.Content[i+1])
		}
	}
	return nil
}

// keys returns the keys of the mapping m in document order; nil for anything that is not a
// mapping.
func keys(m *yaml.Node) []string {
	m = deref(m)
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(m.Content); i += 2 {
		out = append(out, m.Content[i].Value)
	}
	return out
}

// scalar returns the text of a scalar node; "" for anything else.
func scalar(n *yaml.Node) string {
	n = deref(n)
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// items returns the elements of a sequence node; nil for anything else.
func items(n *yaml.Node) []*yaml.Node {
	n = deref(n)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, len(n.Content))
	for i, c := range n.Content {
		out[i] = deref(c)
	}
	return out
}
