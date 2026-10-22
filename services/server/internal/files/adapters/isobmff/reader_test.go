// SPDX-License-Identifier: Apache-2.0

package isobmff_test

import (
	"bytes"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/isobmff"
	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// read runs ReadMovieInfo over the whole of data.
func read(data []byte) (isobmff.MovieInfo, error) {
	return isobmff.ReadMovieInfo(bytes.NewReader(data), int64(len(data)))
}

// videoTrak is a video track of 1920x1080 with the identity matrix.
func videoTrak() []byte { return trak(tkhd(0, 1920, 1080, identity), "vide") }

// goodFiles are well-formed files with the result ReadMovieInfo must return; the fuzz target
// uses them as seed corpus.
func goodFiles() []struct {
	name string
	data []byte
	want app.MovieInfo
} {
	full := app.MovieInfo{Width: 1920, Height: 1080, HasSize: true, DurationMS: 10500, HasDuration: true}
	return []struct {
		name string
		data []byte
		want app.MovieInfo
	}{
		{"mp4 mvhd version 0", cat(ftyp("isom"), box("moov", mvhd0(600, 6300), videoTrak()), mdat(32)), full},
		{"mp4 mvhd version 1", cat(ftyp("isom"), box("moov", mvhd1(90000, 1<<32), trak(tkhd(1, 1280, 720, identity), "vide"))),
			app.MovieInfo{Width: 1280, Height: 720, HasSize: true, DurationMS: 47721858, HasDuration: true}},
		{"video track after audio track", cat(ftyp("mp42"), box("moov", mvhd0(600, 6300),
			trak(tkhd(0, 0, 0, identity), "soun"), trak(tkhd(0, 640, 480, identity), "soun"), videoTrak())), full},
		{"quicktime with wide and free", cat(ftyp("qt  "), box("wide"), box("free", make([]byte, 7)), mdat(16),
			box("moov", mvhd0(600, 6300), videoTrak())), full},
		{"moov after mdat", cat(ftyp("isom"), mdat(64), box("moov", mvhd0(600, 6300), videoTrak())), full},
		{"matrix 90 degrees swaps", cat(ftyp("isom"), box("moov", mvhd0(600, 6300), trak(tkhd(0, 1920, 1080, rotate90), "vide"))),
			app.MovieInfo{Width: 1080, Height: 1920, HasSize: true, DurationMS: 10500, HasDuration: true}},
		{"matrix 270 degrees swaps", cat(ftyp("isom"), box("moov", mvhd0(600, 6300), trak(tkhd(0, 1920, 1080, rotate270), "vide"))),
			app.MovieInfo{Width: 1080, Height: 1920, HasSize: true, DurationMS: 10500, HasDuration: true}},
		{"fragmented with mehd version 0", cat(ftyp("iso6"), box("moov", mvhd0(1000, 0), videoTrak(), box("mvex", mehd0(4250)))),
			app.MovieInfo{Width: 1920, Height: 1080, HasSize: true, DurationMS: 4250, HasDuration: true}},
		{"fragmented with mehd version 1", cat(ftyp("iso6"), box("moov", mvhd1(1000, ^uint64(0)), videoTrak(), box("mvex", mehd1(9000)))),
			app.MovieInfo{Width: 1920, Height: 1080, HasSize: true, DurationMS: 9000, HasDuration: true}},
		{"audio only", cat(ftyp("M4A "), box("moov", mvhd0(44100, 441000), trak(tkhd(0, 0, 0, identity), "soun"))),
			app.MovieInfo{DurationMS: 10000, HasDuration: true}},
		{"no duration and no video", cat(ftyp("isom"), box("moov", mvhd0(0, 6300))), app.MovieInfo{}},
		{"moov with size 0 to the end", cat(ftyp("isom"), mdat(8), openBox("moov", mvhd0(600, 6300), videoTrak())), full},
		{"moov with largesize", cat(ftyp("isom"), largeBox("moov", mvhd0(600, 6300), videoTrak()), mdat(8)), full},
		{"duration overflow gives no duration", cat(ftyp("isom"), box("moov", mvhd1(1, ^uint64(0)-1), videoTrak())),
			app.MovieInfo{Width: 1920, Height: 1080, HasSize: true}},
	}
}

// TestReadMovieInfo: size and duration of well-formed MP4 and QuickTime files (brief M3-06
// Design note 11).
func TestReadMovieInfo(t *testing.T) {
	for _, tc := range goodFiles() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := read(tc.data)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			viaPort, err := app.MovieReader(isobmff.Reader{}).ReadMovieInfo(bytes.NewReader(tc.data), int64(len(tc.data)))
			if err != nil || viaPort != tc.want {
				t.Fatalf("via port: got %+v, %v", viaPort, err)
			}
		})
	}
}

// TestReadMovieInfoMalformed: a broken box structure is an error with an empty result, never a
// panic.
func TestReadMovieInfoMalformed(t *testing.T) {
	good := cat(ftyp("isom"), box("moov", mvhd0(600, 6300), videoTrak()))
	childTooLarge := cat(ftyp("isom"), box("moov", mvhd0(600, 6300), videoTrak()))
	// The trak header inside moov claims 4 bytes more than moov holds.
	trakAt := len(ftyp("isom")) + 8 + len(mvhd0(600, 6300))
	copy(childTooLarge[trakAt:], be32(u32(len(videoTrak())+4)))
	cases := []struct {
		name string
		data []byte
	}{
		{"child larger than its parent", childTooLarge},
		{"top-level box larger than the file", cat(ftyp("isom"), be32(4096), []byte("moov"), mvhd0(600, 6300))},
		{"size smaller than the header", cat(ftyp("isom"), be32(7), []byte("free"))},
		{"largesize smaller than the header", cat(ftyp("isom"), be32(1), []byte("free"), be64(15))},
		{"truncated mvhd", cat(ftyp("isom"), box("moov", fullBox("mvhd", 0, be32(0), be32(0), be32(600))))},
		{"truncated tkhd", cat(ftyp("isom"), box("moov", box("trak", fullBox("tkhd", 0, make([]byte, 40)))))},
		{"truncated hdlr", cat(ftyp("isom"), box("moov", box("trak", box("mdia", fullBox("hdlr", 0, be32(0))))))},
		{"truncated header", cat(ftyp("isom"), []byte{0, 0, 0})},
		{"truncated largesize", cat(ftyp("isom"), be32(1), []byte("moov"), []byte{0, 0})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := read(tc.data)
			if err == nil {
				t.Fatalf("no error, got %+v", got)
			}
			if got != (app.MovieInfo{}) {
				t.Fatalf("result not empty: %+v", got)
			}
		})
	}
	t.Run("size beyond the readable bytes", func(t *testing.T) {
		got, err := isobmff.ReadMovieInfo(bytes.NewReader(good[:20]), int64(len(good)))
		if err == nil || got != (app.MovieInfo{}) {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
	t.Run("negative size", func(t *testing.T) {
		if _, err := isobmff.ReadMovieInfo(bytes.NewReader(good), -1); err == nil {
			t.Fatal("no error")
		}
	})
}

// FuzzReadMovieInfo: no input panics, and an error always comes with an empty result.
func FuzzReadMovieInfo(f *testing.F) {
	for _, tc := range goodFiles() {
		f.Add(tc.data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := read(data)
		if err != nil && got != (app.MovieInfo{}) {
			t.Fatalf("error %v with result %+v", err, got)
		}
	})
}
