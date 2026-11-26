// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// Actions of docs/spec/27-computer-use.md 6.4.
const (
	actScreenshot     = "screenshot"
	actZoom           = "zoom"
	actLeftClick      = "left_click"
	actRightClick     = "right_click"
	actMiddleClick    = "middle_click"
	actDoubleClick    = "double_click"
	actTripleClick    = "triple_click"
	actMouseMove      = "mouse_move"
	actCursorPosition = "cursor_position"
	actScroll         = "scroll"
	actType           = "type"
	actKey            = "key"
	actWait           = "wait"
)

// Members of args that several actions share.
const (
	argCoordinate = "coordinate"
	argScaleID    = "scale_id"
	argModifiers  = "modifiers"
	argRegion     = "region"
)

// allowedArgs are the members of args per action; scale_id belongs to every action with a
// coordinate or a region (27 6.2: all coordinates are pixels of the screenshot named by scale_id).
var allowedArgs = map[string][]string{
	actScreenshot:     {},
	actZoom:           {argRegion, argScaleID},
	actLeftClick:      {argCoordinate, argScaleID, argModifiers},
	actRightClick:     {argCoordinate, argScaleID, argModifiers},
	actMiddleClick:    {argCoordinate, argScaleID, argModifiers},
	actDoubleClick:    {argCoordinate, argScaleID, argModifiers},
	actTripleClick:    {argCoordinate, argScaleID, argModifiers},
	actMouseMove:      {argCoordinate, argScaleID},
	actCursorPosition: {},
	actScroll:         {argCoordinate, argScaleID, "direction", "amount"},
	actType:           {"text"},
	actKey:            {"keys", "repeat"},
	actWait:           {"duration_ms"},
}

// requiredArgs are the members each action needs.
var requiredArgs = map[string][]string{
	actZoom:        {argRegion, argScaleID},
	actLeftClick:   {argCoordinate, argScaleID},
	actRightClick:  {argCoordinate, argScaleID},
	actMiddleClick: {argCoordinate, argScaleID},
	actDoubleClick: {argCoordinate, argScaleID},
	actTripleClick: {argCoordinate, argScaleID},
	actMouseMove:   {argCoordinate, argScaleID},
	actScroll:      {argCoordinate, argScaleID, "direction", "amount"},
	actType:        {"text"},
	actKey:         {"keys"},
	actWait:        {"duration_ms"},
}

var (
	modifierNames = []string{"shift", "ctrl", "alt", "cmd", "win"}
	directions    = []string{"up", "down", "left", "right"}
)

// args are the arguments of an action.
type args struct {
	Coordinate []int64  `json:"coordinate"`
	Region     []int64  `json:"region"`
	ScaleID    int64    `json:"scale_id"`
	Modifiers  []string `json:"modifiers"`
	Direction  string   `json:"direction"`
	Amount     int64    `json:"amount"`
	Text       string   `json:"text"`
	Keys       string   `json:"keys"`
	Repeat     *int64   `json:"repeat"`
	DurationMs int64    `json:"duration_ms"`
}

// parseArgs decodes and checks the arguments of action; false means invalid_action.
func (s *csession) parseArgs(action string, raw json.RawMessage) (args, bool) {
	m, ok := argMembers(action, raw)
	if !ok {
		return args{}, false
	}
	var a args
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&a) != nil {
		return args{}, false
	}
	_, hasCoord := m[argCoordinate]
	_, hasRegion := m[argRegion]
	if hasCoord && len(a.Coordinate) != 2 || hasRegion && len(a.Region) != 4 {
		return args{}, false
	}
	return a, validArgs(action, a, s.o.Settings.KeyRepeatMax, s.o.Settings.WaitMaxMs)
}

