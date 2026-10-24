// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestFileImageConfigKeys (brief M3-06 design note 13): every key is registered with the default
// of docs/spec/12-files.md 18 and chapter 12; ImageConfig binds them; the cross-check
// thumbnail_sizes_ordered refuses equal and descending sizes and names keys, never values.
func TestFileImageConfigKeys(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"file.image.max_pixels": "50000000", "file.image.jpeg_quality": "90", "file.image.processing_timeout": "60s",
		"file.image.max_concurrent": "2", "file.thumbnail.small_px": "320", "file.thumbnail.medium_px": "1024",
		"file.thumbnail.large_px": "2048", "file.thumbnail.jpeg_quality": "82",
	}
	keys := map[string]config.Key{}
	for _, k := range schema(t).Keys() {
		keys[k.Name] = k
	}
	for name, def := range want {
		k, ok := keys[name]
		if !ok || k.Default != def || k.Chapter != "12-files.md" || k.Doc == "" {
			t.Errorf("%s: %+v, want default %s in chapter 12", name, k, def)
		}
	}
	for _, unbuilt := range []string{"file.image.libvips_enabled", "file.poster_max_bytes"} {
		if _, ok := keys[unbuilt]; ok {
			t.Errorf("%s is registered", unbuilt)
		}
	}
	cfg, err := wire.ImageConfig(filesValues(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxPixels != 50000000 || cfg.JPEGQuality != 90 || cfg.ProcessingTimeout != time.Minute || cfg.MaxConcurrent != 2 ||
		cfg.SmallPx != 320 || cfg.MediumPx != 1024 || cfg.LargePx != 2048 || cfg.ThumbnailJPEGQuality != 82 {
		t.Errorf("ImageConfig %+v", cfg)
	}
	for name, env := range map[string][]string{
		"equal":      {"BOTHY_FILE__THUMBNAIL__SMALL_PX=1024"},
		"descending": {"BOTHY_FILE__THUMBNAIL__MEDIUM_PX=4096"},
		"large":      {"BOTHY_FILE__THUMBNAIL__LARGE_PX=999"},
	} {
		got := load(t, env...)
		if len(got) != 1 || !strings.Contains(got[0].String(), ": thumbnail_sizes_ordered: ") ||
			!strings.Contains(got[0].String(), "file.thumbnail.small_px") || !strings.Contains(got[0].String(), "file.thumbnail.large_px") {
			t.Errorf("%s: problems %v, want thumbnail_sizes_ordered", name, got)
		}
		noValues(t, got, "1024", "4096", "999", "320", "2048")
	}
	for _, bad := range []string{"BOTHY_FILE__IMAGE__JPEG_QUALITY=101", "BOTHY_FILE__THUMBNAIL__JPEG_QUALITY=0", "BOTHY_FILE__IMAGE__MAX_PIXELS=0",
		"BOTHY_FILE__IMAGE__MAX_CONCURRENT=0", "BOTHY_FILE__IMAGE__PROCESSING_TIMEOUT=0s"} {
		if got := load(t, bad); len(got) != 1 {
			t.Errorf("%s: problems %v, want one", bad, got)
		}
	}
}

// TestThumbnailJobRegistration: every role registers files.thumbnail on the queue files with an
// argument type of IDs only.
func TestThumbnailJobRegistration(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{TerminalSweepInterval: time.Minute, FilesGCInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute,
		KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, FilesImageTimeout: time.Minute, StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("files.thumbnail"); !ok || q != jobs.QueueFiles {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	if typ := reg.ArgTypes()["files.thumbnail"]; typ != reflect.TypeFor[filesjobs.ThumbnailArgs]() {
		t.Fatalf("argument type %v", typ)
	}
}
