// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// FieldInvalidFormat marks a project binding text that breaks its format rule (design note 4 of
// M2-03, docs/DECISIONS.md D-M2-03-3).
const FieldInvalidFormat = "invalid_format"

// RunnerConfig is the runner configuration of a binding after the schema
// project_runner_config.v1 (09 section 18.1): the model and the tools the runner allows in this
// project. It holds no secrets, and the server never interprets the strings. A nil field is an
// absent member; AllowedTools nil and empty are different documents, as jsonb compares them.
type RunnerConfig struct {
	// Model is the model the runner passes as --model; nil when absent.
	Model *string
	// AllowedTools narrows the tools of the configured tool class; nil when absent.
	AllowedTools []string
}

// Equal reports whether c and o are the same document (the jsonb comparison of design note 5 of
// M2-03): the same model, and the same tools in the same order with the same presence.
func (c RunnerConfig) Equal(o RunnerConfig) bool {
	if (c.Model == nil) != (o.Model == nil) || (c.Model != nil && *c.Model != *o.Model) {
		return false
	}
	return (c.AllowedTools == nil) == (o.AllowedTools == nil) && slices.Equal(c.AllowedTools, o.AllowedTools)
}

// Clone returns a copy of c that shares no memory with it.
func (c RunnerConfig) Clone() RunnerConfig {
	out := RunnerConfig{AllowedTools: slices.Clone(c.AllowedTools)}
	if c.AllowedTools != nil && out.AllowedTools == nil {
		out.AllowedTools = []string{}
	}
	if c.Model != nil {
		m := *c.Model
		out.Model = &m
	}
	return out
}

// Provision tells the serving runner whether it creates the folder of a binding (03
// project_bindings, 09 section 18.12, M4-43).
type Provision string

// The values of project_bindings.provision; "" in an input is ProvisionNone.
const (
	// ProvisionNone creates nothing; the folder must exist.
	ProvisionNone Provision = "none"
	// ProvisionInit creates an empty folder with git init.
	ProvisionInit Provision = "init"
	// ProvisionClone clones repository_url, which must be https.
	ProvisionClone Provision = "clone"
)

// provisionRefPattern is the one-segment name a folder the runner creates must have (03
// project_bindings): no slash, no leading dot or hyphen, ASCII only.
var provisionRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ProjectBinding is one row of project_bindings (03 section 4.5): the binding of a forum to a
// development project. RepositoryURL and DefaultBranch are "" when absent.
type ProjectBinding struct {
	// ChannelID is the bound forum.
	ChannelID ids.ID
	// WorkspaceRef is the opaque reference the runner resolves; never a server path.
	WorkspaceRef string
	// RepositoryURL is display only; the server never fetches it.
	RepositoryURL string
	DefaultBranch string
	RunnerConfig  RunnerConfig
	// Provision is whether the runner creates the folder (M4-43); never "" in a stored row.
	Provision Provision
	// Ready is the readiness the serving runner reported (09 section 4.8).
	Ready bool
	// ReadyReason is the code the runner reported with Ready false (09 section 18.12); "" for
	// none.
	ReadyReason string
	// CreatedBy and CreatedAt name the first PUT; a replace keeps them.
	CreatedBy ids.ID
	CreatedAt time.Time
	// Version is the ETag; every change raises it by one.
	Version int64
}

// ProjectBindingInput is the body of putProjectBinding (schema ProjectBindingPut). A nil pointer
// is an absent member.
type ProjectBindingInput struct {
	WorkspaceRef                 string
	RepositoryURL, DefaultBranch *string
	RunnerConfig                 RunnerConfig
	// Provision is the member provision; "" when absent, which means ProvisionNone.
	Provision Provision
	// Reason is the mandatory reason of the audit entry; it is never stored with the binding.
	Reason string
}

// BindingLimits are the limits of a binding, bound from project.binding_text_max_chars and
// audit.reason_max_chars at request time.
type BindingLimits struct {
	// TextMaxChars is the longest workspace_ref, repository_url, default_branch, model and tool
	// name in Unicode characters.
	TextMaxChars int
	// ReasonMaxChars is the longest reason in Unicode characters.
	ReasonMaxChars int
}

