// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// wantCode checks that err is validation_failed with one field error of code on pointer.
func wantCode(t *testing.T, err error, pointer, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer || e.Fields[0].Code != code {
		t.Fatalf("error %v (%+v), want validation_failed %s on %s", err, e, code, pointer)
	}
}

// limits are the text limits of the tests, standing in for the configuration keys.
var limits = domain.Limits{ChannelName: 8, ChannelTopic: 12, TagName: 5}

// TestChannelName_TrimAndEmpty: surrounding white space is trimmed; only white space is refused.
func TestChannelName_TrimAndEmpty(t *testing.T) {
	t.Parallel()
	if got, err := domain.ChannelName("  tickets ", limits.ChannelName); err != nil || got != "tickets" {
		t.Fatalf("ChannelName = %q, %v", got, err)
	}
	for _, n := range []string{"", "   ", " "} {
		_, err := domain.ChannelName(n, limits.ChannelName)
		wantCode(t, err, "/name", domain.FieldRequired)
	}
}

// TestSEC_T07_ChannelAndTagNamesRejectControlAndBidi: every C0 and C1 control and every bidi
// control is refused in a channel name, a topic and a tag name (SEC-IN-4).
func TestSEC_T07_ChannelAndTagNamesRejectControlAndBidi(t *testing.T) {
	t.Parallel()
	var bad []rune
	for r := rune(0); r < 0x20; r++ {
		bad = append(bad, r)
	}
	for r := rune(0x7f); r <= 0x9f; r++ {
		bad = append(bad, r)
	}
	bad = append(bad, 0x061c, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069)
	big := domain.Limits{ChannelName: 100, ChannelTopic: 100, TagName: 100}
	for _, r := range bad {
		s := "ab" + string(r) + "c"
		if _, err := domain.ChannelName(s, big.ChannelName); err == nil {
			t.Errorf("name with %U accepted", r)
		} else {
			wantPointer(t, err, "/name")
		}
		if _, err := domain.ChannelTopic(s, big.ChannelTopic); err == nil {
			t.Errorf("topic with %U accepted", r)
		} else {
			wantPointer(t, err, "/topic")
		}
		if _, err := domain.TagName(s, big.TagName); err == nil {
			t.Errorf("tag name with %U accepted", r)
		} else {
			wantPointer(t, err, "/name")
		}
	}
}

// wantPointer checks that err is validation_failed with one field error on pointer.
func wantPointer(t *testing.T, err error, pointer string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer {
		t.Fatalf("error %v, want validation_failed on %s", err, pointer)
	}
	if strings.ContainsAny(e.Fields[0].Detail, "\u202e\x00") {
		t.Fatalf("detail repeats the input: %q", e.Fields[0].Detail)
	}
}

// TestCreateChannel_ForumSortOnlyForForum: forum_default_sort only with kind forum; dm cannot be
// created; an unknown sort or kind is refused.
func TestCreateChannel_ForumSortOnlyForForum(t *testing.T) {
	t.Parallel()
	sort := "created"
	if err := domain.CheckCreateKind(domain.ChannelForum, &sort); err != nil {
		t.Fatal(err)
	}
	if err := domain.CheckCreateKind(domain.ChannelForum, nil); err != nil {
		t.Fatal(err)
	}
	if err := domain.CheckCreateKind(domain.ChannelText, nil); err != nil {
		t.Fatal(err)
	}
	wantCode(t, domain.CheckCreateKind(domain.ChannelText, &sort), "/forum_default_sort", domain.FieldNotAllowedForKind)
	wantCode(t, domain.CheckCreateKind(domain.ChannelDM, nil), "/kind", domain.FieldInvalid)
	odd := "newest"
	wantCode(t, domain.CheckCreateKind(domain.ChannelForum, &odd), "/forum_default_sort", domain.FieldInvalid)
}

