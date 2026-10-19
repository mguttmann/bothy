// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// behaviorLimits stand in for channel.prompt_max_bytes and channel.history_backfill_limit_max;
// the tests derive every boundary from them, never from literals.
var behaviorLimits = domain.BehaviorLimits{PromptMaxBytes: 16, BackfillMax: 50}

// fieldCodes returns pointer:code of every field error, in order.
func fieldCodes(fs []errs.FieldError) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Pointer+":"+f.Code)
	}
	return out
}

// TestValidateBehavior: the rules of design note 5 of M2-02 with limits from BehaviorLimits.
func TestValidateBehavior(t *testing.T) {
	t.Parallel()
	l := behaviorLimits
	ok := domain.Behavior{HistoryBackfillLimit: 1, Prompt: "Be kind.", Skills: []string{"support-triage", "faq"}}
	for _, c := range []struct {
		name string
		edit func(*domain.Behavior)
		want []string
	}{
		{"valid", func(*domain.Behavior) {}, nil},
		{"backfill zero", func(b *domain.Behavior) { b.HistoryBackfillLimit = 0 }, nil},
		{"backfill at maximum", func(b *domain.Behavior) { b.HistoryBackfillLimit = l.BackfillMax }, nil},
		{"backfill above maximum", func(b *domain.Behavior) { b.HistoryBackfillLimit = l.BackfillMax + 1 }, []string{"/history_backfill_limit:too_large"}},
		{"backfill below zero", func(b *domain.Behavior) { b.HistoryBackfillLimit = -1 }, []string{"/history_backfill_limit:too_small"}},
		{"prompt at limit", func(b *domain.Behavior) { b.Prompt = strings.Repeat("a", l.PromptMaxBytes) }, nil},
		{"prompt one byte above", func(b *domain.Behavior) { b.Prompt = strings.Repeat("a", l.PromptMaxBytes+1) }, []string{"/prompt:too_long"}},
		{"prompt multi-byte above", func(b *domain.Behavior) { b.Prompt = strings.Repeat("ä", l.PromptMaxBytes/2) + "a" }, []string{"/prompt:too_long"}},
		{"prompt with newlines and tabs", func(b *domain.Behavior) { b.Prompt = "one\n\ttwo" }, nil},
		{"empty prompt", func(b *domain.Behavior) { b.Prompt = "" }, nil},
		{"no skills", func(b *domain.Behavior) { b.Skills = []string{} }, nil},
		{"empty skill", func(b *domain.Behavior) { b.Skills = []string{"a", ""} }, []string{"/skills/1:required"}},
		{"control character", func(b *domain.Behavior) { b.Skills = []string{"a\x01b"} }, []string{"/skills/0:invalid_characters"}},
		{"C1 control", func(b *domain.Behavior) { b.Skills = []string{"a\u0085b"} }, []string{"/skills/0:invalid_characters"}},
		{"bidi control", func(b *domain.Behavior) { b.Skills = []string{"a‮b"} }, []string{"/skills/0:invalid_characters"}},
		{"invalid UTF-8", func(b *domain.Behavior) { b.Skills = []string{"a\xffb"} }, []string{"/skills/0:invalid_characters"}},
		{"duplicate", func(b *domain.Behavior) { b.Skills = []string{"x", "y", "x"} }, []string{"/skills/2:duplicate"}},
		{"several failures together", func(b *domain.Behavior) {
			b.HistoryBackfillLimit = l.BackfillMax + 1
			b.Prompt = strings.Repeat("a", l.PromptMaxBytes+1)
			b.Skills = []string{"", "x", "x"}
		}, []string{"/history_backfill_limit:too_large", "/prompt:too_long", "/skills/0:required", "/skills/2:duplicate"}},
	} {
		b := ok
		b.Skills = slices.Clone(ok.Skills)
		c.edit(&b)
		got := fieldCodes(domain.ValidateBehavior(b, l))
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// TestValidateBehavior_NoInputInDetail: no field error repeats the prompt or a skill reference
// (06 section 5 point 3, R-AZ-09).
func TestValidateBehavior_NoInputInDetail(t *testing.T) {
	t.Parallel()
	prompt := "SECRETPROMPTTEXT" + strings.Repeat("p", behaviorLimits.PromptMaxBytes)
	b := domain.Behavior{HistoryBackfillLimit: 0, Prompt: prompt, Skills: []string{"SKILLREF\x01", "SKILLDUP", "SKILLDUP"}}
	for _, f := range domain.ValidateBehavior(b, behaviorLimits) {
		if strings.Contains(f.Detail, "SECRETPROMPT") || strings.Contains(f.Detail, "SKILL") {
			t.Errorf("detail %q repeats the input", f.Detail)
		}
	}
}

// TestChangedFields: every field alone and combined, in the closed order of the codes; the order
// of the skills counts.
func TestChangedFields(t *testing.T) {
	t.Parallel()
	base := domain.Behavior{RequireMention: true, AutoThread: true, HistoryBackfillLimit: 20, Prompt: "p", Skills: []string{"a", "b"}}
	for _, c := range []struct {
		name string
		edit func(*domain.Behavior)
		want []string
	}{
		{"none", func(*domain.Behavior) {}, nil},
		{"require_mention", func(b *domain.Behavior) { b.RequireMention = false }, []string{"require_mention"}},
		{"thread_require_mention", func(b *domain.Behavior) { b.ThreadRequireMention = true }, []string{"thread_require_mention"}},
		{"free_response", func(b *domain.Behavior) { b.FreeResponse = true }, []string{"free_response"}},
		{"auto_thread", func(b *domain.Behavior) { b.AutoThread = false }, []string{"auto_thread"}},
		{"allow_mentioned_agents", func(b *domain.Behavior) { b.AllowMentionedAgents = true }, []string{"allow_mentioned_agents"}},
		{"history_backfill_limit", func(b *domain.Behavior) { b.HistoryBackfillLimit = 0 }, []string{"history_backfill_limit"}},
		{"prompt", func(b *domain.Behavior) { b.Prompt = "q" }, []string{"prompt"}},
		{"skills order", func(b *domain.Behavior) { b.Skills = []string{"b", "a"} }, []string{"skills"}},
		{"skills added", func(b *domain.Behavior) { b.Skills = []string{"a", "b", "c"} }, []string{"skills"}},
		{"skills cleared", func(b *domain.Behavior) { b.Skills = nil }, []string{"skills"}},
		{"all", func(b *domain.Behavior) {
			*b = domain.Behavior{ThreadRequireMention: true, FreeResponse: true, AllowMentionedAgents: true, HistoryBackfillLimit: 1, Prompt: "x", Skills: []string{"c"}}
		}, []string{"require_mention", "thread_require_mention", "free_response", "auto_thread", "allow_mentioned_agents", "history_backfill_limit", "prompt", "skills"}},
	} {
		b := base
		b.Skills = slices.Clone(base.Skills)
		c.edit(&b)
		if got := domain.ChangedFields(base, b); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	empty := domain.Behavior{Skills: []string{}}
	if got := domain.ChangedFields(empty, domain.Behavior{}); len(got) != 0 {
		t.Errorf("nil versus empty skills: %v, want no change", got)
	}
}

// TestConfigChanged: prompt or skills change the agent configuration; switches and backfill alone
// do not.
func TestConfigChanged(t *testing.T) {
	t.Parallel()
	base := domain.Behavior{RequireMention: true, HistoryBackfillLimit: 20, Prompt: "p", Skills: []string{"a"}}
	for _, c := range []struct {
		name string
		edit func(*domain.Behavior)
		want bool
	}{
		{"prompt", func(b *domain.Behavior) { b.Prompt = "q" }, true},
		{"skills", func(b *domain.Behavior) { b.Skills = []string{"b"} }, true},
		{"switches", func(b *domain.Behavior) { b.RequireMention, b.FreeResponse, b.AutoThread = false, true, true }, false},
		{"backfill", func(b *domain.Behavior) { b.HistoryBackfillLimit = 3 }, false},
		{"nothing", func(*domain.Behavior) {}, false},
	} {
		b := base
		b.Skills = slices.Clone(base.Skills)
		c.edit(&b)
		if got := domain.ConfigChanged(base, b); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// FuzzValidateBehavior: arbitrary prompt bytes and skill lists never panic, and every accepted
// input keeps the limits and the skill rules.
func FuzzValidateBehavior(f *testing.F) {
	f.Add("prompt", "a\x00b", "a", 3)
	f.Add(strings.Repeat("x", 40), "", "‮", -1)
	f.Add("\xff\xfe", "dup", "dup", 1000)
	f.Fuzz(func(t *testing.T, prompt, s1, s2 string, backfill int) {
		b := domain.Behavior{HistoryBackfillLimit: backfill, Prompt: prompt, Skills: []string{s1, s2}}
		if len(domain.ValidateBehavior(b, behaviorLimits)) > 0 {
			return
		}
		if backfill < 0 || backfill > behaviorLimits.BackfillMax || len(prompt) > behaviorLimits.PromptMaxBytes {
			t.Fatalf("accepted out of range: backfill %d, prompt %d bytes", backfill, len(prompt))
		}
		for _, s := range b.Skills {
			if s == "" || !utf8.ValidString(s) || strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || (r >= 0x7f && r <= 0x9f) || r == 0x202e }) {
				t.Fatalf("accepted an invalid skill reference")
			}
		}
		if s1 == s2 {
			t.Fatalf("accepted a duplicate skill reference")
		}
	})
}
