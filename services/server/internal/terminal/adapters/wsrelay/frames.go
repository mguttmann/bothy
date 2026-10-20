// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"errors"
	"io"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The frames of the device stream and the data connection after identify (docs/spec/24-terminal.md
// sections 5.2 and 5.4): one WebSocket binary message is exactly one Noise message of at most
// domain.NoiseMaxMessage bytes; a text frame or a larger message is a protocol error (4002). The
// library's read limit is switched off and the limit is enforced here, so the close code is 4002,
// not the library's 1009.

// errProtocol is a text frame after identify or a binary message above the Noise limit.
var errProtocol = errors.New("wsrelay: protocol error")

// readFrame returns the next Noise message of r: the bytes of one binary message. A text message
// or a binary message above domain.NoiseMaxMessage is errProtocol; the end of the connection is
// the read error (a websocket.CloseError when the peer sent a close frame).
func readFrame(ctx context.Context, ws *websocket.Conn) ([]byte, error) {
	typ, rd, err := ws.Reader(ctx)
	if err != nil {
		return nil, err
	}
	return classifyFrame(typ, rd)
}

// classifyFrame is the pure part of readFrame: it reads at most one byte above the Noise limit,
// never more, and never returns a frame above it.
func classifyFrame(typ websocket.MessageType, rd io.Reader) ([]byte, error) {
	if typ != websocket.MessageBinary {
		return nil, errProtocol
	}
	b, err := io.ReadAll(io.LimitReader(rd, domain.NoiseMaxMessage+1))
	if err != nil {
		return nil, err
	}
	if len(b) > domain.NoiseMaxMessage {
		return nil, errProtocol
	}
	return b, nil
}
