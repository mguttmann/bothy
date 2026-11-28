// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// testScreen renders a 1280x832 screen: a light page, a dark block whose left edge is at blockX
// (a page or a dialog that changes), and a caret of 2x16 pixels when caret is set.
func testScreen(t *testing.T, blockX int, caret bool) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, 1280, 832))
	for y := 0; y < 832; y++ {
		for x := 0; x < 1280; x++ {
			c := color.RGBA{R: 240, G: 240, B: 240, A: 255}
			if x >= blockX && x < blockX+400 && y >= 200 && y < 600 {
				c = color.RGBA{R: 30, G: 60, B: 120, A: 255}
			}
			if caret && x >= 100 && x < 102 && y >= 100 && y < 116 {
				c = color.RGBA{A: 255}
			}
			m.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, m, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func gridOf(t *testing.T, img []byte) lumaGrid {
	t.Helper()
	g, ok := lumaOf(img)
	if !ok {
		t.Fatal("no grid")
	}
	return g
}

func TestLumaDiffers(t *testing.T) {
	page := testScreen(t, 100, false)
	if gridOf(t, page).differs(gridOf(t, testScreen(t, 100, false))) {
		t.Error("the same screen differs")
	}
	if gridOf(t, page).differs(gridOf(t, testScreen(t, 100, true))) {
		t.Error("a blinking caret counts as a change")
	}
	if !gridOf(t, page).differs(gridOf(t, testScreen(t, 700, false))) {
		t.Error("a moved block does not count as a change")
	}
	small := gridOf(t, page)
	small.w--
	if !gridOf(t, page).differs(small) {
		t.Error("another size does not count as a change")
	}
	if _, ok := lumaOf([]byte{0xab, 0xcd}); ok {
		t.Error("no JPEG decoded")
	}
}

// vclock is a virtual clock: sleep advances it, a look costs lookCost and sees the screen of
// screenAt at the time the look starts.
type vclock struct {
	t        *testing.T
	start    time.Time
	now      time.Time
	lookCost time.Duration
	screenAt func(time.Duration) []byte
	looks    int
	fail     bool
	ended    bool
}

func newVclock(t *testing.T, cost time.Duration, screenAt func(time.Duration) []byte) *vclock {
	s := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	return &vclock{t: t, start: s, now: s, lookCost: cost, screenAt: screenAt}
}

func (v *vclock) waiter() quietWaiter {
	return quietWaiter{
		now: func() time.Time { return v.now },
		sleep: func(d time.Duration) bool {
			if d > 0 {
				v.now = v.now.Add(d)
			}
			return !v.ended
		},
		look: func() (outcome, bool) {
			v.looks++
			img := bytes.Clone(v.screenAt(v.now.Sub(v.start))) // the waiter clears every look it drops
			v.now = v.now.Add(v.lookCost)
			if v.fail {
				return outcome{code: CodeScreenLocked}, false
			}
			return outcome{code: CodeOK, img: img}, true
		},
		cost: func() time.Duration { return v.lookCost },
	}
}

func (v *vclock) elapsed() time.Duration { return v.now.Sub(v.start) }

func TestQuietWait(t *testing.T) {
	still := testScreen(t, 100, false)
	moved := testScreen(t, 700, false)
	caret := testScreen(t, 100, true)
	// frames alternates two screens every 40 ms: a video or a game.
	frames := func(d time.Duration) []byte {
		if d/(40*time.Millisecond)%2 == 0 {
			return still
		}
		return moved
	}
	cases := []struct {
		name        string
		limit, idle time.Duration
		cost        time.Duration
		screenAt    func(time.Duration) []byte
		fail        bool
		end         quietEnd
		from, to    time.Duration
	}{
		{name: "still screen ends after the idle time", limit: 5 * time.Second, idle: quietIdleWait, cost: 30 * time.Millisecond,
			screenAt: func(time.Duration) []byte { return still }, end: quietStable, from: quietIdleWait, to: quietIdleWait + 200*time.Millisecond},
		{name: "a page that arrives ends quietFor after it", limit: 5 * time.Second, idle: quietIdleWait, cost: 30 * time.Millisecond,
			screenAt: func(d time.Duration) []byte {
				if d < 800*time.Millisecond {
					return still
				}
				return moved
			}, end: quietStable, from: 800*time.Millisecond + quietFor, to: 800*time.Millisecond + quietFor + 250*time.Millisecond},
		{name: "a blinking caret does not keep the wait open", limit: 5 * time.Second, idle: quietIdleWait, cost: 30 * time.Millisecond,
			screenAt: func(d time.Duration) []byte {
				if d/(500*time.Millisecond)%2 == 0 {
					return still
				}
				return caret
			}, end: quietStable, from: quietIdleWait, to: quietIdleWait + 200*time.Millisecond},
		{name: "a video runs to the limit and no longer", limit: 3 * time.Second, idle: quietIdleWait, cost: 30 * time.Millisecond,
			screenAt: frames, end: quietCapped, from: 3 * time.Second, to: 3 * time.Second},
		{name: "a failed look waits the limit as before", limit: 2 * time.Second, idle: quietIdleWait, cost: 30 * time.Millisecond,
			screenAt: func(time.Duration) []byte { return still }, fail: true, end: quietCapped, from: 2 * time.Second, to: 2 * time.Second},
		{name: "settle of a still screen", limit: 500 * time.Millisecond, idle: quietIdleSettle, cost: 30 * time.Millisecond,
			screenAt: func(time.Duration) []byte { return still }, end: quietStable, from: quietIdleSettle, to: quietIdleSettle + 100*time.Millisecond},
		{name: "settle with a look too slow for its limit sleeps the limit", limit: 500 * time.Millisecond, idle: quietIdleSettle, cost: 600 * time.Millisecond,
			screenAt: func(time.Duration) []byte { return still }, end: quietCapped, from: 500 * time.Millisecond, to: 500 * time.Millisecond},
		{name: "a wait shorter than the floor", limit: 50 * time.Millisecond, idle: quietIdleWait, cost: 30 * time.Millisecond,
			screenAt: func(time.Duration) []byte { return still }, end: quietCapped, from: 50 * time.Millisecond, to: 50 * time.Millisecond},
		{name: "wait 0", limit: 0, idle: quietIdleWait, cost: 30 * time.Millisecond,
			screenAt: func(time.Duration) []byte { return still }, end: quietCapped, from: 0, to: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := newVclock(t, c.cost, c.screenAt)
			v.fail = c.fail
			o, end := v.waiter().run(c.limit, c.idle)
			if end != c.end {
				t.Fatalf("end %d, want %d", end, c.end)
			}
			if got := v.elapsed(); got < c.from || got > c.to {
				t.Errorf("took %v, want %v to %v", got, c.from, c.to)
			}
			if got := v.elapsed(); got > c.limit {
				t.Errorf("took %v, longer than the limit %v", got, c.limit)
			}
			if c.end == quietStable && (o.code != CodeOK || len(o.img) == 0) {
				t.Errorf("a quiet end without the last look: %+v", o.code)
			}
			if c.end != quietStable && o.img != nil {
				t.Error("an image without a quiet end")
			}
		})
	}
}

func TestQuietWaitSessionEnds(t *testing.T) {
	still := testScreen(t, 100, false)
	v := newVclock(t, 30*time.Millisecond, func(time.Duration) []byte { return still })
	v.ended = true
	if _, end := v.waiter().run(5*time.Second, quietIdleWait); end != quietGone {
		t.Errorf("end %d, want gone", end)
	}
	if v.looks != 0 {
		t.Errorf("%d looks after the end", v.looks)
	}
}

// TestQuietInSession is the adaptive wait through a session and the helper channel: a still
// screen ends a wait of 5 s and the settle of a click early with the image of the last look, a
// quick wait ends early without an image, and a screen that keeps changing runs to the limit.
func TestQuietInSession(t *testing.T) {
	r := newRig(t)
	fh := r.connect(hello())
	still, moved := testScreen(t, 100, false), testScreen(t, 700, false)
	fh.set(func(f *fakeHelper) { f.img, f.imgW, f.imgH = still, 1280, 832 })
	r.waitState(func(s HelperState) bool { return s.Connected })
	const settle = 2 * time.Second
	op := openOp(sidA)
	op.Settings.SettleMs = settle.Milliseconds()
	r.m.Open(context.Background(), op, r.n)
	d, ok := r.handshake(sidA, nil)
	if !ok {
		t.Fatalf("handshake refused: %v", r.n.endedOf(sidA))
	}
	if st := d.state(); st.State != frame.StateActive {
		t.Fatalf("first cu_state %+v", st)
	}
	timed := func(name, args string, quick bool) (frame.Result, []byte, time.Duration) {
		start := time.Now()
		res, img, _ := d.send(name, args, quick)
		return res, img, time.Since(start)
	}
	first, img, took := timed(actWait, `{"duration_ms":5000}`, false)
	if first.Code != CodeOK || !bytes.Equal(img, still) || took < quietIdleWait || took > quietIdleWait+time.Second {
		t.Errorf("wait on a still screen: %+v with %d image bytes after %v", first, len(img), took)
	}
	captures := len(fh.opsOf(OpCapture))
	if res, img, took := timed(actLeftClick, `{"coordinate":[10,10],"scale_id":`+itoa(first.ScaleID)+`}`, false); res.Code != CodeOK || !bytes.Equal(img, still) ||
		took < quietIdleSettle || took > settle-500*time.Millisecond {
		t.Errorf("settle on a still screen: %+v with %d image bytes after %v", res, len(img), took)
	}
	// The image of the click is its last look, not one more capture.
	looks := len(fh.opsOf(OpCapture)) - captures
	if res, img, took := timed(actWait, `{"duration_ms":5000}`, true); res.Code != CodeOK || res.Image || img != nil ||
		took > quietIdleWait+time.Second {
		t.Errorf("quick wait on a still screen: %+v after %v", res, took)
	}
	if looks < 2 {
		t.Errorf("the settle looked %d times", looks)
	}

	stop := make(chan struct{})
	flipped := make(chan struct{})
	go func() {
		defer close(flipped)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
			}
			img := still
			if i%2 == 0 {
				img = moved
			}
			fh.set(func(f *fakeHelper) { f.img = img })
		}
	}()
	res, img, took := timed(actWait, `{"duration_ms":1200}`, false)
	close(stop)
	<-flipped
	if res.Code != CodeOK || len(img) == 0 || took < 1200*time.Millisecond {
		t.Errorf("wait on a changing screen: %+v with %d image bytes after %v", res, len(img), took)
	}
	d.sendFrame(frame.Close())
	r.n.waitEnded(t, sidA)
}

// TestQuietValuesOfTheSpec pins the values that docs/spec/27-computer-use.md 6.4 and
// D-CU-speed-2-2 and 2-3 name; a change here changes the spec in the same change.
func TestQuietValuesOfTheSpec(t *testing.T) {
	got := []time.Duration{quietFloor, quietEvery, quietFor, quietIdleWait, quietIdleSettle}
	want := []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 300 * time.Millisecond, 1500 * time.Millisecond, 400 * time.Millisecond}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("value %d is %v, the spec says %v", i, got[i], want[i])
		}
	}
	if quietCell != 8 || quietCellDelta != 16 || quietShare != 2 {
		t.Errorf("cell %d, delta %d, share %d per mille; the spec says 8, 16, 2", quietCell, quietCellDelta, quietShare)
	}
}
