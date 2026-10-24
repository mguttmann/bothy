// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares the durable event importer appends (05 section 6): import.progress after
// every committed change of a source (22 14, M8-03). internal/wire lists it in
// moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("importer", authorize.EventImportProgress)
}

// Source kinds (03 import_sources.kind).
const (
	// KindDiscord is a Discord server with the mapping of schema import_source.discord.v1.
	KindDiscord = "discord"
	// KindArchive is an archive import; not enabled (400 validation_failed, not_enabled).
	KindArchive = "archive"
)

// Source states (03 import_sources.state; M8-03 design note 3).
const (
	StateDraft     = "draft"
	StateRunning   = "running"
	StatePaused    = "paused"
	StateCompleted = "completed"
	StateFailed    = "failed"
)

// Item states (03 import_items.state).
const (
	ItemPending = "pending"
	ItemPosted  = "posted"
	ItemSkipped = "skipped"
	ItemFailed  = "failed"
)

// Leads of a mapped channel (22 12.1).
const (
	LeadDiscord = "discord"
	LeadHere    = "bothy"
)

// Reasons of 409 conflict of the import operations (M8-03 design notes 3 to 5).
const (
	// ReasonImportState: the operation is not allowed in the current state of the source.
	ReasonImportState = "import_state"
	// ReasonLeadNotHere: completeImport while an imported channel still has lead discord.
	ReasonLeadNotHere = "lead_not_" + LeadHere
	// ReasonChannelNotImported: setImportLead for a mapped channel whose item is not posted.
	ReasonChannelNotImported = "channel_not_imported"
	// ReasonImportNotRunning: applyImportBatch or setImportCursor on a source that is not running.
	ReasonImportNotRunning = "import_not_running"
	// ReasonTargetOutsideSource: a batch target that is not in a channel of this source.
	ReasonTargetOutsideSource = "target_outside_source"
	// ReasonThreadNotImported: a thread batch whose thread (or channel of a main thread) is not
	// posted and is not created by the batch (M8-11 design note 1).
	ReasonThreadNotImported = "thread_not_imported"
	// ReasonAuthorNotImported: a message or file whose author item is skipped or missing (M8-11
	// design note 4 a).
	ReasonAuthorNotImported = "author_not_imported"
)

// Source is an import source as the API shows it (schema ImportSource).
type Source struct {
	ID ids.ID
	// Kind is KindDiscord or KindArchive; Name the owner's label.
	Kind, Name string
	// Config is the stored normalized mapping (import_sources.config).
	Config json.RawMessage
	// State is one of the State constants.
	State string
	// Stats are the counters (server.* of the server, tool.* of the tool); Cursor the object of
	// the last setImportCursor, {} before.
	Stats, Cursor json.RawMessage
	// CreatedBy is the owner that created the source.
	CreatedBy ids.ID
	// CreatedAt is the creation time; StartedAt and FinishedAt are zero while unset.
	CreatedAt, StartedAt, FinishedAt time.Time
}

// Item is an imported item (schema ImportItem). Zero values are absent members.
type Item struct {
	ID ids.ID
	// OriginKey is the stable key of the source object; ItemKind its kind.
	OriginKey, ItemKind string
	// TargetKind and TargetID name the created or adopted object; empty for a skipped item.
	TargetKind string
	TargetID   ids.ID
	// State is one of the Item constants; Error is "" for none.
	State, Error string
	// SourceChanged reports a repeated key with a different content_sha256 (22 7.2 item 4).
	SourceChanged bool
	// Session is the outcome of the member earlier_session of a thread item (agents.EarlierTakenOver,
	// EarlierAlreadyPresent or EarlierDroppedLate, M8-06 design note 3); "" when the item carried
	// none or has no thread.
	Session string
	// CreatedAt is the creation time.
	CreatedAt time.Time
}

