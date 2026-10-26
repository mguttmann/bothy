// SPDX-License-Identifier: Apache-2.0

// Command termgen is the output generator of the terminal performance level of the desktop app
// (brief M4-38 Design note 2; docs/spec/15-client-architecture.md 19 and 23.5): the client stack
// copies it into the host container and names it as two `command` profiles of the host service
// (docs/spec/24-terminal.md 11.1), so its output takes the real path of a session (PTY, Noise,
// relay, renderer). It writes only synthetic text.
//
// Modes:
//
//	termgen rate --interval 100ms --lines 1100 --width 64
//	    one line "TERMGEN <seq> <padding>" per interval (sequence from 1, six digits), paced on an
//	    absolute schedule so a late line does not shift the following ones; then "TERMGEN-DONE".
//	    The app measures client.perf.terminal_frame_to_paint_p95 per sequence number.
//	termgen burst --chunk-bytes 262144 --chunks 16 --width 80
//	    waits for a line on stdin, then writes one chunk of exactly chunk-bytes bytes of lines
//	    "BURST <chunk> <line> <padding>" at full speed and the end line "TERMGEN-END <chunk>
//	    <bytes>", once per chunk; then "TERMGEN-DONE". The app types the line feed once it drew
//	    the end line of the chunk before, so at most one chunk is in flight: the host's send queue
//	    and the relay buffer disconnect a reader that falls behind (24 6.2), and the app measures
//	    client.perf.terminal_throughput_min per chunk, without the acknowledgement round trips.
//
// Exit code 0 after TERMGEN-DONE, 2 for bad arguments, 1 when writing or reading fails (the
// session ended).
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// The markers the app's performance level looks for (apps/client/perf_test/terminal_perf.dart).
const (
	ratePrefix = "TERMGEN "
	endPrefix  = "TERMGEN-END "
	doneLine   = "TERMGEN-DONE\n"
	burstLine  = "BURST "
	// writeBlock is the size of one write of a burst: large writes keep the PTY reads, and with
	// them the messages of the host service, large.
	writeBlock = 64 << 10
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, realClock{}))
}

// clock is the time source of the rate mode; tests pass their own.
type clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

// run runs the mode of args and returns the exit code.
func run(args []string, in io.Reader, out, errOut io.Writer, c clock) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(errOut, "usage: termgen rate|burst [flags]")
		return 2
	}
	fs := flag.NewFlagSet("termgen "+args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	width := fs.Int("width", 64, "line length without the line feed")
	var err error
	switch args[0] {
	case "rate":
		interval := fs.Duration("interval", 100*time.Millisecond, "time between two lines")
		lines := fs.Int("lines", 1100, "number of lines")
		if fs.Parse(args[1:]) != nil || *interval <= 0 || *lines <= 0 || *width < len(rateLine(1, 0)) {
			return 2
		}
		err = Rate(out, c, *interval, *lines, *width)
	case "burst":
		chunk := fs.Int("chunk-bytes", 256<<10, "bytes of one chunk")
		chunks := fs.Int("chunks", 16, "number of chunks")
		if fs.Parse(args[1:]) != nil || *chunks <= 0 || *width < 32 || *chunk < *width+1 {
			return 2
		}
		err = Burst(out, bufio.NewReader(in), *chunk, *chunks, *width)
	default:
		_, _ = fmt.Fprintf(errOut, "termgen: unknown mode %q\n", args[0])
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintln(errOut, "termgen:", err)
		return 1
	}
	return 0
}

// rateLine returns line seq of the rate mode, padded to width characters plus a line feed.
func rateLine(seq, width int) string {
	head := fmt.Sprintf("%s%06d ", ratePrefix, seq)
	if pad := width - len(head); pad > 0 {
		head += strings.Repeat("r", pad)
	}
	return head + "\n"
}

// Rate writes lines lines, one per interval on the schedule start + (seq-1) * interval, then the
// done line.
func Rate(out io.Writer, c clock, interval time.Duration, lines, width int) error {
	start := c.Now()
	for seq := 1; seq <= lines; seq++ {
		if wait := start.Add(time.Duration(seq-1) * interval).Sub(c.Now()); wait > 0 {
			c.Sleep(wait)
		}
		if _, err := io.WriteString(out, rateLine(seq, width)); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, doneLine)
	return err
}

// ChunkBody returns the bytes of chunk i: lines "BURST <i> <line> <padding>" of width characters
// plus a line feed, the last one shortened so the body has exactly size bytes.
func ChunkBody(i, size, width int) []byte {
	b := make([]byte, 0, size)
	for line := 0; len(b) < size; line++ {
		l := fmt.Sprintf("%s%02d %06d ", burstLine, i, line)
		if pad := width - len(l); pad > 0 {
			l += strings.Repeat("b", pad)
		}
		l += "\n"
		if rest := size - len(b); len(l) > rest {
			// The last line: as many characters as fit, still ending with a line feed.
			l = l[:rest-1] + "\n"
		}
		b = append(b, l...)
	}
	return b
}

// EndLine returns the end line of chunk i of size bytes.
func EndLine(i, size int) string { return fmt.Sprintf("%s%d %d\n", endPrefix, i, size) }

// errNoAck is the end of stdin before the acknowledgement of a chunk.
var errNoAck = errors.New("stdin ended before the next chunk was acknowledged")

// Burst writes chunks chunks of size bytes, each after one line of in and followed by its end
// line, then the done line.
func Burst(out io.Writer, in *bufio.Reader, size, chunks, width int) error {
	for i := 0; i < chunks; i++ {
		if _, err := in.ReadString('\n'); err != nil {
			return errNoAck
		}
		body := ChunkBody(i, size, width)
		for len(body) > 0 {
			n := min(writeBlock, len(body))
			if _, err := out.Write(body[:n]); err != nil {
				return err
			}
			body = body[n:]
		}
		if _, err := io.WriteString(out, EndLine(i, size)); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, doneLine)
	return err
}