// TestChannelAndTagTextLimits: a value of exactly the limit passes, one character more is
// too_long on its pointer; characters are Unicode characters.
func TestChannelAndTagTextLimits(t *testing.T) {
	t.Parallel()
	at := func(n int) string { return strings.Repeat("ü", n) }
	if _, err := domain.ChannelName(at(limits.ChannelName), limits.ChannelName); err != nil {
		t.Fatal(err)
	}
	if _, err := domain.ChannelTopic(at(limits.ChannelTopic), limits.ChannelTopic); err != nil {
		t.Fatal(err)
	}
	if _, err := domain.TagName(at(limits.TagName), limits.TagName); err != nil {
		t.Fatal(err)
	}
	_, err := domain.ChannelName(at(limits.ChannelName+1), limits.ChannelName)
	wantCode(t, err, "/name", domain.FieldTooLong)
	_, err = domain.ChannelTopic(at(limits.ChannelTopic+1), limits.ChannelTopic)
	wantCode(t, err, "/topic", domain.FieldTooLong)
	_, err = domain.TagName(at(limits.TagName+1), limits.TagName)
	wantCode(t, err, "/name", domain.FieldTooLong)
	if got, err := domain.ChannelTopic("", limits.ChannelTopic); err != nil || got != "" {
		t.Fatalf("empty topic = %q, %v", got, err)
	}
}

// TestMovePosition_ExactlyOneNeighbour: both neighbours are refused, none means the start, one
// names the neighbour.
func TestMovePosition_ExactlyOneNeighbour(t *testing.T) {
	t.Parallel()
	a, b, self := ids.New(), ids.New(), ids.New()
	order := []domain.Keyed{{ID: a, SortKey: "a0"}, {ID: b, SortKey: "a1"}}
	wantCode(t, domain.MovePosition{AfterID: a, BeforeID: b}.Check(), "/before_id", domain.FieldNotAllowed)
	if i, err := (domain.MovePosition{}).Index(order, self); err != nil || i != -1 {
		t.Fatalf("none = %d, %v", i, err)
	}
	if i, err := (domain.MovePosition{AfterID: b}).Index(order, self); err != nil || i != 1 {
		t.Fatalf("after b = %d, %v", i, err)
	}
	if i, err := (domain.MovePosition{BeforeID: b}).Index(order, self); err != nil || i != 0 {
		t.Fatalf("before b = %d, %v", i, err)
	}
	_, err := domain.MovePosition{AfterID: ids.New()}.Index(order, self)
	wantCode(t, err, "/after_id", domain.FieldInvalid)
	_, err = domain.MovePosition{BeforeID: self}.Index(order, self)
	wantCode(t, err, "/before_id", domain.FieldInvalid)
	key, err := domain.KeyAt(order, 0)
	if err != nil || key <= "a0" || key >= "a1" {
		t.Fatalf("KeyAt(0) = %q, %v", key, err)
	}
	if key, err := domain.KeyAt(order, -1); err != nil || key >= "a0" {
		t.Fatalf("KeyAt(-1) = %q, %v", key, err)
	}
	if key, err := domain.KeyAt(nil, -1); err != nil || key == "" {
		t.Fatalf("KeyAt on empty = %q, %v", key, err)
	}
}

// TestChannelOperatorRepresentation: inherit takes the space default as effective agent, agent
// names its own agent, none has neither; absent agents stay zero.
func TestChannelOperatorRepresentation(t *testing.T) {
	t.Parallel()
	def, own := ids.New(), ids.New()
	for _, c := range []struct {
		name           string
		inherit        bool
		agent, space   ids.ID
		mode           domain.OperatorMode
		wantAgent, eff ids.ID
	}{
		{"inherit with default", true, ids.ID{}, def, domain.OperatorInherit, ids.ID{}, def},
		{"inherit without default", true, ids.ID{}, ids.ID{}, domain.OperatorInherit, ids.ID{}, ids.ID{}},
		{"agent", false, own, def, domain.OperatorAgent, own, own},
		{"none", false, ids.ID{}, def, domain.OperatorNone, ids.ID{}, ids.ID{}},
	} {
		ch := domain.Channel{OperatorInherit: c.inherit, OperatorAgentID: c.agent}
		op := ch.Operator(c.space)
		if op.Mode != c.mode || op.AgentID != c.wantAgent || op.EffectiveAgentID != c.eff {
			t.Errorf("%s: %+v", c.name, op)
		}
	}
}

