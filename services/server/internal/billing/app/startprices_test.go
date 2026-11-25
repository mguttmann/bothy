// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/billing/domain/startprices"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// priceRepo keeps price rows; every other method of app.Repository is unused here.
type priceRepo struct {
	app.Repository
	rows []domain.PriceRow
	by   []ids.ID
}

func (r *priceRepo) ListPrices(context.Context) ([]domain.PriceRow, error) { return r.rows, nil }

func (r *priceRepo) InsertPrice(_ context.Context, p domain.PriceRow, createdBy ids.ID) error {
	r.rows = append(r.rows, p)
	r.by = append(r.by, createdBy)
	return nil
}

// directUoW runs fn at once.
type directUoW struct{}

func (directUoW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	return fn(ctx)
}

// entries keeps the recorded audit entries.
type entries struct {
	auditing.Recorder
	list []auditing.Entry
}

func (e *entries) Record(_ context.Context, en auditing.Entry) error {
	e.list = append(e.list, en)
	return nil
}

// TestStartPriceLoader_LoadsOnceIntoAnEmptyTable (AUD-PKG-R2-owner-server, #894): an empty price
// table gets every row of the start file with the owner as created_by and one audit entry
// price.load_start; a table with any row is left as it is, without an audit entry.
func TestStartPriceLoader_LoadsOnceIntoAnEmptyTable(t *testing.T) {
	t.Parallel()
	want, err := startprices.Rows()
	if err != nil {
		t.Fatal(err)
	}
	repo, rec, owner := &priceRepo{}, &entries{}, ids.New()
	l, err := app.NewStartPriceLoader(directUoW{}, repo, rec, nil)
	if err != nil {
		t.Fatal(err)
	}
	n, err := l.Load(context.Background(), owner)
	if err != nil || n != len(want) || len(repo.rows) != len(want) {
		t.Fatalf("Load = %d, %v; %d rows; want %d", n, err, len(repo.rows), len(want))
	}
	for i, by := range repo.by {
		if by != owner || repo.rows[i].SourceURL == "" || repo.rows[i].RetrievedAt.IsZero() {
			t.Fatalf("row %d: %+v by %v", i, repo.rows[i], by)
		}
	}
	if len(rec.list) != 1 || rec.list[0].Action != "price.load_start" {
		t.Fatalf("audit %+v, want one price.load_start", rec.list)
	}
	if n, err := l.Load(context.Background(), owner); err != nil || n != 0 || len(repo.rows) != len(want) || len(rec.list) != 1 {
		t.Fatalf("second Load = %d, %v; %d rows, %d entries; want nothing new", n, err, len(repo.rows), len(rec.list))
	}
}
