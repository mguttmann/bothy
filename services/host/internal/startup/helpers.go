// SPDX-License-Identifier: Apache-2.0

package startup

import (
	"context"
	"errors"
	"fmt"

	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/localchan"
)

// ChannelHelpers is the doctor.Helpers of the service: it dials the endpoint of each helper,
// checks the peer against the session user and the main PID the service manager reports, and
// sends nothing on a mismatch.
type ChannelHelpers struct {
	GOOS     string
	Users    hostos.Users
	Services hostos.ServiceManager
	// Dial defaults to localchan.DialHelper.
	Dial func(ctx context.Context, endpoint string, expect func() (localchan.Expect, error)) (*localchan.Conn, error)
	// Endpoint defaults to localchan.HelperEndpoint.
	Endpoint func(goos, sessionUser string) string
}

// Open dials the channel of the helper of user, checks its peer and its hello and returns the
// verified channel; M2-45 keeps one open per session user as its session channel. Errors: one
// wrapping doctor.ErrPeerMismatch for a failed peer check (nothing was sent), any other for a
// helper that cannot be reached or answers a wrong hello.
func (h ChannelHelpers) Open(ctx context.Context, user string) (*localchan.Conn, error) {
	dial, endpoint := h.Dial, h.Endpoint
	if dial == nil {
		dial = localchan.DialHelper
	}
	if endpoint == nil {
		endpoint = localchan.HelperEndpoint
	}
	c, err := dial(ctx, endpoint(h.GOOS, user), func() (localchan.Expect, error) {
		a, err := h.Users.Lookup(user)
		if err != nil {
			return localchan.Expect{}, err
		}
		pid, err := h.Services.MainPID(ctx, hostos.HelperServiceName(h.GOOS, user))
		if err != nil {
			return localchan.Expect{}, err
		}
		return localchan.Expect{UID: a.ID, PID: pid}, nil
	})
	if errors.Is(err, localchan.ErrPeerMismatch) {
		return nil, fmt.Errorf("%w", doctor.ErrPeerMismatch)
	}
	if err != nil {
		return nil, err
	}
	var hello struct {
		Version     int    `json:"version"`
		SessionUser string `json:"session_user"`
	}
	if err := c.Call(ctx, localchan.OpHello, struct{}{}, localchan.OpHello, &hello); err != nil || hello.Version != localchan.ProtocolVersion || hello.SessionUser != user {
		_ = c.Close()
		return nil, errors.New("startup: helper hello failed")
	}
	return c, nil
}

// Check implements doctor.Helpers.
func (h ChannelHelpers) Check(ctx context.Context, user string, req doctor.Request) (doctor.HelperResult, error) {
	c, err := h.Open(ctx, user)
	if err != nil {
		return doctor.HelperResult{}, err
	}
	defer func() { _ = c.Close() }()
	var res doctor.HelperResult
	if err := c.Call(ctx, localchan.OpCheckRun, req, localchan.OpCheckResult, &res); err != nil {
		return doctor.HelperResult{}, err
	}
	return res, nil
}

// Scan implements doctor.Helpers.
func (h ChannelHelpers) Scan(ctx context.Context, user string, req doctor.Request) (string, error) {
	c, err := h.Open(ctx, user)
	if err != nil {
		return "", err
	}
	defer func() { _ = c.Close() }()
	var res struct {
		Code string `json:"code"`
	}
	if err := c.Call(ctx, localchan.OpScanRun, req, localchan.OpScanResult, &res); err != nil {
		return "", err
	}
	return res.Code, nil
}
