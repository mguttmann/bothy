// SPDX-License-Identifier: Apache-2.0

package computer

import "github.com/OWNER/bothy/services/host/internal/frame"

// Chunks splits img into the contents of cu_image_data frames of at most frame.MaxImageChunk
// bytes each, in order (docs/spec/27-computer-use.md 6.3, CU-T10). The chunks alias img. An empty
// image gives no chunk.
//
// Example: an image of frame.MaxImageChunk+1 bytes gives two chunks of frame.MaxImageChunk and 1.
func Chunks(img []byte) [][]byte {
	out := make([][]byte, 0, (len(img)+frame.MaxImageChunk-1)/frame.MaxImageChunk)
	for len(img) > 0 {
		n := min(len(img), frame.MaxImageChunk)
		out = append(out, img[:n])
		img = img[n:]
	}
	return out
}
