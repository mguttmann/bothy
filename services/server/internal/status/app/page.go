// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// The event, audit and field names of the status page (05 sections 4.12 and 6).
const (
	eventStatusUpdated = "status.updated"
	receiveStatusRead  = "status.read"
	attrOp             = "op"
	attrComponentID    = "component_id"
	attrIncidentID     = "incident_id"
	attrNoChange       = "no_change"
	opCreateComponent  = "create_component"
	opUpdateComponent  = "update_component"
	opDeleteComponent  = "delete_component"
	opCreateIncident   = "create_incident"
	opAddUpdate        = "add_incident_update"
	codeTaken          = "taken"
	codeRequired       = "required"
	codeInvalid        = "invalid"
	codeInvalidEnum    = "invalid_enum"
)

// ErrUnknownAgent reports an agent_id that names no agent (the foreign key of status_components).
var ErrUnknownAgent = errors.New("status: agent_id names no agent")

// PageConfig holds the keys the status page reads (16 14, 05 10, 06 20).
type PageConfig struct {
	// PublicEnabled is status.public_enabled: the page without login.
	PublicEnabled bool
	// HistoryWindow is status.history_window: how long resolved incidents stay on the page.
	HistoryWindow time.Duration
	// Maintenance is server.maintenance_mode = read_only (M7-08): the page reports the
	// maintenance from the configuration, without writing status_components (domain.InMaintenance).
	Maintenance bool
}

// PageDeps are the collaborators of the status page use cases.
type PageDeps struct {
	Store      PageStore
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	// Audit records the allow entries of status.manage.
	Audit auditing.Recorder
	// Events appends status.updated.
	Events eventlog.Appender
	// Clock nil means clock.System().
	Clock clock.Clock
}

// Page implements status.Status: the read model with its visibility and the six owner
// operations (brief M7-09, docs/spec/16-operations.md 12). It is safe for concurrent use.
type Page struct {
	cfg PageConfig
	d   PageDeps
}

// Compile-time check: Page is the inbound port of the status page.
var _ status.Status = (*Page)(nil)

