// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"log/slog"

	authztelemetry "github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	"github.com/OWNER/bothy/services/server/internal/chat"
	chathttp "github.com/OWNER/bothy/services/server/internal/chat/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/files"
	fileshttp "github.com/OWNER/bothy/services/server/internal/files/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	messaginghttp "github.com/OWNER/bothy/services/server/internal/messaging/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	searchhttp "github.com/OWNER/bothy/services/server/internal/search/adapters/httpapi"
	searchmetrics "github.com/OWNER/bothy/services/server/internal/search/adapters/otelmetrics"
	searchpg "github.com/OWNER/bothy/services/server/internal/search/adapters/postgres"
	searchapp "github.com/OWNER/bothy/services/server/internal/search/app"
)

// SearchConfig binds the keys of the search coordinator (docs/spec/14-search.md section 15, brief
// M3-14 design note 12). The error names the key whose binding failed, never a value.
func SearchConfig(v *config.Values) (searchapp.Config, error) {
	var c searchapp.Config
	if err := config.Bind(v, &c); err != nil {
		return searchapp.Config{}, err
	}
	return c, nil
}

// Search is the search module of the process (M3-14): the coordinator of search.Search and its
// REST handlers. Filter mismatches of the page check count in bothy.authz.errors and the duration
// of a search in bothy.search.duration, both on the global meter provider of platform/otel.
type Search struct {
	// Coordinator implements search.Search.
	Coordinator *searchapp.Coordinator
	// Handlers are the REST handlers of search and searchThreadTitles.
	Handlers *searchhttp.Finder
}

// SearchReads are the read operations of the owning modules the search reads through (14
// section 10, R-SU-09).
type SearchReads struct {
	Messaging messaging.SearchReads
	Chat      chat.SearchReads
	Files     files.SearchReads
}

// NewSearch builds the search module over pool with the configurations languages of
// search.languages, the unit of work uow, the Authorizer az, the read operations r, the clock
// (nil means clock.System()) and log (nil means slog.Default()). It opens no connection. Errors:
// a missing collaborator or a failing instrument.
func NewSearch(cfg searchapp.Config, languages []string, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, r SearchReads,
	clk clock.Clock, log *slog.Logger) (*Search, error) {
	if r.Messaging == nil || r.Chat == nil || r.Files == nil {
		return nil, errors.New("wire: search needs the read operations of messaging, chat and files")
	}
	if clk == nil {
		clk = clock.System()
	}
	if log == nil {
		log = slog.Default()
	}
	index, err := searchpg.New(pool, languages, cfg.PrefixMinChars, searchpg.Reads{Messages: r.Messaging, Chat: r.Chat, Files: r.Files})
	if err != nil {
		return nil, err
	}
	counter, err := authztelemetry.NewErrorCounter(nil)
	if err != nil {
		return nil, err
	}
	metrics, err := searchmetrics.New(nil)
	if err != nil {
		return nil, err
	}
	coord, err := searchapp.NewCoordinator(cfg, searchapp.Deps{
		Authorizer: az, UoW: uow, Index: index, Structure: r.Chat, Cursors: searchhttp.Cursors{}, Clock: clk,
		Metrics: metrics, Errors: counter, Log: log,
	})
	if err != nil {
		return nil, err
	}
	h, err := searchhttp.New(coord, searchhttp.Objects{Messages: r.Messaging, Threads: r.Chat, Files: r.Files}, searchhttp.Renderers{
		Message: messaginghttp.MessageREST, Thread: chathttp.ThreadREST, File: fileshttp.FileREST,
	})
	if err != nil {
		return nil, err
	}
	return &Search{Coordinator: coord, Handlers: h}, nil
}

// fillHandlers sets the nil search handler of h to the one s built; a nil s (a role or a test
// without the module) leaves h unchanged.
func (s *Search) fillHandlers(h *Handlers) {
	if s != nil && h.Finder == nil {
		h.Finder = s.Handlers
	}
}
