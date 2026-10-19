// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// hostState is a host row with its valid session user assignments, the input of the view.
type hostState struct {
	host  domain.Host
	users []domain.SessionUser
}

// load reads the host id with its valid assignments; found is false for an unknown id.
func (s *Service) load(ctx context.Context, id ids.ID) (hostState, bool, error) {
	h, found, err := s.d.Store.Host(ctx, id)
	if err != nil || !found {
		return hostState{}, found, err
	}
	return s.withUsers(ctx, h)
}

// withUsers adds the valid assignments of h.
func (s *Service) withUsers(ctx context.Context, h domain.Host) (hostState, bool, error) {
	users, err := s.d.Store.ValidSessionUsers(ctx, []ids.ID{h.ID})
	if err != nil {
		return hostState{}, false, err
	}
	return hostState{host: h, users: users[h.ID]}, true, nil
}

// view maps a host to the port DTO (schema Host, brief M2-39 Design note 8): disabled_reason only
// with disabled; platform, the reported values, versions, last_seen_at and the keys only when set;
// credential_holder_id only with personal; profiles and doctor always (empty lists when nothing is
// reported); reported_session_users only once the host has been paired; session_users the valid
// assignments; connected from the presence of the control connections.
func (s *Service) view(ctx context.Context, h hostState) terminal.HostView {
	r := h.host
	v := terminal.HostView{
		ID: r.ID, Name: r.Name, State: string(r.State), Platform: r.Platform, AssignedUserID: r.AssignedUserID,
		LLMCredentials: r.LLMCredentials, DeclaredServerHost: r.DeclaredServerHost, ReportedIsServerHost: r.ReportedIsServerHost,
		NoisePublicKey: r.NoisePublicKey, KeyFingerprint: r.KeyFingerprint, KeyGeneration: r.KeyGeneration,
		RequireLocalConfirm: r.RequireLocalConfirm, Connected: s.d.Presence.Connected(ctx, r.ID), HostVersion: r.HostVersion,
		OSVersion: r.OSVersion, LastSeenAt: r.LastSeenAt, CreatedAt: r.CreatedAt, Version: r.Version,
		Profiles: []terminal.Profile{}, Doctor: []terminal.DoctorCheck{},
	}
	if r.State == domain.StateDisabled {
		v.DisabledReason = r.DisabledReason
	}
	if !r.HasKeys() {
		// The retained fingerprint describes dropped keys; a client must never pin it (M2-41).
		v.KeyFingerprint, v.NoisePublicKey = "", nil
	}
	if r.LLMCredentials == "personal" {
		v.CredentialHolderID = r.CredentialHolderID
	}
	for _, p := range r.Profiles {
		v.Profiles = append(v.Profiles, terminal.Profile{Name: p.Name, DisplayName: p.DisplayName, Kind: p.Kind})
	}
	for _, c := range r.Doctor {
		v.Doctor = append(v.Doctor, terminal.DoctorCheck{Check: c.Check, OK: c.OK, Code: c.Code, SessionUser: c.SessionUser})
	}
	if r.Paired() {
		v.ReportedSessionUsers = []terminal.ReportedSessionUser{}
		for _, u := range r.ReportedUsers {
			v.ReportedSessionUsers = append(v.ReportedSessionUsers, terminal.ReportedSessionUser{OSUser: u.OSUser, Workdirs: u.Workdirs})
		}
	}
	for _, u := range h.users {
		v.SessionUsers = append(v.SessionUsers, terminal.SessionUser{UserID: u.UserID, OSUser: u.OSUser, CreatedAt: u.CreatedAt})
	}
	return v
}

