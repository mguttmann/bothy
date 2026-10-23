// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestSetAvatarQuery covers the query of M3-10: SetAvatar writes only inside a unit of work, sets
// and clears avatar_file_id of a person, refuses a file without files row through the foreign key
// and reports that and an unknown person as internal.
func TestSetAvatarQuery(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex := e.mustAddUser(t, "alex", "alex@example.com").Principal.ID
	file := ids.New()
	e.exec(t, `INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose)
		VALUES ($1, $3, '\x00', 1, 'image/png', 'avatar', $2, 'avatar')`, file.String(), alex.String(), "avatars/"+file.String())

	if err := e.repo.SetAvatar(ctx, alex, file); err == nil {
		t.Fatal("SetAvatar outside a unit of work")
	}
	avatar := func() ids.ID {
		rec, _, err := e.repo.User(ctx, alex)
		if err != nil {
			t.Fatal(err)
		}
		return rec.User.Principal.AvatarFileID
	}
	if err := e.write(t, func(ctx context.Context) error { return e.repo.SetAvatar(ctx, alex, file) }); err != nil {
		t.Fatal(err)
	}
	if got := avatar(); got != file {
		t.Fatalf("avatar %s, want %s", got, file)
	}
	// The file id comes from Stage, never from the client, so a missing files row is internal.
	err := e.write(t, func(ctx context.Context) error { return e.repo.SetAvatar(ctx, alex, ids.New()) })
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("SetAvatar of an unknown file = %v, want internal", err)
	}
	err = e.write(t, func(ctx context.Context) error { return e.repo.SetAvatar(ctx, ids.New(), file) })
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("SetAvatar of an unknown person = %v, want internal", err)
	}
	if err := e.write(t, func(ctx context.Context) error { return e.repo.SetAvatar(ctx, alex, ids.ID{}) }); err != nil {
		t.Fatal(err)
	}
	if got := avatar(); !got.IsZero() {
		t.Fatalf("avatar %s after clearing", got)
	}
}
