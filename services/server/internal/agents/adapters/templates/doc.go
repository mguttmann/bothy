// SPDX-License-Identifier: Apache-2.0

// Package templates embeds the shipped agent templates (docs/spec/26-builtin-agents.md 7.2): the
// folder agents/ is a copy of templates/agents/ at the repository root that go generate writes
// with tools/templatecopy, because Go embeds only files below its module; make codegen-check holds
// the copy equal to the source. Edit the templates at the repository root, never here. The package
// stands below adapters/ because go-arch-lint attaches only the layers of a module to it.
//
// Example:
//
//	cat, err := templatefs.Load(templates.FS(), cfg.TemplatesDir, cfg.DefinitionLimits(), log)
package templates

import (
	"embed"
	"io/fs"
)

//go:generate go run ../../../../tools/templatecopy -src ../../../../../../templates/agents -out agents

// files is the generated copy; all: keeps a file whose name starts with a dot or an underscore,
// so the loader sees such a file and refuses the template instead of never seeing it.
//
//go:embed all:agents
var files embed.FS

// FS returns the shipped templates with one folder per template at its root.
func FS() fs.FS {
	sub, err := fs.Sub(files, "agents")
	if err != nil {
		// fs.Sub fails only for an invalid path, and "agents" is valid.
		panic(err)
	}
	return sub
}
