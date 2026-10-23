// SPDX-License-Identifier: Apache-2.0

package domain

// The dynamic part of a live-box patch (M2-10): Apply applies the checked operations of ParseOps
// to the locked state of the run, op by op, and returns what to store (Change) and what to send
// to clients (the normalized RFC 6902 operations of Design note 4).

import (
	"bytes"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Default values of a new step (09 4.7).
const (
	stepStatusDefault = "running"
	emptyObject       = "{}"
)

// The fixed details of the dynamic failures; like the static ones they never repeat a value.
const (
	detailPatchFailed   = "the patch does not apply to the run"
	detailTargetExists  = "the target already exists"
	detailTargetMissing = "the target does not exist"
	detailLimitReached  = "the run has more steps than it may keep and none of them can be summarized"
)

// workStep is one step during Apply: its current values, its values at base_version and how the
// patch touched it.
type workStep struct {
	DocStep
	// orig holds the values at base_version of a step that existed there.
	orig DocStep
	// base reports that the step existed at base_version; added that the adapter added it in this
	// patch; deleted that the summarization removed it.
	base, added, deleted bool
	// detailSet reports that the patch replaced the detail.
	detailSet bool
}

// applier holds the intermediate state of one Apply.
type applier struct {
	lim  Limits
	env  Env
	base DocumentState

	statusLine, excerpt, model string
	metrics                    map[string]json.RawMessage
	basePlan, plan             []PlanItem
	cost                       Money

	// steps are all steps in ordinal order (base steps first, then the added ones, whose
	// ordinals are higher); a new summary step is appended last.
	steps     []*workStep
	byID      map[ids.ID]*workStep
	added     []*workStep
	firstAdd  int
	followups map[ids.ID]string
	writes    []FollowupWrite
	norm      []NormOp
}

// Apply applies the checked operations to a copy of state, in order, and never changes state
// itself. Each operation sets its member with set semantics (add and replace set the member also
// when it is absent; remove of an absent member does nothing). A step add needs a free id and a
// known parent; a step member replace and a follow-up replace need an existing target. After the
// step adds, the summarization of steps_summary.go keeps at most lim.MaxStepsPersisted
// (run.max_steps_persisted) steps. Excerpts are cut to lim.ExcerptMaxChars
// (run.excerpt_max_chars) and details to lim.StepDetailMaxBytes (run.step_detail_max_bytes)
// before they are stored or sent.
//
// It returns the Change to store and the normalized operations: one per effective change,
// computed against the state just before the operation, with the stored value encoded as the run
// document encodes it; no-ops are dropped. Applied in order to the document at base_version they
// give the document at the new version.
//
// Errors: the first dynamic failure returns an *errs.Error with code validation_failed and one
// field error naming the operation (patch_target_exists, patch_target_missing, limit_reached),
// and a zero Change: the caller rolls back. A stored metrics or plan object that is not JSON is
// an internal_error.
//
// Example:
//
//	ops, fieldErrs := domain.ParseOps(raw)
//	if len(fieldErrs) > 0 { ... }
//	ch, norm, err := domain.Apply(state, ops, limits, domain.Env{Now: now, NewID: ids.New})
func Apply(state DocumentState, ops []Op, lim Limits, env Env) (Change, []NormOp, error) {
	a, err := newApplier(state, lim, env)
	if err != nil {
		return Change{}, nil, err
	}
	for _, op := range ops {
		if err := a.apply(op); err != nil {
			return Change{}, nil, err
		}
	}
	if a.firstAdd >= 0 {
		if err := a.summarize(); err != nil {
			return Change{}, nil, err
		}
	}
	return a.change(), a.norm, nil
}

// newApplier copies state into an applier; the copies keep Apply from changing its input.
func newApplier(state DocumentState, lim Limits, env Env) (*applier, error) {
	a := &applier{
		lim: lim, env: env, base: state,
		statusLine: state.StatusLine, excerpt: state.LastTextExcerpt, model: state.Model,
		cost:      state.Cost,
		metrics:   map[string]json.RawMessage{},
		byID:      make(map[ids.ID]*workStep, len(state.Steps)),
		firstAdd:  -1,
		followups: make(map[ids.ID]string, len(state.Followups)),
	}
	if len(bytes.TrimSpace(state.Metrics)) > 0 {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(state.Metrics, &m); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, err, "stored run metrics are not a JSON object")
		}
		for k, v := range m {
			a.metrics[k] = v
		}
	}
	plan, err := decodeStoredPlan(state.Plan)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "stored run plan is not a JSON object")
	}
	a.basePlan, a.plan = plan, plan
	for _, s := range state.Steps {
		ws := &workStep{DocStep: s, orig: s, base: true}
		a.steps = append(a.steps, ws)
		a.byID[s.ID] = ws
	}
	for id, st := range state.Followups {
		a.followups[id] = st
	}
	return a, nil
}