// ValidateProjectBinding checks in against l (design note 4 of M2-03) and returns every failing
// field in the order workspace_ref, repository_url, default_branch, runner_config.model,
// runner_config.allowed_tools, provision, reason; nil when in is valid. With provision init or
// clone the workspace_ref must also be one segment of provisionRefPattern, and clone requires an
// https repository_url without query or fragment (03 project_bindings, M4-43). Every text is checked for control
// and bidi characters first (kernel/textcheck, invalid_characters), then for its length
// (too_long), then for its format (invalid_format). A detail never repeats the input.
//
// Example:
//
//	if fs := domain.ValidateProjectBinding(in, domain.BindingLimits{TextMaxChars: 512, ReasonMaxChars: 500}); len(fs) > 0 {
//		return errs.New(errs.CodeValidationFailed, "the project binding is not valid").WithFields(fs...)
//	}
func ValidateProjectBinding(in ProjectBindingInput, l BindingLimits) []errs.FieldError {
	var out []errs.FieldError
	add := func(pointer, code, detail string) {
		out = append(out, errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
	}
	text := func(pointer, what, v string, format func(string) bool) {
		switch {
		case textcheck.DisplayText(v) != nil:
			add(pointer, FieldInvalidCharacters, what+" contains a forbidden character")
		case utf8.RuneCountInString(v) > l.TextMaxChars:
			add(pointer, FieldTooLong, what+" is longer than project.binding_text_max_chars")
		case !format(v):
			add(pointer, FieldInvalidFormat, what+" has an invalid format")
		}
	}
	creates := in.Provision == ProvisionInit || in.Provision == ProvisionClone
	refFormat, urlFormat := workspaceRefFormat, repositoryURLFormat
	if creates {
		refFormat = func(v string) bool { return workspaceRefFormat(v) && provisionRefPattern.MatchString(v) }
	}
	if in.Provision == ProvisionClone {
		urlFormat = cloneURLFormat
	}
	if strings.TrimSpace(in.WorkspaceRef) == "" && textcheck.DisplayText(in.WorkspaceRef) == nil {
		add("/workspace_ref", FieldRequired, "workspace_ref is required")
	} else {
		text("/workspace_ref", "workspace_ref", in.WorkspaceRef, refFormat)
	}
	switch {
	case in.RepositoryURL != nil:
		text("/repository_url", "repository_url", *in.RepositoryURL, urlFormat)
	case in.Provision == ProvisionClone:
		add("/repository_url", FieldRequired, "repository_url is required to clone")
	}
	if in.DefaultBranch != nil {
		text("/default_branch", "default_branch", *in.DefaultBranch, branchFormat)
	}
	if in.RunnerConfig.Model != nil {
		text("/runner_config/model", "the model", *in.RunnerConfig.Model, argumentFormat)
	}
	for i, tool := range in.RunnerConfig.AllowedTools {
		text("/runner_config/allowed_tools/"+strconv.Itoa(i), "a tool name", tool, argumentFormat)
	}
	if !validProvision(in.Provision) {
		add("/provision", FieldInvalidFormat, "provision is none, init or clone")
	}
	switch {
	case textcheck.DisplayText(in.Reason) != nil:
		add("/reason", FieldInvalidCharacters, "the reason contains a forbidden character")
	case strings.TrimSpace(in.Reason) == "":
		add("/reason", FieldRequired, "the reason is required")
	case utf8.RuneCountInString(in.Reason) > l.ReasonMaxChars:
		add("/reason", FieldTooLong, "the reason is longer than audit.reason_max_chars")
	}
	return out
}

// workspaceRefFormat reports a workspace_ref that cannot name a server path (03 project_bindings
// "never an absolute path of the server"): no white space at either end, no leading /, \, ~ or -,
// no drive letter with colon and no segment .. between / and \. The runner still confines the
// resolved path under runner.projects_root (09 section 18.1); this is defense in depth.
func workspaceRefFormat(v string) bool {
	if hasOuterSpace(v) || strings.ContainsAny(v[:1], `/\~-`) {
		return false
	}
	if len(v) >= 2 && v[1] == ':' && ('a' <= v[0]|0x20 && v[0]|0x20 <= 'z') {
		return false
	}
	for _, seg := range strings.FieldsFunc(v, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return false
		}
	}
	return true
}

