// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"testing"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// set changes the fake helper's answers under its lock.
func (f *fakeHelper) set(edit func(*fakeHelper)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	edit(f)
}

// TestActionOutcomes: the outcomes of 27 6.4 beside the plain success: cursor_position before any
// screenshot, a helper that refuses or answers without a cursor, a refused capture, a refused
// injection, the quick pause, wait with and without its image, the repeat of key and the argument
// checks that inject nothing.
func TestActionOutcomes(t *testing.T) {
	r := newRig(t)
	fh := r.connect(hello())
	r.waitState(func(s HelperState) bool { return s.Connected })
	d, st := r.open(sidA)
	if st.State != frame.StateActive {
		t.Fatalf("first cu_state %+v", st)
	}

	// Without a screenshot the position takes a capture first, drops its image and answers in
	// pixels of that capture.
	res, img, _ := d.action(actCursorPosition, `{}`)
	if res.Code != CodeOK || res.Cursor == nil || res.ScaleID < 1 || len(img) != 0 {
		t.Fatalf("first cursor_position %+v with %d image bytes", res, len(img))
	}
	if len(fh.opsOf(OpCapture)) != 1 {
		t.Errorf("captures before the first position: %d", len(fh.opsOf(OpCapture)))
	}
	scaleID := itoa(res.ScaleID)

	fh.set(func(f *fakeHelper) { f.noCursor = true })
	if res, _, _ := d.action(actCursorPosition, `{}`); res.Code != CodeOK || res.Cursor != nil {
		t.Errorf("position without a cursor %+v", res)
	}
	fh.set(func(f *fakeHelper) { f.noCursor, f.posCode = false, CodeSecureDesktop })
	if res, _, _ := d.action(actCursorPosition, `{}`); res.Code != CodeSecureDesktop || res.Cursor != nil {
		t.Errorf("refused position %+v", res)
	}
	// A pointer beyond the screen edge stays inside the screenshot.
	fh.set(func(f *fakeHelper) { f.posCode, f.cursor = "", XY{X: 99999, Y: 99999} })
	if res, _, _ := d.action(actCursorPosition, `{}`); res.Code != CodeOK || res.Cursor == nil || res.Cursor.X != 1279 || res.Cursor.Y != 830 {
		t.Errorf("position beyond the screen edge %+v", res)
	}

	// A refused capture answers its code without an image; a click still counts as done.
	fh.set(func(f *fakeHelper) { f.capCode = CodeSecureDesktop })
	if res, img, _ := d.action(actScreenshot, `{}`); res.Code != CodeSecureDesktop || res.Image || len(img) != 0 {
		t.Errorf("refused screenshot %+v", res)
	}
	if res, img, _ := d.action(actLeftClick, `{"coordinate":[10,10],"scale_id":`+scaleID+`}`); res.Code != CodeOK || res.Image || len(img) != 0 {
		t.Errorf("click with a refused image %+v", res)
	}
	fh.set(func(f *fakeHelper) { f.capCode = "" })

	// A refused injection answers its code and takes no image.
	captures := len(fh.opsOf(OpCapture))
	fh.set(func(f *fakeHelper) { f.doCode = CodeOSRefused })
	if res, _, _ := d.action(actLeftClick, `{"coordinate":[10,10],"scale_id":`+scaleID+`}`); res.Code != CodeOSRefused || res.Image {
		t.Errorf("refused click %+v", res)
	}
	if len(fh.opsOf(OpCapture)) != captures {
		t.Error("a refused click took an image")
	}
	fh.set(func(f *fakeHelper) { f.doCode = "" })

	// quick: the input is done, no image; wait with quick has no image, without it one.
	if res, _, _ := d.quickAction(actLeftClick, `{"coordinate":[10,10],"scale_id":`+scaleID+`}`); res.Code != CodeOK || res.Image {
		t.Errorf("quick click %+v", res)
	}
	if res, _, _ := d.quickAction(actWait, `{"duration_ms":0}`); res.Code != CodeOK || res.Image {
		t.Errorf("quick wait %+v", res)
	}
	if res, _, _ := d.action(actWait, `{"duration_ms":0}`); res.Code != CodeOK || !res.Image {
		t.Errorf("wait %+v", res)
	}

	// The repeat of key reaches the helper.
	if res, _, _ := d.action(actKey, `{"keys":"Tab","repeat":3}`); res.Code != CodeOK {
		t.Errorf("key with repeat %+v", res)
	}
	dos := fh.opsOf(OpDo)
	if do := dos[len(dos)-1].(*Do); do.Keys != "Tab" || do.Repeat != 3 {
		t.Errorf("do %+v", do)
	}

	// Argument checks: a wrong type, a short coordinate, an unknown modifier and a zoom of an
	// unknown scale ID inject and capture nothing.
	before, captures := len(fh.opsOf(OpDo)), len(fh.opsOf(OpCapture))
	for _, c := range []struct{ name, args string }{
		{actLeftClick, `{"coordinate":"10,10","scale_id":` + scaleID + `}`},
		{actLeftClick, `{"coordinate":[10],"scale_id":` + scaleID + `}`},
		{actLeftClick, `{"coordinate":[10,10],"scale_id":` + scaleID + `,"modifiers":["hyper"]}`},
		{actType, `{"text":""}`},
	} {
		if res, _, _ := d.action(c.name, c.args); res.Code != CodeInvalidAction || res.Image {
			t.Errorf("%s %s: %+v", c.name, c.args, res)
		}
	}
	if res, _, _ := d.action(actZoom, `{"region":[0,0,10,10],"scale_id":99}`); res.Code == CodeOK || res.Image {
		t.Errorf("zoom of an unknown scale ID %+v", res)
	}
	if len(fh.opsOf(OpDo)) != before || len(fh.opsOf(OpCapture)) != captures {
		t.Error("a refused action reached the helper")
	}
	d.sendFrame(frame.Close())
	r.n.waitEnded(t, sidA)
}
