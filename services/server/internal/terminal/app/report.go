// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// ReportOutcome is the result of RecordSessionUserReport: Changed reports a new report that was
// written; EndedSessions the number of sessions it ended and Ended their ids (M2-43 sends
// session.close for each after the commit).
type ReportOutcome struct {
	Changed       bool
	EndedSessions int
	Ended         []ids.ID
}

// RecordSessionUserReport records the session users a host reports as rights state (03 grants
// paragraph on permission_version, 07 SR12, 24 section 2.2 point 5; Design note 8 of M2-40). The
// cause is the action of the report: host.pair (M2-41) or host.connect (M2-43), which audit their
// own calls; this function writes no audit entry.
//
// Each entry is checked first (the name with NewOSUser and host.os_user_max_chars, the working
// directories as display texts); an invalid report writes nothing and returns validation_failed.
// A report equal to the stored one (SameReport) returns Changed false without a transaction and
// without a rights version, because a host repeats its report after every reconnect. Otherwise
// one unit of work, declared as the rights change cause on the host with the people of its valid
// assignments, runs ApplySessionUserReport. Errors: validation_failed, not_found for an unknown
// host, the errors of the store.
func (s *Service) RecordSessionUserReport(ctx context.Context, cause authorize.Action, hostID ids.ID, report []domain.ReportedSessionUser) (ReportOutcome, error) {
	clean, err := s.checkReport(report)
	if err != nil {
		return ReportOutcome{}, err
	}
	h, found, err := s.d.Store.Host(ctx, hostID)
	if err != nil {
		return ReportOutcome{}, err
	}
	if !found {
		return ReportOutcome{}, invisible()
	}
	if domain.SameReport(h.ReportedUsers, clean) {
		return ReportOutcome{}, nil
	}
	assigned, err := s.d.Store.ValidSessionUsers(ctx, []ids.ID{hostID})
	if err != nil {
		return ReportOutcome{}, err
	}
	var people []ids.ID
	for _, u := range assigned[hostID] {
		people = append(people, u.UserID)
	}
	var out ReportOutcome
	ref := txn.Ref{Kind: resourceHost, ID: hostID}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.ApplySessionUserReport(ctx, hostID, clean)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(cause), Resource: ref, Principals: people, Targets: []txn.Ref{ref}}))
	if errors.Is(err, errUnchanged) {
		return ReportOutcome{}, nil
	}
	return out, err
}

// errUnchanged ends the unit of a report that a concurrent report already wrote.
var errUnchanged = errors.New("terminal: the report is already stored")

// ApplySessionUserReport writes report as the reported session users of the host hostID inside
// the caller's rights change (Design note 8 of M2-40): it locks the host row, compares again
// (equal rolls the unit back without a change), writes reported_session_users with the next
// version, ends with access_revoked the sessions of every person whose assigned name the report
// no longer names (07 SR12) and appends host.updated. M2-41 (pairHost) and M2-43 (report) call it
// with a checked report. Errors: not_found for an unknown host, the errors of the store.
func (s *Service) ApplySessionUserReport(ctx context.Context, hostID ids.ID, report []domain.ReportedSessionUser) (ReportOutcome, error) {
	h, found, err := s.d.Store.LockHost(ctx, hostID)
	if err != nil {
		return ReportOutcome{}, err
	}
	if !found {
		return ReportOutcome{}, invisible()
	}
	if domain.SameReport(h.ReportedUsers, report) {
		return ReportOutcome{}, errUnchanged
	}
	vanished := domain.VanishedNames(h.ReportedUsers, report)
	h.ReportedUsers, h.Version = report, h.Version+1
	if err := s.d.Store.UpdateReportedSessionUsers(ctx, h.ID, report, h.Version); err != nil {
		return ReportOutcome{}, err
	}
	rows, err := s.d.Store.LockSessionUsers(ctx, h.ID)
	if err != nil {
		return ReportOutcome{}, err
	}
	var ended []ids.ID
	for _, r := range rows {
		if r.Revoked || !slices.Contains(vanished, r.OSUser) {
			continue
		}
		list, err := s.endSessions(ctx, SessionSelector{HostID: h.ID, UserID: r.UserID}, domain.CloseAccessRevoked)
		if err != nil {
			return ReportOutcome{}, err
		}
		ended = append(ended, list...)
	}
	st, _, err := s.withUsers(ctx, h)
	if err != nil {
		return ReportOutcome{}, err
	}
	return ReportOutcome{Changed: true, EndedSessions: len(ended), Ended: ended}, s.emitUpdated(ctx, s.view(ctx, st))
}

// checkReport validates a report (schema host_reported_session_users.v1): each name a session user
// name of at most host.os_user_max_chars characters, each working directory a display text
// (SEC-IN-4). It returns the report with empty lists for missing working directories. Errors:
// validation_failed at /session_users/<i>/os_user or /session_users/<i>/workdirs/<j>, never with
// the value.
func (s *Service) checkReport(report []domain.ReportedSessionUser) ([]domain.ReportedSessionUser, error) {
	out := make([]domain.ReportedSessionUser, 0, len(report))
	for i, u := range report {
		prefix := "/session_users/" + strconv.Itoa(i)
		if _, err := domain.NewOSUser(u.OSUser, s.cfg.OSUserMaxChars, prefix+"/os_user"); err != nil {
			return nil, err
		}
		dirs := make([]string, 0, len(u.Workdirs))
		for j, d := range u.Workdirs {
			if err := textcheck.DisplayText(d); err != nil || d == "" {
				return nil, domain.FieldError(prefix+"/workdirs/"+strconv.Itoa(j), domain.FieldInvalid, "the working directory is no display name")
			}
			dirs = append(dirs, d)
		}
		out = append(out, domain.ReportedSessionUser{OSUser: u.OSUser, Workdirs: dirs})
	}
	return out, nil
}