// argMembers returns the members of raw when action is known, every member is allowed for it,
// none is null and every required one is present.
func argMembers(action string, raw json.RawMessage) (map[string]json.RawMessage, bool) {
	allowed, known := allowedArgs[action]
	var m map[string]json.RawMessage
	if !known || json.Unmarshal(raw, &m) != nil {
		return nil, false
	}
	for k, v := range m {
		if !slices.Contains(allowed, k) || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return nil, false
		}
	}
	for _, k := range requiredArgs[action] {
		if _, ok := m[k]; !ok {
			return nil, false
		}
	}
	return m, true
}

// validArgs applies the value rules of 27 6.4: known modifiers, a direction and an amount of at
// least 1 for scroll, a text for type, keys and a repeat up to computer.key_repeat_max for key,
// and a duration up to computer.wait_max for wait.
func validArgs(action string, a args, repeatMax, waitMax int64) bool {
	if slices.ContainsFunc(a.Modifiers, func(x string) bool { return !slices.Contains(modifierNames, x) }) {
		return false
	}
	switch action {
	case actScroll:
		return slices.Contains(directions, a.Direction) && a.Amount >= 1
	case actType:
		return a.Text != ""
	case actKey:
		return a.Keys != "" && (a.Repeat == nil || (*a.Repeat >= 1 && *a.Repeat <= repeatMax))
	case actWait:
		return a.DurationMs >= 0 && a.DurationMs <= waitMax
	}
	return true
}

// outcome is the result of one action before it is sent.
type outcome struct {
	code    string
	img     []byte
	format  int
	width   int64
	height  int64
	scaleID int64
	cursor  *frame.Point
}

// act runs one action and sends its result (27 6.4); the helper is asked only after every check
// passed, so a refused action injects nothing.
func (s *csession) act(a frame.Action) {
	s.count(a.Action)
	defer s.reportState(false)
	if s.paused() {
		s.sendResult(frame.Result{Seq: a.Seq, Code: CodeScreenLocked, App: s.frontApp()}, nil, 0)
		return
	}
	o := s.run1(a)
	if s.ctx.Err() != nil {
		clear(o.img)
		return
	}
	s.sendResult(frame.Result{Seq: a.Seq, Code: o.code, Width: o.width, Height: o.height, ScaleID: o.scaleID, App: s.frontApp(), Cursor: o.cursor},
		o.img, o.format)
}

// run1 executes a and returns its outcome.
func (s *csession) run1(a frame.Action) outcome {
	ar, ok := s.parseArgs(a.Action, a.Args)
	if !ok {
		return outcome{code: CodeInvalidAction}
	}
	switch a.Action {
	case actScreenshot:
		return s.capture(nil, -1)
	case actZoom:
		r, code := s.scale.Region(ar.ScaleID, ar.Region[0], ar.Region[1], ar.Region[2], ar.Region[3])
		if code != "" {
			return outcome{code: code}
		}
		return s.capture(&r, ar.ScaleID)
	case actCursorPosition:
		return s.position()
	case actWait:
		if !s.sleep(time.Duration(ar.DurationMs) * time.Millisecond) {
			return outcome{code: CodeOK}
		}
		return s.capture(nil, -1)
	}
	return s.input(a.Action, ar)
}

// input runs an action that injects input (do), then the automatic image after computer.settle
// when computer.auto_screenshot is on.
func (s *csession) input(action string, ar args) outcome {
	d := Do{Op: OpDo, Action: action, Modifiers: ar.Modifiers, Direction: ar.Direction, Amount: ar.Amount, Text: ar.Text, Keys: ar.Keys}
	if ar.Repeat != nil {
		d.Repeat = *ar.Repeat
	}
	if ar.Coordinate != nil {
		p, code := s.scale.ToScreen(ar.ScaleID, ar.Coordinate[0], ar.Coordinate[1])
		if code != "" {
			return outcome{code: code}
		}
		d.X, d.Y = &p.X, &p.Y
	}
	if s.paused() {
		// A lock that arrived after the action was taken from the queue: nothing is injected
		// (Design note 4; the helper checks the lock before every injection as well).
		return outcome{code: CodeScreenLocked}
	}
	d.Seq = s.m.c.Helper.NextSeq()
	r, err := s.m.c.Helper.Request(s.ctx, d.Seq, d)
	if err != nil {
		return outcome{code: CodeOSRefused}
	}
	s.observe(r.Done)
	if r.Done.Code != CodeOK || !s.o.Settings.AutoScreenshot {
		return outcome{code: r.Done.Code}
	}
	if !s.sleep(time.Duration(s.o.Settings.SettleMs) * time.Millisecond) {
		return outcome{code: CodeOK}
	}
	if o := s.capture(nil, -1); o.code == CodeOK {
		return o
	}
	return outcome{code: CodeOK}
}

