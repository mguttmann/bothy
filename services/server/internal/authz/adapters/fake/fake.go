// SPDX-License-Identifier: Apache-2.0

// Package fake holds in-memory implementations of the source ports of authz (PrincipalSource,
// StructureSource, AgentPolicySource, CredentialSource), of its Store and of the rights version,
// for the tests of authz and of later modules. Only test files import it. Every fake is safe for
// concurrent use and can fail on demand (Fail), so tests prove that a source error never allows.
//
// Example:
//
//	st := fake.NewStructure()
//	st.PutSpace(support)
//	st.PutChannel(authz.ChannelFacts{ID: general, SpaceID: support, Kind: authz.ChannelText})
package fake

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ErrInjected is the error a fake returns after Fail(true).
var ErrInjected = errors.New("fake: injected source error")

// failer is the switchable error of every fake.
type failer struct {
	mu   sync.Mutex
	fail bool
}

// Fail makes every later call of the fake return ErrInjected (true) or work again (false).
func (f *failer) Fail(on bool) {
	f.mu.Lock()
	f.fail = on
	f.mu.Unlock()
}

func (f *failer) err() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return ErrInjected
	}
	return nil
}

func sortIDs(list []ids.ID) []ids.ID {
	slices.SortFunc(list, compareIDs)
	return list
}

// Principals is an in-memory authz.PrincipalSource.
type Principals struct {
	failer
	mu   sync.Mutex
	rows map[ids.ID]authz.PrincipalFacts
	// passkeys are the people with an active passkey (M1-40).
	passkeys map[ids.ID]bool
	// profiles are the names, locales and time zones set with PutProfile (M1-49).
	profiles map[ids.ID]authz.PrincipalProfile
}

// Compile-time check.
var _ authz.PrincipalSource = (*Principals)(nil)

// NewPrincipals returns an empty source.
func NewPrincipals() *Principals { return &Principals{rows: map[ids.ID]authz.PrincipalFacts{}} }

// Put adds or replaces the facts of f.ID.
func (s *Principals) Put(f authz.PrincipalFacts) {
	s.mu.Lock()
	s.rows[f.ID] = f
	s.mu.Unlock()
}

