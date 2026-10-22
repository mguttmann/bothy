// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// TestDeclaredImageTypeAllowed holds the declared types of the schema ImageUpload (brief M3-10
// design note 2b): JPEG, PNG and WebP for avatar and branding, nothing else; attachments are not
// image uploads and never pass this rule.
func TestDeclaredImageTypeAllowed(t *testing.T) {
	for _, p := range []domain.Purpose{domain.PurposeAvatar, domain.PurposeBranding} {
		for _, mt := range []string{"image/jpeg", "image/png", "image/webp", "IMAGE/PNG", "image/png; charset=binary"} {
			if !domain.DeclaredImageTypeAllowed(p, mt) {
				t.Errorf("%s %q refused", p, mt)
			}
		}
		for _, mt := range []string{"image/gif", "image/svg+xml", "image/heic", "text/html", "application/octet-stream", "", "image/png;;"} {
			if domain.DeclaredImageTypeAllowed(p, mt) {
				t.Errorf("%s %q accepted", p, mt)
			}
		}
	}
	for _, p := range []domain.Purpose{domain.PurposeAttachment, domain.PurposeGDPRExport} {
		if domain.DeclaredImageTypeAllowed(p, "image/png") {
			t.Errorf("%s accepted as an image upload", p)
		}
	}
}

// TestStoredOriginalAllowed holds the purpose rule of 12 7.1 for avatar and branding (design note
// 4): only an original the server filtered or converted; attachments are never refused by it.
func TestStoredOriginalAllowed(t *testing.T) {
	allowed := []domain.OriginalKind{domain.OriginalFilteredJPEG, domain.OriginalFilteredPNG, domain.OriginalConvertedWebP}
	refused := []domain.OriginalKind{domain.OriginalReencodedGIF, domain.OriginalUnchanged}
	for _, p := range []domain.Purpose{domain.PurposeAvatar, domain.PurposeBranding} {
		for _, k := range allowed {
			if !domain.StoredOriginalAllowed(p, k) {
				t.Errorf("%s %v refused", p, k)
			}
		}
		for _, k := range refused {
			if domain.StoredOriginalAllowed(p, k) {
				t.Errorf("%s %v accepted", p, k)
			}
		}
	}
	for _, k := range append(allowed, refused...) {
		if !domain.StoredOriginalAllowed(domain.PurposeAttachment, k) {
			t.Errorf("attachment %v refused", k)
		}
	}
}

// TestPlannedOriginal maps the plan of the detected type to the original kind it would store:
// animated WebP, unconvertible still WebP, GIF, HEIC and every other type are refused for avatar
// and branding already in the scan pass, a still WebP only once its conversion fails.
func TestPlannedOriginal(t *testing.T) {
	cases := map[string]domain.OriginalKind{
		"image/jpeg": domain.OriginalFilteredJPEG, "image/png": domain.OriginalFilteredPNG, "image/apng": domain.OriginalFilteredPNG,
		"image/webp": domain.OriginalConvertedWebP, "image/gif": domain.OriginalReencodedGIF, "image/heic": domain.OriginalUnchanged,
		"text/html": domain.OriginalUnchanged, "image/svg+xml": domain.OriginalUnchanged, "image/tiff": domain.OriginalUnchanged,
	}
	for mt, want := range cases {
		if got := domain.PlannedOriginal(domain.OriginalPlanFor(mt)); got != want {
			t.Errorf("%s: %v, want %v", mt, got, want)
		}
	}
	for _, p := range []domain.Purpose{domain.PurposeAvatar, domain.PurposeBranding} {
		for _, mt := range []string{"image/gif", "image/heic", "image/avif", "image/tiff", "image/svg+xml", "text/html", "application/pdf"} {
			if domain.StoredOriginalAllowed(p, domain.PlannedOriginal(domain.OriginalPlanFor(mt))) {
				t.Errorf("%s %s passes the scan", p, mt)
			}
		}
	}
}

// TestImageCap takes the smaller of the two keys in either order (design note 2c) and names the
// key that set it.
func TestImageCap(t *testing.T) {
	for _, c := range []struct {
		maxUpload, part, want int64
		key                   string
	}{
		{100, 50, 50, "file.upload_part_bytes"},
		{50, 100, 50, "file.max_upload_bytes"},
		{70, 70, 70, "file.max_upload_bytes"},
	} {
		if got := domain.ImageCap(c.maxUpload, c.part); got != c.want {
			t.Errorf("ImageCap(%d, %d) = %d, want %d", c.maxUpload, c.part, got, c.want)
		}
		if got := domain.ImageCapSetting(c.maxUpload, c.part); got != c.key {
			t.Errorf("ImageCapSetting(%d, %d) = %s, want %s", c.maxUpload, c.part, got, c.key)
		}
	}
}

// TestFixedOriginalName holds the fixed original_name of avatar and logo files (design note 2f):
// the multipart file name never reaches the row.
func TestFixedOriginalName(t *testing.T) {
	if got := domain.FixedOriginalName(domain.PurposeAvatar); got != "avatar" {
		t.Errorf("avatar: %q", got)
	}
	if got := domain.FixedOriginalName(domain.PurposeBranding); got != "logo" {
		t.Errorf("branding: %q", got)
	}
	if got := domain.FixedOriginalName(domain.PurposeAttachment); got != "" {
		t.Errorf("attachment: %q", got)
	}
}

// TestImagePurpose reports the purposes of the image operations.
func TestImagePurpose(t *testing.T) {
	for p, want := range map[domain.Purpose]bool{
		domain.PurposeAvatar: true, domain.PurposeBranding: true, domain.PurposeAttachment: false, domain.PurposeGDPRExport: false, "": false,
	} {
		if got := p.Image(); got != want {
			t.Errorf("%q.Image() = %v", p, got)
		}
	}
}
