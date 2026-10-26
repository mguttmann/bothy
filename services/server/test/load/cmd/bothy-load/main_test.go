// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/test/load"
)

func TestRun_UsageErrorsExitTwo(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"no subcommand": nil,
		"unknown":       {"seed"},
		"no targets":    {"ramp", "--base-url", "http://127.0.0.1:1", "--public-url", "https://chat.example.com", "--dir", "/nonexistent"},
		"no base url":   {"ramp", "--public-url", "https://chat.example.com", "--dir", "/x", "--message-create-p99", "150ms", "--fanout-p99", "200ms", "--messages-per-minute", "60"},
		"falling rates": {"ramp", "--base-url", "http://127.0.0.1:1", "--public-url", "https://chat.example.com", "--dir", "/x", "--message-create-p99", "150ms", "--fanout-p99", "200ms", "--messages-per-minute", "60", "--rates", "x"},
		"unknown flag":  {"ramp", "--nope"},
	} {
		var out bytes.Buffer
		if code := run(args, &out); code != 2 {
			t.Errorf("%s: exit %d, want 2 (%s)", name, code, out.String())
		}
	}
}

func TestParseRates(t *testing.T) {
	t.Parallel()
	got, err := parseRates("")
	if err != nil || !slices.Equal(got, load.RampRates) {
		t.Errorf("empty: %v %v, want the ramp of params.go", got, err)
	}
	got, err = parseRates("10, 20.5")
	if err != nil || !slices.Equal(got, []float64{10, 20.5}) {
		t.Errorf("list: %v %v", got, err)
	}
	if _, err := parseRates("10,,20"); err == nil {
		t.Error("an empty entry was accepted")
	}
}
