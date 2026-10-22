// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image/gif"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// GIF block structure (GIF89a specification, sections 17 to 27).
const (
	// gifHeaderSize is the signature and version ("GIF87a" or "GIF89a").
	gifHeaderSize = 6
	// gifScreenSize is the logical screen descriptor: width, height, packed, background, aspect.
	gifScreenSize = 7
	// gifDescriptorSize is the image descriptor after its 0x2C introducer: left, top, width,
	// height, packed.
	gifDescriptorSize = 9
	// gifExtension introduces an extension block, followed by its label.
	gifExtension = 0x21
	// gifImage introduces an image descriptor.
	gifImage = 0x2C
	// gifTrailer ends the data stream.
	gifTrailer = 0x3B
	// gifApplication is the label of an application extension.
	gifApplication = 0xFF
	// gifPlainText, gifGraphicControl and gifComment are the other extension labels (GIF89a).
	gifPlainText      = 0x01
	gifGraphicControl = 0xF9
	gifComment        = 0xFE
	// gifPlainTextFixed is the fixed part image/gif reads after the plain text label, whatever its
	// block size byte says.
	gifPlainTextFixed = 13
	// gifGraphicControlFixed is the whole graphic control extension as image/gif reads it: block
	// size 4, four data bytes and the terminator, without any further sub-block.
	gifGraphicControlFixed = 6
	// gifGraphicControlSize is the block size image/gif requires in a graphic control extension.
	gifGraphicControlSize = 4
	// gifColorTableFlag in a packed byte says a colour table follows.
	gifColorTableFlag = 0x80
	// gifColorTableBits in a packed byte hold log2(entries) - 1 of the colour table.
	gifColorTableBits = 0x07
	// gifLoopSubBlockSize is the size of the loop sub-block of a NETSCAPE2.0 extension: the
	// sub-block id 1 and a 16-bit little-endian loop count.
	gifLoopSubBlockSize = 3
)

// gifLoopApps are the identifiers of the application extensions that carry the loop count.
var gifLoopApps = []string{"NETSCAPE2.0", "ANIMEXTS1.0"}

// gifInfo is what scanGIF reads from the block structure of a GIF.
type gifInfo struct {
	Width, Height int // logical screen size
	Frames        int // number of image descriptors
	LoopCount     int // from the NETSCAPE2.0 (or ANIMEXTS1.0) application extension; 0 = forever
	HasLoop       bool
}

// gifScanner reads the blocks of one GIF; it keeps nothing but the small fixed-size headers.
type gifScanner struct {
	r    *bufio.Reader
	info gifInfo
}

// scanGIF scans the block structure of r without LZW decoding: header, logical screen
// descriptor, global colour table, then extensions and image descriptors with their sub-blocks
// until the trailer 0x3B. The memory is bounded because sub-blocks are skipped, not kept.
// Errors wrap app.ErrImageMalformed for a bad signature, truncated input or an unknown block;
// any other read error of r is returned wrapped as it is, since it says nothing about the file.
func scanGIF(r io.Reader) (gifInfo, error) {
	s := &gifScanner{r: bufio.NewReader(r)}
	if err := s.header(); err != nil {
		return gifInfo{}, gifReadErr(err)
	}
	for {
		done, err := s.block()
		if err != nil {
			return gifInfo{}, gifReadErr(err)
		}
		if done {
			return s.info, nil
		}
	}
}

// gifReadErr turns an end of input into app.ErrImageMalformed (the file is truncated) and keeps
// every other error.
func gifReadErr(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("%w: gif truncated", app.ErrImageMalformed)
	}
	return err
}

// header reads the signature, the logical screen descriptor and skips the global colour table.
func (s *gifScanner) header() error {
	var head [gifHeaderSize + gifScreenSize]byte
	if _, err := io.ReadFull(s.r, head[:]); err != nil {
		return err
	}
	sig := string(head[:gifHeaderSize])
	if sig != "GIF87a" && sig != "GIF89a" {
		return fmt.Errorf("%w: gif signature", app.ErrImageMalformed)
	}
	s.info.Width = int(head[6]) | int(head[7])<<8
	s.info.Height = int(head[8]) | int(head[9])<<8
	return s.colorTable(head[10])
}

// colorTable skips the colour table that the packed byte of a screen or image descriptor
// announces, if any.
func (s *gifScanner) colorTable(packed byte) error {
	if packed&gifColorTableFlag == 0 {
		return nil
	}
	_, err := s.r.Discard(3 << (packed&gifColorTableBits + 1))
	return err
}