// NewPage returns the use cases over d with cfg. Errors: a missing collaborator or a history
// window that is not positive.
func NewPage(cfg PageConfig, d PageDeps) (*Page, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Events == nil {
		return nil, errors.New("status: the status page needs the store, the unit of work, the Authorizer, the recorder and the appender")
	}
	if cfg.HistoryWindow <= 0 {
		return nil, errors.New("status: status.history_window must be positive")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Page{cfg: cfg, d: d}, nil
}

// instance is the resource of the three status actions.
var instance = authorize.Resource{Kind: authorize.ResourceInstance}

// check authorizes act on the instance for a.
func (p *Page) check(ctx context.Context, a actor.Actor, act authorize.Action) error {
	d, err := p.d.Authorizer.Authorize(ctx, a, act, instance)
	if err != nil {
		return err
	}
	return d.Err()
}

// Read implements status.Status (getStatus).
func (p *Page) Read(ctx context.Context, a actor.Actor) (status.Page, error) {
	if err := p.check(ctx, a, authorize.ActionStatusRead); err != nil {
		return status.Page{}, err
	}
	return p.read(ctx, func(c domain.Component) (bool, error) {
		if !isAgentComponent(c) {
			return true, nil
		}
		if c.AgentID.IsZero() {
			return false, nil
		}
		// A check per element of a list runs in filter mode: a hidden agent is no denial to audit
		// (07 4.2).
		d, err := p.d.Authorizer.Authorize(authorize.Filter(ctx), a, authorize.ActionAgentRead,
			authorize.Resource{Kind: authorize.ResourceAgent, ID: c.AgentID, OwnerID: c.AgentID})
		if err != nil {
			return false, err
		}
		return d.Allowed(), nil
	})
}

// ReadPublic implements status.Status (getPublicStatus): 404 unless status.public_enabled, so the
// answer does not reveal the configuration (06 4 item 5); never an agent component.
func (p *Page) ReadPublic(ctx context.Context) (status.Page, error) {
	if !p.cfg.PublicEnabled {
		return status.Page{}, errs.New(errs.CodeNotFound, "")
	}
	if err := p.check(ctx, actor.Anonymous(), authorize.ActionStatusPublicRead); err != nil {
		return status.Page{}, err
	}
	return p.read(ctx, func(c domain.Component) (bool, error) { return !isAgentComponent(c), nil })
}

// isAgentComponent reports a component of an agent: it has an agent or the check
// agent.presence. Such components are visible only with agent.read on their agent and never
// count for the overall state, which status.updated reports to every reader.
func isAgentComponent(c domain.Component) bool {
	return !c.AgentID.IsZero() || c.HealthSource == domain.SourceAgentPresence
}

// read builds the page of the components visible decides on; an incident is shown when it has no
// components or at least one visible one, with only the visible ones.
func (p *Page) read(ctx context.Context, visible func(domain.Component) (bool, error)) (status.Page, error) {
	comps, err := p.d.Store.Components(ctx)
	if err != nil {
		return status.Page{}, err
	}
	if p.cfg.Maintenance {
		for i := range comps {
			if !isAgentComponent(comps[i]) {
				comps[i].State = domain.InMaintenance(comps[i].State)
			}
		}
	}
	incs, err := p.d.Store.Incidents(ctx, p.d.Clock.Now().Add(-p.cfg.HistoryWindow))
	if err != nil {
		return status.Page{}, err
	}
	state := overall(comps)
	if p.cfg.Maintenance {
		state = domain.InMaintenance(state)
	}
	page := status.Page{OverallState: string(state), Components: []status.Component{}, Incidents: []status.Incident{}}
	seen := map[ids.ID]bool{}
	for _, c := range comps {
		ok, err := visible(c)
		if err != nil {
			return status.Page{}, err
		}
		if ok {
			seen[c.ID] = true
			page.Components = append(page.Components, toComponent(c))
		}
	}
	for _, inc := range incs {
		if shown, ok := visibleIncident(inc, seen); ok {
			page.Incidents = append(page.Incidents, toIncident(shown))
		}
	}
	return page, nil
}

// visibleIncident returns inc with only the components in seen and whether it is shown: at least
// one visible component, or none at all for an incident of the owner. An automatic incident
// always names its component; once that component is gone (a disabled agent, a deleted
// component) it is shown to nobody, because its title names the component, and so is a hidden
// one, whose last component belonged to an agent (D-M7-09-5).
func visibleIncident(inc domain.Incident, seen map[ids.ID]bool) (domain.Incident, bool) {
	if inc.Hidden {
		return domain.Incident{}, false
	}
	shown := make([]ids.ID, 0, len(inc.ComponentIDs))
	for _, id := range inc.ComponentIDs {
		if seen[id] {
			shown = append(shown, id)
		}
	}
	if len(shown) == 0 && (len(inc.ComponentIDs) > 0 || inc.Automatic()) {
		return domain.Incident{}, false
	}
	inc.ComponentIDs = shown
	return inc, true
}

// overall is the worst state of the components that belong to no agent.
func overall(comps []domain.Component) domain.ComponentState {
	states := make([]domain.ComponentState, 0, len(comps))
	for _, c := range comps {
		if !isAgentComponent(c) {
			states = append(states, c.State)
		}
	}
	return domain.Worst(states...)
}

// CreateComponent implements status.Status (createStatusComponent).
func (p *Page) CreateComponent(ctx context.Context, a actor.Actor, in status.ComponentInput) (status.Component, error) {
	var out domain.Component
	err := p.d.UoW.Do(actor.WithActor(ctx, a), func(ctx context.Context) error {
		comps, err := p.manage(ctx, a)
		if err != nil {
			return err
		}
		out, err = p.create(ctx, comps, in)
		return err
	})
	if err != nil {
		return status.Component{}, err
	}
	return toComponent(out), nil
}

// manage authorizes status.manage for a and locks the components.
func (p *Page) manage(ctx context.Context, a actor.Actor) ([]domain.Component, error) {
	if err := p.check(ctx, a, authorize.ActionStatusManage); err != nil {
		return nil, err
	}
	return p.d.Store.LockComponents(ctx)
}

// create is CreateComponent over the locked components comps.
func (p *Page) create(ctx context.Context, comps []domain.Component, in status.ComponentInput) (domain.Component, error) {
	c := domain.Component{ID: ids.New(), State: domain.StateOperational, UpdatedAt: p.d.Clock.Now()}
	if err := applyInput(&c, in); err != nil {
		return domain.Component{}, err
	}
	order, ok := domain.Place(componentIDs(comps), c.ID, in.AfterComponentID)
	if !ok {
		return domain.Component{}, fieldError("/after_component_id", codeInvalid)
	}
	c.SortKey = domain.SortKey(slices.Index(order, c.ID))
	inserted, err := p.d.Store.InsertComponent(ctx, c)
	if err != nil {
		return domain.Component{}, mapComponentError(err)
	}
	if !inserted {
		return domain.Component{}, fieldError("/name", codeTaken)
	}
	if err := renumber(ctx, p.d.Store, comps, order); err != nil {
		return domain.Component{}, err
	}
	if err := p.audit(ctx, opCreateComponent, attrComponentID, c.ID, false); err != nil {
		return domain.Component{}, err
	}
	return c, p.emitChange(ctx, append(comps, c), changedComponents(c), nil)
}

// UpdateComponent implements status.Status (updateStatusComponent). A change without effect
// answers the component, writes nothing, appends no event and audits no_change (R-API-27).
func (p *Page) UpdateComponent(ctx context.Context, a actor.Actor, id ids.ID, in status.ComponentInput) (status.Component, error) {
	var out domain.Component
	err := p.d.UoW.Do(actor.WithActor(ctx, a), func(ctx context.Context) error {
		comps, err := p.manage(ctx, a)
		if err != nil {
			return err
		}
		out, err = p.update(ctx, comps, id, in)
		return err
	})
	if err != nil {
		return status.Component{}, err
	}
	return toComponent(out), nil
}

// update is UpdateComponent over the locked components comps.
func (p *Page) update(ctx context.Context, comps []domain.Component, id ids.ID, in status.ComponentInput) (domain.Component, error) {
	at := slices.IndexFunc(comps, func(c domain.Component) bool { return c.ID == id })
	if at < 0 {
		return domain.Component{}, errs.New(errs.CodeNotFound, "")
	}
	before := comps[at]
	next := before
	if err := applyInput(&next, in); err != nil {
		return domain.Component{}, err
	}
	order, ok := domain.Place(componentIDs(comps), id, in.AfterComponentID)
	if !ok {
		return domain.Component{}, fieldError("/after_component_id", codeInvalid)
	}
	moved := !slices.Equal(order, componentIDs(comps))
	if next == before && !moved {
		return before, p.audit(ctx, opUpdateComponent, attrComponentID, id, true)
	}
	if next != before {
		next.UpdatedAt = p.d.Clock.Now()
		if err := p.d.Store.UpdateComponent(ctx, next); err != nil {
			return domain.Component{}, mapComponentError(err)
		}
	}
	if err := renumber(ctx, p.d.Store, comps, order); err != nil {
		return domain.Component{}, err
	}
	next.SortKey = domain.SortKey(slices.Index(order, id))
	comps[at] = next
	if err := p.audit(ctx, opUpdateComponent, attrComponentID, id, false); err != nil {
		return domain.Component{}, err
	}
	return next, p.emitChange(ctx, comps, changedComponents(next), nil)
}

// mapComponentError turns the store errors of a component write into the field errors of the
// body.
func mapComponentError(err error) error {
	switch {
	case errors.Is(err, ErrNameTaken):
		return fieldError("/name", codeTaken)
	case errors.Is(err, ErrUnknownAgent):
		return fieldError("/agent_id", codeInvalid)
	}
	return err
}

// DeleteComponent implements status.Status (deleteStatusComponent). A standard component comes
// back with the next round of status.probe (D-M7-09-2).
func (p *Page) DeleteComponent(ctx context.Context, a actor.Actor, id ids.ID) error {
	return p.d.UoW.Do(actor.WithActor(ctx, a), func(ctx context.Context) error {
		if err := p.check(ctx, a, authorize.ActionStatusManage); err != nil {
			return err
		}
		comps, err := p.d.Store.LockComponents(ctx)
		if err != nil {
			return err
		}
		at := slices.IndexFunc(comps, func(c domain.Component) bool { return c.ID == id })
		if at < 0 {
			return errs.New(errs.CodeNotFound, "")
		}
		gone := comps[at]
		if _, err := p.d.Store.DeleteComponent(ctx, id); err != nil {
			return err
		}
		if err := p.audit(ctx, opDeleteComponent, attrComponentID, id, false); err != nil {
			return err
		}
		return p.emitChange(ctx, slices.Delete(comps, at, at+1), changedComponents(gone), nil)
	})
}

// CreateIncident implements status.Status (createStatusIncident): the incident with created_by
// the owner, its components and its first update in one unit of work.
func (p *Page) CreateIncident(ctx context.Context, a actor.Actor, in status.IncidentCreate) (status.Incident, error) {
	var out domain.Incident
	err := p.d.UoW.Do(actor.WithActor(ctx, a), func(ctx context.Context) error {
		if err := p.check(ctx, a, authorize.ActionStatusManage); err != nil {
			return err
		}
		comps, err := p.d.Store.LockComponents(ctx)
		if err != nil {
			return err
		}
		inc, err := p.newIncident(in, comps, a.ID)
		if err != nil {
			return err
		}
		first := domain.IncidentUpdate{ID: ids.New(), IncidentID: inc.ID, State: inc.State, BodyMD: in.BodyMD, CreatedBy: a.ID, CreatedAt: p.d.Clock.Now()}
		if err := p.d.Store.InsertIncident(ctx, inc, first); err != nil {
			return err
		}
		inc.Updates = []domain.IncidentUpdate{first}
		out = inc
		if err := p.audit(ctx, opCreateIncident, attrIncidentID, inc.ID, false); err != nil {
			return err
		}
		return p.emitChange(ctx, comps, nil, changedIncident(inc, comps))
	})
	if err != nil {
		return status.Incident{}, err
	}
	return toIncident(out), nil
}

// changedIncident returns the id of inc for the payload of status.updated, nothing when all of its
// components belong to agents (D-M7-09-5).
func changedIncident(inc domain.Incident, comps []domain.Component) []ids.ID {
	if len(inc.ComponentIDs) == 0 {
		if inc.Automatic() {
			// Its component is gone; the incident is shown to nobody.
			return nil
		}
		return []ids.ID{inc.ID}
	}
	for _, c := range comps {
		if slices.Contains(inc.ComponentIDs, c.ID) && !isAgentComponent(c) {
			return []ids.ID{inc.ID}
		}
	}
	return nil
}

// newIncident validates in against the locked components comps and returns the incident of
// owner by.
func (p *Page) newIncident(in status.IncidentCreate, comps []domain.Component, by ids.ID) (domain.Incident, error) {
	var fields []errs.FieldError
	add := func(ptr, code string) { fields = append(fields, errs.FieldError{Pointer: ptr, Code: code}) }
	if c := domain.TextProblem(in.Title, true); c != "" {
		add("/title", c)
	}
	if c := domain.TextProblem(in.BodyMD, true); c != "" {
		add("/body_md", c)
	}
	inc := domain.Incident{ID: ids.New(), Title: in.Title, Kind: domain.IncidentKind(in.Kind), Impact: domain.Impact(in.Impact),
		State: domain.IncidentState(in.State), StartedAt: in.StartedAt, CreatedBy: by}
	if !inc.Kind.Valid() {
		add("/kind", codeInvalidEnum)
	}
	if !inc.Impact.Valid() {
		add("/impact", codeInvalidEnum)
	}
	switch {
	case in.State == "" && inc.Kind == domain.KindMaintenance:
		inc.State = domain.IncidentScheduled
	case in.State == "":
		inc.State = domain.IncidentInvestigating
	case !inc.State.Valid():
		add("/state", codeInvalidEnum)
	}
	if in.StartedAt.IsZero() {
		add("/started_at", codeRequired)
	}
	known := componentIDs(comps)
	for i, id := range in.ComponentIDs {
		if !slices.Contains(known, id) {
			add("/component_ids/"+strconv.Itoa(i), codeInvalid)
			continue
		}
		if !slices.Contains(inc.ComponentIDs, id) {
			inc.ComponentIDs = append(inc.ComponentIDs, id)
		}
	}
	if len(fields) > 0 {
		return domain.Incident{}, errs.New(errs.CodeValidationFailed, "").WithFields(fields...)
	}
	slices.SortFunc(inc.ComponentIDs, compareIDs)
	if inc.State == domain.IncidentResolved {
		inc.ResolvedAt = p.d.Clock.Now()
	}
	return inc, nil
}

// AddIncidentUpdate implements status.Status (addStatusIncidentUpdate): the update with the
// owner as created_by and the state of the incident; resolved sets resolved_at, another state
// clears it. Earlier updates, also those of the job status.probe, stay as they are.
func (p *Page) AddIncidentUpdate(ctx context.Context, a actor.Actor, id ids.ID, in status.IncidentUpdateCreate) (status.Incident, error) {
	var out domain.Incident
	err := p.d.UoW.Do(actor.WithActor(ctx, a), func(ctx context.Context) error {
		if err := p.check(ctx, a, authorize.ActionStatusManage); err != nil {
			return err
		}
		state, err := validUpdate(in)
		if err != nil {
			return err
		}
		comps, err := p.d.Store.LockComponents(ctx)
		if err != nil {
			return err
		}
		inc, found, err := p.d.Store.LockIncident(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return errs.New(errs.CodeNotFound, "")
		}
		u := domain.IncidentUpdate{ID: ids.New(), IncidentID: id, State: state, BodyMD: in.BodyMD, CreatedBy: a.ID, CreatedAt: p.d.Clock.Now()}
		inc.ResolvedAt = resolvedAt(inc, state, u.CreatedAt)
		if err := p.d.Store.AddIncidentUpdate(ctx, u, inc.ResolvedAt); err != nil {
			return err
		}
		inc.State = state
		inc.Updates = append(inc.Updates, u)
		out = inc
		if err := p.audit(ctx, opAddUpdate, attrIncidentID, id, false); err != nil {
			return err
		}
		return p.emitChange(ctx, comps, nil, changedIncident(inc, comps))
	})
	if err != nil {
		return status.Incident{}, err
	}
	return toIncident(out), nil
}

// validUpdate checks the body of addStatusIncidentUpdate and returns its state.
func validUpdate(in status.IncidentUpdateCreate) (domain.IncidentState, error) {
	state := domain.IncidentState(in.State)
	var fields []errs.FieldError
	if !state.Valid() {
		fields = append(fields, errs.FieldError{Pointer: "/state", Code: codeInvalidEnum})
	}
	if c := domain.TextProblem(in.BodyMD, true); c != "" {
		fields = append(fields, errs.FieldError{Pointer: "/body_md", Code: c})
	}
	if len(fields) > 0 {
		return "", errs.New(errs.CodeValidationFailed, "").WithFields(fields...)
	}
	return state, nil
}

// resolvedAt is resolved_at after an update to state at now: kept when the incident was resolved
// already, now when it becomes resolved, zero (NULL) for any other state.
func resolvedAt(inc domain.Incident, state domain.IncidentState, now time.Time) time.Time {
	switch {
	case state != domain.IncidentResolved:
		return time.Time{}
	case inc.State == domain.IncidentResolved && !inc.ResolvedAt.IsZero():
		return inc.ResolvedAt
	}
	return now
}

// applyInput validates in and applies it to c: the texts, the state and the check with its agent.
func applyInput(c *domain.Component, in status.ComponentInput) error {
	fields := applyTexts(c, in)
	if in.State != nil {
		c.State = domain.ComponentState(*in.State)
		if !c.State.Valid() {
			fields = append(fields, errs.FieldError{Pointer: "/state", Code: codeInvalidEnum})
		}
	}
	fields = append(fields, applyCheck(c, in)...)
	if len(fields) > 0 {
		return errs.New(errs.CodeValidationFailed, "").WithFields(fields...)
	}
	return nil
}

// applyTexts applies name and description and returns their field errors.
func applyTexts(c *domain.Component, in status.ComponentInput) []errs.FieldError {
	var fields []errs.FieldError
	if code := domain.NameProblem(in.Name); code != "" {
		fields = append(fields, errs.FieldError{Pointer: "/name", Code: code})
	}
	c.Name = in.Name
	if in.Description != nil {
		if code := domain.TextProblem(*in.Description, false); code != "" {
			fields = append(fields, errs.FieldError{Pointer: "/description", Code: code})
		}
		c.Description = *in.Description
	}
	return fields
}

// applyCheck applies health_source and agent_id and returns their field errors: a known key or
// none, an agent exactly with agent.presence.
func applyCheck(c *domain.Component, in status.ComponentInput) []errs.FieldError {
	var fields []errs.FieldError
	if in.HealthSource != nil {
		c.HealthSource = *in.HealthSource
		if _, ok := domain.Checks(c.HealthSource); c.HealthSource != "" && !ok {
			fields = append(fields, errs.FieldError{Pointer: "/health_source", Code: codeInvalid})
		}
	}
	if in.AgentID != nil {
		c.AgentID = *in.AgentID
	}
	switch {
	case c.HealthSource == domain.SourceAgentPresence && c.AgentID.IsZero():
		fields = append(fields, errs.FieldError{Pointer: "/agent_id", Code: codeRequired})
	case !c.AgentID.IsZero() && c.HealthSource != domain.SourceAgentPresence:
		fields = append(fields, errs.FieldError{Pointer: "/agent_id", Code: codeInvalid})
	}
	return fields
}

// fieldError is validation_failed with one field.
func fieldError(ptr, code string) error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: ptr, Code: code})
}

