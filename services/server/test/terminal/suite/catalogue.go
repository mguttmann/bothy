// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"slices"

	"go.yaml.in/yaml/v3"
)

// Catalogue is catalogue.yaml of the suite (Design note 7): one entry per ID of 24 14 and per
// AZ-M, ID-T and SEC-T ID that threat model 6.7 names. tools/docs/terminal_catalogue.py holds it
// equal to the chapters in make docs; this loader is the same strict decoding for the suite.
type Catalogue struct {
	Format  int     `yaml:"format"`
	Entries []Entry `yaml:"entries"`
}

// Entry is one ID of the catalogue.
type Entry struct {
	ID            string         `yaml:"id"`
	Title         string         `yaml:"title"`
	Expected      string         `yaml:"expected"`
	Runs          []CatalogueRun `yaml:"runs"`
	ImplementedBy []Implemented  `yaml:"implemented_by"`
	Deferred      *Deferred      `yaml:"deferred"`
	DeferredParts []DeferredPart `yaml:"deferred_parts"`
	Threats       []string       `yaml:"threats"`
}

// CatalogueRun is one place an ID runs: kind e2e, platform, perf or unit, the reference, and the
// operating system of a platform run.
type CatalogueRun struct {
	Kind string `yaml:"kind"`
	Ref  string `yaml:"ref"`
	OS   string `yaml:"os"`
}

// Implemented names the ticket and the merged tests that recorded the red run of an ID.
type Implemented struct {
	Ticket string   `yaml:"ticket"`
	Tests  []string `yaml:"tests"`
}

// Deferred names the ticket of an ID outside exit criterion 8.
type Deferred struct {
	Ticket string `yaml:"ticket"`
	Reason string `yaml:"reason"`
}

// DeferredPart is a part of an ID that another ticket proves.
type DeferredPart struct {
	Part   string `yaml:"part"`
	Ticket string `yaml:"ticket"`
}

// The kinds of runs and the systems of platform runs.
var (
	runKinds  = []string{"e2e", "platform", "perf", "unit"}
	runOSes   = []string{"linux", "macos", "windows10", "windows11"}
	ticketRE  = regexp.MustCompile(`^(M[0-9]+|V[0-9]+)-[0-9]+$`)
	catalogID = regexp.MustCompile(`^(TM-T|AZ-M|ID-T|SEC-T)[0-9]+$`)
)

// LoadCatalogue decodes the catalogue at path strictly and checks its structure: unknown fields,
// duplicate IDs, a run with an unknown kind or os, a deferred entry on an ID of planIDs (the PLAN
// row M2-48 and exit criterion 8) and a deferred_parts entry without ticket are errors.
//
// Errors: the first decode error, or every structural problem joined.
func LoadCatalogue(path string, planIDs []string) (*Catalogue, error) {
	b, err := os.ReadFile(path) //nolint:gosec // G304: the catalogue of the suite
	if err != nil {
		return nil, err
	}
	return ParseCatalogue(b, planIDs)
}

// ParseCatalogue is LoadCatalogue on the bytes of a catalogue.
func ParseCatalogue(b []byte, planIDs []string) (*Catalogue, error) {
	var c Catalogue
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("catalogue.yaml: %w", err)
	}
	var probs []string
	add := func(format string, args ...any) { probs = append(probs, fmt.Sprintf(format, args...)) }
	if c.Format != 1 {
		add("format %d, want 1", c.Format)
	}
	seen := map[string]bool{}
	for _, e := range c.Entries {
		if !catalogID.MatchString(e.ID) {
			add("%q is no catalogue ID", e.ID)
		}
		if seen[e.ID] {
			add("%s twice", e.ID)
		}
		seen[e.ID] = true
		for _, r := range e.Runs {
			if !slices.Contains(runKinds, r.Kind) {
				add("%s: unknown run kind %q", e.ID, r.Kind)
			}
			if r.Kind == "platform" && !slices.Contains(runOSes, r.OS) {
				add("%s: platform run %s with unknown os %q", e.ID, r.Ref, r.OS)
			}
			if r.Kind != "platform" && r.OS != "" {
				add("%s: os on a %s run", e.ID, r.Kind)
			}
			if r.Ref == "" {
				add("%s: a run without ref", e.ID)
			}
		}
		if e.Deferred != nil && slices.Contains(planIDs, e.ID) {
			add("%s is in the PLAN row and exit criterion 8 and cannot be deferred", e.ID)
		}
		if e.Deferred != nil && !ticketRE.MatchString(e.Deferred.Ticket) {
			add("%s: deferred without a ticket", e.ID)
		}
		for _, p := range e.DeferredParts {
			if !ticketRE.MatchString(p.Ticket) {
				add("%s: deferred part %q without a ticket", e.ID, p.Part)
			}
		}
	}
	if len(probs) > 0 {
		return nil, fmt.Errorf("catalogue.yaml: %v", probs)
	}
	return &c, nil
}
