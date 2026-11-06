// SPDX-License-Identifier: Apache-2.0

package interview

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The roles of a turn (schema AgentInterviewTurn).
const (
	// RoleBuilder marks a question of the builder.
	RoleBuilder = "builder"
	// RoleOwner marks an answer of the person.
	RoleOwner = "owner"
)

// The field codes of this package besides those of package definition and kernel/textcheck: the
// request validation vocabulary of 06 section 5 and the catalogue code of 26 7.4.
const (
	CodeTooManyItems      = "too_many_items"
	CodeTooLong           = "too_long"
	CodeTooShort          = "too_short"
	CodeTooLarge          = "too_large"
	CodeInvalidEnum       = "invalid_enum"
	CodeMCPNotInCatalogue = "mcp_not_in_catalogue"
)

// Turn is one entry of the interview so far: a question of the builder or an answer of the person.
type Turn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// CheckTurns checks the interview so far (26 6.1 item 1): at most maxTurns turns
// (agents.interview.max_turns, /turns too_many_items), each with a known role (invalid_enum), a
// non-empty text (too_short) of at most maxChars characters (agents.interview.answer_max_chars,
// too_long) that is Markdown-like display text (line breaks allowed, the textcheck rule otherwise).
// No turn is the start of an interview. Errors: validation_failed with one field error per
// finding; no detail repeats a text.
func CheckTurns(turns []Turn, maxTurns, maxChars int) error {
	if len(turns) > maxTurns {
		return fields(errs.FieldError{Pointer: "/turns", Code: CodeTooManyItems, Detail: "more turns than agents.interview.max_turns"})
	}
	var out []errs.FieldError
	for i, t := range turns {
		p := "/turns/" + strconv.Itoa(i)
		if t.Role != RoleBuilder && t.Role != RoleOwner {
			out = append(out, errs.FieldError{Pointer: p + "/role", Code: CodeInvalidEnum, Detail: "the role is builder or owner"})
		}
		switch {
		case t.Text == "":
			out = append(out, errs.FieldError{Pointer: p + "/text", Code: CodeTooShort, Detail: "the text must not be empty"})
		case utf8.RuneCountInString(t.Text) > maxChars:
			out = append(out, errs.FieldError{Pointer: p + "/text", Code: CodeTooLong, Detail: "the text is longer than agents.interview.answer_max_chars"})
		default:
			if rule := markdownRule(t.Text); rule != "" {
				out = append(out, errs.FieldError{Pointer: p + "/text", Code: rule, Detail: "the text contains a forbidden character"})
			}
		}
	}
	if len(out) > 0 {
		return fields(out...)
	}
	return nil
}

// fields returns validation_failed with the field errors fs.
func fields(fs ...errs.FieldError) error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(fs...)
}

// markdownRule returns the textcheck rule a text with line breaks breaks, "" when it passes: line
// feed and carriage return are allowed, every other control or bidi character is not.
func markdownRule(s string) string {
	err := textcheck.DisplayText(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s))
	var te *textcheck.Error
	if errors.As(err, &te) {
		return string(te.Rule)
	}
	return ""
}

// CatalogueEntry is one template of the catalogue for the builder (26 7.4): slug, name,
// description, tool class wish and its MCP servers.
type CatalogueEntry struct {
	Slug, Name, Description, ToolClassWish string
	MCPServers                             []definition.MCPServer
}

// Limits are the limits a draft keeps: the definition limits of agents.definition.* and
// agents.mcp_stdio_runners, and the longest question of an answer (agents.interview.answer_max_chars).
type Limits struct {
	Definition       definition.Limits
	QuestionMaxChars int
}

// PayloadInput are the members of agent.interview_requested.
type PayloadInput struct {
	RequestID ids.ID
	Turns     []Turn
	WantDraft bool
	Catalogue []CatalogueEntry
	Limits    Limits
	ExpiresAt time.Time
}

// timestamp is RFC 3339 in UTC with milliseconds (04 section 2.2).
const timestamp = "2006-01-02T15:04:05.000Z"

type catalogueJSON struct {
	Slug          string                 `json:"slug"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	ToolClassWish string                 `json:"tool_class_wish"`
	MCPServers    []definition.MCPServer `json:"mcp_servers"`
}

type limitsJSON struct {
	InstructionsMaxBytes     int      `json:"instructions_max_bytes"`
	MaxSkills                int      `json:"max_skills"`
	SkillDescriptionMaxChars int      `json:"skill_description_max_chars"`
	SkillBodyMaxBytes        int      `json:"skill_body_max_bytes"`
	MaxSkillFiles            int      `json:"max_skill_files"`
	MaxMCPServers            int      `json:"max_mcp_servers"`
	MaxAllowedTools          int      `json:"max_allowed_tools"`
	MaxCostUSDPerRun         float64  `json:"max_cost_usd_per_run"`
	MCPStdioRunners          []string `json:"mcp_stdio_runners"`
	QuestionMaxChars         int      `json:"question_max_chars"`
}

type payloadJSON struct {
	RequestID string          `json:"request_id"`
	Turns     []Turn          `json:"turns"`
	WantDraft bool            `json:"want_draft"`
	Catalogue []catalogueJSON `json:"catalogue"`
	Limits    limitsJSON      `json:"limits"`
	ExpiresAt string          `json:"expires_at"`
}

// ErrPayloadTooLarge reports that the interview does not fit sync.event_payload_max_bytes.
var ErrPayloadTooLarge = errors.New("agents: agent.interview_requested does not fit sync.event_payload_max_bytes")

// Payload encodes the payload of agent.interview_requested (26 6.1 item 1, schema
// AgentInterviewRequestedPayload): no null, no token, the time in UTC with milliseconds. Errors:
// ErrPayloadTooLarge when it is not shorter than maxBytes (sync.event_payload_max_bytes); the
// turns are the person's text, so the server never cuts them.
//
// Example:
//
//	b, err := interview.Payload(in, 65536)
func Payload(in PayloadInput, maxBytes int) ([]byte, error) {
	lim := in.Limits.Definition
	runners := lim.StdioRunners
	if runners == nil {
		runners = []string{}
	}
	p := payloadJSON{
		RequestID: in.RequestID.String(), Turns: in.Turns, WantDraft: in.WantDraft, Catalogue: make([]catalogueJSON, 0, len(in.Catalogue)),
		ExpiresAt: in.ExpiresAt.UTC().Format(timestamp),
		Limits: limitsJSON{
			InstructionsMaxBytes: lim.InstructionsMaxBytes, MaxSkills: lim.MaxSkills, SkillDescriptionMaxChars: lim.SkillDescriptionMaxChars,
			SkillBodyMaxBytes: lim.SkillBodyMaxBytes, MaxSkillFiles: lim.MaxSkillFiles, MaxMCPServers: lim.MaxMCPServers,
			MaxAllowedTools: lim.MaxAllowedTools, MaxCostUSDPerRun: lim.MaxCostUSDPerRun, MCPStdioRunners: runners,
			QuestionMaxChars: in.Limits.QuestionMaxChars,
		},
	}
	if p.Turns == nil {
		p.Turns = []Turn{}
	}
	for _, c := range in.Catalogue {
		servers := c.MCPServers
		if servers == nil {
			servers = []definition.MCPServer{}
		}
		p.Catalogue = append(p.Catalogue, catalogueJSON{Slug: c.Slug, Name: c.Name, Description: c.Description, ToolClassWish: c.ToolClassWish, MCPServers: servers})
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if len(b) >= maxBytes {
		return nil, ErrPayloadTooLarge
	}
	return b, nil
}
