// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// TestChunking: images of 0, 1, exactly a multiple and one byte over a multiple of
// frame.MaxImageChunk bytes split into data frames at the frame limit and reassemble to the
// source (CU-T10).
func TestChunking(t *testing.T) {
	for _, n := range []int{0, 1, frame.MaxImageChunk, 2 * frame.MaxImageChunk, 2*frame.MaxImageChunk + 1} {
		img := make([]byte, n)
		for i := range img {
			img[i] = byte(i * 7)
		}
		chunks := Chunks(img)
		var got []byte
		for _, c := range chunks {
			b, err := frame.Encode(frame.ImageData(c))
			if err != nil {
				t.Fatalf("%d bytes: %v", n, err)
			}
			if len(b) > frame.MaxLen {
				t.Fatalf("%d bytes: frame of %d", n, len(b))
			}
			f, err := frame.Decode(b, frame.KindComputer, frame.FromHost)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, f.Data...)
		}
		want := (n + frame.MaxImageChunk - 1) / frame.MaxImageChunk
		if len(chunks) != want || !bytes.Equal(got, img) {
			t.Errorf("%d bytes: %d chunks (want %d), equal %v", n, len(chunks), want, bytes.Equal(got, img))
		}
	}
}
