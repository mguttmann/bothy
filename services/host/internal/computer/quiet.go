// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"hash/maphash"
	"image"
	"image/jpeg"
	"time"
)

// The adaptive wait of docs/spec/27-computer-use.md 6.4 (D-CU-speed-2-1 to 4): `wait` and the
// settle after an input action end as soon as the screen is quiet, at the latest at their limit.
// The host compares the screenshots it takes from the helper anyway; the helper is unchanged.
const (
	// quietFloor is the shortest time before the first look at the screen.
	quietFloor = 100 * time.Millisecond
	// quietFor is how long the screen must stay unchanged after a change.
	quietFor = 300 * time.Millisecond
	// quietIdleWait and quietIdleSettle end a wait or a settle in which nothing changed at all.
	quietIdleWait   = 1500 * time.Millisecond
	quietIdleSettle = 400 * time.Millisecond
	// quietEvery is the time from the start of one look to the start of the next.
	quietEvery = 100 * time.Millisecond
	// quietCell is the edge in image pixels of the cells whose mean brightness is compared;
	// quietCellDelta is the change of a cell's mean that counts, quietShare the part of all cells
	// in per mille that must change before the screen counts as changed. A blinking caret, a
	// spinner or a ticking clock stays below it; a new page, a dialog or a video does not.
	quietCell      = 8
	quietCellDelta = 16
	quietShare     = 2
)

// quietEnd says how an adaptive wait ended.
type quietEnd int

const (
	// quietGone: the session ended during the wait.
	quietGone quietEnd = iota + 1
	// quietCapped: the limit was reached (a screen that never settled, a failed look, or no
	// time for one more look); the caller takes its image as before.
	quietCapped
	// quietStable: the screen was quiet; the outcome is the last look, taken at the end.
	quietStable
)

// lumaGrid is the mean brightness of the cells of one screenshot.
type lumaGrid struct {
	w, h  int
	cells []uint32
}

// lumaOf decodes a JPEG screenshot into its cell grid; false when the bytes are no JPEG.
func lumaOf(img []byte) (lumaGrid, bool) {
	m, err := jpeg.Decode(bytes.NewReader(img))
	if err != nil {
		return lumaGrid{}, false
	}
	b := m.Bounds()
	w, h := (b.Dx()+quietCell-1)/quietCell, (b.Dy()+quietCell-1)/quietCell
	if w == 0 || h == 0 {
		return lumaGrid{}, false
	}
	sum := make([]uint32, w*h)
	n := make([]uint32, w*h)
	yc, isYCbCr := m.(*image.YCbCr)
	for y := 0; y < b.Dy(); y++ {
		cells, counts := sum[(y/quietCell)*w:][:w], n[(y/quietCell)*w:][:w]
		if isYCbCr {
			row := yc.Y[y*yc.YStride:][:b.Dx()]
			for x, v := range row {
				cells[x/quietCell] += uint32(v)
				counts[x/quietCell]++
			}
			continue
		}
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, _ := m.At(b.Min.X+x, b.Min.Y+y).RGBA()
			cells[x/quietCell] += (19595*r + 38470*g + 7471*bl + 1<<15) >> 24
			counts[x/quietCell]++
		}
	}
	for i := range sum {
		sum[i] /= n[i]
	}
	return lumaGrid{w: w, h: h, cells: sum}, true
}

// differs reports whether the screen changed between a and b beyond the small regions of
// quietShare: another size, or more than quietShare per mille of the cells by more than
// quietCellDelta.
func (a lumaGrid) differs(b lumaGrid) bool {
	if a.w != b.w || a.h != b.h {
		return true
	}
	changed := 0
	for i, v := range a.cells {
		d := int64(v) - int64(b.cells[i])
		if d > quietCellDelta || d < -quietCellDelta {
			changed++
		}
	}
	return changed*1000 > len(a.cells)*quietShare
}