// repositoryURLFormat reports an absolute http or https URL with a host and without user
// information: the link schemes clients render (SEC-OUT-1) and no credentials in the database (03
// principle 9).
func repositoryURLFormat(v string) bool {
	u, err := url.Parse(v)
	if err != nil || hasOuterSpace(v) {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return (scheme == "https" || scheme == "http") && u.Opaque == "" && u.User == nil && u.Hostname() != ""
}

// cloneURLFormat reports a repository_url the runner may clone (09 section 18.12): the display
// rule with the scheme https only and without query or fragment.
func cloneURLFormat(v string) bool {
	u, err := url.Parse(v)
	return err == nil && repositoryURLFormat(v) && strings.EqualFold(u.Scheme, "https") &&
		u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.Contains(v, "#")
}

// validProvision reports a known provision; "" is the absent member.
func validProvision(p Provision) bool {
	return p == "" || p == ProvisionNone || p == ProvisionInit || p == ProvisionClone
}

// branchFormat reports a branch name without white space and .., not empty and not an option.
func branchFormat(v string) bool {
	if v == "" || strings.HasPrefix(v, "-") || strings.Contains(v, "..") {
		return false
	}
	return !strings.ContainsFunc(v, unicode.IsSpace)
}

// argumentFormat reports a model or tool name the runner may pass as one process argument: not
// empty and not starting with - (09 section 18.2).
func argumentFormat(v string) bool {
	return v != "" && !strings.HasPrefix(v, "-")
}

// hasOuterSpace reports white space at the start or the end of v.
func hasOuterSpace(v string) bool {
	first, _ := utf8.DecodeRuneInString(v)
	last, _ := utf8.DecodeLastRuneInString(v)
	return unicode.IsSpace(first) || unicode.IsSpace(last)
}

// BindingChangeKind is the result of a PUT (the audit detail op).
type BindingChangeKind string

// The kinds of a PUT and the op of a DELETE (audit detail op, design note 6 of M2-03).
const (
	BindingCreated   BindingChangeKind = "created"
	BindingReplaced  BindingChangeKind = "replaced"
	BindingUnchanged BindingChangeKind = "unchanged"
	BindingDeleted   BindingChangeKind = "deleted"
)

// BindingChange is the plan of a PUT: its kind, the row after it and whether it reset a reported
// readiness.
type BindingChange struct {
	Kind BindingChangeKind
	// Next is the row after the PUT; for BindingUnchanged the stored row.
	Next ProjectBinding
	// ReadyReset is true when the PUT replaced a binding whose ready was true.
	ReadyReset bool
}

// PlanBindingPut decides a PUT of in over the stored binding current, nil for none (design note 5
// of M2-03, docs/DECISIONS.md D-M2-03-4 and D-M2-03-5). Without a row it creates one with ready
// false and version 1; the caller sets CreatedBy, and created_at comes from the database. A PUT
// replaces the whole binding: an absent optional member clears its column, an absent provision is
// none, and the reported ready_reason of a new or replaced binding is empty. Equal values are
// BindingUnchanged, which keeps ready and the version, so a retried PUT never resets readiness;
// any difference is BindingReplaced with version + 1 and ready false, because the runner reported
// readiness for the old configuration. CreatedBy and CreatedAt stay.
func PlanBindingPut(current *ProjectBinding, in ProjectBindingInput) BindingChange {
	next := ProjectBinding{WorkspaceRef: in.WorkspaceRef, RunnerConfig: in.RunnerConfig.Clone(), Provision: in.Provision}
	if next.Provision == "" {
		next.Provision = ProvisionNone
	}
	if in.RepositoryURL != nil {
		next.RepositoryURL = *in.RepositoryURL
	}
	if in.DefaultBranch != nil {
		next.DefaultBranch = *in.DefaultBranch
	}
	if current == nil {
		next.Version = 1
		return BindingChange{Kind: BindingCreated, Next: next}
	}
	next.ChannelID, next.CreatedBy, next.CreatedAt = current.ChannelID, current.CreatedBy, current.CreatedAt
	if next.WorkspaceRef == current.WorkspaceRef && next.RepositoryURL == current.RepositoryURL &&
		next.DefaultBranch == current.DefaultBranch && next.RunnerConfig.Equal(current.RunnerConfig) &&
		next.Provision == current.Provision {
		return BindingChange{Kind: BindingUnchanged, Next: *current}
	}
	next.Version = current.Version + 1
	return BindingChange{Kind: BindingReplaced, Next: next, ReadyReset: current.Ready}
}
