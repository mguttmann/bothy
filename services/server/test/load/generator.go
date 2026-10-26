// SPDX-License-Identifier: Apache-2.0

package load

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// PersonSpec is one generated person: the handle, display name and e-mail of createUser.
type PersonSpec struct {
	Handle, DisplayName, Email string
}

// Generator derives the people and the message texts of a run from a seed. Equal seeds give
// equal worlds and texts, so two runs of one commit offer the same load (brief M7-18
// requirement 12). The zero value uses seed 0; NewGenerator(Seed) is the run's generator.
type Generator struct {
	seed uint64
}

// NewGenerator returns the generator of seed.
func NewGenerator(seed uint64) Generator { return Generator{seed: seed} }

// named are the first people after the owner Alex (white label, brief rule 8).
var named = []PersonSpec{
	{Handle: "sam", DisplayName: "Sam", Email: "sam@example.com"},
	{Handle: "robin", DisplayName: "Robin", Email: "robin@example.com"},
}

// Person returns person i (from 0): Sam, Robin, then the numbered test people person-<n> with
// n from 1. The owner of the installation (Alex) is not among them.
//
// Example:
//
//	NewGenerator(Seed).Person(2) // {person-1, Person 1, person-1@example.com}
func (g Generator) Person(i int) PersonSpec {
	if i < len(named) {
		return named[i]
	}
	n := i - len(named) + 1
	return PersonSpec{Handle: fmt.Sprintf("person-%d", n), DisplayName: fmt.Sprintf("Person %d", n), Email: fmt.Sprintf("person-%d@example.com", n)}
}

// words is the vocabulary of the generated texts: plain lower-case words, no mention, no link and
// no markup, so a text never triggers an agent, a preview or a mention hint.
var words = strings.Fields(`the a team plan build test check deploy release note review fix update
	server client message thread channel space today tomorrow morning later after before with
	without about from into ready done open closed new old small large quick slow first last next
	please thanks sure maybe yes no meeting draft report numbers schedule budget week month list
	question answer idea change detail summary status issue error log metric chart doc page`)

// Content returns the text of message i of step: MinContentRunes to MaxContentRunes runes (ASCII,
// so also bytes) of words from a fixed vocabulary, the length uniform over the range. It depends only on the seed,
// step and i.
func (g Generator) Content(step, i int) string {
	// #nosec G404 -- a seeded, reproducible load generator, not a secret
	r := rand.New(rand.NewPCG(g.seed, uint64(step)<<32|uint64(uint32(i)))) //nolint:gosec // G115: the bit pattern only seeds the generator
	n := MinContentRunes + r.IntN(MaxContentRunes-MinContentRunes+1)
	var b strings.Builder
	for b.Len() < n {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(words[r.IntN(len(words))])
	}
	text := b.String()[:n]
	if text[n-1] == ' ' {
		// A cut right after a word: end with a letter, so the length stays n without padding.
		text = text[:n-1] + "s"
	}
	return text
}