// TestChannelUpdateAndArchive: only sent fields that differ change with one version step; the
// notification default can be set and cleared but not both; archive sets the time and the
// version.
func TestChannelUpdateAndArchive(t *testing.T) {
	t.Parallel()
	ch := domain.NewChannel(ids.New(), ids.New(), domain.ChannelText, "tickets", "", "a0", ids.New(), 20)
	if ch.Version != 1 || ch.SettingsVersion != 1 || !ch.OperatorInherit || ch.Archived() {
		t.Fatalf("new channel %+v", ch)
	}
	same := "tickets"
	if got := ch.Update(domain.ChannelChange{Name: &same}); len(got) != 0 || ch.Version != 1 {
		t.Fatalf("no change gave %v, version %d", got, ch.Version)
	}
	topic, level := "Customer", "mentions"
	got := ch.Update(domain.ChannelChange{Topic: &topic, DefaultNotificationLevel: &level})
	if strings.Join(got, ",") != "topic,default_notification_level" || ch.Version != 2 {
		t.Fatalf("changed %v, version %d", got, ch.Version)
	}
	got = ch.Update(domain.ChannelChange{ClearDefaultNotificationLevel: true})
	if strings.Join(got, ",") != "default_notification_level" || ch.DefaultNotificationLevel != "" || ch.Version != 3 {
		t.Fatalf("clear %v, %+v", got, ch)
	}
	wantCode(t, domain.ChannelChange{DefaultNotificationLevel: &level, ClearDefaultNotificationLevel: true}.Check(domain.ChannelText),
		"/clear_default_notification_level", domain.FieldNotAllowed)
	wantCode(t, domain.ChannelChange{}.Check(domain.ChannelText), "", domain.FieldRequired)
	sort := "created"
	wantCode(t, domain.ChannelChange{ForumDefaultSort: &sort}.Check(domain.ChannelText), "/forum_default_sort", domain.FieldNotAllowedForKind)
	odd := "loud"
	wantCode(t, domain.ChannelChange{DefaultNotificationLevel: &odd}.Check(domain.ChannelText), "/default_notification_level", domain.FieldInvalid)
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	ch.Archive(at)
	if !ch.Archived() || ch.Version != 4 {
		t.Fatalf("archived %+v", ch)
	}
}

// TestForumTagUpdate: clear_color removes the color and excludes color; at least one field; a
// change reports its fields.
func TestForumTagUpdate(t *testing.T) {
	t.Parallel()
	tag := domain.ForumTag{ID: ids.New(), ChannelID: ids.New(), Name: "bug", Color: "moss", SortKey: "a0"}
	color := "rowan"
	wantCode(t, domain.TagChange{Color: &color, ClearColor: true}.Check(), "/clear_color", domain.FieldNotAllowedWithColor)
	wantCode(t, domain.TagChange{}.Check(), "", domain.FieldRequired)
	if got := tag.Update(domain.TagChange{ClearColor: true}); strings.Join(got, ",") != "color" || tag.Color != "" {
		t.Fatalf("clear %v, %+v", got, tag)
	}
	yes := true
	if got := tag.Update(domain.TagChange{Restricted: &yes}); strings.Join(got, ",") != "restricted" || !tag.Restricted {
		t.Fatalf("restricted %v, %+v", got, tag)
	}
	if got := tag.Update(domain.TagChange{Restricted: &yes}); len(got) != 0 {
		t.Fatalf("no change gave %v", got)
	}
}
