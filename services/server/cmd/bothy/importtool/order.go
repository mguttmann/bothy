// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/api/restclient"
)

// keyDigits are the base-62 digits of the fractional index of the server in ASCII order.
const keyDigits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// errKeyRange reports a sort key the scheme has no next integer for (head z with 26 digits z).
var errKeyRange = errors.New("no sort key after the last key of the server")

// placement is where a run puts what it creates (22 5.1 rule 4): the new spaces after the last
// sort key of the spaces of the server, the channels of a category whose target space the server
// already holds after the last channel of that space; each block keeps Discord order. The server
// keeps the key of an item it posted (a known origin key is answered, not written again), so a
// later run moves nothing an earlier run placed.
type placement struct {
	lastSpace   string            // "" on a server without spaces
	block       map[string]string // channel block by Discord category whose target space is live
	lastChannel map[string]string // last channel key by block: one block per live space, so the
	// categories that share an existing space fill it as one block in Discord order
}

// blockOf returns the channel block of a category: its live target space, else the category.
func (pl placement) blockOf(category string) string {
	if b, ok := pl.block[category]; ok {
		return b
	}
	return category
}

// placementOf reads the last keys of placement from the spaces of the server and the channels of
// every live target space of a mapped category (an existing space of the mapping or a space this
// source posted).
func (c *collisions) placementOf(ctx context.Context) (placement, error) {
	keys := make([]string, 0, len(c.spaces))
	for _, sp := range c.spaces {
		keys = append(keys, sp.SortKey)
	}
	pl := placement{lastSpace: lastKey(keys), block: map[string]string{}, lastChannel: map[string]string{}}
	for _, s := range c.m.Spaces {
		if s.Target.Skip {
			continue
		}
		space, ok := c.liveTarget(s.Category)
		if !ok {
			continue
		}
		chans, err := c.liveChannels(ctx, space)
		if err != nil {
			return placement{}, err
		}
		keys = keys[:0]
		for _, ch := range chans {
			keys = append(keys, ch.SortKey)
		}
		pl.block[s.Category] = "space:" + space.String()
		pl.lastChannel[pl.block[s.Category]] = lastKey(keys)
	}
	return pl, nil
}

// liveTarget returns the target space of a category when it is a live space of the server.
func (c *collisions) liveTarget(category string) (restclient.Uuid, bool) {
	space, ok := targetSpace(c.m, category, c.posted)
	if !ok || !slices.ContainsFunc(c.spaces, func(s restclient.Space) bool { return s.Id == space }) {
		return space, false
	}
	return space, true
}

// liveChannels returns the channels of a space, read once per run.
func (c *collisions) liveChannels(ctx context.Context, space restclient.Uuid) ([]restclient.Channel, error) {
	if chans, ok := c.chans[space]; ok {
		return chans, nil
	}
	chans, err := c.r.channels(ctx, space)
	if err != nil {
		return nil, err
	}
	c.chans[space] = chans
	return chans, nil
}

// lastKey returns the greatest valid key in byte order, "" when there is none.
func lastKey(keys []string) string {
	last := ""
	for _, k := range keys {
		if _, ok := integerPart(k); ok && k > last {
			last = k
		}
	}
	return last
}

// nextKey returns the next integer key after k ("a0" after ""), the step of fracindex.KeyBetween
// with an open upper end in the chat module (docs/decisions.d/M1-52.md, M1-52/2); the tool may
// not import that module, so the step is here.
func nextKey(k string) (string, error) {
	if k == "" {
		return "a0", nil
	}
	ip, ok := integerPart(k)
	if !ok {
		return "", errKeyRange
	}
	head, ds := ip[0], []byte(ip[1:])
	for i := len(ds) - 1; i >= 0; i-- {
		d := strings.IndexByte(keyDigits, ds[i]) + 1
		if d < len(keyDigits) {
			ds[i] = keyDigits[d]
			return string(head) + string(ds), nil
		}
		ds[i] = '0'
	}
	switch head {
	case 'Z':
		return "a0", nil
	case 'z':
		return "", errKeyRange
	}
	h := head + 1
	if h > 'a' {
		ds = append(ds, '0')
	} else {
		ds = ds[:len(ds)-1]
	}
	return string(h) + string(ds), nil
}

// integerPart returns the integer part of a valid key: a head letter (a to z: 1 to 26 digits, A to
// Z: 26 to 1) with its digits, then a fraction that does not end with 0; A with 26 zeros alone is
// no key.
func integerPart(k string) (string, bool) {
	if k == "" {
		return "", false
	}
	n := 0
	switch h := k[0]; {
	case h >= 'a' && h <= 'z':
		n = int(h-'a') + 2
	case h >= 'A' && h <= 'Z':
		n = int('Z'-h) + 2
	}
	if n == 0 || n > len(k) {
		return "", false
	}
	for i := 1; i < len(k); i++ {
		if strings.IndexByte(keyDigits, k[i]) < 0 {
			return "", false
		}
	}
	if (len(k) > n && k[len(k)-1] == '0') || k == "A"+strings.Repeat("0", 26) {
		return "", false
	}
	return k[:n], true
}

// keySeq hands out the keys of one block after its last key.
type keySeq struct {
	last string
}

// next returns the next key of the block.
func (s *keySeq) next() (string, error) {
	k, err := nextKey(s.last)
	if err != nil {
		return "", err
	}
	s.last = k
	return k, nil
}
