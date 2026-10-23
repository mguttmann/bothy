// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	importerhttp "github.com/OWNER/bothy/services/server/internal/importer/adapters/httpapi"
	importerpg "github.com/OWNER/bothy/services/server/internal/importer/adapters/postgres"
	importerapp "github.com/OWNER/bothy/services/server/internal/importer/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// ImporterConfig binds the keys of the importer use cases (the name, handle and page limits of
// the modules the import writes into, M8-03). The error names the key whose binding failed.
func ImporterConfig(v *config.Values) (importerapp.Config, error) {
	var c importerapp.Config
	if err := config.Bind(v, &c); err != nil {
		return importerapp.Config{}, err
	}
	return c, nil
}

// ImporterDeps are the collaborators of the importer module from other steps: the recorder and
// appender, the structure port of chat and the people port of identity.
type ImporterDeps struct {
	Audit     auditing.Recorder
	Events    eventlog.Appender
	Structure chat.ImportStructure
	People    identity.ImportPeople
	// Threads, Messages and Files are the ports of the content items (M8-11): the threads of
	// chat, the imported messages of messaging and the import files of files (bound lazily, the
	// files step runs after the importer).
	Threads  chat.ImportThreads
	Messages messaging.ImportMessages
	Files    files.ImportFiles
}

// Importer is the importer module of the process (M8-03): the use cases of importer.Imports and
// the REST handlers of the tag Import.
type Importer struct {
	// Service implements importer.Imports.
	Service *importerapp.Service
	// Handlers are the REST handlers of the eleven import operations.
	Handlers *importerhttp.Imports
}

// NewImporter builds the importer module over pool with the unit of work uow, the Authorizer az,
// the collaborators of d and the clock (nil means clock.System()). It opens no connection.
// Errors: a missing collaborator or a limit the use cases refuse.
func NewImporter(cfg importerapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, d ImporterDeps,
	clk clock.Clock) (*Importer, error) {
	if pool == nil || d.Structure == nil || d.People == nil {
		return nil, errors.New("wire: the importer module needs the pool, the structure of chat and the people of identity")
	}
	svc, err := importerapp.New(cfg, importerapp.Deps{
		Store: importerpg.NewRepository(pool), UoW: uow, Authorizer: az, Audit: d.Audit, Events: d.Events,
		Structure: d.Structure, People: d.People, Threads: d.Threads, Messages: d.Messages, Files: d.Files, Cursors: importerhttp.Cursors{},
		Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	return &Importer{Service: svc, Handlers: importerhttp.New(svc)}, nil
}

// fillHandlers sets the nil import handlers of h.
func (i *Importer) fillHandlers(h *Handlers) {
	if h.Imports == nil {
		h.Imports = i.Handlers
	}
}

// LazyImportFiles is the port files.ImportFiles for the importer (M8-11), bound to the files module
// once its start step has run: the importer is built in the chat step before the files step, like
// messaging with LazyAttachments. Until Bind every method fails with an internal error; the server
// accepts requests only after every step ran. It is safe for concurrent use.
type LazyImportFiles struct {
	p atomic.Pointer[files.ImportFiles]
}

// Compile-time check: LazyImportFiles is the port.
var _ files.ImportFiles = (*LazyImportFiles)(nil)

// Bind sets the import files of the files module; a second call replaces the first.
func (l *LazyImportFiles) Bind(f files.ImportFiles) { l.p.Store(&f) }

// get returns the bound port or the internal error of an unbound one.
func (l *LazyImportFiles) get() (files.ImportFiles, error) {
	if p := l.p.Load(); p != nil && *p != nil {
		return *p, nil
	}
	return nil, errs.New(errs.CodeInternalError, "wire: the import files of the files module are not bound yet")
}

// Reserve implements files.ImportFiles.
func (l *LazyImportFiles) Reserve(ctx context.Context, r files.ImportReserve) (files.ImportSlot, error) {
	f, err := l.get()
	if err != nil {
		return files.ImportSlot{}, err
	}
	return f.Reserve(ctx, r)
}

// Slot implements files.ImportFiles.
func (l *LazyImportFiles) Slot(ctx context.Context, fileID ids.ID) (files.ImportSlot, error) {
	f, err := l.get()
	if err != nil {
		return files.ImportSlot{}, err
	}
	return f.Slot(ctx, fileID)
}

// UploadPart implements files.ImportFiles.
func (l *LazyImportFiles) UploadPart(ctx context.Context, cmd files.PutPart) (files.PartRef, error) {
	f, err := l.get()
	if err != nil {
		return files.PartRef{}, err
	}
	return f.UploadPart(ctx, cmd)
}

// Complete implements files.ImportFiles.
func (l *LazyImportFiles) Complete(ctx context.Context, cmd files.CompleteUpload) (files.File, error) {
	f, err := l.get()
	if err != nil {
		return files.File{}, err
	}
	return f.Complete(ctx, cmd)
}