// quietWaiter runs one adaptive wait over its clock, sleep and look; the session supplies them,
// tests supply a virtual clock.
type quietWaiter struct {
	now func() time.Time
	// sleep waits d; false when the session ended.
	sleep func(d time.Duration) bool
	// look takes a screenshot; false when it failed.
	look func() (outcome, bool)
	// cost is the expected duration of one look; no look starts that would end after the limit.
	cost func() time.Duration
}

// run waits at most limit. It looks at the screen every quietEvery from quietFloor on and ends
// quietStable when the screen stayed unchanged for quietFor after a change, or for idle since the
// start when nothing changed at all; the outcome is then the last look. A failed look stops the
// looking, and the wait runs to its limit as before.
func (w quietWaiter) run(limit, idle time.Duration) (outcome, quietEnd) {
	start := w.now()
	deadline := start.Add(limit)
	if !w.sleep(min(quietFloor, limit)) {
		return outcome{}, quietGone
	}
	q := quietState{seed: maphash.MakeSeed(), start: start, idle: idle}
	for {
		t := w.now()
		if !t.Before(deadline) || t.Add(w.cost()).After(deadline) {
			return outcome{}, w.rest(deadline)
		}
		o, stable, ok := w.step(&q, t)
		switch {
		case !ok:
			return outcome{}, w.rest(deadline)
		case stable:
			return o, quietStable
		}
		next := t.Add(quietEvery)
		if next.After(deadline) {
			next = deadline
		}
		if d := next.Sub(w.now()); d > 0 && !w.sleep(d) {
			return outcome{}, quietGone
		}
	}
}

// step takes one look that starts at t; it returns the look when the screen is quiet and clears
// it otherwise. false: the look failed.
func (w quietWaiter) step(q *quietState, t time.Time) (outcome, bool, bool) {
	o, ok := w.look()
	stable := false
	if ok {
		stable, ok = q.see(t, o.img)
	}
	if ok && stable {
		return o, true, true
	}
	clear(o.img)
	return outcome{}, false, ok
}

// quietState is what one adaptive wait has seen: the reference look (the first one, or the last
// one that differed from its reference), its time and whether the screen changed at all.
type quietState struct {
	seed          maphash.Seed
	start         time.Time
	idle          time.Duration
	seen, changed bool
	ref           lumaGrid
	refSum        uint64
	refAt         time.Time
}

// see takes the look img that started at t and reports whether the screen is quiet now; false
// as its second result when img is no JPEG. An unchanged screen gives the same bytes, so only
// another screenshot is decoded.
func (q *quietState) see(t time.Time, img []byte) (stable, ok bool) {
	sum := maphash.Bytes(q.seed, img)
	if q.seen && sum == q.refSum {
		return q.quietAt(t), true
	}
	g, ok := lumaOf(img)
	switch {
	case !ok:
		return false, false
	case !q.seen:
		q.seen, q.ref, q.refSum, q.refAt = true, g, sum, t
		return false, true
	case g.differs(q.ref):
		q.changed, q.ref, q.refSum, q.refAt = true, g, sum, t
		return false, true
	}
	return q.quietAt(t), true
}

// quietAt reports whether a look at t that equals the reference ends the wait.
func (q *quietState) quietAt(t time.Time) bool {
	if q.changed {
		return t.Sub(q.refAt) >= quietFor
	}
	return t.Sub(q.start) >= q.idle
}

// rest sleeps until deadline and reports quietCapped, or quietGone when the session ended.
func (w quietWaiter) rest(deadline time.Time) quietEnd {
	if !w.sleep(deadline.Sub(w.now())) {
		return quietGone
	}
	return quietCapped
}

// quiet runs the adaptive wait of the session with the given limit and idle time.
func (s *csession) quiet(limit, idle time.Duration) (outcome, quietEnd) {
	w := quietWaiter{
		now:   s.m.c.Clock.Now,
		sleep: s.sleep,
		look: func() (outcome, bool) {
			o := s.capture(nil, -1)
			return o, o.code == CodeOK
		},
		cost: func() time.Duration { return time.Duration(s.captureCost.Load()) },
	}
	return w.run(limit, idle)
}