// observe records the screen and app of a done.
func (s *csession) observe(d Done) {
	if d.Screen.Width > 0 && d.Screen.Height > 0 {
		s.scale.Observe(d.Screen)
	}
	s.seeApp(d.App)
}

// capture asks the helper for a screenshot (JPEG) or, with region, a zoom (PNG); a screenshot gets
// the scale ID of its geometry, a zoom keeps zoomID, the screenshot space of its region (27 6.4).
func (s *csession) capture(region *Rect, zoomID int64) outcome {
	set := s.o.Settings
	c := Capture{Op: OpCapture, MaxEdge: set.ImageMaxEdge, Format: frame.FormatJPEG, Quality: set.JPEGQuality, Region: region}
	format := formatJPEG
	if region != nil {
		c.Format, c.Quality, format = frame.FormatPNG, 100, formatPNG
	}
	c.Seq = s.m.c.Helper.NextSeq()
	r, err := s.m.c.Helper.Request(s.ctx, c.Seq, c)
	if err != nil {
		return outcome{code: CodeOSRefused}
	}
	s.observe(r.Done)
	if r.Done.Code != CodeOK || r.Done.Image == nil || len(r.Image) == 0 || r.Done.Image.Width <= 0 || r.Done.Image.Height <= 0 ||
		r.Done.Image.Format != c.Format {
		clear(r.Image)
		code := r.Done.Code
		if code == CodeOK {
			code = CodeOSRefused
		}
		return outcome{code: code}
	}
	id := zoomID
	if region == nil {
		id = s.scale.Assign(r.Done.Screen, r.Done.Image.Width, r.Done.Image.Height)
	}
	return outcome{code: CodeOK, img: r.Image, format: format, width: r.Done.Image.Width, height: r.Done.Image.Height, scaleID: id}
}

// position answers cursor_position from the helper's pointer position in pixels of the current
// screenshot geometry; without one it takes a capture first to learn it and drops the image.
func (s *csession) position() outcome {
	seq := s.m.c.Helper.NextSeq()
	r, err := s.m.c.Helper.Request(s.ctx, seq, Position{Op: OpPosition, Seq: seq})
	if err != nil {
		return outcome{code: CodeOSRefused}
	}
	s.observe(r.Done)
	if r.Done.Code != CodeOK || r.Done.Cursor == nil {
		return outcome{code: r.Done.Code}
	}
	id := s.scale.Current()
	if id == 0 {
		o := s.capture(nil, -1)
		clear(o.img)
		if o.code != CodeOK {
			return outcome{code: o.code}
		}
		id = o.scaleID
	}
	p, ok := s.scale.ToImage(id, *r.Done.Cursor)
	if !ok {
		return outcome{code: CodeStaleScreenshot}
	}
	return outcome{code: CodeOK, scaleID: id, cursor: &p}
}

// sleep waits d unless the session ends; false when it ended.
func (s *csession) sleep(d time.Duration) bool {
	if d <= 0 {
		return s.ctx.Err() == nil
	}
	t := s.m.c.Clock.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C():
		return true
	case <-s.ctx.Done():
		return false
	}
}
