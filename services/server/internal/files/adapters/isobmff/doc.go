// SPDX-License-Identifier: Apache-2.0

// Package isobmff is the box reader of the files module: it reads display size and duration of
// MP4 and QuickTime files (ISO/IEC 14496-12, the ISO base media file format) for the job
// files.thumbnail (docs/spec/12-files.md 7.3 and 8.1, brief M3-06 Design note 11). It
// implements app.MovieReader.
//
// The reader walks box headers through io.ReaderAt, descends only into moov, trak, mdia and
// mvex, and reads only the small boxes mvhd, tkhd, hdlr and mehd; every other box, mdat
// included, is skipped by its offset, so a large movie costs a handful of small reads. The
// values are display hints, not checks: an error means that nothing is known, and the caller
// never blocks a file because of it.
//
// Example:
//
//	info, err := isobmff.ReadMovieInfo(f, size)
//	if err == nil && info.HasSize {
//		// info.Width x info.Height is the display size after rotation.
//	}
package isobmff
