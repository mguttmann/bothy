// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	importerhttp "github.com/OWNER/bothy/services/server/internal/importer/adapters/httpapi"
	importerpg "github.com/OWNER/bothy/services/server/internal/importer/adapters/postgres"
	importerapp "github.com/OWNER/bothy/services/server/internal/importer/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
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
		Structure: d.Structure, People: d.People, Cursors: importerhttp.Cursors{}, Clock: clk,
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