// Page asks for one page of a list: Cursor is next_cursor of the previous page ("" first page),
// Limit the page size (0 means api.page_size_default, larger values are capped at
// api.page_size_max).
type Page struct {
	Cursor string
	Limit  int
}

// SourcePage is one page of sources, newest first; NextCursor is "" on the last page.
type SourcePage struct {
	Items      []Source
	NextCursor string
}

// ItemQuery asks for one page of the items of a source; State "" lists every state.
type ItemQuery struct {
	Page
	State string
}

// ItemPage is one page of items in ascending id; NextCursor is "" on the last page.
type ItemPage struct {
	Items      []Item
	NextCursor string
}

// CreateSourceCmd is the body of createImport (schema ImportSourceCreate).
type CreateSourceCmd struct {
	Kind, Name string
	// Config is the mapping as sent.
	Config json.RawMessage
}

// LeadCmd is the body of setImportLead (schema ImportLeadSet).
type LeadCmd struct {
	// SourceChannel is the Discord channel ID; Lead is LeadDiscord or LeadHere.
	SourceChannel, Lead string
}

// BatchObject is one item of a batch (schema ImportObject).
type BatchObject struct {
	OriginKey, ItemKind string
	// ContentSHA256 is the hex hash of the converted content; "" when absent.
	ContentSHA256 string
	// Data are the fields of the target object.
	Data json.RawMessage
}

// Batch is the body of applyImportBatch (schema ImportBatch).
type Batch struct {
	// ThreadOriginKey is thread:<id> or channel:<id> for a thread batch (M8-11), "" for a
	// structure batch.
	ThreadOriginKey string
	// ThreadOriginKeySet reports that the member was sent (also as "").
	ThreadOriginKeySet bool
	Items              []BatchObject
}

// BatchResult is the answer of applyImportBatch (schema ImportBatchResult): the items in request
// order and, for a thread batch, threads.unread_floor_seq of its thread after the batch.
type BatchResult struct {
	Items []Item
	// UnreadFloorSeq is set for a thread batch whose thread exists.
	UnreadFloorSeq *int64
}

// FileCmd is the body of createImportFile (schema ImportFileCreate).
type FileCmd struct {
	// OriginKey is file:<attachment id>.
	OriginKey string
	// ChannelID is the target channel of the attachment; AuthorID the target of the posted user
	// item of the author.
	ChannelID, AuthorID ids.ID
	// Name, SizeBytes and SHA256 (64 hex digits) describe the bytes; DeclaredMIMEType is "" when
	// absent.
	Name             string
	SizeBytes        int64
	SHA256           string
	DeclaredMIMEType string
}

// FileSlot is the answer of createImportFile and completeImportFile (schema ImportFileSlot).
type FileSlot struct {
	Item Item
	// Upload is the open session; nil once the file item is posted.
	Upload *UploadPlan
}

// UploadPlan is the open upload session of a file item (schema UploadSession, mode direct).
type UploadPlan struct {
	FileID ids.ID
	// Mode is direct.
	Mode          string
	PartSizeBytes int64
	PartCount     int
	ExpiresAt     time.Time
}

// PartRef is one confirmed part with its ETag (schemas UploadPartResult and UploadComplete).
type PartRef struct {
	PartNumber int
	ETag       string
}

// FilePartCmd is one part of uploadImportFilePart.
type FilePartCmd struct {
	// FileID is the reserved file; PartNumber the number from 1.
	FileID     ids.ID
	PartNumber int
	// ContentDigest is the Content-Digest header as received.
	ContentDigest string
	// Body streams the part.
	Body io.Reader
}

// FileCompleteCmd is the body of completeImportFile (schema UploadComplete).
type FileCompleteCmd struct {
	FileID ids.ID
	Parts  []PartRef
}

// CursorCmd is the body of setImportCursor (schema ImportCursorSet).
type CursorCmd struct {
	// Cursor replaces import_sources.cursor.
	Cursor json.RawMessage
	// Stats are merged member by member into stats.tool; nil when absent.
	Stats json.RawMessage
}

