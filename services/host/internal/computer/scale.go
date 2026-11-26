// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"math"
	"sync"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// geometry is one distinct geometry of a screenshot: the screen in screen space and the size of
// the image in pixels (docs/spec/27-computer-use.md 6.5 item 3).
type geometry struct {
	screen     Screen
	imgW, imgH int64
}

// ScaleMap assigns a scale ID per distinct geometry and keeps every geometry of the session, so an
// outdated ID answers stale_screenshot instead of invalid_action (Ruling D-CU-03-3). It knows the
// latest screen the helper reported; a geometry whose screen differs from it is outdated. It is
// safe for concurrent use.
type ScaleMap struct {
	mu      sync.Mutex
	next    int64
	ids     map[int64]geometry
	byGeo   map[geometry]int64
	current Screen
	known   bool
}

// NewScaleMap returns an empty map; the first ID is 1.
func NewScaleMap() *ScaleMap {
	return &ScaleMap{ids: map[int64]geometry{}, byGeo: map[geometry]int64{}}
}

// Assign returns the scale ID of the geometry of a screenshot of s with an image of w by h pixels,
// a new one when the geometry is new; s also becomes the current screen.
func (m *ScaleMap) Assign(s Screen, w, h int64) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current, m.known = s, true
	g := geometry{screen: s, imgW: w, imgH: h}
	if id, ok := m.byGeo[g]; ok {
		return id
	}
	m.next++
	m.ids[m.next] = g
	m.byGeo[g] = m.next
	return m.next
}

// Observe records s as the current screen (every done of the helper carries it).
func (m *ScaleMap) Observe(s Screen) {
	m.mu.Lock()
	m.current, m.known = s, true
	m.mu.Unlock()
}

// Current returns the scale ID of the newest geometry whose screen is the current screen, or 0.
func (m *ScaleMap) Current() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best int64
	for id, g := range m.ids {
		if m.known && g.screen == m.current && id > best {
			best = id
		}
	}
	return best
}

// lookup returns the geometry of id or the code of its refusal.
func (m *ScaleMap) lookup(id int64) (geometry, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.ids[id]
	switch {
	case !ok:
		return geometry{}, CodeInvalidAction
	case m.known && g.screen != m.current:
		return geometry{}, CodeStaleScreenshot
	}
	return g, ""
}

// ToScreen maps the pixel (x, y) of the screenshot id into screen space: x times the width of the
// screen per image pixel plus the origin, rounded to the nearest pixel of the screen (1/scale of a
// unit) and to two decimals, and kept inside the screen. It returns invalid_action for an unknown
// id or a pixel outside the image and stale_screenshot for an outdated geometry (CU-T11, CU-T12).
func (m *ScaleMap) ToScreen(id, x, y int64) (XY, string) {
	g, code := m.lookup(id)
	if code != "" {
		return XY{}, code
	}
	if x < 0 || y < 0 || x >= g.imgW || y >= g.imgH {
		return XY{}, CodeInvalidAction
	}
	return XY{X: g.axis(x, g.imgW, g.screen.X, g.screen.Width), Y: g.axis(y, g.imgH, g.screen.Y, g.screen.Height)}, ""
}

// axis maps one coordinate of an image of n pixels onto the screen interval from origin with size.
func (g geometry) axis(v, n int64, origin, size float64) float64 {
	scale := g.screen.Scale
	if scale <= 0 {
		scale = 1
	}
	p := origin + float64(v)*size/float64(n)
	p = math.Round(p*scale) / scale
	p = math.Round(p*100) / 100
	hi := origin + size - 1/scale
	return math.Max(origin, math.Min(p, hi))
}

// Region maps the zoom region [x0, y0, x1, y1] of the screenshot id into a rectangle in screen
// space; x1 and y1 may equal the image size. An empty or reversed region or one outside the image
// is invalid_action.
func (m *ScaleMap) Region(id, x0, y0, x1, y1 int64) (Rect, string) {
	g, code := m.lookup(id)
	if code != "" {
		return Rect{}, code
	}
	if x0 < 0 || y0 < 0 || x1 > g.imgW || y1 > g.imgH || x1 <= x0 || y1 <= y0 {
		return Rect{}, CodeInvalidAction
	}
	a := XY{X: g.axis(x0, g.imgW, g.screen.X, g.screen.Width), Y: g.axis(y0, g.imgH, g.screen.Y, g.screen.Height)}
	bx := g.edge(x1, g.imgW, g.screen.X, g.screen.Width)
	by := g.edge(y1, g.imgH, g.screen.Y, g.screen.Height)
	return Rect{X: a.X, Y: a.Y, Width: round2(bx - a.X), Height: round2(by - a.Y)}, ""
}

// edge maps the far edge of a region, which may be the edge of the image itself.
func (g geometry) edge(v, n int64, origin, size float64) float64 {
	if v >= n {
		return origin + size
	}
	scale := g.screen.Scale
	if scale <= 0 {
		scale = 1
	}
	p := origin + float64(v)*size/float64(n)
	return math.Round(math.Round(p*scale)/scale*100) / 100
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ToImage maps a point in screen space into pixels of the screenshot id, kept inside the image (the
// pointer may rest on another screen, which this stage does not control); false when the id is
// unknown or outdated.
func (m *ScaleMap) ToImage(id int64, p XY) (frame.Point, bool) {
	g, code := m.lookup(id)
	if code != "" || g.screen.Width <= 0 || g.screen.Height <= 0 {
		return frame.Point{}, false
	}
	x := int64(math.Round((p.X - g.screen.X) * float64(g.imgW) / g.screen.Width))
	y := int64(math.Round((p.Y - g.screen.Y) * float64(g.imgH) / g.screen.Height))
	return frame.Point{X: max(0, min(x, g.imgW-1)), Y: max(0, min(y, g.imgH-1))}, true
}
