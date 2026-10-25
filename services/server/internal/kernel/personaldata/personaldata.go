// SPDX-License-Identifier: Apache-2.0

package personaldata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// File is a file of the export package of docs/spec/20-privacy.md section 5.2, by its path in the
// archive. A name ending in .json is one JSON object whose members are the sections of the
// participants; a name ending in .jsonl holds one JSON object per line.
type File string

// The files of the package that participants write (20 section 5.2). README.txt and
// manifest.json are written by privacy itself.
const (
	// FileProfile holds the own fields of principals and users.
	FileProfile File = "profile.json"
	// FileIdentities holds the linked identities, passkeys and the local credential (no secret).
	FileIdentities File = "identities.json"
	// FileDevices holds devices, sessions (times, login method, last IP), tokens and push targets.
	FileDevices File = "devices.json"
	// FileAccess holds own grants, groups and the owner status, with times and reason.
	FileAccess File = "access.json"
	// FileMessages holds the own messages with revisions, thread and channel ID and times.
	FileMessages File = "messages.jsonl"
	// FileReactions holds the own reactions.
	FileReactions File = "reactions.jsonl"
	// FileMentions holds the mentions of the person (only message ID and time).
	FileMentions File = "mentions.jsonl"
	// FileFiles holds the list of own uploads (name, type, size, checksum); the bytes of each
	// upload are under files/ (Writer.Content).
	FileFiles File = "files/files.json"
	// FileRuns holds the runs triggered by the person (state, times, costs).
	FileRuns File = "runs.jsonl"
	// FileInteractions holds the own interactions with their values.
	FileInteractions File = "interactions.jsonl"
	// FilePersonal holds read state, drafts, settings, notifications, quiet hours and templates.
	FilePersonal File = "personal.json"
	// FileAudit holds the audit entries with the person as actor or resource.
	FileAudit File = "audit.jsonl"
)

// Files returns every file participants write, in the order of 20 section 5.2.
func Files() []File {
	return []File{
		FileProfile, FileIdentities, FileDevices, FileAccess, FileMessages, FileReactions, FileMentions,
		FileFiles, FileRuns, FileInteractions, FilePersonal, FileAudit,
	}
}

// Lines reports a JSON Lines file (one record per Writer.Line).
func (f File) Lines() bool { return strings.HasSuffix(string(f), ".jsonl") }

// Known reports one of the files of Files.
func (f File) Known() bool { return slices.Contains(Files(), f) }

// Writer receives the export of the participants (20 section 5.2). privacy implements it; a
// participant never sees the archive. Values are encoded with encoding/json: use omitempty or
// omitzero for absent members, never a JSON null, and only the fields the schema of
// docs/api/gdpr-export/v1 names.
type Writer interface {
	// Section sets the member key of the JSON object file f to v. Errors: f is a JSON Lines file
	// or not one of Files, key is not lower snake case or was set before, v cannot be encoded.
	Section(f File, key string, v any) error
	// Line appends v as one line to the JSON Lines file f. Errors: f is no JSON Lines file of
	// Files, v cannot be encoded, the storage of the package fails.
	Line(f File, v any) error
	// Content writes the bytes of the own upload id under files/<id> and returns the number of
	// bytes and the SHA-256 (lower-case hex) of what was written. Errors: id was written before,
	// reading r or the storage of the package fails.
	Content(id ids.ID, r io.Reader) (size int64, sha256Hex string, err error)
}

// Export is the input of Participant.Export.
type Export struct {
	// Subject is the person (users.principal_id) whose data is exported.
	Subject ids.ID
	// Readable reports whether the subject may still read the channel channelID; names of
	// channels and threads go into the package only then, otherwise only their IDs (20 section
	// 5.2). nil means no channel is readable.
	Readable func(ctx context.Context, channelID ids.ID) (bool, error)
}

// CanRead calls Readable; false without one.
func (e Export) CanRead(ctx context.Context, channelID ids.ID) (bool, error) {
	if e.Readable == nil {
		return false, nil
	}
	return e.Readable(ctx, channelID)
}

// Step is a step of the account deletion of 20 section 4.4, in the order the job runs them.
type Step int

// The steps of 20 section 4.4. Participants get StepDisable to StepPseudonymize; StepAudit (the
// audit entry and the deletion journal) is done by privacy itself.
const (
	// StepDisable disables the account like user.disable (08 section 11); the last owner is
	// refused with errs.CodeLastOwner.
	StepDisable Step = 1
	// StepSecurity deletes identities, passkeys, local credentials, devices, keys, sessions,
	// tokens, push registrations and enrollment codes.
	StepSecurity Step = 2
	// StepPersonal deletes sidebar, read state, drafts, settings, notification settings, quiet
	// hours and personal templates.
	StepPersonal Step = 3
	// StepAccess removes memberships and revokes grants; the rows stay with the ID as evidence.
	StepAccess Step = 4
	// StepContent handles the contents by Erasure.Content: keep them, or delete own messages
	// as in 20 section 4.2 and own files as in 4.3.
	StepContent Step = 5
	// StepPseudonymize replaces the clear data of principals and users (20 section 4.4 item 6).
	StepPseudonymize Step = 6
	// StepAudit writes the audit entry of the deletion; privacy runs it, no participant.
	StepAudit Step = 7
)

