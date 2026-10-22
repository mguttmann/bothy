// SPDX-License-Identifier: Apache-2.0

package profileimages_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/profileimages"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// recorder records the calls of the adapters.
type recorder struct {
	purposes []domain.Purpose
	by       []ids.ID
	variant  domain.Variant
}

func (r *recorder) StageImage(_ context.Context, in app.ImageUpload) (ids.ID, error) {
	r.purposes, r.by = append(r.purposes, in.Purpose), append(r.by, in.UploadedBy)
	return ids.New(), nil
}

func (r *recorder) CompleteStagedImage(_ context.Context, _, by ids.ID, p domain.Purpose) error {
	r.purposes, r.by = append(r.purposes, p), append(r.by, by)
	return nil
}

func (r *recorder) ReleaseImage(_ context.Context, _, by ids.ID, p domain.Purpose) error {
	r.purposes, r.by = append(r.purposes, p), append(r.by, by)
	return nil
}

func (r *recorder) OpenImageVariant(_ context.Context, _ ids.ID, p domain.Purpose, v domain.Variant) (app.ImageVariant, error) {
	r.purposes, r.variant = append(r.purposes, p), v
	return app.ImageVariant{Body: io.NopCloser(strings.NewReader("x")), MIMEType: "image/png", SizeBytes: 1}, nil
}

// TestAdaptersFixThePurpose holds the delegation of the two source ports: avatar for identity,
// branding and the variant medium for instance, the uploader passed through, none for the logo
// release (any owner replaces the logo).
func TestAdaptersFixThePurpose(t *testing.T) {
	ctx := context.Background()
	alex := ids.New()
	r := &recorder{}
	var av identity.AvatarFiles = profileimages.Avatar{Images: r}
	_, _ = av.Stage(ctx, alex, identity.AvatarImage{DeclaredType: "image/png"})
	_ = av.Attach(ctx, ids.New(), alex)
	_ = av.Release(ctx, ids.New(), alex)
	for i, p := range r.purposes {
		if p != domain.PurposeAvatar || r.by[i] != alex {
			t.Fatalf("avatar call %d: %s %v", i, p, r.by[i])
		}
	}
	r = &recorder{}
	var lg instance.LogoFiles = profileimages.Logo{Images: r}
	_, _ = lg.Stage(ctx, alex, instance.LogoImage{DeclaredType: "image/png"})
	_ = lg.Attach(ctx, ids.New(), alex)
	_ = lg.Release(ctx, ids.New())
	v, err := lg.OpenVariant(ctx, ids.New())
	if err != nil || v.ContentType != "image/png" || v.SizeBytes != 1 || r.variant != domain.VariantMedium {
		t.Fatalf("variant %+v %v %s", v, err, r.variant)
	}
	for _, p := range r.purposes {
		if p != domain.PurposeBranding {
			t.Fatalf("logo purpose %s", p)
		}
	}
	if !r.by[2].IsZero() {
		t.Fatal("the logo release names an uploader")
	}
}
