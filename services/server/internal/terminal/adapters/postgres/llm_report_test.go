// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
)

// TestRepository_LlmCredentialsAndReport (M2-40): the declaration of the LLM login is written
// with the next version and the holder only with personal; the personal hosts are listed in
// ascending order; the reported session users are written as host_reported_session_users.v1
// (missing workdirs as an empty list); the valid assignments of a person come with the report of
// their host, ended ones and a zero person give nothing; writes outside a unit of work and on an
// unknown host fail.
func TestRepository_LlmCredentialsAndReport(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	robin, alex := e.person(t, "robin"), e.person(t, "alex")
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	r1, err := terminaltest.InsertHost(ctx, e.owner, terminaltest.Host{Name: "Lab", State: "active", AssignedUserID: alex, CreatedBy: robin,
		ReportedSessionUsers: []string{"bothy-s-alex"}})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := terminaltest.InsertHost(ctx, e.owner, terminaltest.Host{Name: "Desk", State: "active", CreatedBy: robin,
		LLMCredentials: domain.LLMPersonal, CredentialHolderID: robin})
	if err != nil {
		t.Fatal(err)
	}

	personal := domain.Declaration{Credentials: domain.LLMPersonal, HolderID: alex}
	if err := e.repo.UpdateLlmCredentials(ctx, r1, personal, robin, at, 2); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("declaration outside a unit of work: %v", err)
	}
	if err := e.write(func(ctx context.Context) error {
		return e.repo.UpdateLlmCredentials(ctx, ids.New(), personal, robin, at, 2)
	}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("declaration on an unknown host: %v", err)
	}
	if err := e.write(func(ctx context.Context) error { return e.repo.UpdateLlmCredentials(ctx, r1, personal, robin, at, 2) }); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := e.repo.Host(ctx, r1); got.LLMCredentials != domain.LLMPersonal || got.CredentialHolderID != alex || got.Version != 2 {
		t.Fatalf("after personal %+v", got)
	}
	hosts, err := e.repo.PersonalHosts(ctx)
	if err != nil || len(hosts) != 2 || hosts[0].ID.String() >= hosts[1].ID.String() {
		t.Fatalf("PersonalHosts %v %v", hosts, err)
	}
	// A holder that comes with organization is not stored.
	org := domain.Declaration{Credentials: domain.LLMOrganization, HolderID: alex}
	if err := e.write(func(ctx context.Context) error { return e.repo.UpdateLlmCredentials(ctx, r1, org, robin, at, 3) }); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := e.repo.Host(ctx, r1); got.LLMCredentials != domain.LLMOrganization || !got.CredentialHolderID.IsZero() || got.Version != 3 {
		t.Fatalf("after organization %+v", got)
	}
	if hosts, err := e.repo.PersonalHosts(ctx); err != nil || len(hosts) != 1 || hosts[0].ID != r2 {
		t.Fatalf("PersonalHosts after organization %v %v", hosts, err)
	}

	report := []domain.ReportedSessionUser{{OSUser: "bothy-s-alex"}, {OSUser: "bothy-s-sam", Workdirs: []string{"/srv/a"}}}
	if err := e.repo.UpdateReportedSessionUsers(ctx, r1, report, 4); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("report outside a unit of work: %v", err)
	}
	if err := e.write(func(ctx context.Context) error { return e.repo.UpdateReportedSessionUsers(ctx, ids.New(), report, 4) }); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("report on an unknown host: %v", err)
	}
	if err := e.write(func(ctx context.Context) error { return e.repo.UpdateReportedSessionUsers(ctx, r1, report, 4) }); err != nil {
		t.Fatal(err)
	}
	got, _, _ := e.repo.Host(ctx, r1)
	if got.Version != 4 || !domain.SameReport(got.ReportedUsers, report) || got.ReportedUsers[0].Workdirs == nil {
		t.Fatalf("after the report %+v", got)
	}

	if _, err := terminaltest.InsertSessionUser(ctx, e.owner, terminaltest.SessionUser{HostID: r1, UserID: alex, OSUser: "bothy-s-alex", CreatedBy: robin}); err != nil {
		t.Fatal(err)
	}
	if _, err := terminaltest.InsertSessionUser(ctx, e.owner, terminaltest.SessionUser{HostID: r2, UserID: alex, OSUser: "bothy-s-old", CreatedBy: robin, RevokedBy: robin}); err != nil {
		t.Fatal(err)
	}
	own, err := e.repo.OwnSessionUsers(ctx, alex)
	if err != nil || len(own) != 1 || own[0].HostID != r1 || own[0].OSUser != "bothy-s-alex" || !domain.SameReport(own[0].HostReport, report) {
		t.Fatalf("OwnSessionUsers %+v %v", own, err)
	}
	if own, err := e.repo.OwnSessionUsers(ctx, ids.ID{}); err != nil || own != nil {
		t.Fatalf("OwnSessionUsers of nobody %v %v", own, err)
	}
}
