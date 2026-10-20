// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The PNG structures the filter reads (PNG specification, third edition, 5.2 and 5.3).
const (
	// pngSignature are the eight bytes every PNG starts with.
	pngSignature = "\x89PNG\r\n\x1a\n"
	// pngChunkHeaderLen is the length field and the chunk type.
	pngChunkHeaderLen = 8
	// pngCRCLen is the CRC-32 after the chunk data.
	pngCRCLen = 4
	// pngMaxChunkLen is the largest chunk length the specification allows (2^31-1).
	pngMaxChunkLen = 1<<31 - 1
	// ihdrLen is the data length of IHDR: width, height, bit depth, colour type, compression,
	// filter and interlace method.
	ihdrLen = 13
	// ihdrColourTypeAt is the offset of the colour type in the IHDR data.
	ihdrColourTypeAt = 9
)

// The chunk types the filter reads besides copying them.
const (
	chunkIHDR = "IHDR"
	chunkIEND = "IEND"
	chunkICCP = "iCCP"
	chunkACTL = "acTL"
)

// The reasons of the structure errors that occur at several places.
const (
	reasonBeyondInput = "chunk length beyond the input"
	reasonSignature   = "missing signature"
)

// pngInfo are the facts of one PNG the filter collects while it copies.
type pngInfo struct {
	Width, Height int    // from IHDR
	ICCP          []byte // the data of the iCCP chunk (name, NUL, method, compressed profile), nil when none
	Animated      bool   // an acTL chunk exists
	ColourType    byte   // IHDR colour type
}

// stripPNG streams the cleaned PNG of r into w (brief M3-06 design note 6): it checks the
// signature, requires IHDR first, checks the CRC of every chunk, copies the chunks of
// domain.PNGChunkKept unchanged (length, type, data, CRC), drops all others, stops after IEND
// and ignores the bytes after it. Chunk data is streamed; only IHDR and iCCP are held for
// pngInfo. A CRC mismatch, a length beyond the input or above 2^31-1, a missing IHDR first or a
// missing IEND wrap app.ErrImageMalformed; a write error of w and a read error of r are returned
// unwrapped. On an error the output is incomplete and must be discarded.
func stripPNG(r io.Reader, w io.Writer) (pngInfo, error) {
	bw := bufio.NewWriter(w)
	p := &pngWalker{br: bufio.NewReader(r), out: bw, crc: crc32.NewIEEE()}
	if err := p.run(); err != nil {
		return pngInfo{}, err
	}
	if err := bw.Flush(); err != nil {
		return pngInfo{}, err
	}
	return p.info, nil
}

// pngWalker reads a PNG chunk by chunk and copies the kept chunks to out.
type pngWalker struct {
	br   *bufio.Reader
	out  io.Writer
	crc  hash.Hash32
	info pngInfo
}

// run checks and copies the signature and walks the chunks up to IEND.
func (p *pngWalker) run() error {
	var sig [len(pngSignature)]byte
	if _, err := io.ReadFull(p.br, sig[:]); err != nil {
		return pngReadErr(err, reasonSignature)
	}
	if string(sig[:]) != pngSignature {
		return malformedPNG(reasonSignature)
	}
	if _, err := p.out.Write(sig[:]); err != nil {
		return err
	}
	for first := true; ; first = false {
		typ, err := p.chunk(first)
		if err != nil {
			return err
		}
		if typ == chunkIEND {
			return nil
		}
	}
}

