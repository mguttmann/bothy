// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/host/internal/noise"
)

// Server is one bothy-computer-mcp process: the tools of 27 7 over MCP on stdio. It holds at
// most one session. Serve handles one request at a time, so the tool calls of a turn run in
// order and the session needs no lock.
type Server struct {
	cfg  Config
	req  *requestClient
	pins *pinStore
	log  *slog.Logger

	sess *session
	// initialized is set by initialize of the legacy protocol era.
	initialized bool
}

// New returns a server for cfg. Every field of ConfigFromEnv is required; the error wraps
// errConfig (IsConfigError).
func New(cfg Config) (*Server, error) {
	if cfg.Dir == "" || cfg.Nonce == "" || cfg.Timeout <= 0 || cfg.PinsPath == "" || cfg.Server == nil {
		return nil, fmt.Errorf("%w: incomplete", errConfig)
	}
	if cfg.Random == nil {
		cfg.Random = rand.Reader
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Server{cfg: cfg, log: cfg.Log,
		req:  &requestClient{dir: cfg.Dir, nonce: cfg.Nonce, timeout: cfg.Timeout, random: cfg.Random},
		pins: &pinStore{path: cfg.PinsPath, random: cfg.Random}}, nil
}

// The log line of a tool call: the tool, the code and the duration only (27 10.5).
const (
	msgTool        = "computer tool"
	attrTool       = "tool"
	attrCode       = "code"
	attrDurationMS = "duration_ms"
)

// callResult is the outcome of a tool call before it becomes MCP content: one text block and at
// most one image.
type callResult struct {
	text    string
	img     []byte
	mime    string
	isError bool
}

// failure returns the result of a code with its sentence and an optional detail.
func failure(code, detail string) callResult {
	text := code + ": " + codeHelp[code]
	if detail != "" {
		text += " (" + detail + ")"
	}
	return callResult{text: text, isError: true}
}

// callTool runs the tool name with its arguments and logs the kind, the code and the duration.
func (s *Server) callTool(ctx context.Context, name string, args json.RawMessage) callResult {
	start := time.Now()
	res := s.dispatch(ctx, name, args)
	code := codeOK
	if res.isError {
		code, _, _ = strings.Cut(res.text, ":")
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, msgTool, slog.String(attrTool, name), slog.String(attrCode, code),
		slog.Int64(attrDurationMS, time.Since(start).Milliseconds()))
	return res
}

func (s *Server) dispatch(ctx context.Context, name string, args json.RawMessage) callResult {
	switch name {
	case toolListDesktops:
		if _, _, err := parseNoArgs(args); err != nil {
			return failure(CodeInvalidAction, err.Error())
		}
		return s.listDesktops(ctx)
	case toolOpenSession:
		return s.openSession(ctx, args)
	case toolDone:
		if _, _, err := parseNoArgs(args); err != nil {
			return failure(CodeInvalidAction, err.Error())
		}
		return s.done(ctx)
	case toolBatch:
		return s.batch(ctx, args)
	}
	return s.single(ctx, name, args)
}

// parseNoArgs accepts an empty object.
func parseNoArgs(raw json.RawMessage) (map[string]any, actionArgs, error) {
	return parseAction(actScreenshot, raw)
}

// listDesktops asks the runner for the desktops (27 10.2).
func (s *Server) listDesktops(ctx context.Context) callResult {
	items, ref, err := s.req.list(ctx)
	if r, ok := requestFailure(ref, err); ok {
		return r
	}
	b, _ := json.Marshal(map[string]any{"desktops": items})
	return callResult{text: string(b)}
}

// requestFailure maps a refusal or an error of the request client.
func requestFailure(ref *Refusal, err error) (callResult, bool) {
	switch {
	case ref != nil:
		return failure(ref.ToolCode(), "refused with "+ref.Code), true
	case errors.Is(err, errTimeout):
		return failure(CodeHostOffline, "the runner did not answer"), true
	case err != nil:
		return failure(CodeHostOffline, "the runner's answer was unusable"), true
	}
	return callResult{}, false
}

// openSession resolves the desktop, asks the runner to open with the hash of a fresh ephemeral
// key, checks the pin and attaches (27 5.1 steps 3 to 9, 10.4).
func (s *Server) openSession(ctx context.Context, raw json.RawMessage) callResult {
	var a struct {
		Desktop string `json:"desktop"`
	}
	if err := strictDecode(raw, &a); err != nil || a.Desktop == "" {
		return failure(CodeInvalidAction, "open_session needs desktop, a name or a host ID")
	}
	if s.sess != nil && s.sess.ended == nil {
		return failure(CodeBusy, "this run already controls "+s.sess.desktop+"; call done first")
	}
	hostID, name, res, ok := s.resolve(ctx, a.Desktop)
	if !ok {
		return res
	}
	eph, err := noise.GenerateKeyPair(s.cfg.Random)
	if err != nil {
		return failure(CodeHostOffline, "no ephemeral key")
	}
	defer clear(eph.Private[:])
	cek, _ := noise.EphemeralKeyHash(eph.Public[:])
	pending, err := s.pins.pending(hostID)
	if err != nil {
		return failure(CodeHostKeyMismatch, "the pin file cannot be read")
	}
	grant, ref, err := s.req.open(ctx, hostID, cek, pending)
	if r, ok := requestFailure(ref, err); ok {
		return r
	}
	key, err := grant.hostKey()
	if err != nil {
		return failure(CodeHostOffline, "the session grant has no valid key")
	}
	switch v, err := s.pins.check(hostID, key, grant.KeyGeneration, pending); {
	case err != nil:
		return failure(CodeHostKeyMismatch, "the pin file cannot be read or written")
	case v == pinMismatch:
		return failure(CodeHostKeyMismatch, "")
	}
	sess, err := attach(ctx, attachParams{grant: grant, hostID: hostID, desktop: name, hostKey: key, ephemeral: eph, cfg: s.cfg})
	if err != nil {
		return attachFailure(err)
	}
	s.sess = sess
	text := fmt.Sprintf("Computer session open on %s (%s), state %s.", name, sess.platform, sess.state)
	if sess.state == "paused" {
		return callResult{text: text + " The screen is locked: actions answer screen_locked until the owner unlocks it. Take a screenshot to see the screen after the unlock."}
	}
	return s.firstImage(ctx, sess, text)
}

// firstImage takes the first screenshot of an active session, so the agent sees the screen
// without an extra call (27 7). The session stays open when the screenshot fails: the result
// names the code and asks for a screenshot.
func (s *Server) firstImage(ctx context.Context, sess *session, text string) callResult {
	r, err := sess.do(ctx, actScreenshot, json.RawMessage(`{}`), false)
	switch {
	case err != nil:
		return s.sessionFailure(err)
	case r.code != codeOK || r.img == nil:
		clear(r.img)
		return withState(sess, callResult{text: text + " The first screenshot failed: " + failure(r.code, "").text + "; take a screenshot."})
	}
	out := render(actScreenshot, r, nil)
	out.text = text + "\n" + out.text
	return withState(sess, out)
}

// attachFailure maps an error of attach.
func attachFailure(err error) callResult {
	var ref *attachRefusedError
	var ended *endedError
	switch {
	case errors.As(err, &ref):
		return failure(ref.toolCode(), "the session could not start: "+ref.reason)
	case errors.As(err, &ended):
		return failure(CodeStopped, ended.reason)
	case errors.Is(err, errStream):
		return failure(CodeStopped, endStreamFault)
	}
	return failure(CodeHostOffline, "the attach connection failed")
}

// resolve returns the host ID and the name of desktop: a host ID is taken as it is, a name is
// looked up in the list (exact first, then ignoring case, and only when unique).
func (s *Server) resolve(ctx context.Context, desktop string) (string, string, callResult, bool) {
	if _, err := noise.ParseID(desktop); err == nil {
		return strings.ToLower(desktop), desktop, callResult{}, true
	}
	items, ref, err := s.req.list(ctx)
	if r, ok := requestFailure(ref, err); ok {
		return "", "", r, false
	}
	var exact, folded []Desktop
	for _, d := range items {
		if d.Name == desktop {
			exact = append(exact, d)
		} else if strings.EqualFold(d.Name, desktop) {
			folded = append(folded, d)
		}
	}
	match := exact
	if len(match) == 0 {
		match = folded
	}
	switch len(match) {
	case 0:
		return "", "", failure(CodeNotEnabled, "no desktop of that name; list_desktops names them"), false
	case 1:
		return match[0].HostID, match[0].Name, callResult{}, true
	}
	return "", "", failure(CodeInvalidAction, "several desktops have that name; use the host ID"), false
}

// done ends the session with close.
func (s *Server) done(ctx context.Context) callResult {
	if s.sess == nil || s.sess.ended != nil {
		return s.noSession()
	}
	dctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	s.sess.done(dctx)
	s.sess = nil
	return callResult{text: "done: the computer session ended."}
}

// noSession answers a call without a live session: stopped with the reason when the last
// session ended other than by done, no_session otherwise.
func (s *Server) noSession() callResult {
	if s.sess != nil && s.sess.ended != nil && s.sess.ended.reason != "done" {
		return failure(CodeStopped, s.sess.ended.reason)
	}
	return failure(CodeNoSession, "")
}

// live returns the session when it can take an action.
func (s *Server) live() (*session, callResult, bool) {
	if s.sess == nil || s.sess.ended != nil {
		return nil, s.noSession(), false
	}
	return s.sess, callResult{}, true
}

// step is one action ready to send.
type step struct {
	action string
	args   map[string]any
	parts  []string // the parts of a type text
}

// prepare checks one action and fills in the scale_id of the last image.
func (s *Server) prepare(sess *session, action string, raw json.RawMessage) (step, error) {
	args, a, err := parseAction(action, raw)
	if err != nil {
		return step{}, err
	}
	if usesScale(action) && a.ScaleID == nil {
		if sess.lastScale == 0 {
			return step{}, fmt.Errorf("%s needs a scale_id: take a screenshot first", action)
		}
		args[argScaleID] = sess.lastScale
	}
	st := step{action: action, args: args}
	if action == actType {
		st.parts = splitText(a.Text)
	}
	return st, nil
}

// single runs one action tool.
func (s *Server) single(ctx context.Context, name string, raw json.RawMessage) callResult {
	sess, res, ok := s.live()
	if !ok {
		return res
	}
	st, err := s.prepare(sess, name, raw)
	if err != nil {
		return failure(CodeInvalidAction, err.Error())
	}
	r, err := s.run(ctx, sess, st, false)
	if err != nil {
		return s.sessionFailure(err)
	}
	return withState(sess, render(name, r, nil))
}

// withState adds the line of a paused session (27 8.2): the screen is locked, and the next
// actions answer screen_locked until the owner unlocks it.
func withState(sess *session, r callResult) callResult {
	if sess.ended == nil && sess.state == "paused" {
		r.text += "\nstate: paused (the screen is locked)"
	}
	return r
}

// run sends one step: a type text as its parts in order, stopping at the first part that fails.
// The result is that of the last part sent. quick marks the step as a batch item before the last
// one; every part of a type text but the last is quick as well, so only the end of the text
// waits computer.settle and returns an image (27 6.4, 7).
func (s *Server) run(ctx context.Context, sess *session, st step, quick bool) (result, error) {
	if st.action != actType {
		b, _ := json.Marshal(st.args)
		return sess.do(ctx, st.action, b, quick)
	}
	var r result
	for i, p := range st.parts {
		clear(r.img)
		b, _ := json.Marshal(map[string]string{argText: p})
		var err error
		if r, err = sess.do(ctx, actType, b, quick || i < len(st.parts)-1); err != nil || r.code != codeOK {
			return r, err
		}
	}
	return r, nil
}

// sessionFailure maps an error of a running session: an end is stopped with its reason.
func (s *Server) sessionFailure(err error) callResult {
	if errors.Is(err, errActionTooLong) {
		return failure(CodeInvalidAction, "the action does not fit one frame")
	}
	var ended *endedError
	if errors.As(err, &ended) {
		return failure(CodeStopped, ended.reason)
	}
	return failure(CodeStopped, endConnectionLost)
}

// batch runs the actions in order; the first failure stops it, every later item answers
// notExecuted, and the last image comes once at the end (27 7). Every item before the last is
// sent with quick, so the host skips its settle wait and its image; only the last item waits
// computer.settle and returns the image. Every item is checked before the first runs, so a
// malformed batch runs nothing.
func (s *Server) batch(ctx context.Context, raw json.RawMessage) callResult {
	sess, res, ok := s.live()
	if !ok {
		return res
	}
	names, items, err := parseBatch(raw)
	if err != nil {
		return failure(CodeInvalidAction, err.Error())
	}
	var lines []string
	var last result
	failed := false
	for i, name := range names {
		if failed {
			lines = append(lines, fmt.Sprintf("%d. %s: %s", i+1, name, notExecuted))
			continue
		}
		r, line, ok := s.batchItem(ctx, sess, name, items[i], i < len(names)-1)
		failed = !ok
		lines = append(lines, fmt.Sprintf("%d. %s: %s", i+1, name, line))
		if r.img != nil {
			clear(last.img)
			last = r
		} else if last.img == nil {
			last.app = r.app
		}
	}
	out := withState(sess, render(toolBatch, last, lines))
	out.isError = failed
	return out
}

// parseBatch returns the action names and the arguments of the items, each checked against
// 27 6.4.
func parseBatch(raw json.RawMessage) ([]string, []json.RawMessage, error) {
	var b struct {
		Actions []json.RawMessage `json:"actions"`
	}
	if err := strictDecode(raw, &b); err != nil || len(b.Actions) == 0 {
		return nil, nil, errors.New("batch needs actions, a list of at least one action")
	}
	names := make([]string, len(b.Actions))
	items := make([]json.RawMessage, len(b.Actions))
	for i, it := range b.Actions {
		var m map[string]json.RawMessage
		if json.Unmarshal(it, &m) != nil || m == nil || json.Unmarshal(m[argAction], &names[i]) != nil || !isAction(names[i]) {
			return nil, nil, fmt.Errorf("item %d of batch has no known action", i+1)
		}
		delete(m, argAction)
		items[i], _ = json.Marshal(m)
		if _, _, err := parseAction(names[i], items[i]); err != nil {
			return nil, nil, fmt.Errorf("item %d: %w", i+1, err)
		}
	}
	return names, items, nil
}

// batchItem runs one item of a batch and returns its result, its line and whether it succeeded;
// quick marks an item before the last one.
func (s *Server) batchItem(ctx context.Context, sess *session, name string, raw json.RawMessage, quick bool) (result, string, bool) {
	st, err := s.prepare(sess, name, raw)
	if err != nil {
		return result{}, failure(CodeInvalidAction, err.Error()).text, false
	}
	r, err := s.run(ctx, sess, st, quick)
	switch {
	case err != nil:
		return result{}, s.sessionFailure(err).text, false
	case r.code != codeOK:
		return r, failure(r.code, "").text, false
	}
	return r, "ok" + cursorText(r), true
}

// isAction reports whether name is one of the thirteen actions.
func isAction(name string) bool {
	_, ok := argRules[name]
	return ok
}

// cursorText is the pointer of a cursor_position result.
func cursorText(r result) string {
	if r.cursor == nil {
		return ""
	}
	return fmt.Sprintf(", cursor x %d, y %d in scale_id %d", r.cursor.X, r.cursor.Y, r.scaleID)
}

// render builds the text block and the image of a result: the code line (or the lines of a
// batch), the image's width, height and scale_id, the frontmost app, and the data line.
func render(name string, r result, lines []string) callResult {
	var b strings.Builder
	out := callResult{}
	switch {
	case lines != nil:
		b.WriteString(strings.Join(lines, "\n"))
	case r.code == codeOK:
		b.WriteString(name + ": ok" + cursorText(r))
	default:
		b.WriteString(failure(r.code, "").text)
		out.isError = true
	}
	if r.img != nil {
		fmt.Fprintf(&b, "\nimage: width %d, height %d, scale_id %d", r.width, r.height, r.scaleID)
		out.img, out.mime = r.img, "image/jpeg"
		if r.format == "png" {
			out.mime = "image/png"
		}
	}
	if r.app.ID != "" || r.app.Name != "" {
		fmt.Fprintf(&b, "\nfrontmost app: %s (%s)", r.app.Name, r.app.ID)
	}
	b.WriteString("\n" + dataLine)
	out.text = b.String()
	return out
}

// closeSession ends the reader and the connection of a live session without close; the server
// sees the end of the connection (27 8.3 connection_lost).
func (s *Server) closeSession() {
	if s.sess != nil {
		s.sess.close()
	}
}