// The JSON form of the schema Host of docs/api/openapi.yaml: the payload member host of
// host.updated and the member current of 412 version_conflict. Optional members are left out
// (ADR 0053).
type (
	profileJSON struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Kind        string `json:"kind"`
	}
	doctorJSON struct {
		Check       string `json:"check"`
		OK          bool   `json:"ok"`
		Code        string `json:"code,omitempty"`
		SessionUser string `json:"session_user,omitempty"`
	}
	sessionUserJSON struct {
		UserID    string `json:"user_id"`
		OSUser    string `json:"os_user"`
		CreatedAt string `json:"created_at"`
	}
	reportedJSON struct {
		OSUser   string   `json:"os_user"`
		Workdirs []string `json:"workdirs"`
	}
	hostJSON struct {
		ID                   string            `json:"id"`
		Name                 string            `json:"name"`
		State                string            `json:"state"`
		DisabledReason       string            `json:"disabled_reason,omitempty"`
		Platform             string            `json:"platform,omitempty"`
		AssignedUserID       string            `json:"assigned_user_id,omitempty"`
		LLMCredentials       string            `json:"llm_credentials"`
		CredentialHolderID   string            `json:"credential_holder_id,omitempty"`
		DeclaredServerHost   bool              `json:"declared_server_host"`
		ReportedIsServerHost *bool             `json:"reported_is_server_host,omitempty"`
		NoisePublicKey       string            `json:"noise_public_key,omitempty"`
		KeyFingerprint       string            `json:"key_fingerprint,omitempty"`
		KeyGeneration        int64             `json:"key_generation"`
		Profiles             []profileJSON     `json:"profiles"`
		Doctor               []doctorJSON      `json:"doctor"`
		RequireLocalConfirm  *bool             `json:"require_local_confirm,omitempty"`
		Connected            bool              `json:"connected"`
		HostVersion          string            `json:"host_version,omitempty"`
		OSVersion            string            `json:"os_version,omitempty"`
		LastSeenAt           string            `json:"last_seen_at,omitempty"`
		SessionUsers         []sessionUserJSON `json:"session_users,omitempty"`
		ReportedSessionUsers *[]reportedJSON   `json:"reported_session_users,omitempty"`
		CreatedAt            string            `json:"created_at"`
		Version              int64             `json:"version"`
	}
)

// timestampLayout is the form of every API time (06 section 2): UTC, milliseconds, suffix Z.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// stamp formats t; the zero time is absent.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format(timestampLayout)
}

// optionalID formats id; the zero ID is absent.
func optionalID(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// hostBody converts the view into the JSON form of the schema Host with every member.
func hostBody(v terminal.HostView) hostJSON {
	out := hostJSON{
		ID: v.ID.String(), Name: v.Name, State: v.State, DisabledReason: v.DisabledReason, Platform: v.Platform,
		AssignedUserID: optionalID(v.AssignedUserID), LLMCredentials: v.LLMCredentials, CredentialHolderID: optionalID(v.CredentialHolderID),
		DeclaredServerHost: v.DeclaredServerHost, ReportedIsServerHost: v.ReportedIsServerHost, KeyFingerprint: v.KeyFingerprint,
		KeyGeneration: v.KeyGeneration, Profiles: []profileJSON{}, Doctor: []doctorJSON{}, RequireLocalConfirm: v.RequireLocalConfirm,
		Connected: v.Connected, HostVersion: v.HostVersion, OSVersion: v.OSVersion, LastSeenAt: stamp(v.LastSeenAt),
		CreatedAt: stamp(v.CreatedAt), Version: v.Version,
	}
	if len(v.NoisePublicKey) > 0 {
		out.NoisePublicKey = base64.RawURLEncoding.EncodeToString(v.NoisePublicKey)
	}
	for _, p := range v.Profiles {
		out.Profiles = append(out.Profiles, profileJSON(p))
	}
	for _, c := range v.Doctor {
		out.Doctor = append(out.Doctor, doctorJSON(c))
	}
	for _, u := range v.SessionUsers {
		out.SessionUsers = append(out.SessionUsers, sessionUserJSON{UserID: u.UserID.String(), OSUser: u.OSUser, CreatedAt: stamp(u.CreatedAt)})
	}
	if v.ReportedSessionUsers != nil {
		list := make([]reportedJSON, 0, len(v.ReportedSessionUsers))
		for _, u := range v.ReportedSessionUsers {
			list = append(list, reportedJSON{OSUser: u.OSUser, Workdirs: u.Workdirs})
		}
		out.ReportedSessionUsers = &list
	}
	return out
}

// eventHostBody is hostBody without noise_public_key: host.updated never carries a key (05
// section 6, ruling of M2-39); clients read keys with getHost. The fingerprint is a digest, not a
// key, and stays so clients can warn.
func eventHostBody(v terminal.HostView) hostJSON {
	out := hostBody(v)
	out.NoisePublicKey = ""
	return out
}