// chunk reads, checks and copies or drops one chunk and returns its type.
func (p *pngWalker) chunk(first bool) (string, error) {
	var head [pngChunkHeaderLen]byte
	if _, err := io.ReadFull(p.br, head[:]); err != nil {
		return "", pngReadErr(err, "missing IEND")
	}
	length := binary.BigEndian.Uint32(head[:4])
	if length > pngMaxChunkLen {
		return "", malformedPNG("chunk length above 2^31-1")
	}
	typ := string(head[4:])
	if first != (typ == chunkIHDR) {
		return "", malformedPNG("IHDR not first")
	}
	keep := domain.PNGChunkKept(typ)
	if err := p.emit(keep, head[:]); err != nil {
		return "", err
	}
	p.crc.Reset()
	_, _ = p.crc.Write(head[4:]) // a hash never fails to write
	if err := p.data(typ, int(length), keep); err != nil {
		return "", err
	}
	return typ, p.checkCRC(keep)
}

// data reads the n data bytes of a chunk of type typ into the CRC and, when keep is set, the
// output. IHDR and iCCP are read whole for pngInfo; every other chunk is streamed.
func (p *pngWalker) data(typ string, n int, keep bool) error {
	switch typ {
	case chunkIHDR, chunkICCP:
		data, err := p.small(n)
		if err != nil {
			return err
		}
		if err := p.noteSmall(typ, data); err != nil {
			return err
		}
		return p.emit(keep, data)
	case chunkACTL:
		p.info.Animated = true
	}
	return p.stream(n, keep)
}

// small reads the n data bytes of a small chunk. The buffer grows with the data actually read, so
// a declared length beyond the input allocates nothing large.
func (p *pngWalker) small(n int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(p.br, int64(n)))
	if err != nil {
		return nil, err
	}
	if len(data) != n {
		return nil, malformedPNG(reasonBeyondInput)
	}
	_, _ = p.crc.Write(data) // a hash never fails to write
	return data, nil
}

// noteSmall records IHDR (which must have 13 bytes) and the first iCCP in pngInfo.
func (p *pngWalker) noteSmall(typ string, data []byte) error {
	if typ == chunkICCP {
		if p.info.ICCP == nil {
			p.info.ICCP = data
		}
		return nil
	}
	if len(data) != ihdrLen {
		return malformedPNG("IHDR length")
	}
	p.info.Width = int(binary.BigEndian.Uint32(data[0:]))
	p.info.Height = int(binary.BigEndian.Uint32(data[4:]))
	p.info.ColourType = data[ihdrColourTypeAt]
	return nil
}

// stream passes n data bytes through the CRC and, when keep is set, to the output, one reader
// buffer at a time, so an IDAT is never held whole.
func (p *pngWalker) stream(n int, keep bool) error {
	for n > 0 {
		buf, err := p.br.Peek(min(n, p.br.Size()))
		if err != nil {
			return pngReadErr(err, reasonBeyondInput)
		}
		_, _ = p.crc.Write(buf) // a hash never fails to write
		if err := p.emit(keep, buf); err != nil {
			return err
		}
		if _, err := p.br.Discard(len(buf)); err != nil {
			return err
		}
		n -= len(buf)
	}
	return nil
}

// checkCRC reads the CRC of the chunk, compares it with the computed one and copies it when keep
// is set.
func (p *pngWalker) checkCRC(keep bool) error {
	var crc [pngCRCLen]byte
	if _, err := io.ReadFull(p.br, crc[:]); err != nil {
		return pngReadErr(err, reasonBeyondInput)
	}
	if !bytes.Equal(crc[:], p.crc.Sum(nil)) {
		return malformedPNG("CRC mismatch")
	}
	return p.emit(keep, crc[:])
}

// emit writes b to the output when keep is set.
func (p *pngWalker) emit(keep bool, b []byte) error {
	if !keep {
		return nil
	}
	_, err := p.out.Write(b)
	return err
}

// malformedPNG returns an error wrapping app.ErrImageMalformed with the reason.
func malformedPNG(reason string) error {
	return fmt.Errorf("%w: png %s", app.ErrImageMalformed, reason)
}

// pngReadErr turns a premature end of the input into a malformed PNG with the reason; every other
// read error is a failure of the store and returned unwrapped, so the job retries.
func pngReadErr(err error, reason string) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return malformedPNG(reason)
	}
	return err
}
