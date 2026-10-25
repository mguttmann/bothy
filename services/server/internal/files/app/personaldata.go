// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// PersonalFile is an own upload of a data subject as the export package lists it (M7-06,
// docs/spec/20-privacy.md section 5.2).
type PersonalFile struct {
	ID        ids.ID
	ObjectKey string
	// Name is files.original_name, MIMEType the stored (detected) type.
	Name, MIMEType string
	Purpose        string
	CreatedAt      time.Time
	// ChannelID is the channel of the attachment row, else files.scope_channel_id; zero for none.
	ChannelID ids.ID
	// TranscriptStatus, TranscriptText and TranscriptLanguage are the transcript columns; the
	// last two are "" for NULL.
	TranscriptStatus, TranscriptText, TranscriptLanguage string
}

// PersonalDataStore is the outgoing port of the personal data participant and of the export
// packages to the tables of the module (M7-06); adapters/postgres implements it. The jobs of
// privacy have no principal, so the reads are keyed by the data subject as uploader (the
// exceptions of AZ-P12).
type PersonalDataStore interface {
	// PersonalFiles returns the own uploads of uploader of purpose attachment or avatar that are
	// not deleted, complete and deliverable, ordered by created_at and id.
	PersonalFiles(ctx context.Context, uploader ids.ID) ([]PersonalFile, error)
	// OpenUploadsOf returns the files of the open upload sessions of uploader that are not
	// deleted.
	OpenUploadsOf(ctx context.Context, uploader ids.ID) ([]ids.ID, error)
	// UnattachedFilesOf returns the files of purpose of uploader that are neither deleted nor
	// attached to a message.
	UnattachedFilesOf(ctx context.Context, uploader ids.ID, purpose string) ([]ids.ID, error)
	// InsertExportFile inserts the files row of an export package, without an upload session.
	// It needs a unit of work.
	InsertExportFile(ctx context.Context, f FileRecord) error
}

// The section of files/files.json the participant writes.
const sectionFiles = "files"

// personalTables are the tables of 20 section 2 with personal data that the module owns;
// message_attachments is marked "no" there and stays out.
var personalTables = []string{"files", "file_thumbnails", "file_upload_sessions", "file_upload_parts", "file_text_extracts"}

// PersonalData is the participant of the module files in the privacy requests (M7-06,
// personaldata.Participant): it exports the own uploads with their bytes and erases the files of
// the steps of 20 section 4.4 through the deletion and abort paths of the module (markDeleted,
// the abort of M3-03), so redaction, file.updated and files.purge_objects follow as for any
// deletion. It is safe for concurrent use.
type PersonalData struct {
	s     *Service
	store PersonalDataStore
}

// Compile-time check: PersonalData is a participant.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over the files use cases s and store. Export needs the
// object store of s (Deps.Store) and reports an internal error without one; Module and Tables need
// nothing, so the coverage test of internal/wire builds the participant without storage.
// Errors: a missing collaborator.
//
// Example:
//
//	p, err := app.NewPersonalData(svc, postgres.NewRepository(pool))
//	err = registry.Register(p)
func NewPersonalData(s *Service, store PersonalDataStore) (*PersonalData, error) {
	if s == nil || store == nil {
		return nil, errors.New("files: NewPersonalData needs the use cases and the store")
	}
	return &PersonalData{s: s, store: store}, nil
}

// Module implements personaldata.Participant.
func (p *PersonalData) Module() string { return "files" }

// Tables implements personaldata.Participant: files, file_thumbnails, file_upload_sessions,
// file_upload_parts and file_text_extracts. Thumbnails, sessions, parts and text extracts are
// derived from a file and go with it; the export holds the originals only.
func (p *PersonalData) Tables() []string { return append([]string(nil), personalTables...) }

// exportedFile is one entry of the section "files" of files/files.json
// (docs/api/gdpr-export/v1/sections/files.files.schema.json).
type exportedFile struct {
	ID         ids.ID    `json:"id"`
	Name       string    `json:"name"`
	MediaType  string    `json:"media_type"`
	SizeBytes  int64     `json:"size_bytes"`
	SHA256     string    `json:"sha256"`
	Purpose    string    `json:"purpose"`
	CreatedAt  time.Time `json:"created_at"`
	ChannelID  ids.ID    `json:"channel_id,omitzero"`
	Transcript string    `json:"transcript,omitempty"`
	// TranscriptLanguage is the language of Transcript; only with it.
	TranscriptLanguage string `json:"transcript_language,omitempty"`
}