// planItemJSON is one item of run_plan.v1 as stored and sent.
type planItemJSON struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

// planJSON is run_plan.v1 as stored and sent.
type planJSON struct {
	Items []planItemJSON `json:"items"`
}

// decodeStoredPlan reads the items of the stored object runs.plan; "{}" and an empty column
// have none.
func decodeStoredPlan(raw json.RawMessage) ([]PlanItem, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var p planJSON
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	items := make([]PlanItem, 0, len(p.Items))
	for _, it := range p.Items {
		items = append(items, PlanItem(it))
	}
	return items, nil
}

// encodePlan encodes items as the member plan of the document.
func encodePlan(items []PlanItem) json.RawMessage {
	p := planJSON{Items: make([]planItemJSON, 0, len(items))}
	for _, it := range items {
		p.Items = append(p.Items, planItemJSON(it))
	}
	return encodeCanonical(p)
}

// apiRetryJSON is RunApiRetry as stored and sent; absent members are left out.
type apiRetryJSON struct {
	Attempt      int64   `json:"attempt"`
	MaxRetries   *int64  `json:"max_retries,omitempty"`
	RetryDelayMS *int64  `json:"retry_delay_ms,omitempty"`
	Error        *string `json:"error,omitempty"`
}

// moneyJSON is Money as stored and sent.
type moneyJSON struct {
	AmountMicros int64  `json:"amount_micros"`
	Currency     string `json:"currency"`
	IsEstimate   bool   `json:"is_estimate"`
}

// runStepJSON is RunStep as the run document sends it.
type runStepJSON struct {
	ID           string          `json:"id"`
	Ordinal      int             `json:"ordinal"`
	ParentStepID string          `json:"parent_step_id,omitempty"`
	Kind         string          `json:"kind"`
	Name         string          `json:"name"`
	Title        string          `json:"title"`
	Status       string          `json:"status"`
	Detail       json.RawMessage `json:"detail"`
	StartedAt    string          `json:"started_at"`
	FinishedAt   string          `json:"finished_at,omitempty"`
}

// encodeStep encodes s as the run document sends a step.
func encodeStep(s DocStep) json.RawMessage {
	j := runStepJSON{
		ID: s.ID.String(), Ordinal: s.Ordinal, Kind: s.Kind, Name: s.Name, Title: s.Title,
		Status: s.Status, Detail: s.Detail, StartedAt: docTime(s.StartedAt),
	}
	if j.Detail == nil {
		j.Detail = json.RawMessage(emptyObject)
	}
	if !s.ParentStepID.IsZero() {
		j.ParentStepID = s.ParentStepID.String()
	}
	if !s.FinishedAt.IsZero() {
		j.FinishedAt = docTime(s.FinishedAt)
	}
	return encodeCanonical(j)
}

// docTime formats t as getRun answers it (time.Time as JSON after the cut to milliseconds), so
// normalized ops applied to a snapshot give exactly the document of getRun.
func docTime(t time.Time) string { return t.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano) }

// storedTime is t as the database keeps it: UTC, microseconds.
func storedTime(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }

