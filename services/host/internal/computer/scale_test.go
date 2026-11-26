// SPDX-License-Identifier: Apache-2.0

package computer

import "testing"

// TestScaleMapping: a coordinate of a screenshot maps back with the factor of its geometry and
// rounds to the nearest pixel inside the screen (CU-T11).
func TestScaleMapping(t *testing.T) {
	cases := []struct {
		name         string
		screen       Screen
		imgW, imgH   int64
		x, y         int64
		wantX, wantY float64
	}{
		{"factor 1", Screen{0, 0, 1280, 800, 1}, 1280, 800, 100, 200, 100, 200},
		{"retina factor 2", Screen{0, 0, 1512, 982, 2}, 1280, 831, 640, 400, 756, 472.5},
		{"retina corner", Screen{0, 0, 1512, 982, 2}, 1280, 831, 1279, 830, 1511, 981},
		{"windows 1.25", Screen{0, 0, 1536, 864, 1.25}, 1280, 720, 101, 0, 121.6, 0},
		{"windows 1.5 origin left", Screen{-1920, 0, 1920, 1080, 1.5}, 1280, 720, 1279, 719, -1.33, 1078.67},
		{"origin below", Screen{100, -50, 1280, 800, 1}, 1280, 800, 0, 0, 100, -50},
	}
	for _, c := range cases {
		m := NewScaleMap()
		id := m.Assign(c.screen, c.imgW, c.imgH)
		got, code := m.ToScreen(id, c.x, c.y)
		if code != "" || got.X != c.wantX || got.Y != c.wantY {
			t.Errorf("%s: (%d, %d) maps to (%v, %v) %q, want (%v, %v)", c.name, c.x, c.y, got.X, got.Y, code, c.wantX, c.wantY)
		}
		// Every mapped point lies inside the screen.
		for _, p := range [][2]int64{{0, 0}, {c.imgW - 1, c.imgH - 1}} {
			q, _ := m.ToScreen(id, p[0], p[1])
			if q.X < c.screen.X || q.Y < c.screen.Y || q.X >= c.screen.X+c.screen.Width || q.Y >= c.screen.Y+c.screen.Height {
				t.Errorf("%s: corner %v maps outside the screen to %+v", c.name, p, q)
			}
		}
	}
}

// TestScaleIDs: one scale ID per distinct geometry, kept for the session.
func TestScaleIDs(t *testing.T) {
	m := NewScaleMap()
	a := m.Assign(Screen{0, 0, 1512, 982, 2}, 1280, 831)
	if b := m.Assign(Screen{0, 0, 1512, 982, 2}, 1280, 831); b != a {
		t.Errorf("same geometry got %d and %d", a, b)
	}
	c := m.Assign(Screen{0, 0, 1920, 1080, 1}, 1280, 720)
	if c == a || a < 1 {
		t.Errorf("ids %d and %d", a, c)
	}
}

// TestStaleAndInvalid: an outdated scale ID is stale_screenshot, an unknown one or a point outside
// the image invalid_action (CU-T12).
func TestStaleAndInvalid(t *testing.T) {
	m := NewScaleMap()
	id := m.Assign(Screen{0, 0, 1280, 800, 1}, 1280, 800)
	for _, p := range [][2]int64{{-1, 0}, {0, -1}, {1280, 0}, {0, 800}} {
		if _, code := m.ToScreen(id, p[0], p[1]); code != CodeInvalidAction {
			t.Errorf("point %v: %q", p, code)
		}
	}
	if _, code := m.ToScreen(id+7, 1, 1); code != CodeInvalidAction {
		t.Errorf("unknown id: %q", code)
	}
	m.Observe(Screen{0, 0, 1920, 1080, 1})
	if _, code := m.ToScreen(id, 1, 1); code != CodeStaleScreenshot {
		t.Errorf("after a resolution change: %q", code)
	}
	if _, code := m.Region(id, 0, 0, 10, 10); code != CodeStaleScreenshot {
		t.Errorf("region after a resolution change: %q", code)
	}
	m.Observe(Screen{0, 0, 1280, 800, 1})
	if _, code := m.ToScreen(id, 1, 1); code != "" {
		t.Errorf("back to the geometry: %q", code)
	}
}

// TestRegion maps a zoom region into screen space; an empty or reversed region is invalid.
func TestRegion(t *testing.T) {
	m := NewScaleMap()
	id := m.Assign(Screen{0, 0, 1512, 982, 2}, 1280, 831)
	r, code := m.Region(id, 640, 400, 960, 600)
	if code != "" || r.X != 756 || r.Y != 472.5 || r.Width != 378 || r.Height != 236.5 {
		t.Errorf("region %+v %q", r, code)
	}
	if _, code := m.Region(id, 10, 10, 10, 20); code != CodeInvalidAction {
		t.Errorf("empty region: %q", code)
	}
	if _, code := m.Region(id, 10, 10, 1281, 20); code != CodeInvalidAction {
		t.Errorf("region outside: %q", code)
	}
}

// TestToImage maps a pointer position back into screenshot pixels.
func TestToImage(t *testing.T) {
	m := NewScaleMap()
	id := m.Assign(Screen{0, 0, 1512, 982, 2}, 1280, 831)
	p, ok := m.ToImage(id, XY{X: 756, Y: 472.5})
	if !ok || p.X != 640 || p.Y != 400 {
		t.Errorf("ToImage = %+v %v", p, ok)
	}
	if p, ok := m.ToImage(id, XY{X: 5000, Y: -3}); !ok || p.X != 1279 || p.Y != 0 {
		t.Errorf("a point outside the screen: %+v %v, want the nearest pixel", p, ok)
	}
	if _, ok := m.ToImage(id+1, XY{X: 1, Y: 1}); ok {
		t.Error("an unknown id maps")
	}
}