// componentIDs returns the ids of comps in their order.
func componentIDs(comps []domain.Component) []ids.ID {
	out := make([]ids.ID, len(comps))
	for i, c := range comps {
		out[i] = c.ID
	}
	return out
}

// renumber writes the sort keys of order where they differ from the locked comps.
func renumber(ctx context.Context, store PageStore, comps []domain.Component, order []ids.ID) error {
	have := map[ids.ID]string{}
	for _, c := range comps {
		have[c.ID] = c.SortKey
	}
	keys := map[ids.ID]string{}
	for i, id := range order {
		if k, ok := have[id]; ok && k != domain.SortKey(i) {
			keys[id] = domain.SortKey(i)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return store.SetSortKeys(ctx, keys)
}

// audit records the allow entry of status.manage with the operation and the id, never a text.
func (p *Page) audit(ctx context.Context, op, attr string, id ids.ID, noChange bool) error {
	attrs := auditing.Attrs{attrOp: auditing.Code(op), attr: auditing.ID(id)}
	if noChange {
		attrs[attrNoChange] = auditing.Bool(true)
	}
	return p.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionStatusManage), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	})
}

// changedComponents returns the id of c for the payload of status.updated, nothing for an agent
// component: every reader with status.read receives the event, an agent component only those
// with agent.read (D-M7-09-5).
func changedComponents(c domain.Component) []ids.ID {
	if isAgentComponent(c) {
		return nil
	}
	return []ids.ID{c.ID}
}