// emit appends one normalized operation.
func (a *applier) emit(op OpKind, path string, value json.RawMessage) {
	a.norm = append(a.norm, NormOp{Op: op, Path: path, Value: value})
}

// fail is the error of a dynamic check: validation_failed with one field error at the operation
// (suffix "" ) or below it.
func fail(index int, suffix, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detailPatchFailed).WithFields(errs.FieldError{
		Pointer: "/ops/" + strconv.Itoa(index) + suffix, Code: code, Detail: detail,
	})
}

// apply applies one operation.
func (a *applier) apply(op Op) error {
	switch op.Target {
	case TargetStatusLine:
		a.setText(&a.statusLine, "/status_line", op)
	case TargetLastTextExcerpt:
		if s, ok := op.Value.(string); ok {
			op.Value = CutExcerpt(s, a.lim.ExcerptMaxChars)
		}
		a.setText(&a.excerpt, "/last_text_excerpt", op)
	case TargetModel:
		a.setText(&a.model, "/model", op)
	case TargetMetric:
		var raw json.RawMessage
		switch v := op.Value.(type) {
		case int64:
			raw = encodeCanonical(v)
		case float64:
			raw = encodeCanonical(v)
		}
		a.setMetric(op.Field, raw)
	case TargetLastAPIRetry:
		var raw json.RawMessage
		if r, ok := op.Value.(APIRetry); ok && op.Kind != OpRemove {
			raw = encodeCanonical(apiRetryJSON(r))
		}
		a.setMetric("last_api_retry", raw)
	case TargetCost:
		if m, ok := op.Value.(Money); ok && m != a.cost {
			a.emit(OpReplace, "/cost", encodeCanonical(moneyJSON(m)))
			a.cost = m
		}
	case TargetPlan:
		var items []PlanItem
		if p, ok := op.Value.(Plan); ok && op.Kind != OpRemove {
			items = p.Items
		}
		a.setPlan(items)
	case TargetStep:
		return a.addStep(op)
	case TargetStepMember:
		return a.setStepMember(op)
	case TargetFollowupState:
		return a.setFollowup(op)
	}
	return nil
}

// setText sets a text column; "" is NULL, so setting "" removes the member.
func (a *applier) setText(cur *string, path string, op Op) {
	v := ""
	if op.Kind != OpRemove {
		v, _ = op.Value.(string)
	}
	switch {
	case v == *cur:
		return
	case v == "":
		a.emit(OpRemove, path, nil)
	case *cur == "":
		a.emit(OpAdd, path, encodeCanonical(v))
	default:
		a.emit(OpReplace, path, encodeCanonical(v))
	}
	*cur = v
}

// setMetric sets the metrics member key to raw, or removes it for nil.
func (a *applier) setMetric(key string, raw json.RawMessage) {
	path := "/metrics/" + escapeToken(key)
	old, present := a.metrics[key]
	switch {
	case raw == nil:
		if !present {
			return
		}
		delete(a.metrics, key)
		a.emit(OpRemove, path, nil)
		return
	case !present:
		a.emit(OpAdd, path, raw)
	case bytes.Equal(canonicalOf(old), canonicalOf(raw)):
		return
	default:
		a.emit(OpReplace, path, raw)
	}
	a.metrics[key] = raw
}

// setPlan sets the plan; a plan without items is absent from the document.
func (a *applier) setPlan(items []PlanItem) {
	exists := len(a.plan) > 0
	switch {
	case len(items) == 0:
		if exists {
			a.emit(OpRemove, "/plan", nil)
		}
		a.plan = nil
		return
	case !exists:
		a.emit(OpAdd, "/plan", encodePlan(items))
	case slices.Equal(a.plan, items):
		return
	default:
		a.emit(OpReplace, "/plan", encodePlan(items))
	}
	a.plan = slices.Clone(items)
}

