// SPDX-License-Identifier: Apache-2.0

package demo

import "math/rand/v2"

// seedStream is the second word of the PCG state; the seed of the command is the first. A fixed
// value, so the same seed always draws the same plan (16 16.4).
const seedStream = 0x6465_6d6f // "demo"

// Planned is one warm-up prompt as the seed drew it: where it goes, its title, the text variant
// and the person who posts it.
type Planned struct {
	Space, Channel, Title, Text, Person string
}

// Plan draws the warm-up of the data set with seed: values.warmup_prompts prompts with
// replacement, for each the text variant and the person among the people with trigger on its
// space. Every choice of the demo that is not fixed by the data set comes from here, so the same
// seed and data set give the same plan on every machine (16 16.4).
//
// Example:
//
//	for _, p := range ds.Plan(7) { fmt.Println(p.Person, p.Title) }
func (ds *DataSet) Plan(seed uint64) []Planned {
	r := rand.New(rand.NewPCG(seed, seedStream)) // #nosec G404 -- a reproducible plan, not a secret
	out := make([]Planned, 0, ds.Values.WarmupPrompts)
	for range ds.Values.WarmupPrompts {
		p := ds.Prompts[r.IntN(len(ds.Prompts))]
		text := p.Texts[r.IntN(len(p.Texts))]
		people := ds.Space(p.Space).triggerPeople()
		person := people[r.IntN(len(people))]
		out = append(out, Planned{Space: p.Space, Channel: p.Channel, Title: p.Title, Text: text, Person: person})
	}
	return out
}