// emitChange appends status.updated with the overall state of comps after the change.
func (p *Page) emitChange(ctx context.Context, comps []domain.Component, components, incidents []ids.ID) error {
	return appendUpdated(ctx, p.d.Events, overall(comps), components, incidents)
}

// updatedPayload is the payload of status.updated (schema StatusUpdatedPayload).
type updatedPayload struct {
	OverallState string   `json:"overall_state"`
	Components   []ids.ID `json:"changed_component_ids,omitempty"`
	Incidents    []ids.ID `json:"changed_incident_ids,omitempty"`
}

// appendUpdated appends status.updated (05 section 6): resource instance without id and version,
// empty scope, receive action status.read, the actor of the request in ctx or none (the job). A
// change that names no component and no incident touched only agent components, which never count
// for the overall state; it appends nothing, because every reader with status.read receives the
// event and one without agent.read must not learn that a foreign agent changed (D-M7-09-5, AZ-P5).
func appendUpdated(ctx context.Context, events eventlog.Appender, state domain.ComponentState, components, incidents []ids.ID) error {
	if len(components) == 0 && len(incidents) == 0 {
		return nil
	}
	pl := updatedPayload{OverallState: string(state), Components: sortedIDs(components), Incidents: sortedIDs(incidents)}
	raw, err := json.Marshal(pl)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "status: status.updated payload")
	}
	kind, id := eventActor(ctx)
	return events.Append(ctx, eventlog.Event{
		Type: eventStatusUpdated, SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: id,
		ResourceKind: string(authorize.ResourceInstance), ReceiveAction: receiveStatusRead, Payload: raw,
	})
}