// addStep adds a step of the adapter with the next ordinal.
func (a *applier) addStep(op Op) error {
	in, _ := op.Value.(StepInput)
	if _, exists := a.byID[op.ID]; exists {
		return fail(op.Index, "", fieldTargetExists, detailTargetExists)
	}
	if !in.ParentStepID.IsZero() {
		if _, exists := a.byID[in.ParentStepID]; !exists {
			return fail(op.Index, "/value/parent_step_id", fieldTargetMissing, detailTargetMissing)
		}
	}
	s := DocStep{
		ID: op.ID, Ordinal: a.maxOrdinal() + 1, ParentStepID: in.ParentStepID,
		Kind: in.Kind, Name: in.Name, Title: in.Title, Status: in.Status,
		StartedAt: storedTime(a.env.Now),
	}
	if s.Status == "" {
		s.Status = stepStatusDefault
	}
	detail := in.Detail
	if detail == nil {
		detail = json.RawMessage(emptyObject)
	}
	s.Detail = CutDetail(detail, a.lim.StepDetailMaxBytes)
	if !in.StartedAt.IsZero() {
		s.StartedAt = storedTime(in.StartedAt)
	}
	if !in.FinishedAt.IsZero() {
		s.FinishedAt = storedTime(in.FinishedAt)
	}
	ws := &workStep{DocStep: s, added: true}
	a.steps = append(a.steps, ws)
	a.byID[s.ID] = ws
	a.added = append(a.added, ws)
	if a.firstAdd < 0 {
		a.firstAdd = op.Index
	}
	a.emit(OpAdd, "/steps/"+s.ID.String(), encodeStep(s))
	return nil
}

// maxOrdinal returns the highest ordinal of the run so far, 0 without steps.
func (a *applier) maxOrdinal() int {
	m := 0
	for _, s := range a.steps {
		m = max(m, s.Ordinal)
	}
	return m
}

// setStepMember replaces status, title, detail or finished_at of an existing step.
func (a *applier) setStepMember(op Op) error {
	ws, ok := a.byID[op.ID]
	if !ok {
		return fail(op.Index, "", fieldTargetMissing, detailTargetMissing)
	}
	path := "/steps/" + op.ID.String() + "/" + op.Field
	switch op.Field {
	case memberStatus, memberTitle:
		cur := &ws.Status
		if op.Field == memberTitle {
			cur = &ws.Title
		}
		v, _ := op.Value.(string)
		if v == *cur {
			return nil
		}
		a.emit(OpReplace, path, encodeCanonical(v))
		*cur = v
	case memberDetail:
		raw, _ := op.Value.(json.RawMessage)
		d := CutDetail(raw, a.lim.StepDetailMaxBytes)
		// A detail that was not loaded (nil) is unknown and so always replaced.
		if ws.Detail != nil && bytes.Equal(canonicalOf(ws.Detail), d) {
			return nil
		}
		a.emit(OpReplace, path, d)
		ws.Detail, ws.detailSet = d, true
	case "finished_at":
		t, _ := op.Value.(time.Time)
		t = storedTime(t)
		switch {
		case ws.FinishedAt.IsZero():
			a.emit(OpAdd, path, encodeCanonical(docTime(t)))
		case ws.FinishedAt.Equal(t):
			return nil
		default:
			a.emit(OpReplace, path, encodeCanonical(docTime(t)))
		}
		ws.FinishedAt = t
	}
	return nil
}

// setFollowup stores the state of a follow-up of the run; whether the move is forward is
// M2-11's rule, not Apply's.
func (a *applier) setFollowup(op Op) error {
	cur, ok := a.followups[op.ID]
	if !ok {
		return fail(op.Index, "", fieldTargetMissing, detailTargetMissing)
	}
	v, _ := op.Value.(string)
	if v == cur {
		return nil
	}
	a.emit(OpReplace, "/followups/"+op.ID.String()+"/state", encodeCanonical(v))
	a.followups[op.ID] = v
	a.writes = append(a.writes, FollowupWrite{MessageID: op.ID, State: v})
	return nil
}

// encodeMetrics encodes the metrics object with sorted members.
func (a *applier) encodeMetrics() json.RawMessage {
	m := make(map[string]any, len(a.metrics))
	for k, v := range a.metrics {
		m[k] = v
	}
	return encodeCanonical(m)
}