// Origin answers "where does this object come from" (index (target_kind, target_id)).
type Origin struct {
	SourceID, ItemID    ids.ID
	ItemKind, OriginKey string
}

// SourceChanged is the in-transaction event of every committed change of a source (state,
// mapping, items, stats or cursor), published inside the unit of work that appends
// import.progress. No module subscribes yet; the console (M8-08) and M8-11 may.
type SourceChanged struct {
	// SourceID is the source; State its state after the change.
	SourceID ids.ID
	State    string
}

// Imports is the incoming port of the importer module (22 8, M8-03 design note 1). Every method
// with an actor authorizes it with import.read, import.manage (step-up, audit) or import.apply
// (audit of denials) on the source and runs its writes in one unit of work with the durable event
// import.progress; an unknown and an invisible source give the same 404 not_found. Errors are
// kernel/errs errors.
type Imports interface {
	// ListSources returns one page of the sources, newest first (import.read).
	ListSources(ctx context.Context, a actor.Actor, p Page) (SourcePage, error)
	// CreateSource validates the mapping field by field and stores a draft source
	// (import.manage). Errors: validation_failed with every violation at once.
	CreateSource(ctx context.Context, a actor.Actor, cmd CreateSourceCmd) (Source, error)
	// GetSource returns the source with its cursor (import.read).
	GetSource(ctx context.Context, a actor.Actor, id ids.ID) (Source, error)
	// ListItems returns one page of the items of the source (import.read).
	ListItems(ctx context.Context, a actor.Actor, id ids.ID, q ItemQuery) (ItemPage, error)
	// Start moves draft or paused to running (import.manage); a running source answers unchanged.
	Start(ctx context.Context, a actor.Actor, id ids.ID) (Source, error)
	// Pause moves running to paused (import.manage).
	Pause(ctx context.Context, a actor.Actor, id ids.ID) (Source, error)
	// Cancel ends draft, running or paused as failed with server.cancelled (import.manage).
	Cancel(ctx context.Context, a actor.Actor, id ids.ID) (Source, error)
	// Complete ends running or paused as completed when every imported channel has lead `bothy`
	// (import.manage).
	Complete(ctx context.Context, a actor.Actor, id ids.ID) (Source, error)
	// SetLead changes the lead of one mapped channel and the read-only flag of its target channel
	// in one transaction (import.manage).
	SetLead(ctx context.Context, a actor.Actor, id ids.ID, cmd LeadCmd) (Source, error)
	// ApplyBatch writes a structure batch or a thread batch of a running source as one
	// transaction (import.apply).
	ApplyBatch(ctx context.Context, a actor.Actor, id ids.ID, b Batch) (BatchResult, error)
	// CreateFile reserves the file of a file item, or answers the reserved one (import.apply,
	// 22 6.5 and 7.2 item 3).
	CreateFile(ctx context.Context, a actor.Actor, id ids.ID, cmd FileCmd) (FileSlot, error)
	// UploadFilePart stores one part of the file of a file item of the source (import.apply).
	UploadFilePart(ctx context.Context, a actor.Actor, id ids.ID, cmd FilePartCmd) (PartRef, error)
	// CompleteFile completes the upload of the file of a file item and posts the item
	// (import.apply).
	CompleteFile(ctx context.Context, a actor.Actor, id ids.ID, cmd FileCompleteCmd) (FileSlot, error)
	// SetCursor replaces the cursor and merges the tool counters of a running source
	// (import.apply).
	SetCursor(ctx context.Context, a actor.Actor, id ids.ID, cmd CursorCmd) (Source, error)
	// SourceOfTarget returns the item that created or adopted the object kind/id; found is false
	// when no import touched it. It never authorizes.
	SourceOfTarget(ctx context.Context, kind string, id ids.ID) (o Origin, found bool, err error)
}
