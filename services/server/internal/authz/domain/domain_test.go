// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func TestLevelOrderAndParse(t *testing.T) {
	t.Parallel()
	if domain.LevelNone >= domain.LevelRead || domain.LevelRead >= domain.LevelWrite || domain.LevelWrite >= domain.LevelTrigger {
		t.Fatal("levels are not ordered none < read < write < trigger")
	}
	for _, tt := range []struct {
		in   string
		want domain.Level
	}{{"read", domain.LevelRead}, {"write", domain.LevelWrite}, {"trigger", domain.LevelTrigger}} {
		got, err := domain.ParseLevel(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
		if got.String() != tt.in {
			t.Errorf("String() = %q, want %q", got.String(), tt.in)
		}
	}
	for _, bad := range []string{"", "none", "Read", "admin", " read"} {
		if _, err := domain.ParseLevel(bad); err == nil {
			t.Errorf("ParseLevel(%q) passed", bad)
		}
	}
	if domain.LevelNone.String() != "" {
		t.Errorf("LevelNone.String() = %q, want empty", domain.LevelNone.String())
	}
	if domain.MaxLevel(domain.LevelRead, domain.LevelTrigger) != domain.LevelTrigger ||
		domain.MaxLevel(domain.LevelWrite, domain.LevelNone) != domain.LevelWrite {
		t.Error("MaxLevel")
	}
	if domain.MinLevel(domain.LevelRead, domain.LevelTrigger) != domain.LevelRead ||
		domain.MinLevel(domain.LevelWrite, domain.LevelNone) != domain.LevelNone {
		t.Error("MinLevel")
	}
}

func TestCapsSetOperations(t *testing.T) {
	t.Parallel()
	all, err := domain.ParseCaps([]string{"terminal", "manage", "export", "upload", "approve_actions"})
	if err != nil {
		t.Fatal(err)
	}
	if got := all.Names(); !reflect.DeepEqual(got, []string{"approve_actions", "export", "manage", "terminal", "upload"}) {
		t.Errorf("Names() = %q, want sorted", got)
	}
	for _, c := range []domain.Capability{domain.CapApproveActions, domain.CapUpload, domain.CapExport, domain.CapManage, domain.CapTerminal} {
		if !all.Has(c) {
			t.Errorf("all lacks %v", c)
		}
	}
	dup, err := domain.ParseCaps([]string{"upload", "upload", "manage"})
	if err != nil {
		t.Fatal(err)
	}
	if got := dup.Names(); !reflect.DeepEqual(got, []string{"manage", "upload"}) {
		t.Errorf("duplicates did not collapse: %q", got)
	}
	if _, err := domain.ParseCaps([]string{"upload", "admin"}); err == nil {
		t.Error("unknown capability passed")
	}
	if _, err := domain.ParseCaps([]string{"Upload"}); err == nil {
		t.Error("upper case capability passed")
	}
	empty, err := domain.ParseCaps(nil)
	if err != nil || empty != 0 || len(empty.Names()) != 0 || empty.Names() == nil {
		t.Errorf("ParseCaps(nil) = %v, %v, names %#v; want empty set with a non-nil empty slice", empty, err, empty.Names())
	}
	upload, _ := domain.ParseCaps([]string{"upload"})
	manage, _ := domain.ParseCaps([]string{"manage"})
	both := upload.Union(manage)
	if !both.Has(domain.CapUpload) || !both.Has(domain.CapManage) || both.Has(domain.CapExport) {
		t.Errorf("Union = %q", both.Names())
	}
	if both.Intersect(upload) != upload || upload.Intersect(manage) != 0 {
		t.Error("Intersect")
	}
	if !upload.SubsetOf(both) || both.SubsetOf(upload) || !domain.Caps(0).SubsetOf(upload) || !both.SubsetOf(both) {
		t.Error("SubsetOf")
	}
}