// sortedIDs returns the distinct ids ascending; nil for none.
func sortedIDs(list []ids.ID) []ids.ID {
	if len(list) == 0 {
		return nil
	}
	out := slices.Clone(list)
	slices.SortFunc(out, compareIDs)
	return slices.Compact(out)
}

// eventActor returns the actor of the request in ctx; none outside a request (the job).
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.ID.IsZero() {
		return "", ids.ID{}
	}
	switch a.Kind {
	case actor.KindUser:
		return eventlog.ActorUser, a.ID
	case actor.KindAgent:
		return eventlog.ActorAgent, a.ID
	case actor.KindApp:
		return eventlog.ActorApp, a.ID
	case actor.KindHost, actor.KindAnonymous:
	}
	return "", ids.ID{}
}

// toComponent maps a component to the port.
func toComponent(c domain.Component) status.Component {
	return status.Component{
		ID: c.ID, Name: c.Name, Description: c.Description, SortKey: c.SortKey, State: string(c.State),
		HealthSource: c.HealthSource, AgentID: c.AgentID, UpdatedAt: c.UpdatedAt,
	}
}

// toIncident maps an incident to the port.
func toIncident(i domain.Incident) status.Incident {
	out := status.Incident{
		ID: i.ID, Title: i.Title, Kind: string(i.Kind), Impact: string(i.Impact), State: string(i.State),
		StartedAt: i.StartedAt, ResolvedAt: i.ResolvedAt, ComponentIDs: slices.Clone(i.ComponentIDs),
		Updates: make([]status.IncidentUpdate, 0, len(i.Updates)),
	}
	if out.ComponentIDs == nil {
		out.ComponentIDs = []ids.ID{}
	}
	for _, u := range i.Updates {
		out.Updates = append(out.Updates, status.IncidentUpdate{ID: u.ID, State: string(u.State), BodyMD: u.BodyMD, CreatedAt: u.CreatedAt})
	}
	return out
}

// compareIDs orders ids by their bytes, the order of uuid in PostgreSQL.
func compareIDs(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }
