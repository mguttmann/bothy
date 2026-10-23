// SPDX-License-Identifier: Apache-2.0

package searchkit_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// testMarkers are fixed markers of the right shape; NewMarkers draws them in production.
var testMarkers = searchkit.Markers{Start: "0123456789abcdef0123456789abcdef", Stop: "fedcba9876543210fedcba9876543210"}

// mark wraps s in the test markers like ts_headline does with StartSel and StopSel.
func mark(s string) string { return testMarkers.Start + s + testMarkers.Stop }

func TestSplit(t *testing.T) {
	t.Parallel()
	look := "a1b2c3d4e5f60718293a4b5c6d7e8f90" // 32 hex characters that are not a marker
	cases := []struct {
		name     string
		headline string
		want     []searchkit.Segment
	}{
		{name: "empty", headline: "", want: []searchkit.Segment{}},
		{name: "no marker", headline: "plain text", want: []searchkit.Segment{{Text: "plain text"}}},
		{name: "one match", headline: "the " + mark("invoice") + " export fails", want: []searchkit.Segment{
			{Text: "the "}, {Text: "invoice", Match: true}, {Text: " export fails"},
		}},
		{name: "match at the edges", headline: mark("a") + " b " + mark("c"), want: []searchkit.Segment{
			{Text: "a", Match: true}, {Text: " b "}, {Text: "c", Match: true},
		}},
		{name: "adjacent matches join", headline: mark("in") + mark("voice") + " x", want: []searchkit.Segment{
			{Text: "invoice", Match: true}, {Text: " x"},
		}},
		{name: "repeated matches", headline: mark("run") + " and " + mark("run"), want: []searchkit.Segment{
			{Text: "run", Match: true}, {Text: " and "}, {Text: "run", Match: true},
		}},
		{name: "html stays text", headline: "<b>bold</b> &lt; " + mark("x") + " " + look, want: []searchkit.Segment{
			{Text: "<b>bold</b> &lt; "}, {Text: "x", Match: true}, {Text: " " + look},
		}},
		{name: "empty match drops", headline: "a" + mark("") + "b", want: []searchkit.Segment{{Text: "ab"}}},
		{name: "start without stop", headline: "a" + testMarkers.Start + "b", want: []searchkit.Segment{
			{Text: "a"}, {Text: "b", Match: true},
		}},
		{name: "stray stop", headline: "a" + testMarkers.Stop + "b", want: []searchkit.Segment{{Text: "ab"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := searchkit.Split(c.headline, testMarkers)
			if !equalSegments(got, c.want) {
				t.Fatalf("Split(%q) = %+v, want %+v", c.headline, got, c.want)
			}
			if j := join(got); j != stripMarkers(c.headline, testMarkers) {
				t.Fatalf("texts join to %q, want the headline without markers", j)
			}
		})
	}
}

func TestSplit_NeverNil(t *testing.T) {
	t.Parallel()
	if searchkit.Split("", testMarkers) == nil {
		t.Fatal("Split returned nil; a hit always has a segments array (no JSON null)")
	}
}

func FuzzSplit(f *testing.F) {
	for _, s := range []string{"", "x", mark("x"), "<b>" + mark("a") + "</b>", testMarkers.Stop + testMarkers.Start,
		testMarkers.Start, testMarkers.Start + testMarkers.Start + "x" + testMarkers.Stop + testMarkers.Stop, "\x00\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, headline string) {
		segs := searchkit.Split(headline, testMarkers)
		if j := join(segs); j != stripMarkers(headline, testMarkers) {
			t.Fatalf("texts join to %q, want %q", j, stripMarkers(headline, testMarkers))
		}
		for i, s := range segs {
			if s.Text == "" {
				t.Fatalf("segment %d is empty", i)
			}
			if i > 0 && segs[i-1].Match == s.Match {
				t.Fatalf("segments %d and %d have the same match flag", i-1, i)
			}
		}
	})
}

func TestNewMarkers(t *testing.T) {
	t.Parallel()
	a, err := searchkit.NewMarkers(nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := searchkit.NewMarkers(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{a.Start, a.Stop, b.Start, b.Stop} {
		if len(m) != 32 || strings.Trim(m, "0123456789abcdef") != "" {
			t.Fatalf("marker %q is not 128 bits in lower-case hex", m)
		}
	}
	if a.Start == a.Stop || a == b {
		t.Fatalf("markers repeat: %+v %+v", a, b)
	}
	if _, err := searchkit.NewMarkers(bytes.NewReader(make([]byte, 64))); err == nil {
		t.Fatal("equal start and stop markers were accepted")
	}
	boom := errors.New("boom")
	if _, err := searchkit.NewMarkers(failingReader{boom}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the reader's error", err)
	}
	if _, err := searchkit.NewMarkers(bytes.NewReader(make([]byte, 3))); err == nil {
		t.Fatal("a short read was accepted")
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

var _ io.Reader = failingReader{}

func equalSegments(a, b []searchkit.Segment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func join(segs []searchkit.Segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return b.String()
}

// stripMarkers removes the markers from s left to right, always the earliest occurrence of
// either marker first: the reading Split documents.
func stripMarkers(s string, m searchkit.Markers) string {
	var b strings.Builder
	for {
		i, j := strings.Index(s, m.Start), strings.Index(s, m.Stop)
		switch {
		case i < 0 && j < 0:
			b.WriteString(s)
			return b.String()
		case j < 0 || (i >= 0 && i < j):
			b.WriteString(s[:i])
			s = s[i+len(m.Start):]
		default:
			b.WriteString(s[:j])
			s = s[j+len(m.Stop):]
		}
	}
}
