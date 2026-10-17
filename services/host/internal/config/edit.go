// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"fmt"
	"os"
	"slices"

	"go.yaml.in/yaml/v3"
)

// AddSessionUser appends name to host_service.session_users of the file at path. It edits the
// YAML node tree, so comments and every other key stay. The file must already be valid; the
// caller checks that name is not configured yet.
func AddSessionUser(path, name string) error {
	return edit(path, func(hs *yaml.Node) {
		seq := child(hs, "session_users")
		if seq == nil {
			seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}
			hs.Content = append(hs.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: tagStr, Value: "session_users"}, seq)
		}
		seq.Style = 0
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: tagStr, Value: name})
	})
}

// RemoveSessionUser removes name from host_service.session_users and its key from
// host_service.claude_workdirs of the file at path (24 section 2.2 point 3); nothing else
// changes.
func RemoveSessionUser(path, name string) error {
	return edit(path, func(hs *yaml.Node) {
		if seq := child(hs, "session_users"); seq != nil {
			seq.Content = slices.DeleteFunc(seq.Content, func(n *yaml.Node) bool { return n.Value == name })
		}
		if wd := child(hs, "claude_workdirs"); wd != nil && wd.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(wd.Content); i += 2 {
				if wd.Content[i].Value == name {
					wd.Content = slices.Delete(wd.Content, i, i+2)
					break
				}
			}
		}
	})
}

func child(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func edit(path string, fn func(hs *yaml.Node)) error {
	b, err := os.ReadFile(path) //nolint:gosec // the configuration file chosen by the administrator
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return invalid("host_service", "not a YAML document")
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: tagMap}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return invalid("host_service", "the file must be a mapping with the key host_service")
	}
	hs := child(root, "host_service")
	if hs == nil {
		hs = &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: tagStr, Value: "host_service"}, hs)
	}
	if hs.Kind != yaml.MappingNode {
		return invalid("host_service", "must be a mapping")
	}
	fn(hs)
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	return writeAtomic(path, out.Bytes())
}
