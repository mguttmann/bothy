// SPDX-License-Identifier: Apache-2.0

package searchkit

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// markerBytes is the size of one marker: 128 bits (14 4.6).
const markerBytes = 16

// Markers are StartSel and StopSel of ts_headline: random lower-case hex strings of 128 bits per
// request (14 4.6, R-SU-06). Hex keeps them free of the characters ts_headline's option syntax
// gives a meaning, and a content text cannot know them in advance, so HTML or a look-alike in a
// message stays text.
type Markers struct {
	// Start is StartSel.
	Start string
	// Stop is StopSel.
	Stop string
}

// NewMarkers draws two markers from r (crypto/rand when r is nil). Errors: the error of r, a
// short read, or two equal markers (a broken source).
func NewMarkers(r io.Reader) (Markers, error) {
	if r == nil {
		r = rand.Reader
	}
	var b [2 * markerBytes]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return Markers{}, fmt.Errorf("searchkit: draw the highlight markers: %w", err)
	}
	m := Markers{Start: hex.EncodeToString(b[:markerBytes]), Stop: hex.EncodeToString(b[markerBytes:])}
	if m.Start == m.Stop {
		return Markers{}, errors.New("searchkit: the random source gave two equal highlight markers")
	}
	return m, nil
}

// Segment is one piece of an excerpt: Match marks a hit of the query (14 4.6).
type Segment struct {
	// Text is the piece, verbatim from the content.
	Text string
	// Match reports a matched piece.
	Match bool
}

// Split splits the ts_headline result headline at the markers m into segments. It reads left to
// right and always takes the earliest occurrence of either marker: Start opens a match, Stop
// closes it, a Start inside a match and a Stop outside one are dropped, and a match without Stop
// runs to the end. Empty markers split nothing. Empty pieces are dropped and neighbours with the same Match are joined, so the
// texts of the result, joined, equal headline without its markers. The result is never nil.
//
// Example:
//
//	Split("the "+m.Start+"invoice"+m.Stop+" fails", m)
//	// [{Text: "the "}, {Text: "invoice", Match: true}, {Text: " fails"}]
func Split(headline string, m Markers) []Segment {
	out := []Segment{}
	add := func(text string, match bool) {
		switch {
		case text == "":
		case len(out) > 0 && out[len(out)-1].Match == match:
			out[len(out)-1].Text += text
		default:
			out = append(out, Segment{Text: text, Match: match})
		}
	}
	if m.Start == "" || m.Stop == "" {
		// Not markers of NewMarkers; nothing can be split.
		add(headline, false)
		return out
	}
	match := false
	for headline != "" {
		i, j := strings.Index(headline, m.Start), strings.Index(headline, m.Stop)
		if i < 0 && j < 0 {
			add(headline, match)
			break
		}
		if j < 0 || (i >= 0 && i < j) {
			add(headline[:i], match)
			headline = headline[i+len(m.Start):]
			match = true
			continue
		}
		add(headline[:j], match)
		headline = headline[j+len(m.Stop):]
		match = false
	}
	return out
}
