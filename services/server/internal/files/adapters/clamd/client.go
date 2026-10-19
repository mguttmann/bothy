// SPDX-License-Identifier: Apache-2.0

package clamd

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The commands of the client, NUL framed as clamd recommends (12 5.1 item 2, source Q7).
var (
	cmdPing     = []byte("zPING\x00")
	cmdInstream = []byte("zINSTREAM\x00")
	pong        = []byte("PONG")
)

// pingReplyMax bounds the reply to zPING; PONG and its NUL fit with room to spare.
const pingReplyMax = 64

// lengthBytes is the size of the length prefix of a chunk (network byte order).
const lengthBytes = 4

// errNoReply reports a connection that clamd closed without any reply.
var errNoReply = errors.New("clamd: connection closed without a reply")

// ErrNotPong reports a reply to zPING other than PONG.
var ErrNotPong = errors.New("clamd: PING was not answered with PONG")

// Client is the Scanner of the files module over a clamd Unix socket. It is safe for concurrent
// use; at most Config.MaxConcurrent scans run at once, the others wait or end with their context.
type Client struct {
	path    string
	chunk   int
	timeout time.Duration
	sem     chan struct{}
}

// Compile-time check: Client is the Scanner port of app.
var _ app.Scanner = (*Client)(nil)

// New returns a client for cfg. It opens no connection. Errors: an address that is not unix://
// plus an absolute path (ErrAddress), and chunk size, timeout or parallelism that are not
// positive; every error names the key, never the value.
func New(cfg Config) (*Client, error) {
	p, err := cfg.check()
	if err != nil {
		return nil, err
	}
	return &Client{path: p, chunk: cfg.ChunkBytes, timeout: cfg.Timeout, sem: make(chan struct{}, cfg.MaxConcurrent)}, nil
}

// Ping implements app.Scanner: it sends zPING and wants PONG. A connection error, a deadline of
// ctx and any other reply are errors.
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := c.withDeadline(ctx)
	defer cancel()
	conn, stop, err := c.connect(ctx)
	if err != nil {
		return err
	}
	defer stop()
	if _, err := conn.Write(cmdPing); err != nil {
		return c.transport(ctx, "sending PING", err)
	}
	reply, err := io.ReadAll(io.LimitReader(conn, pingReplyMax))
	if err != nil {
		return c.transport(ctx, "reading the PING reply", err)
	}
	if i := bytes.IndexByte(reply, 0); i >= 0 {
		reply = reply[:i]
	}
	if !bytes.Equal(reply, pong) {
		return ErrNotPong
	}
	return nil
}

// Scan implements app.Scanner with zINSTREAM (12 5.1): it waits for a free slot of
// file.scan.max_concurrent, then sends r in chunks of at most file.scan.chunk_bytes, each read of
// r that returned bytes as one chunk with its length, ends with a chunk of length zero and reads
// the reply until clamd closes the connection. A write error does not end the call at once:
// clamd closes the stream at its size limit and replies first, so the reply counts when there is
// one. The reply is bounded by file.scan.chunk_bytes; a longer one is a protocol error. Errors:
// the context (also while waiting for a slot), connection and read errors of r, and replies
// domain.ParseClamdReply refuses (wrapping domain.ErrClamdProtocol).
func (c *Client) Scan(ctx context.Context, r io.Reader) (domain.Verdict, error) {
	ctx, cancel := c.withDeadline(ctx)
	defer cancel()
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return domain.Verdict{}, fmt.Errorf("clamd: waiting for a scan slot: %w", ctx.Err())
	}
	defer func() { <-c.sem }()
	conn, stop, err := c.connect(ctx)
	if err != nil {
		return domain.Verdict{}, err
	}
	defer stop()
	if _, err := conn.Write(cmdInstream); err != nil {
		return c.replyAfterWriteError(ctx, conn, err)
	}
	buf := make([]byte, lengthBytes+c.chunk)
	for {
		n, rerr := r.Read(buf[lengthBytes:])
		if n > 0 {
			binary.BigEndian.PutUint32(buf[:lengthBytes], uint32(n)) //nolint:gosec // G115: n <= file.scan.chunk_bytes <= maxChunkBytes
			if _, err := conn.Write(buf[:lengthBytes+n]); err != nil {
				return c.replyAfterWriteError(ctx, conn, err)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return domain.Verdict{}, fmt.Errorf("clamd: reading the stream: %w", rerr)
		}
	}
	if _, err := conn.Write(make([]byte, lengthBytes)); err != nil {
		return c.replyAfterWriteError(ctx, conn, err)
	}
	return c.readReply(ctx, conn)
}

// replyAfterWriteError reads the reply clamd may have sent before it closed the stream (the
// stream limit); without one the write error counts.
func (c *Client) replyAfterWriteError(ctx context.Context, conn net.Conn, werr error) (domain.Verdict, error) {
	v, err := c.readReply(ctx, conn)
	if err == nil {
		return v, nil
	}
	return domain.Verdict{}, c.transport(ctx, "sending the stream", werr)
}

// readReply reads until clamd closes the connection, at most file.scan.chunk_bytes, and parses it.
func (c *Client) readReply(ctx context.Context, conn net.Conn) (domain.Verdict, error) {
	reply, err := io.ReadAll(io.LimitReader(conn, int64(c.chunk)+1))
	if len(reply) == 0 {
		// A connection closed without a word is a scanner that went away, not a reply.
		if err == nil {
			err = errNoReply
		}
		return domain.Verdict{}, c.transport(ctx, "reading the reply", err)
	}
	if len(reply) > c.chunk {
		return domain.Verdict{}, fmt.Errorf("%w: the reply exceeds file.scan.chunk_bytes", domain.ErrClamdProtocol)
	}
	if err != nil {
		// A reset after a complete reply still leaves the reply; an incomplete one fails to parse.
		if v, perr := domain.ParseClamdReply(reply); perr == nil {
			return v, nil
		}
		return domain.Verdict{}, c.transport(ctx, "reading the reply", err)
	}
	return domain.ParseClamdReply(reply)
}

// withDeadline gives ctx the deadline file.scan.timeout when it has none.
func (c *Client) withDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.timeout)
}

// connect dials the socket and ties the connection to ctx: its deadline becomes the connection
// deadline, and cancelling ctx interrupts blocked reads and writes. stop closes the connection.
func (c *Client) connect(ctx context.Context) (net.Conn, func(), error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.path)
	if err != nil {
		return nil, nil, c.transport(ctx, "connecting", err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	release := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	return conn, func() {
		release()
		_ = conn.Close()
	}, nil
}

// transport wraps a connection error; when ctx ended, its error is wrapped as well, so the scan
// pass can tell a timeout from an unreachable scanner. The text names the step, never data.
func (c *Client) transport(ctx context.Context, step string, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("clamd: %s: %w (%w)", step, ctx.Err(), err)
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		// The connection deadline is the deadline of ctx; its timer may not have fired yet.
		return fmt.Errorf("clamd: %s: %w (%w)", step, context.DeadlineExceeded, err)
	}
	return fmt.Errorf("clamd: %s: %w", step, err)
}