// Export implements personaldata.Participant: every own upload of purpose attachment or avatar
// that is not deleted, complete and deliverable goes with its bytes under files/<id>
// (Writer.Content) and one entry of the section "files" of files/files.json, whose size and
// SHA-256 are those of the written bytes. Export packages are never exported again; open, failed
// and blocked uploads have no object to export (blocked objects are purged, 20 4.3). The transcript
// of an own voice message goes with it when it is done: the person spoke it, and it is no secret.
// The channel goes as its ID only; its name is the business of chat. Objects are read outside any
// transaction (02 8.5).
//
// Errors: the storage of the rows or the objects (an object missing for a live file is an internal
// error) and those of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	list, err := p.store.PersonalFiles(ctx, e.Subject)
	if err != nil {
		return err
	}
	out := make([]exportedFile, 0, len(list))
	for _, f := range list {
		size, sum, err := p.content(ctx, f, w)
		if err != nil {
			return err
		}
		x := exportedFile{
			ID: f.ID, Name: f.Name, MediaType: f.MIMEType, SizeBytes: size, SHA256: sum, Purpose: f.Purpose,
			CreatedAt: f.CreatedAt.UTC(), ChannelID: f.ChannelID,
		}
		if f.TranscriptStatus == transcriptDone && f.TranscriptText != "" {
			x.Transcript, x.TranscriptLanguage = f.TranscriptText, f.TranscriptLanguage
		}
		out = append(out, x)
	}
	return w.Section(personaldata.FileFiles, sectionFiles, out)
}

// content streams the stored original of f into w and returns what w counted.
func (p *PersonalData) content(ctx context.Context, f PersonalFile, w personaldata.Writer) (int64, string, error) {
	if p.s.d.Store == nil {
		return 0, "", errs.New(errs.CodeInternalError, "files: the export needs the object store")
	}
	body, err := p.s.d.Store.Get(ctx, f.ObjectKey, nil)
	switch {
	case errors.Is(err, ErrNotFound):
		return 0, "", errs.Wrap(errs.CodeInternalError, err, "files: the object of a live file is missing")
	case err != nil:
		return 0, "", imageStorageError(err)
	}
	defer func() { _ = body.Close() }()
	return w.Content(f.ID, body)
}

// Erase implements personaldata.Participant, acting as the system (no actor):
//
//   - StepPersonal: every open upload session of the person is aborted like abortUpload (session
//     aborted, file failed, parts gone, file.updated without actor, files.purge_objects); an
//     unfinished upload is transient working state like a draft, not content.
//   - StepContent with privacy.account_deletion_content delete: every own attachment upload that
//     is not attached is marked deleted through markDeleted (20 4.3: redaction of file.updated,
//     files.purge_objects). Attached uploads go with their message, which messaging deletes in the
//     same step (20 4.2); an attached file always belongs to a message of its uploader. With
//     keep_pseudonymized nothing happens.
//   - StepPseudonymize: the own avatar files are marked deleted the same way ("avatar removed",
//     20 4.4 item 6), whatever identity does with the reference.
//
// Export packages (gdpr_export) stay: privacy owns their life through ExportPackages. Every file
// runs in its own unit of work and only files not handled yet are touched, so a second call
// finishes the step and repeats nothing. Other steps return nil.
//
// Errors: those of the storage; the job retries the step.
func (p *PersonalData) Erase(ctx context.Context, step personaldata.Step, e personaldata.Erasure) error {
	switch step {
	case personaldata.StepPersonal:
		return p.abortOpenUploads(ctx, e.Subject)
	case personaldata.StepContent:
		if e.Content != personaldata.DeleteContent {
			return nil
		}
		return p.deleteUnattached(ctx, e.Subject, domain.PurposeAttachment)
	case personaldata.StepPseudonymize:
		return p.deleteUnattached(ctx, e.Subject, domain.PurposeAvatar)
	case personaldata.StepDisable, personaldata.StepSecurity, personaldata.StepAccess, personaldata.StepAudit:
	}
	return nil
}

// abortOpenUploads aborts the open sessions of subject. A session that completed or ended
// meanwhile, or a file deleted meanwhile, is no error: the abort leaves it alone.
func (p *PersonalData) abortOpenUploads(ctx context.Context, subject ids.ID) error {
	list, err := p.store.OpenUploadsOf(ctx, subject)
	if err != nil {
		return err
	}
	for _, id := range list {
		err := p.s.abort(ctx, actor.Actor{}, subject, id)
		if k := errs.KindOf(err); err != nil && k != errs.KindNotFound && k != errs.KindConflict {
			return err
		}
	}
	return nil
}

// deleteUnattached marks every unattached file of purpose of subject deleted, each in its own unit
// of work after the lock of its row and a new read (an attach or deletion meanwhile wins).
func (p *PersonalData) deleteUnattached(ctx context.Context, subject ids.ID, purpose domain.Purpose) error {
	list, err := p.store.UnattachedFilesOf(ctx, subject, string(purpose))
	if err != nil {
		return err
	}
	for _, id := range list {
		if err := p.s.d.UoW.Do(ctx, func(ctx context.Context) error {
			found, err := p.s.d.Cleanup.LockFile(ctx, id)
			if err != nil || !found {
				return err
			}
			cur, found, err := p.s.d.Cleanup.CleanupRecord(ctx, id)
			if err != nil || !found || cur.Deleted || cur.Attached {
				return err
			}
			return p.s.markDeleted(ctx, Deletion{FileIDs: []ids.ID{id}, Purpose: string(purpose), UploadedBy: subject})
		}); err != nil {
			return err
		}
	}
	return nil
}
