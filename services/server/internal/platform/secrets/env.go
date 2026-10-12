// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"strings"
)

// envDriver reads "env:" references from a snapshot of the process environment.
type envDriver struct {
	vars map[string]string
}

// NewEnvDriver returns the driver for "env:" references over environ, a list of "NAME=value"
// entries as os.Environ returns them.
//
// The driver keeps its own copy of the variables: only cmd/bothy reads the process environment
// (02 "Configuration loading" item 5), and a process environment does not change from outside, so
// the snapshot gives the values os.LookupEnv would. When a name occurs twice the first entry
// wins, as with os.Getenv; entries without "=" are ignored. An unset or empty variable is
// ErrNotFound: an empty password or key is never valid. Values are used as they are, without the
// newline rule of the file driver. env: references are allowed but not recommended, because child
// processes and crash reports inherit the environment (16-operations.md 5.2).
func NewEnvDriver(environ []string) Driver {
	vars := make(map[string]string, len(environ))
	for _, entry := range environ {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, seen := vars[name]; !seen {
			vars[name] = value
		}
	}
	return &envDriver{vars: vars}
}

// Scheme returns "env".
func (*envDriver) Scheme() string { return schemeEnv }

// Get returns the variable ref names; see NewEnvDriver for the rules.
func (d *envDriver) Get(ctx context.Context, ref Ref) (Value, Version, error) {
	if err := checkDriverRef(ctx, schemeEnv, ref); err != nil {
		return Value{}, "", err
	}
	value, ok := d.vars[ref.Path]
	switch {
	case !ok:
		return Value{}, "", &RefError{Ref: ref.String(), Err: ErrNotFound, Detail: "variable is not set"}
	case value == "":
		return Value{}, "", &RefError{Ref: ref.String(), Err: ErrNotFound, Detail: "variable is empty"}
	}
	content := []byte(value)
	return NewValue(content), versionOf(content), nil
}
