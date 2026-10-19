// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Field error codes of the behaviour rules (docs/DECISIONS.md D-M2-02-2). FieldRequired,
// FieldTooLong and FieldDuplicate are shared with the other rules of this package.
const (
	// FieldTooSmall marks a number below its minimum (the schema code of M1-18).
	FieldTooSmall = "too_small"
	// FieldTooLarge marks a number above its configured maximum.
	FieldTooLarge = "too_large"
	// FieldInvalidCharacters marks a skill reference with invalid UTF-8, a C0 or C1 control or a
	// bidi control (kernel/textcheck).
	FieldInvalidCharacters = "invalid_characters"
)

// The codes of the changed fields of a behaviour change, in their closed order (audit detail
// changed, design note 7 of M2-02).
const (
	ChangedRequireMention       = "require_mention"
	ChangedThreadRequireMention = "thread_require_mention"
	ChangedFreeResponse         = "free_response"
	ChangedAutoThread           = "auto_thread"
	ChangedAllowMentionedAgents = "allow_mentioned_agents"
	ChangedHistoryBackfillLimit = "history_backfill_limit"
	ChangedPrompt               = "prompt"
	ChangedSkills               = "skills"
)

// Behavior is the behaviour of an operator in a channel (03 section 4.5 channels and
// channel_skill_bindings; 09 "Channel switches"): the mention switches, auto_thread, the context
// window, the channel prompt and the ordered skill references. The server never interprets a
// skill reference; the prompt is a multi-line instruction stored unchanged.
type Behavior struct {
	RequireMention, ThreadRequireMention, FreeResponse, AutoThread, AllowMentionedAgents bool
	// HistoryBackfillLimit is the number of earlier messages a run sees (M2-13).
	HistoryBackfillLimit int
	// Prompt is the channel prompt; never in events, logs, traces, metrics or audit details.
	Prompt string
	// Skills are the ordered skill references; nil and empty are the same list.
	Skills []string
}

// BehaviorLimits are the limits of a behaviour, bound from channel.prompt_max_bytes and
// channel.history_backfill_limit_max at request time.
type BehaviorLimits struct {
	PromptMaxBytes, BackfillMax int
}

// ValidateBehavior checks b against l (design note 5 of M2-02) and returns every failing field in
// the order history_backfill_limit, prompt, skills; nil when b is valid. history_backfill_limit
// lies between 0 and l.BackfillMax; the prompt has at most l.PromptMaxBytes bytes of UTF-8 and is
// otherwise free; each skill reference is non-empty, passes kernel/textcheck and appears once. A
// detail never repeats the input, because prompt and skills are readable only with manage.
//
// Example:
//
//	if fs := domain.ValidateBehavior(b, domain.BehaviorLimits{PromptMaxBytes: 32768, BackfillMax: 200}); len(fs) > 0 {
//		return errs.New(errs.CodeValidationFailed, "the behaviour is not valid").WithFields(fs...)
//	}
func ValidateBehavior(b Behavior, l BehaviorLimits) []errs.FieldError {
	var out []errs.FieldError
	add := func(pointer, code, detail string) {
		out = append(out, errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
	}
	switch {
	case b.HistoryBackfillLimit < 0:
		add("/history_backfill_limit", FieldTooSmall, "history_backfill_limit must not be negative")
	case b.HistoryBackfillLimit > l.BackfillMax:
		add("/history_backfill_limit", FieldTooLarge, "history_backfill_limit is above channel.history_backfill_limit_max")
	}
	if len(b.Prompt) > l.PromptMaxBytes {
		add("/prompt", FieldTooLong, "the prompt is longer than channel.prompt_max_bytes")
	}
	seen := make(map[string]bool, len(b.Skills))
	for i, s := range b.Skills {
		pointer := "/skills/" + strconv.Itoa(i)
		switch {
		case s == "":
			add(pointer, FieldRequired, "a skill reference must not be empty")
		case textcheck.DisplayText(s) != nil:
			add(pointer, FieldInvalidCharacters, "the skill reference contains a forbidden character")
		case seen[s]:
			add(pointer, FieldDuplicate, "the skill reference appears more than once")
		}
		seen[s] = true
	}
	return out
}

// ChangedFields returns the codes of the fields that differ between old and new, in the closed
// order require_mention, thread_require_mention, free_response, auto_thread,
// allow_mentioned_agents, history_backfill_limit, prompt, skills. The prompt compares byte by
// byte, the skills in order; nil and empty skill lists are equal.
func ChangedFields(old, new Behavior) []string {
	var out []string
	for _, f := range []struct {
		code    string
		changed bool
	}{
		{ChangedRequireMention, old.RequireMention != new.RequireMention},
		{ChangedThreadRequireMention, old.ThreadRequireMention != new.ThreadRequireMention},
		{ChangedFreeResponse, old.FreeResponse != new.FreeResponse},
		{ChangedAutoThread, old.AutoThread != new.AutoThread},
		{ChangedAllowMentionedAgents, old.AllowMentionedAgents != new.AllowMentionedAgents},
		{ChangedHistoryBackfillLimit, old.HistoryBackfillLimit != new.HistoryBackfillLimit},
		{ChangedPrompt, old.Prompt != new.Prompt},
		{ChangedSkills, !slices.Equal(old.Skills, new.Skills)},
	} {
		if f.changed {
			out = append(out, f.code)
		}
	}
	return out
}

// ConfigChanged reports whether the agent configuration changed between old and new: the prompt
// or the skills (05 section 6 agent.config_changed; a switch-only change reaches the operator
// through channel.settings_changed, docs/DECISIONS.md D-M2-02-3).
func ConfigChanged(old, new Behavior) bool {
	return old.Prompt != new.Prompt || !slices.Equal(old.Skills, new.Skills)
}
