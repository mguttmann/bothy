// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakeAvatars opens the image like the use case does after authorize and returns view.
type fakeAvatars struct {
	caller   actor.Actor
	declared string
	body     string
	view     identity.UserView
	deleted  bool
}

func (f *fakeAvatars) SetMyAvatar(_ context.Context, a actor.Actor, open identity.AvatarOpener) (identity.UserView, error) {
	f.caller = a
	img, err := open()
	if err != nil {
		return identity.UserView{}, err
	}
	b, err := io.ReadAll(img.Body)
	if err != nil {
		return identity.UserView{}, err
	}
	f.declared, f.body = img.DeclaredType, string(b)
	return f.view, nil
}

func (f *fakeAvatars) DeleteMyAvatar(_ context.Context, a actor.Actor) error {
	f.caller, f.deleted = a, true
	return nil
}

// imageBody builds a multipart body with one part per name, each declared as image/png.
func imageBody(t *testing.T, names ...string) *multipart.Reader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, n := range names {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="`+n+`"; filename="me.png"`)
		h.Set("Content-Type", "image/png")
		pw, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = pw.Write([]byte("pixels of " + n))
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return multipart.NewReader(&buf, mw.Boundary())
}

// TestAvatarsHandler: setMyAvatar hands the use case an opener over the part "file" and answers
// 200 with the User, avatar_file_id present when set and absent otherwise; an opener error
// (no part "file", a second part) reaches the problem writer unchanged; deleteMyAvatar answers 204.
func TestAvatarsHandler(t *testing.T) {
	t.Parallel()
	alex := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	ctx := actor.WithActor(context.Background(), alex)
	avatar := ids.New()
	f := &fakeAvatars{view: identity.UserView{PrincipalView: identity.PrincipalView{ID: alex.ID, Handle: "alex", DisplayName: "Alex", AvatarFileID: avatar}}}
	h := httpapi.NewAvatars(f)

	res, err := h.SetMyAvatar(ctx, rest.SetMyAvatarRequestObject{Body: imageBody(t, "file")})
	if err != nil {
		t.Fatal(err)
	}
	ok, isOK := res.(rest.SetMyAvatar200JSONResponse)
	if !isOK || ok.Body.AvatarFileId == nil || ids.ID(*ok.Body.AvatarFileId) != avatar || ok.Body.Handle != "alex" {
		t.Fatalf("response %#v", res)
	}
	if f.caller.ID != alex.ID || f.declared != "image/png" || f.body != "pixels of file" {
		t.Fatalf("use case saw caller %+v, %q, %q", f.caller, f.declared, f.body)
	}

	f.view.AvatarFileID = ids.ID{}
	res, err = h.SetMyAvatar(ctx, rest.SetMyAvatarRequestObject{Body: imageBody(t, "file")})
	if ok, isOK := res.(rest.SetMyAvatar200JSONResponse); err != nil || !isOK || ok.Body.AvatarFileId != nil {
		t.Fatalf("without avatar: %#v, %v", res, err)
	}

	for _, names := range [][]string{{"photo"}, {"file", "extra"}, {}} {
		_, err := h.SetMyAvatar(ctx, rest.SetMyAvatarRequestObject{Body: imageBody(t, names...)})
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed {
			t.Fatalf("parts %v: %v, want validation_failed", names, err)
		}
	}

	del, err := h.DeleteMyAvatar(ctx, rest.DeleteMyAvatarRequestObject{})
	if _, isNoContent := del.(rest.DeleteMyAvatar204Response); err != nil || !isNoContent || !f.deleted || f.caller.ID != alex.ID {
		t.Fatalf("delete: %#v, %v", del, err)
	}

	// Without a principal in ctx the use case sees the anonymous caller and refuses it.
	f.caller = alex
	if _, err := h.DeleteMyAvatar(context.Background(), rest.DeleteMyAvatarRequestObject{}); err != nil || !f.caller.IsAnonymous() {
		t.Fatalf("anonymous caller %+v, %v", f.caller, err)
	}
}