// change assembles the Change from the final state.
func (a *applier) change() Change {
	var ch Change
	a.textColumns(&ch)
	after := DocumentState{
		StatusLine: a.statusLine, LastTextExcerpt: a.excerpt, Model: a.model,
		Metrics: a.base.Metrics, Plan: a.base.Plan, Cost: a.cost, Followups: a.followups,
	}
	baseMetrics := canonicalOf(a.base.Metrics)
	if baseMetrics == nil {
		baseMetrics = json.RawMessage(emptyObject)
	}
	if m := a.encodeMetrics(); !bytes.Equal(canonicalOf(m), baseMetrics) {
		ch.Metrics, after.Metrics = m, m
	}
	if !slices.Equal(a.plan, a.basePlan) {
		p := json.RawMessage(emptyObject)
		if len(a.plan) > 0 {
			p = encodePlan(a.plan)
		}
		ch.Plan, after.Plan = p, p
	}
	if a.cost != a.base.Cost {
		c := a.cost
		ch.Cost = &c
	}
	after.Steps = a.stepChanges(&ch)
	byID := func(x, y ids.ID) int { return bytes.Compare(x[:], y[:]) }
	slices.SortFunc(ch.DeleteSteps, byID)
	slices.SortFunc(ch.UpdateSteps, func(x, y StepUpdate) int { return byID(x.ID, y.ID) })
	byOrdinal := func(x, y DocStep) int { return x.Ordinal - y.Ordinal }
	slices.SortFunc(ch.InsertSteps, byOrdinal)
	slices.SortFunc(after.Steps, byOrdinal)
	for _, ws := range a.added {
		if !ws.deleted {
			ch.AddedSteps = append(ch.AddedSteps, ws.DocStep)
		}
	}
	ch.Followups = a.writes
	ch.After = after
	return ch
}

// textColumns sets the text columns of ch that differ from their values at base_version.
func (a *applier) textColumns(ch *Change) {
	for _, c := range []struct {
		col       *TextColumn
		now, base string
	}{
		{&ch.StatusLine, a.statusLine, a.base.StatusLine},
		{&ch.LastTextExcerpt, a.excerpt, a.base.LastTextExcerpt},
		{&ch.Model, a.model, a.base.Model},
	} {
		if c.now != c.base {
			*c.col = TextColumn{Set: true, Value: c.now}
		}
	}
}

// stepChanges adds the deletes, updates and inserts of the steps to ch, unsorted, and returns the
// steps that remain, in the order of a.steps.
func (a *applier) stepChanges(ch *Change) []DocStep {
	var kept []DocStep
	for _, ws := range a.steps {
		switch {
		case ws.deleted:
			if ws.base {
				ch.DeleteSteps = append(ch.DeleteSteps, ws.ID)
			}
			continue
		case ws.base:
			if u, changed := ws.update(); changed {
				ch.UpdateSteps = append(ch.UpdateSteps, u)
			}
		default:
			ch.InsertSteps = append(ch.InsertSteps, ws.DocStep)
		}
		kept = append(kept, ws.DocStep)
	}
	return kept
}

// update returns the change of a base step against its values at base_version.
func (ws *workStep) update() (StepUpdate, bool) {
	u := StepUpdate{ID: ws.ID}
	changed := false
	if ws.Status != ws.orig.Status {
		s := ws.Status
		u.Status, changed = &s, true
	}
	if ws.Title != ws.orig.Title {
		s := ws.Title
		u.Title, changed = &s, true
	}
	if ws.detailSet && (ws.orig.Detail == nil || !bytes.Equal(canonicalOf(ws.orig.Detail), ws.Detail)) {
		u.Detail, changed = ws.Detail, true
	}
	if !ws.FinishedAt.Equal(ws.orig.FinishedAt) {
		t := ws.FinishedAt
		u.FinishedAt, changed = &t, true
	}
	if ws.Ordinal != ws.orig.Ordinal {
		o := ws.Ordinal
		u.Ordinal, changed = &o, true
	}
	return u, changed
}