// Principal implements authz.PrincipalSource.
func (s *Principals) Principal(_ context.Context, id ids.ID) (authz.PrincipalFacts, bool, error) {
	if err := s.err(); err != nil {
		return authz.PrincipalFacts{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.rows[id]
	return f, ok, nil
}

// SetPasskey records whether userID has an active passkey (HasActivePasskey, M1-40).
func (s *Principals) SetPasskey(userID ids.ID, has bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.passkeys == nil {
		s.passkeys = map[ids.ID]bool{}
	}
	s.passkeys[userID] = has
}

// HasActivePasskey implements authz.PrincipalSource.
func (s *Principals) HasActivePasskey(_ context.Context, userID ids.ID) (bool, error) {
	if err := s.err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.passkeys[userID], nil
}

// ActiveOwnerIDs implements authz.PrincipalSource.
func (s *Principals) ActiveOwnerIDs(_ context.Context) ([]ids.ID, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ids.ID
	for _, f := range s.rows {
		if f.IsOwner && f.Status == authz.PrincipalActive {
			out = append(out, f.ID)
		}
	}
	return sortIDs(out), nil
}

// Refs implements authz.PrincipalSource: the names set with PutProfile, else the id as handle
// and display name, because the facts carry no names.
func (s *Principals) Refs(_ context.Context, principals []ids.ID) (map[ids.ID]authz.PrincipalRef, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[ids.ID]authz.PrincipalRef{}
	for _, id := range principals {
		if p, ok := s.profile(id); ok {
			out[id] = p.PrincipalRef
		}
	}
	return out, nil
}

// PutProfile sets the names, locale and time zone of p.ID that Profiles and Refs report; kind and
// status always come from the facts set with Put.
func (s *Principals) PutProfile(p authz.PrincipalProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profiles == nil {
		s.profiles = map[ids.ID]authz.PrincipalProfile{}
	}
	s.profiles[p.ID] = p
}

// profile returns the profile of a known principal; without PutProfile its handle and display
// name are its id. The caller holds s.mu.
func (s *Principals) profile(id ids.ID) (authz.PrincipalProfile, bool) {
	f, ok := s.rows[id]
	if !ok {
		return authz.PrincipalProfile{}, false
	}
	p, named := s.profiles[id]
	if !named {
		p = authz.PrincipalProfile{PrincipalRef: authz.PrincipalRef{Handle: id.String(), DisplayName: id.String()}}
	}
	p.ID, p.Kind, p.Status = id, f.Kind, f.Status
	return p, true
}

// Profiles implements authz.PrincipalSource.
func (s *Principals) Profiles(_ context.Context, principals []ids.ID) (map[ids.ID]authz.PrincipalProfile, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[ids.ID]authz.PrincipalProfile{}
	for _, id := range principals {
		if p, ok := s.profile(id); ok {
			out[id] = p
		}
	}
	return out, nil
}

// OwnerIDs implements authz.PrincipalSource: every principal whose facts carry IsOwner.
func (s *Principals) OwnerIDs(_ context.Context) ([]ids.ID, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ids.ID
	for _, f := range s.rows {
		if f.IsOwner {
			out = append(out, f.ID)
		}
	}
	return sortIDs(out), nil
}

// Structure is an in-memory authz.StructureSource. ServedTargets follows the rules of its doc
// comment from the channels, the threads and the default operators set with SetDefaultOperator.
type Structure struct {
	failer
	mu              sync.Mutex
	spaces          map[ids.ID]bool
	channels        map[ids.ID]authz.ChannelFacts
	threads         map[ids.ID]authz.ThreadFacts
	defaultOperator map[ids.ID]ids.ID
	archivedSpaces  map[ids.ID]bool
	// spaceDetails and channelDetails are set with PutSpaceDetail and PutChannelDetail (M1-49).
	spaceDetails   map[ids.ID]authz.SpaceDetail
	channelDetails map[ids.ID]authz.ChannelDetail
}

// Compile-time check.
var _ authz.StructureSource = (*Structure)(nil)

// NewStructure returns an empty structure.
func NewStructure() *Structure {
	return &Structure{spaces: map[ids.ID]bool{}, channels: map[ids.ID]authz.ChannelFacts{}, threads: map[ids.ID]authz.ThreadFacts{},
		defaultOperator: map[ids.ID]ids.ID{}, archivedSpaces: map[ids.ID]bool{},
		spaceDetails: map[ids.ID]authz.SpaceDetail{}, channelDetails: map[ids.ID]authz.ChannelDetail{}}
}

// PutSpaceDetail adds the space d.ID with its details; its default operator feeds ServedTargets
// like SetDefaultOperator and a non-zero ArchivedAt marks it archived like SetSpaceArchived.
func (s *Structure) PutSpaceDetail(d authz.SpaceDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spaces[d.ID], s.spaceDetails[d.ID] = true, d
	s.archivedSpaces[d.ID] = !d.ArchivedAt.IsZero()
	if d.DefaultOperatorID.IsZero() {
		delete(s.defaultOperator, d.ID)
	} else {
		s.defaultOperator[d.ID] = d.DefaultOperatorID
	}
}

// PutChannelDetail sets the names and presentation of a channel that ChannelDetails reports; the
// facts of PutChannel (space, kind, operator, read-only, archived, participants) win over the
// same fields of d.
func (s *Structure) PutChannelDetail(d authz.ChannelDetail) {
	s.mu.Lock()
	s.channelDetails[d.ID] = d
	s.mu.Unlock()
}

// SpaceDetails implements authz.StructureSource: the details of PutSpaceDetail, else the id as
// name, with the default operator of SetDefaultOperator.
func (s *Structure) SpaceDetails(_ context.Context, spaceIDs []ids.ID) (map[ids.ID]authz.SpaceDetail, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[ids.ID]authz.SpaceDetail{}
	for _, id := range spaceIDs {
		if !s.spaces[id] {
			continue
		}
		d, ok := s.spaceDetails[id]
		if !ok {
			d = authz.SpaceDetail{ID: id, Name: id.String(), SortKey: "a0", Version: 1}
		}
		d.DefaultOperatorID = s.defaultOperator[id]
		out[id] = d
	}
	return out, nil
}

// ChannelDetails implements authz.StructureSource: the facts of PutChannel with the names of
// PutChannelDetail, else the id as name.
func (s *Structure) ChannelDetails(_ context.Context, channelIDs []ids.ID) (map[ids.ID]authz.ChannelDetail, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[ids.ID]authz.ChannelDetail{}
	for _, id := range channelIDs {
		c, ok := s.channels[id]
		if !ok {
			continue
		}
		out[id] = channelDetail(c, s.channelDetails[id])
	}
	return out, nil
}

// channelDetail merges the facts c into the details d of PutChannelDetail (zero: the id as name).
func channelDetail(c authz.ChannelFacts, d authz.ChannelDetail) authz.ChannelDetail {
	if d.ID.IsZero() {
		d = authz.ChannelDetail{Name: c.ID.String(), SortKey: "a0", Version: 1, SettingsVersion: 1}
	}
	d.ID, d.SpaceID, d.Kind, d.ReadOnly, d.EffectiveOperatorID = c.ID, c.SpaceID, c.Kind, c.ReadOnly, c.EffectiveOperatorID
	d.DMParticipantIDs = slices.Clone(c.DMParticipants)
	if d.OperatorMode == "" {
		d.OperatorMode, d.OperatorAgentID = authz.OperatorNone, ids.ID{}
		if !c.EffectiveOperatorID.IsZero() {
			d.OperatorMode, d.OperatorAgentID = authz.OperatorAgent, c.EffectiveOperatorID
		}
	}
	switch {
	case !c.Archived:
		d.ArchivedAt = time.Time{}
	case d.ArchivedAt.IsZero():
		d.ArchivedAt = time.Unix(0, 0).UTC()
	}
	return d
}

// SetSpaceArchived marks space archived for SpaceArchived; channels report their own Archived.
func (s *Structure) SetSpaceArchived(space ids.ID, archived bool) {
	s.mu.Lock()
	s.archivedSpaces[space] = archived
	s.mu.Unlock()
}

// SpaceArchived implements authz.StructureSource.
func (s *Structure) SpaceArchived(_ context.Context, id ids.ID) (bool, error) {
	if err := s.err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.archivedSpaces[id], nil
}

// SetDefaultOperator sets the default operator of space; the zero agent removes it. It only
// feeds ServedTargets: the effective operator of a channel stays what PutChannel stored.
func (s *Structure) SetDefaultOperator(space, agent ids.ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if agent.IsZero() {
		delete(s.defaultOperator, space)
		return
	}
	s.defaultOperator[space] = agent
}

// ServedTargets implements authz.StructureSource: channels that are not archived with the agent
// as effective operator or as the fixed operator of a non-main thread, and the spaces with the
// agent as default operator; guest channels never count, dm channels have no agents.
func (s *Structure) ServedTargets(_ context.Context, agentID ids.ID) ([]authz.ChannelRef, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[authz.ChannelRef]bool{}
	for _, c := range s.channels {
		if c.Kind != authz.ChannelDM && !c.Archived && c.EffectiveOperatorID == agentID {
			seen[authz.ChannelRef{ChannelID: c.ID, SpaceID: c.SpaceID}] = true
		}
	}
	for _, t := range s.threads {
		c, ok := s.channels[t.ChannelID]
		if ok && !t.Main && t.OperatorAgentID == agentID && c.Kind != authz.ChannelDM && !c.Archived {
			seen[authz.ChannelRef{ChannelID: c.ID, SpaceID: c.SpaceID}] = true
		}
	}
	for space, agent := range s.defaultOperator {
		if agent == agentID {
			seen[authz.ChannelRef{SpaceID: space}] = true
		}
	}
	out := slices.Collect(maps.Keys(seen))
	slices.SortFunc(out, func(a, b authz.ChannelRef) int {
		if c := slices.Compare(a.SpaceID[:], b.SpaceID[:]); c != 0 {
			return c
		}
		return slices.Compare(a.ChannelID[:], b.ChannelID[:])
	})
	return out, nil
}

// PutSpace adds a space.
func (s *Structure) PutSpace(id ids.ID) {
	s.mu.Lock()
	s.spaces[id] = true
	s.mu.Unlock()
}

// PutChannel adds or replaces a channel; DMParticipants are sorted.
func (s *Structure) PutChannel(c authz.ChannelFacts) {
	c.DMParticipants = sortIDs(slices.Clone(c.DMParticipants))
	s.mu.Lock()
	s.channels[c.ID] = c
	s.mu.Unlock()
}

// PutThread adds or replaces a thread.
func (s *Structure) PutThread(t authz.ThreadFacts) {
	s.mu.Lock()
	s.threads[t.ID] = t
	s.mu.Unlock()
}

// Space implements authz.StructureSource.
func (s *Structure) Space(_ context.Context, id ids.ID) (bool, error) {
	if err := s.err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.spaces[id], nil
}

// Spaces implements authz.StructureSource.
func (s *Structure) Spaces(_ context.Context) ([]ids.ID, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortIDs(slices.Collect(maps.Keys(s.spaces))), nil
}

// Channel implements authz.StructureSource.
func (s *Structure) Channel(_ context.Context, id ids.ID) (authz.ChannelFacts, bool, error) {
	if err := s.err(); err != nil {
		return authz.ChannelFacts{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.channels[id]
	c.DMParticipants = slices.Clone(c.DMParticipants)
	return c, ok, nil
}

// Thread implements authz.StructureSource.
func (s *Structure) Thread(_ context.Context, id ids.ID) (authz.ThreadFacts, bool, error) {
	if err := s.err(); err != nil {
		return authz.ThreadFacts{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.threads[id]
	return t, ok, nil
}

// Channels implements authz.StructureSource.
func (s *Structure) Channels(_ context.Context) ([]authz.ChannelFacts, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []authz.ChannelFacts
	for _, id := range sortIDs(slices.Collect(maps.Keys(s.channels))) {
		if c := s.channels[id]; c.Kind != authz.ChannelDM {
			out = append(out, c)
		}
	}
	return out, nil
}

// ChannelsOperatedBy implements authz.StructureSource.
func (s *Structure) ChannelsOperatedBy(_ context.Context, agentID ids.ID) ([]ids.ID, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ids.ID
	for id, c := range s.channels {
		if c.Kind != authz.ChannelDM && c.EffectiveOperatorID == agentID {
			out = append(out, id)
		}
	}
	return sortIDs(out), nil
}

// DMChannelsOf implements authz.StructureSource.
func (s *Structure) DMChannelsOf(_ context.Context, userID ids.ID) ([]ids.ID, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ids.ID
	for id, c := range s.channels {
		if c.Kind == authz.ChannelDM && slices.Contains(c.DMParticipants, userID) {
			out = append(out, id)
		}
	}
	return sortIDs(out), nil
}

// Policies is an in-memory authz.AgentPolicySource.
type Policies struct {
	failer
	mu       sync.Mutex
	policies map[ids.ID]authz.AgentPolicy
	ready    map[[2]ids.ID]bool
	inactive map[ids.ID]bool
}

// Compile-time check.
var _ authz.AgentPolicySource = (*Policies)(nil)

// NewPolicies returns an empty source.
func NewPolicies() *Policies {
	return &Policies{policies: map[ids.ID]authz.AgentPolicy{}, ready: map[[2]ids.ID]bool{}, inactive: map[ids.ID]bool{}}
}

// Put sets the policy of agentID.
func (s *Policies) Put(agentID ids.ID, p authz.AgentPolicy) {
	s.mu.Lock()
	s.policies[agentID] = p
	s.mu.Unlock()
}

// SetDevReady sets the readiness of the dev environment of userID for channelID.
func (s *Policies) SetDevReady(userID, channelID ids.ID, ready bool) {
	s.mu.Lock()
	s.ready[[2]ids.ID{userID, channelID}] = ready
	s.mu.Unlock()
}

// AgentPolicy implements authz.AgentPolicySource.
func (s *Policies) AgentPolicy(_ context.Context, agentID ids.ID) (authz.AgentPolicy, bool, error) {
	if err := s.err(); err != nil {
		return authz.AgentPolicy{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.policies[agentID]
	return p, ok, nil
}

// PersonalAgents implements authz.AgentPolicySource: the agents whose policy is personal and that
// are not marked inactive with SetActive.
func (s *Policies) PersonalAgents(_ context.Context) ([]authz.PersonalAgent, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []authz.PersonalAgent
	for id, p := range s.policies {
		if p.BillingMode == "personal" && !s.inactive[id] {
			out = append(out, authz.PersonalAgent{AgentID: id, CredentialHolderID: p.CredentialHolderID})
		}
	}
	slices.SortFunc(out, func(a, b authz.PersonalAgent) int { return slices.Compare(a.AgentID[:], b.AgentID[:]) })
	return out, nil
}

// SetActive marks agentID active (the default) or inactive for PersonalAgents, which lists active
// agents only (agent.enable, 10 section 5.1).
func (s *Policies) SetActive(agentID ids.ID, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if active {
		delete(s.inactive, agentID)
		return
	}
	s.inactive[agentID] = true
}

// DevEnvironmentReady implements authz.AgentPolicySource.
func (s *Policies) DevEnvironmentReady(_ context.Context, userID, channelID ids.ID) (bool, error) {
	if err := s.err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready[[2]ids.ID{userID, channelID}], nil
}

// Credentials is an in-memory authz.CredentialSource keyed by session, device and token id.
type Credentials struct {
	failer
	mu       sync.Mutex
	sessions map[ids.ID]Session
	devices  map[ids.ID]Device
	tokens   map[ids.ID]Token
}

// Session is a session of the fake.
type Session struct {
	PrincipalID, DeviceID ids.ID
	Revoked               bool
	ExpiresAt             time.Time
}

// Device is a device of the fake.
type Device struct {
	PrincipalID ids.ID
	State       authz.DeviceState
}

// Token is an API token of the fake.
type Token struct {
	PrincipalID ids.ID
	Revoked     bool
	ExpiresAt   time.Time
}

// Compile-time check.
var _ authz.CredentialSource = (*Credentials)(nil)

// NewCredentials returns an empty source.
func NewCredentials() *Credentials {
	return &Credentials{sessions: map[ids.ID]Session{}, devices: map[ids.ID]Device{}, tokens: map[ids.ID]Token{}}
}

// PutSession adds or replaces a session.
func (s *Credentials) PutSession(id ids.ID, v Session) {
	s.mu.Lock()
	s.sessions[id] = v
	s.mu.Unlock()
}

// PutDevice adds or replaces a device.
func (s *Credentials) PutDevice(id ids.ID, v Device) { s.mu.Lock(); s.devices[id] = v; s.mu.Unlock() }

// PutToken adds or replaces a token.
func (s *Credentials) PutToken(id ids.ID, v Token) { s.mu.Lock(); s.tokens[id] = v; s.mu.Unlock() }

// Credentials implements authz.CredentialSource with the rules of its doc comment: a session must
// belong to the principal and the device, a device or token to the principal.
func (s *Credentials) Credentials(_ context.Context, ref authz.CredentialRef) (authz.CredentialFacts, error) {
	if err := s.err(); err != nil {
		return authz.CredentialFacts{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var f authz.CredentialFacts
	if !ref.SessionID.IsZero() {
		if v, ok := s.sessions[ref.SessionID]; ok && v.PrincipalID == ref.PrincipalID && v.DeviceID == ref.DeviceID && !v.Revoked {
			f.SessionActive, f.SessionExpiresAt = true, v.ExpiresAt
		}
	}
	f.DeviceState = authz.DeviceUnknown
	if v, ok := s.devices[ref.DeviceID]; ok && !ref.DeviceID.IsZero() && v.PrincipalID == ref.PrincipalID {
		f.DeviceState = v.State
	}
	if !ref.TokenID.IsZero() {
		if v, ok := s.tokens[ref.TokenID]; ok && v.PrincipalID == ref.PrincipalID && !v.Revoked {
			f.TokenActive, f.TokenExpiresAt = true, v.ExpiresAt
		}
	}
	return f, nil
}

// compareIDs orders ids by their bytes, the creation order of UUIDv7.
func compareIDs(a, b ids.ID) int { return slices.Compare(a[:], b[:]) }