// ParticipantSteps returns the steps participants run, in order.
func ParticipantSteps() []Step {
	return []Step{StepDisable, StepSecurity, StepPersonal, StepAccess, StepContent, StepPseudonymize}
}

// String returns the name of the step as recorded in gdpr_requests.progress.
func (s Step) String() string {
	switch s {
	case StepDisable:
		return "disable"
	case StepSecurity:
		return "security"
	case StepPersonal:
		return "personal"
	case StepAccess:
		return "access"
	case StepContent:
		return "content"
	case StepPseudonymize:
		return "pseudonymize"
	case StepAudit:
		return "audit"
	default:
		return fmt.Sprintf("step(%d)", int(s))
	}
}

// ContentMode is privacy.account_deletion_content (20 section 4.4 item 5).
type ContentMode string

// The values of privacy.account_deletion_content.
const (
	// KeepPseudonymized keeps messages, files and reactions; the author appears as the
	// pseudonymized principal.
	KeepPseudonymized ContentMode = "keep_pseudonymized"
	// DeleteContent deletes all own messages as in 20 section 4.2 and own files as in 4.3.
	DeleteContent ContentMode = "delete"
)

// Valid reports one of the two values.
func (m ContentMode) Valid() bool { return m == KeepPseudonymized || m == DeleteContent }

// Erasure is the input of Participant.Erase.
type Erasure struct {
	// Subject is the person (users.principal_id) whose account is deleted.
	Subject ids.ID
	// RequestID is the privacy request (gdpr_requests.id); audit entries of the steps name it.
	RequestID ids.ID
	// DecidedBy is the owner who approved the deletion; zero when unknown. Steps act as the
	// system (05 rule 10), never with the rights of this owner.
	DecidedBy ids.ID
	// Content is privacy.account_deletion_content.
	Content ContentMode
}

// Participant exports and erases the personal data of one module (20 sections 4.4 and 5.2).
type Participant interface {
	// Module returns the name of the module in architecture/modules.yaml.
	Module() string
	// Tables returns the tables of 20 section 2 with personal data that this participant covers,
	// all of them tables of its module; the coverage test of internal/wire holds them against
	// 20 section 2.
	Tables() []string
	// Export writes the data of e.Subject into w. It reads in its own transactions; the caller
	// passes a context without a unit of work.
	//
	// Errors: those of the storage and of w; the job retries.
	Export(ctx context.Context, e Export, w Writer) error
	// Erase runs step for the tables of the module and returns nil for a step it has nothing to
	// do in. It manages its own transactions; the caller passes a context without a unit of
	// work. It is idempotent: a second call after a crash finishes the step and repeats nothing
	// that is already done.
	//
	// Errors: errs.CodeLastOwner from StepDisable for the last active owner; those of the
	// storage otherwise (the job retries the step).
	Erase(ctx context.Context, step Step, e Erasure) error
}

// tableName is the form of a table name of 03.
var tableName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Registry holds the participants of a process. internal/wire fills it once at the start and
// only reads it afterwards; it is not safe for concurrent writes.
type Registry struct {
	ps []Participant
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds p. Errors: a nil participant, an empty module name, a module registered before,
// a participant without tables, a table that is no table name, or a table another participant
// already names.
func (r *Registry) Register(p Participant) error {
	if p == nil {
		return errors.New("personaldata: Register needs a participant")
	}
	if p.Module() == "" {
		return errors.New("personaldata: a participant needs its module")
	}
	if len(p.Tables()) == 0 {
		return fmt.Errorf("personaldata: the participant of %s names no table", p.Module())
	}
	for _, have := range r.ps {
		if have.Module() == p.Module() {
			return fmt.Errorf("personaldata: module %s has two participants", p.Module())
		}
	}
	for _, t := range p.Tables() {
		if !tableName.MatchString(t) {
			return fmt.Errorf("personaldata: %q is no table name", t)
		}
		for _, have := range r.ps {
			if slices.Contains(have.Tables(), t) {
				return fmt.Errorf("personaldata: table %s has two participants", t)
			}
		}
	}
	r.ps = append(r.ps, p)
	return nil
}

// All returns the participants in the order they were registered.
func (r *Registry) All() []Participant { return slices.Clone(r.ps) }

// Tables returns every table the participants cover, sorted.
func (r *Registry) Tables() []string {
	var out []string
	for _, p := range r.ps {
		out = append(out, p.Tables()...)
	}
	slices.Sort(out)
	return out
}
