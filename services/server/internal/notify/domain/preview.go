// SPDX-License-Identifier: Apache-2.0

package domain

// PreviewMode is the value of the personal setting notify.preview_mode (13 section 6.4 table):
// what a preview may show besides the IDs and the badge.
type PreviewMode string

// The three preview modes.
const (
	// PreviewFull shows names, the excerpt, the attachment count and the image.
	PreviewFull PreviewMode = "full"
	// PreviewNames shows the space, channel, thread and sender names only.
	PreviewNames PreviewMode = "names"
	// PreviewNone shows only kind, thread, message and badge.
	PreviewNone PreviewMode = "none"
)

// Valid reports whether m is one of the three modes.
func (m PreviewMode) Valid() bool {
	switch m {
	case PreviewFull, PreviewNames, PreviewNone:
		return true
	}
	return false
}

// ShowsNames reports whether m carries the four name fields (full and names).
func (m PreviewMode) ShowsNames() bool { return m == PreviewFull || m == PreviewNames }

// ShowsContent reports whether m carries the excerpt and the attachments (full only).
func (m PreviewMode) ShowsContent() bool { return m == PreviewFull }

// ParsePreviewMode returns the mode of the stored value of notify.preview_mode, or def when the
// value is empty or not exactly one of full, names and none (brief M3-22 design note 9 c: such a
// value counts as unset). def is notify.default_preview_mode; the configuration schema keeps it
// one of the three.
//
// Example:
//
//	mode := domain.ParsePreviewMode(stored, domain.PreviewFull)
func ParsePreviewMode(stored string, def PreviewMode) PreviewMode {
	if m := PreviewMode(stored); m.Valid() {
		return m
	}
	return def
}

// PreviewKindAllowed reports whether a hint of kind points at a message, so that a preview of it
// exists: message, mention, dm, run_done, run_failed and input_needed. security, admin and the
// badge sync (sync) carry no message (brief M3-22 design note 9 a).
func PreviewKindAllowed(kind string) bool {
	k := Kind(kind)
	return k.Valid() && k.HasThread()
}