// block reads one block and reports whether it was the trailer.
func (s *gifScanner) block() (bool, error) {
	intro, err := s.r.ReadByte()
	if err != nil {
		return false, err
	}
	switch intro {
	case gifTrailer:
		return true, nil
	case gifImage:
		s.info.Frames++
		return false, s.image()
	case gifExtension:
		return false, s.extension()
	default:
		return false, fmt.Errorf("%w: unknown gif block", app.ErrImageMalformed)
	}
}

// image skips an image descriptor, its local colour table, the LZW minimum code size and the
// image data sub-blocks.
func (s *gifScanner) image() error {
	var desc [gifDescriptorSize]byte
	if _, err := io.ReadFull(s.r, desc[:]); err != nil {
		return err
	}
	if err := s.colorTable(desc[8]); err != nil {
		return err
	}
	if _, err := s.r.ReadByte(); err != nil {
		return err
	}
	return s.skipSubBlocks()
}

// extension reads an extension exactly as image/gif reads it, so the frames the scan counts are
// the frames DecodeAll decodes (the pixel guard relies on it, SEC-UP-4): plain text has 13 fixed
// bytes before its sub-blocks, a graphic control extension is 6 fixed bytes without sub-blocks,
// an application extension has an identifier of the length its size byte gives (0 allowed)
// before its sub-blocks, a comment is sub-blocks only, and any other label is malformed.
func (s *gifScanner) extension() error {
	label, err := s.r.ReadByte()
	if err != nil {
		return err
	}
	switch label {
	case gifPlainText:
		if _, err := s.r.Discard(gifPlainTextFixed); err != nil {
			return err
		}
		return s.skipSubBlocks()
	case gifGraphicControl:
		var gc [gifGraphicControlFixed]byte
		if _, err := io.ReadFull(s.r, gc[:]); err != nil {
			return err
		}
		if gc[0] != gifGraphicControlSize {
			return fmt.Errorf("%w: gif graphic control size", app.ErrImageMalformed)
		}
		return nil
	case gifComment:
		return s.skipSubBlocks()
	case gifApplication:
		return s.application()
	default:
		return fmt.Errorf("%w: unknown gif extension", app.ErrImageMalformed)
	}
}

// application reads an application extension: the identifier block of the length its size byte
// gives, then the sub-blocks; a loop extension sets the loop count.
func (s *gifScanner) application() error {
	n, err := s.r.ReadByte()
	if err != nil {
		return err
	}
	id := make([]byte, n)
	if _, err := io.ReadFull(s.r, id); err != nil {
		return err
	}
	if !gifIsLoopApp(id) {
		return s.skipSubBlocks()
	}
	return s.loopCount()
}

// loopCount reads the sub-blocks after a loop extension identifier: the loop sub-block (id 1)
// sets the loop count, the rest is skipped.
func (s *gifScanner) loopCount() error {
	for {
		data, err := s.subBlock()
		if err != nil || data == nil {
			return err
		}
		if len(data) == gifLoopSubBlockSize && data[0] == 1 {
			s.info.LoopCount = int(data[1]) | int(data[2])<<8
			s.info.HasLoop = true
		}
	}
}

// gifIsLoopApp reports whether id is the identifier of a loop count extension.
func gifIsLoopApp(id []byte) bool {
	for _, name := range gifLoopApps {
		if string(id) == name {
			return true
		}
	}
	return false
}

// subBlock reads one data sub-block (at most 255 bytes, the size byte bounds it) and returns its
// data, or nil for the block terminator.
func (s *gifScanner) subBlock() ([]byte, error) {
	n, err := s.r.ReadByte()
	if err != nil || n == 0 {
		return nil, err
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(s.r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// skipSubBlocks discards data sub-blocks up to and including the block terminator.
func (s *gifScanner) skipSubBlocks() error {
	for {
		n, err := s.r.ReadByte()
		if err != nil || n == 0 {
			return err
		}
		if _, err := s.r.Discard(int(n)); err != nil {
			return err
		}
	}
}

// reencodeGIF decodes data with image/gif DecodeAll and encodes it with EncodeAll, keeping the
// frames, delays, disposal methods, palettes (palette indices unchanged), background index and
// loop count. Comments and every application extension other than the loop count fall away
// because EncodeAll writes none of them. DecodeAll reports LoopCount -1 without a loop
// extension and EncodeAll writes one only for an animation with LoopCount >= 0, so a still GIF
// stays without it. A decode or encode error wraps app.ErrImageUndecodable. The caller checks
// the pixel limit before calling it.
func reencodeGIF(data []byte) ([]byte, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrImageUndecodable, err)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrImageUndecodable, err)
	}
	return buf.Bytes(), nil
}