func TestGrantInEffect(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		g    domain.Grant
		want bool
	}{
		{"revoked", domain.Grant{RevokedAt: t0.Add(-time.Hour)}, false},
		{"expires one second after t", domain.Grant{ExpiresAt: t0.Add(time.Second)}, true},
		{"expires exactly at t", domain.Grant{ExpiresAt: t0}, false},
		{"expired", domain.Grant{ExpiresAt: t0.Add(-time.Hour)}, false},
		{"no expiry", domain.Grant{}, true},
		{"revoked in the future still counts as revoked", domain.Grant{RevokedAt: t0.Add(time.Hour)}, false},
	} {
		if got := tt.g.InEffect(t0); got != tt.want {
			t.Errorf("%s: InEffect = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMembershipInEffect(t *testing.T) {
	t.Parallel()
	live := domain.Group{}
	for _, tt := range []struct {
		name string
		m    domain.Membership
		g    domain.Group
		want bool
	}{
		{"removed", domain.Membership{RemovedAt: t0.Add(-time.Hour)}, live, false},
		{"group deleted", domain.Membership{}, domain.Group{DeletedAt: t0.Add(-time.Hour)}, false},
		{"expiry exactly at t", domain.Membership{ExpiresAt: t0}, live, false},
		{"expired", domain.Membership{ExpiresAt: t0.Add(-time.Second)}, live, false},
		{"expires after t", domain.Membership{ExpiresAt: t0.Add(time.Second)}, live, true},
		{"valid", domain.Membership{}, live, true},
	} {
		if got := tt.m.InEffect(tt.g, t0); got != tt.want {
			t.Errorf("%s: InEffect = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestNextExpiry(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		in   []time.Time
		want time.Time
	}{
		{"none", nil, time.Time{}},
		{"zero values only", []time.Time{{}, {}}, time.Time{}},
		{"past only", []time.Time{t0.Add(-time.Hour), t0.Add(-time.Second)}, time.Time{}},
		{"mixed", []time.Time{t0.Add(3 * time.Hour), {}, t0.Add(-time.Hour), t0.Add(time.Hour), t0.Add(2 * time.Hour)}, t0.Add(time.Hour)},
		{"equal to t ignored", []time.Time{t0, t0.Add(time.Minute)}, t0.Add(time.Minute)},
	} {
		if got := domain.NextExpiry(t0, tt.in...); !got.Equal(tt.want) {
			t.Errorf("%s: NextExpiry = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// caps parses names or fails the test.
func caps(t *testing.T, names ...string) domain.Caps {
	t.Helper()
	c, err := domain.ParseCaps(names)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// wantField checks that err is validation_failed with exactly one field error on pointer.
func wantField(t *testing.T, name string, err error, pointer string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok {
		t.Errorf("%s: error %v is no *errs.Error", name, err)
		return
	}
	if e.Code != errs.CodeValidationFailed || e.Kind() != errs.KindBadRequest {
		t.Errorf("%s: code %q kind %v, want validation_failed bad_request", name, e.Code, e.Kind())
	}
	if len(e.Fields) != 1 || e.Fields[0].Pointer != pointer {
		t.Errorf("%s: fields %+v, want one on %s", name, e.Fields, pointer)
	}
}

func TestNewGrantShape(t *testing.T) {
	t.Parallel()
	p, g, sp, ch, h, by := ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	user := domain.Subject{Kind: domain.SubjectUser, PrincipalID: p}
	group := domain.Subject{Kind: domain.SubjectGroup, GroupID: g}
	agent := domain.Subject{Kind: domain.SubjectAgent, PrincipalID: p}
	inst := domain.Target{Kind: domain.TargetInstance}
	space := domain.Target{Kind: domain.TargetSpace, SpaceID: sp}
	channel := domain.Target{Kind: domain.TargetChannel, ChannelID: ch}
	host := domain.Target{Kind: domain.TargetHost, HostID: h}
	term := caps(t, "terminal")

	valid := []struct {
		name string
		s    domain.Subject
		tg   domain.Target
		l    domain.Level
		c    domain.Caps
		exp  time.Time
	}{
		{"user on instance", user, inst, domain.LevelRead, 0, time.Time{}},
		{"group on space with manage", group, space, domain.LevelTrigger, caps(t, "manage", "export"), t0.Add(time.Hour)},
		{"agent on channel with upload", agent, channel, domain.LevelWrite, caps(t, "upload"), time.Time{}},
		// The bot limits are CheckCreate's and the database's to judge, not NewGrant's.
		{"agent with trigger passes the shape check", agent, channel, domain.LevelTrigger, caps(t, "manage"), time.Time{}},
		{"user on host", user, host, domain.LevelNone, term, time.Time{}},
		{"group on host", group, host, domain.LevelNone, term, time.Time{}},
	}
	for _, tt := range valid {
		gr, err := domain.NewGrant(ids.New(), tt.s, tt.tg, tt.l, tt.c, tt.exp, "project work", by, t0)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if gr.Subject != tt.s || gr.Target != tt.tg || gr.Level != tt.l || gr.Caps != tt.c ||
			!gr.ExpiresAt.Equal(tt.exp) || gr.Reason != "project work" || gr.GrantedBy != by || !gr.GrantedAt.Equal(t0) {
			t.Errorf("%s: grant %+v", tt.name, gr)
		}
	}

	invalid := []struct {
		name    string
		s       domain.Subject
		tg      domain.Target
		l       domain.Level
		c       domain.Caps
		exp     time.Time
		reason  string
		pointer string
	}{
		{"unknown subject kind", domain.Subject{Kind: "robot", PrincipalID: p}, inst, domain.LevelRead, 0, time.Time{}, "r", "/subject"},
		{"user without principal", domain.Subject{Kind: domain.SubjectUser}, inst, domain.LevelRead, 0, time.Time{}, "r", "/subject"},
		{"user with group id", domain.Subject{Kind: domain.SubjectUser, PrincipalID: p, GroupID: g}, inst, domain.LevelRead, 0, time.Time{}, "r", "/subject"},
		{"group without group id", domain.Subject{Kind: domain.SubjectGroup}, inst, domain.LevelRead, 0, time.Time{}, "r", "/subject"},
		{"group with principal", domain.Subject{Kind: domain.SubjectGroup, GroupID: g, PrincipalID: p}, inst, domain.LevelRead, 0, time.Time{}, "r", "/subject"},
		{"app without principal", domain.Subject{Kind: domain.SubjectApp}, inst, domain.LevelRead, 0, time.Time{}, "r", "/subject"},
		{"unknown target kind", user, domain.Target{Kind: "thread"}, domain.LevelRead, 0, time.Time{}, "r", "/target"},
		{"instance with channel id", user, domain.Target{Kind: domain.TargetInstance, ChannelID: ch}, domain.LevelRead, 0, time.Time{}, "r", "/target"},
		{"space without space id", user, domain.Target{Kind: domain.TargetSpace}, domain.LevelRead, 0, time.Time{}, "r", "/target"},
		{"space with channel id too", user, domain.Target{Kind: domain.TargetSpace, SpaceID: sp, ChannelID: ch}, domain.LevelRead, 0, time.Time{}, "r", "/target"},
		{"channel without channel id", user, domain.Target{Kind: domain.TargetChannel, SpaceID: sp}, domain.LevelRead, 0, time.Time{}, "r", "/target"},
		{"host without host id", user, domain.Target{Kind: domain.TargetHost}, domain.LevelNone, term, time.Time{}, "r", "/target"},
		{"host with space id too", user, domain.Target{Kind: domain.TargetHost, HostID: h, SpaceID: sp}, domain.LevelNone, term, time.Time{}, "r", "/target"},
		{"no level on space", user, space, domain.LevelNone, 0, time.Time{}, "r", "/level"},
		{"level out of range", user, space, domain.Level(9), 0, time.Time{}, "r", "/level"},
		{"level on host", user, host, domain.LevelRead, term, time.Time{}, "r", "/level"},
		{"host without terminal", user, host, domain.LevelNone, 0, time.Time{}, "r", "/capabilities"},
		{"host with upload", user, host, domain.LevelNone, caps(t, "terminal", "upload"), time.Time{}, "r", "/capabilities"},
		{"host to an agent", agent, host, domain.LevelNone, term, time.Time{}, "r", "/subject"},
		{"terminal on a space", user, space, domain.LevelWrite, caps(t, "terminal"), time.Time{}, "r", "/capabilities"},
		{"unknown capability bit", user, space, domain.LevelWrite, domain.Caps(1 << 7), time.Time{}, "r", "/capabilities"},
		{"expiry at granted_at", user, space, domain.LevelRead, 0, t0, "r", "/expires_at"},
		{"expiry before granted_at", user, space, domain.LevelRead, 0, t0.Add(-time.Second), "r", "/expires_at"},
		{"empty reason", user, space, domain.LevelRead, 0, time.Time{}, "", "/reason"},
		{"blank reason", user, space, domain.LevelRead, 0, time.Time{}, " \t\u200b", "/reason"},
		{"reason with a bidi control", user, space, domain.LevelRead, 0, time.Time{}, "ok\u202e", "/reason"},
	}
	for _, tt := range invalid {
		_, err := domain.NewGrant(ids.New(), tt.s, tt.tg, tt.l, tt.c, tt.exp, tt.reason, by, t0)
		wantField(t, tt.name, err, tt.pointer)
	}

	// Missing ids of the caller are programming errors, not input errors.
	for name, fn := range map[string]func() error{
		"zero id": func() error {
			_, err := domain.NewGrant(ids.ID{}, user, inst, domain.LevelRead, 0, time.Time{}, "r", by, t0)
			return err
		},
		"zero grantor": func() error {
			_, err := domain.NewGrant(ids.New(), user, inst, domain.LevelRead, 0, time.Time{}, "r", ids.ID{}, t0)
			return err
		},
		"zero time": func() error {
			_, err := domain.NewGrant(ids.New(), user, inst, domain.LevelRead, 0, time.Time{}, "r", by, time.Time{})
			return err
		},
	} {
		if errs.KindOf(fn()) != errs.KindInternal {
			t.Errorf("%s: want kind internal", name)
		}
	}
}

func TestNewGroup(t *testing.T) {
	t.Parallel()
	limits := domain.GroupLimits{NameMaxChars: 10, DescriptionMaxChars: 20}
	by := ids.New()
	g, err := domain.NewGroup(ids.New(), "Support", "", limits, by, t0)
	if err != nil {
		t.Fatal(err)
	}
	if g.Source != domain.GroupSourceLocal || g.Name != "Support" || g.CreatedBy != by || !g.CreatedAt.Equal(t0) {
		t.Errorf("group %+v", g)
	}
	for _, tt := range []struct {
		name, gname, desc, pointer string
	}{
		{"empty name", "", "", "/name"},
		{"blank name", "  ", "", "/name"},
		{"name too long", "Engineering", "", "/name"},
		{"name with control", "Sup\x07port", "", "/name"},
		{"description too long", "Research", strings.Repeat("x", 21), "/description"},
		{"description with bidi", "Research", "a\u202eb", "/description"},
	} {
		_, err := domain.NewGroup(ids.New(), tt.gname, tt.desc, limits, by, t0)
		wantField(t, tt.name, err, tt.pointer)
	}
	if _, err := domain.NewGroup(ids.ID{}, "Support", "", limits, by, t0); errs.KindOf(err) != errs.KindInternal {
		t.Error("zero id: want kind internal")
	}
}

func TestNewMembership(t *testing.T) {
	t.Parallel()
	g, u, by := ids.New(), ids.New(), ids.New()
	m, err := domain.NewMembership(ids.New(), g, u, by, t0.Add(time.Hour), t0)
	if err != nil {
		t.Fatal(err)
	}
	if m.GroupID != g || m.UserID != u || m.AddedBy != by || !m.AddedAt.Equal(t0) || !m.ExpiresAt.Equal(t0.Add(time.Hour)) {
		t.Errorf("membership %+v", m)
	}
	_, err = domain.NewMembership(ids.New(), g, u, by, t0, t0)
	wantField(t, "expiry at added_at", err, "/expires_at")
	if _, err := domain.NewMembership(ids.New(), ids.ID{}, u, by, time.Time{}, t0); errs.KindOf(err) != errs.KindInternal {
		t.Error("zero group: want kind internal")
	}
}

func TestCheckCreate(t *testing.T) {
	t.Parallel()
	owner := domain.Delegator{IsOwner: true}
	writeManage := domain.Delegator{Level: domain.LevelWrite, Caps: caps(t, "manage")}
	triggerManage := domain.Delegator{Level: domain.LevelTrigger, Caps: caps(t, "manage")}
	for _, tt := range []struct {
		name string
		d    domain.Delegator
		g    domain.GrantShape
		want domain.Reason
	}{
		{"AZM31_OwnerTriggerToAgent", owner,
			domain.GrantShape{SubjectKind: domain.SubjectAgent, TargetKind: domain.TargetChannel, Level: domain.LevelTrigger}, domain.ReasonSubjectKindNotAllowed},
		{"OwnerManageToApp", owner,
			domain.GrantShape{SubjectKind: domain.SubjectApp, TargetKind: domain.TargetChannel, Level: domain.LevelRead, Caps: caps(t, "manage")}, domain.ReasonSubjectKindNotAllowed},
		{"OwnerWriteUploadToAgent", owner,
			domain.GrantShape{SubjectKind: domain.SubjectAgent, TargetKind: domain.TargetChannel, Level: domain.LevelWrite, Caps: caps(t, "upload")}, ""},
		{"AZM37_WriteManageGivesTrigger", writeManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetChannel, Level: domain.LevelTrigger}, domain.ReasonDelegationExceedsOwnLevel},
		{"AZM38_CapabilityNotOwned", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetChannel, Level: domain.LevelRead, Caps: caps(t, "export")}, domain.ReasonDelegationExceedsOwnCapabilities},
		{"OwnerOnInstanceTarget", owner,
			domain.GrantShape{SubjectKind: domain.SubjectGroup, TargetKind: domain.TargetInstance, Level: domain.LevelTrigger, Caps: caps(t, "manage", "export")}, ""},
		{"OwnerOnHostTarget", owner,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetHost, Caps: caps(t, "terminal")}, ""},
		{"NonOwnerOnInstanceTarget", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetInstance, Level: domain.LevelRead}, domain.ReasonOwnerOnly},
		{"NonOwnerOnHostTarget", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetHost, Caps: caps(t, "terminal")}, domain.ReasonOwnerOnly},
		{"NonOwnerToAppSubject", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectApp, TargetKind: domain.TargetChannel, Level: domain.LevelRead}, domain.ReasonSubjectKindNotAllowed},
		{"PassingManageDelegationOfManage", writeManage,
			domain.GrantShape{SubjectKind: domain.SubjectGroup, TargetKind: domain.TargetSpace, Level: domain.LevelWrite, Caps: caps(t, "manage")}, ""},
		// Order: rule 1 before rule 3, rule 2 before 3, rule 3 before 4, rule 4 before 5.
		{"Rule1BeforeRule3", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectAgent, TargetKind: domain.TargetInstance, Level: domain.LevelTrigger}, domain.ReasonSubjectKindNotAllowed},
		{"Rule2BeforeRule3", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectAgent, TargetKind: domain.TargetInstance, Level: domain.LevelRead}, domain.ReasonSubjectKindNotAllowed},
		{"Rule3BeforeRule4", writeManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetInstance, Level: domain.LevelTrigger}, domain.ReasonOwnerOnly},
		{"Rule4BeforeRule5", writeManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetChannel, Level: domain.LevelTrigger, Caps: caps(t, "export")}, domain.ReasonDelegationExceedsOwnLevel},
		// Unknown kinds fail closed for non-owners: only people and groups on spaces and channels.
		{"NonOwnerUnknownTargetKind", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: "", Level: domain.LevelRead}, domain.ReasonOwnerOnly},
		{"NonOwnerUnknownSubjectKind", triggerManage,
			domain.GrantShape{SubjectKind: "robot", TargetKind: domain.TargetChannel, Level: domain.LevelRead}, domain.ReasonSubjectKindNotAllowed},
		{"EqualLevelPasses", writeManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetChannel, Level: domain.LevelWrite}, ""},
	} {
		if got := domain.CheckCreate(tt.d, tt.g); got != tt.want {
			t.Errorf("%s: CheckCreate = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestCheckRevoke(t *testing.T) {
	t.Parallel()
	owner := domain.Delegator{IsOwner: true}
	writeManage := domain.Delegator{Level: domain.LevelWrite, Caps: caps(t, "manage")}
	triggerManage := domain.Delegator{Level: domain.LevelTrigger, Caps: caps(t, "manage")}
	for _, tt := range []struct {
		name string
		d    domain.Delegator
		g    domain.GrantShape
		want domain.Reason
	}{
		{"AZM39_RevokeStrongerGrant", writeManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetChannel, Level: domain.LevelTrigger}, domain.ReasonDelegationExceedsOwnLevel},
		{"OwnerRevokesAnything", owner,
			domain.GrantShape{SubjectKind: domain.SubjectAgent, TargetKind: domain.TargetInstance, Level: domain.LevelWrite, Caps: caps(t, "upload")}, ""},
		{"OwnerRevokesHost", owner,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetHost, Caps: caps(t, "terminal")}, ""},
		{"NonOwnerRevokesAgentGrant", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectAgent, TargetKind: domain.TargetChannel, Level: domain.LevelRead}, domain.ReasonSubjectKindNotAllowed},
		{"NonOwnerRevokesInstanceGrant", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetInstance, Level: domain.LevelRead}, domain.ReasonOwnerOnly},
		{"NonOwnerRevokesCapabilityNotOwned", triggerManage,
			domain.GrantShape{SubjectKind: domain.SubjectUser, TargetKind: domain.TargetChannel, Level: domain.LevelRead, Caps: caps(t, "export")}, domain.ReasonDelegationExceedsOwnCapabilities},
		{"NonOwnerRevokesWithinOwnFrame", writeManage,
			domain.GrantShape{SubjectKind: domain.SubjectGroup, TargetKind: domain.TargetChannel, Level: domain.LevelWrite, Caps: caps(t, "manage")}, ""},
	} {
		if got := domain.CheckRevoke(tt.d, tt.g); got != tt.want {
			t.Errorf("%s: CheckRevoke = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestReasonsMatchSpecCodes(t *testing.T) {
	t.Parallel()
	for r, want := range map[domain.Reason]string{
		domain.ReasonSubjectKindNotAllowed:            "subject_kind_not_allowed",
		domain.ReasonOwnerOnly:                        "owner_only",
		domain.ReasonDelegationExceedsOwnLevel:        "delegation_exceeds_own_level",
		domain.ReasonDelegationExceedsOwnCapabilities: "delegation_exceeds_own_capabilities",
	} {
		if string(r) != want {
			t.Errorf("reason %q, want %q", r, want)
		}
	}
}

func TestGrantShapeOf(t *testing.T) {
	t.Parallel()
	g := domain.Grant{Subject: domain.Subject{Kind: domain.SubjectGroup}, Target: domain.Target{Kind: domain.TargetSpace},
		Level: domain.LevelWrite, Caps: caps(t, "manage")}
	want := domain.GrantShape{SubjectKind: domain.SubjectGroup, TargetKind: domain.TargetSpace, Level: domain.LevelWrite, Caps: caps(t, "manage")}
	if got := g.Shape(); got != want {
		t.Errorf("Shape() = %+v, want %+v", got, want)
	}
}
