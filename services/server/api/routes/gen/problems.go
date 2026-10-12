// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"slices"
	"strings"
)

// problemsError collects every problem of a generator run, so that one run names all of them
// instead of stopping at the first (a broken spec or list is fixed in one round). It is the
// error loadInputs returns.
type problemsError struct {
	items []string
}

// add records one problem; where names the file and the place in it, for example
// "operations.yaml: later: updateThing".
func (p *problemsError) add(where, format string, args ...any) {
	p.items = append(p.items, where+": "+fmt.Sprintf(format, args...))
}

// Error returns the problems sorted and one per line; sorting keeps the output stable across
// map iteration orders.
func (p *problemsError) Error() string {
	items := slices.Clone(p.items)
	slices.Sort(items)
	return strings.Join(items, "\n")
}

// err returns p as an error when it holds a problem, nil otherwise.
func (p *problemsError) err() error {
	if len(p.items) == 0 {
		return nil
	}
	slices.Sort(p.items)
	return p
}
