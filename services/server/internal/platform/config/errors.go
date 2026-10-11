// SPDX-License-Identifier: Apache-2.0

package config

import "strings"

// LoadError is the error of Load. It holds every problem of a configuration, sorted by key, so
// that an operator fixes all of them in one round instead of one per start.
type LoadError struct {
	// Problems lists the problems sorted by Key; problems without key (unknown variables and
	// file keys, unreadable files) come first.
	Problems []Problem
}

// Error returns one line per problem in the form of Problem.String.
func (e *LoadError) Error() string {
	lines := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		lines = append(lines, p.String())
	}
	return strings.Join(lines, "\n")
}

// Problem is one finding in a configuration. No field ever contains a value given for a key:
// values can be secrets placed in the wrong key (02 "Konfigurationsladen" item 4).
type Problem struct {
	// Key is the key the problem is about, the keys of a cross-check joined by ", ", or empty
	// for a problem that concerns no key (an unknown variable names itself in Message).
	Key string
	// Source says where the offending text came from: "default", "file <path>",
	// "env BOTHY_X", "flag --role", "environment" (a variable as a whole), "command line" (a key
	// no flag sets), "none" (a required key without value), or for a cross-check
	// "<key> from <source>" for each key.
	Source string
	// Message says what is wrong, in English, without the value.
	Message string
}

// String renders the problem as "config: <key>: <message> (source: <source>)", or without the
// key part when Key is empty, for example "config: unknown variable BOTHY_X (source:
// environment)".
func (p Problem) String() string {
	var b strings.Builder
	b.WriteString("config: ")
	if p.Key != "" {
		b.WriteString(p.Key)
		b.WriteString(": ")
	}
	b.WriteString(p.Message)
	b.WriteString(" (source: ")
	b.WriteString(p.Source)
	b.WriteString(")")
	return b.String()
}
