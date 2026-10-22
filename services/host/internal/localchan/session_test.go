// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
)

// bufRWC is an in-memory channel: writes go to w, reads come from r; werr fails every write.
type bufRWC struct {
	r    bytes.Reader
	w    bytes.Buffer
	werr error
}

func (b *bufRWC) Read(p []byte) (int, error) { return b.r.Read(p) }
func (b *bufRWC) Write(p []byte) (int, error) {
	if b.werr != nil {
		return 0, b.werr
	}
	return b.w.Write(p)
}
func (b *bufRWC) Close() error { return nil }

var testSessionID = [SessionIDLen]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

// TestPacketRoundTrip (24 2.2): a kind 0x01 message carries the session ID and one frame, a kind
// 0x00 message a control message; ReadPacket returns either.
func TestPacketRoundTrip(t *testing.T) {
	ch := &bufRWC{}
	c := NewConn(ch, Peer{})
	if err := c.SendFrame(testSessionID, []byte("frame")); err != nil {
		t.Fatal(err)
	}
	if err := c.Send(OpPTYStarted, "7", PTYStarted{SessionID: "s1"}); err != nil {
		t.Fatal(err)
	}
	ch.r.Reset(ch.w.Bytes())
	p, err := c.ReadPacket()
	if err != nil || !p.IsFrame || p.SessionID != testSessionID || string(p.Frame) != "frame" {
		t.Fatalf("frame packet %+v %v", p, err)
	}
	p, err = c.ReadPacket()
	if err != nil || p.IsFrame || p.Msg.Op != OpPTYStarted || p.Msg.ID != "7" || string(p.Msg.D) != `{"session_id":"s1"}` {
		t.Fatalf("control packet %+v %v", p, err)
	}
	if _, err := c.ReadPacket(); !errors.Is(err, io.EOF) {
		t.Errorf("end of channel %v", err)
	}
}

// TestEncodeFrameLimits: an empty frame and one longer than MaxFrameLen do not fit a kind 0x01
// body; a frame of exactly MaxFrameLen fills the largest length the codec accepts.
func TestEncodeFrameLimits(t *testing.T) {
	for _, n := range []int{0, MaxFrameLen + 1} {
		if b, err := EncodeFrame(testSessionID, make([]byte, n)); !errors.Is(err, ErrFrameTooLarge) || b != nil {
			t.Errorf("%d bytes: %v", n, err)
		}
	}
	b, err := EncodeFrame(testSessionID, make([]byte, MaxFrameLen))
	if err != nil || binary.BigEndian.Uint32(b) != MaxLen {
		t.Fatalf("largest frame %v", err)
	}
	p, err := ReadPacket(bytes.NewReader(b))
	if err != nil || len(p.Frame) != MaxFrameLen {
		t.Errorf("largest frame read %d %v", len(p.Frame), err)
	}
	ch := &bufRWC{}
	if err := NewConn(ch, Peer{}).SendFrame(testSessionID, nil); !errors.Is(err, ErrFrameTooLarge) || ch.w.Len() != 0 {
		t.Errorf("SendFrame of an empty frame %v, %d bytes written", err, ch.w.Len())
	}
}

// TestReadPacketRefuses: a length of zero or above MaxLen, a body shorter than its length, a kind
// 0x01 body without a frame byte after the session ID, an unknown kind and a malformed control
// message each fail, so the channel closes.
func TestReadPacketRefuses(t *testing.T) {
	short := binary.BigEndian.AppendUint32(nil, 10)
	cases := []struct {
		name string
		b    []byte
		want error
	}{
		{"zero length", []byte{0, 0, 0, 0}, ErrTooLarge},
		{"over MaxLen", binary.BigEndian.AppendUint32(nil, MaxLen+1), ErrTooLarge},
		{"body shorter than its length", append(short, 1, 2), io.ErrUnexpectedEOF},
		{"session ID without a frame", raw(KindFrame, testSessionID[:]), ErrMalformed},
		{"unknown kind", raw(0x02, []byte("{}")), ErrUnknownKind},
		{"control message without op", raw(KindControl, []byte(`{"id":"1","d":{}}`)), ErrMalformed},
		{"empty", nil, io.EOF},
	}
	for _, tc := range cases {
		if p, err := ReadPacket(bytes.NewReader(tc.b)); !errors.Is(err, tc.want) || p.IsFrame || p.Msg.Op != "" {
			t.Errorf("%s: %+v %v, want %v", tc.name, p, err, tc.want)
		}
	}
}

// TestSendErrors: a body that does not marshal, a message larger than MaxLen and a failing channel
// are errors of Send and SendFrame, and nothing partial is written.
func TestSendErrors(t *testing.T) {
	ch := &bufRWC{}
	c := NewConn(ch, Peer{})
	if err := c.Send(OpPTYExited, "1", make(chan int)); err == nil || !strings.HasPrefix(err.Error(), "localchan: ") {
		t.Errorf("unmarshalable body %v", err)
	}
	if err := c.Send(OpPTYFailed, "1", PTYFailed{SessionID: strings.Repeat("a", MaxLen)}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("oversized message %v", err)
	}
	if ch.w.Len() != 0 {
		t.Errorf("%d bytes written by failed sends", ch.w.Len())
	}
	broken := errors.New("broken pipe")
	c = NewConn(&bufRWC{werr: broken}, Peer{})
	if err := c.Send(OpPTYStop, "1", PTYStop{SessionID: "s1"}); !errors.Is(err, broken) {
		t.Errorf("Send on a broken channel %v", err)
	}
	if err := c.SendFrame(testSessionID, []byte("x")); !errors.Is(err, broken) {
		t.Errorf("SendFrame on a broken channel %v", err)
	}
}
