// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"
)

// reconnectDelay is the pause of the agent between two connection attempts. A protocol constant,
// not a product window: the service may restart at any time, and the agent must be back before
// the next request without busy looping while the service is down.
const reconnectDelay = time.Second

// Dialog asks the person at the machine about r and returns true only for an explicit allow
// within timeout; it returns false when ctx ends (a cancel of the service or the end of the
// connection), and the dialog closes then.
type Dialog func(ctx context.Context, r Request, timeout time.Duration) bool

// AgentOptions configures RunAgent.
type AgentOptions struct {
	// Endpoint is the endpoint of the service (Endpoint(runtime.GOOS) in production).
	Endpoint string
	// Dialog asks the person (production: NativeDialog).
	Dialog Dialog
	// Log receives codes only; nil discards them.
	Log *slog.Logger
	// Dial connects to the endpoint; nil uses the platform dial (Unix socket or named pipe).
	Dial func(ctx context.Context, endpoint string) (net.Conn, error)
}

// RunAgent connects to the endpoint and answers every confirm request with the dialog, until ctx
// ends; when the connection closes or cannot be made it tries again after reconnectDelay. It
// returns nil when ctx ends and an error only for incomplete options.
func RunAgent(ctx context.Context, o AgentOptions) error {
	if o.Endpoint == "" || o.Dialog == nil {
		return errors.New("confirm: the agent needs an endpoint and a dialog")
	}
	log := o.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	dial := o.Dial
	if dial == nil {
		dial = dialEndpoint
	}
	for {
		c, err := dial(ctx, o.Endpoint)
		if err == nil {
			log.InfoContext(ctx, "confirm_agent_connected")
			serveService(ctx, c, o.Dialog, log)
			log.InfoContext(ctx, "confirm_agent_disconnected")
		} else if ctx.Err() == nil {
			log.DebugContext(ctx, "confirm_agent_dial_failed")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(reconnectDelay):
		}
	}
}

// serveService answers the requests on c until c closes, breaks the line bound or ctx ends; every
// open dialog ends with it.
func serveService(ctx context.Context, c net.Conn, d Dialog, log *slog.Logger) {
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
		stop()
		_ = c.Close()
	}()

	var wmu sync.Mutex
	reply := func(id string, allow bool) {
		wmu.Lock()
		defer wmu.Unlock()
		_, _ = c.Write(encodeLine(answerMsg{Op: opAnswer, ID: id, Allow: allow}))
	}
	open := &openRequests{m: map[string]context.CancelFunc{}}
	r := bufio.NewReaderSize(c, maxLine)
	for {
		line, err := readLine(r)
		if err != nil {
			return
		}
		m, err := decodeService(line)
		if err != nil {
			log.WarnContext(ctx, "confirm_message_ignored")
			continue
		}
		if m.op == opCancel {
			open.cancel(m.id)
			continue
		}
		timeout := time.Duration(m.timeoutMS) * time.Millisecond
		dctx, ok := open.start(ctx, m.id, timeout)
		if !ok {
			log.WarnContext(ctx, "confirm_message_ignored")
			continue
		}
		wg.Go(func() {
			allow := d(dctx, m.req, timeout)
			// Only an allow that came while the request was open counts; a cancelled request
			// gets no answer.
			allow = allow && dctx.Err() == nil
			if open.finish(m.id) {
				reply(m.id, allow)
			}
		})
	}
}

// openRequests are the requests of one connection whose dialog runs, with the function that ends
// each dialog.
type openRequests struct {
	mu sync.Mutex
	m  map[string]context.CancelFunc
}

// start registers id with a dialog context bounded by timeout; false for an id that is open.
func (o *openRequests) start(ctx context.Context, id string, timeout time.Duration) (context.Context, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, dup := o.m[id]; dup {
		return nil, false
	}
	dctx, end := context.WithTimeout(ctx, timeout)
	o.m[id] = end
	return dctx, true
}

// cancel ends the dialog of id; the request then gets no answer.
func (o *openRequests) cancel(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if end, ok := o.m[id]; ok {
		delete(o.m, id)
		end()
	}
}

// finish removes id after its dialog returned and reports whether it was still open.
func (o *openRequests) finish(id string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	end, ok := o.m[id]
	if ok {
		delete(o.m, id)
		end()
	}
	return ok
}
