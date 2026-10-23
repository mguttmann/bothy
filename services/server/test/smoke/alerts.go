// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// Names of the alert channel the end-to-end test of the alert path sets up (brief M7-20 Design
// note 2, docs/observability/alerting.md section 2).
const (
	// AlertChannelName is the name of the space and of the text channel of the alerts.
	AlertChannelName = "Operations"
	// alertAppSlug is the slug of the app that owns the incoming webhook.
	alertAppSlug = "alertmanager"
	// scopeMessagesWrite is the one scope the webhook app needs (11 section 8).
	scopeMessagesWrite = "messages:write"
	// messagePageLimit is the page size of the polls; the test channel holds a few messages.
	messagePageLimit = 100
)

// AlertREST is what the alert path test needs from the server: the enrollment of the owner's
// device, the step-up with its device key and the operations the owner runs in a session with
// step-up (channel, app with write, incoming webhook), plus reading the channel.
type AlertREST interface {
	// SetCredentials sets the DPoP key of every later proof and the access token.
	SetCredentials(k Key, accessToken string)
	// RedeemEnrollmentCode is redeemEnrollmentCode (POST /auth/enroll/redeem), 201.
	RedeemEnrollmentCode(ctx context.Context, body rest.EnrollRedeemRequest) (rest.TokenResponse, error)
	// CreateStepUpChallenge is createStepUpChallenge (POST /auth/step-up/challenge).
	CreateStepUpChallenge(ctx context.Context) (rest.StepUpChallenge, error)
	// CompleteStepUpWithDeviceKey is completeStepUpWithDeviceKey (POST /auth/step-up/device-key).
	CompleteStepUpWithDeviceKey(ctx context.Context, body rest.StepUpDeviceKeyRequest) (rest.StepUpResult, error)
	// CreateSpace is createSpace (POST /spaces), 201.
	CreateSpace(ctx context.Context, body rest.SpaceCreate) (rest.Space, error)
	// CreateChannel is createChannel (POST /spaces/{space_id}/channels), 201.
	CreateChannel(ctx context.Context, spaceID rest.Uuid, body rest.ChannelCreate) (rest.Channel, error)
	// InstallApp is installApp (POST /apps), 201; it returns the id of the app.
	InstallApp(ctx context.Context, body map[string]any) (rest.Uuid, error)
	// CreateGrant is createGrant (POST /grants), 201.
	CreateGrant(ctx context.Context, body rest.GrantCreate) error
	// CreateIncomingWebhook is createIncomingWebhook (POST /channels/{channel_id}/incoming-webhooks), 201.
	CreateIncomingWebhook(ctx context.Context, channelID rest.Uuid, body rest.IncomingWebhookCreate) (rest.IncomingWebhookCreated, error)
	// ListThreadMessages is listThreadMessages (GET /threads/{thread_id}/messages), the newest page.
	ListThreadMessages(ctx context.Context, threadID rest.Uuid) (rest.MessagePage, error)
}

// AlertChannel is the result of SetupAlertChannel: the main thread the webhook posts into and the
// URL of the webhook, a secret (it holds the token) that is never printed.
type AlertChannel struct {
	ChannelID  rest.Uuid
	ThreadID   rest.Uuid
	WebhookURL string
}

// SetupAlertChannel does what the owner does in the console or the desktop app before the setup
// of alerting.md section 2 (brief M7-20 Design note 1): it redeems the enrollment code of
// setupStdout as a device with a DPoP key and a step-up key, steps up with the device key (07 SR5;
// an API token cannot step up), creates the space and the text channel "Operations", installs an
// app with messages:write, grants it write on the channel and creates the incoming webhook in the
// main thread. Errors name the step, never a value.
//
// Example:
//
//	ch, err := smoke.SetupAlertChannel(ctx, api, setupOut)
//	// ch.WebhookURL goes into a file of mode 0600, never to a log.
func SetupAlertChannel(ctx context.Context, api AlertREST, setupStdout []byte) (AlertChannel, error) {
	code, err := ReadEnrollmentCode(setupStdout)
	if err != nil {
		return AlertChannel{}, err
	}
	dpop, err := NewKey()
	if err != nil {
		return AlertChannel{}, err
	}
	stepUp, err := NewKey()
	if err != nil {
		return AlertChannel{}, err
	}
	api.SetCredentials(dpop, "")
	tr, err := api.RedeemEnrollmentCode(ctx, rest.EnrollRedeemRequest{
		EnrollmentCode: code,
		Device: rest.DeviceRegistration{
			DeviceId: newUUIDv7(), Name: "alert path test", Platform: rest.DevicePlatformMacos,
			Alg: rest.DeviceRegistrationAlgES256, PublicJwk: dpop.PublicJWK(),
			StepUpKey: &rest.DeviceKeyRegistration{Alg: rest.DeviceKeyRegistrationAlgES256, PublicJwk: stepUp.PublicJWK()},
		},
	})
	if err != nil {
		return AlertChannel{}, fmt.Errorf("enroll: %w", err)
	}
	api.SetCredentials(dpop, tr.AccessToken)
	ch, err := api.CreateStepUpChallenge(ctx)
	if err != nil {
		return AlertChannel{}, fmt.Errorf("step-up: %w", err)
	}
	sig, err := StepUpSignature(stepUp, ch.Challenge, tr.Session.Id.String(), time.Now())
	if err != nil {
		return AlertChannel{}, err
	}
	if _, err := api.CompleteStepUpWithDeviceKey(ctx, rest.StepUpDeviceKeyRequest{ChallengeId: ch.ChallengeId, Signature: sig}); err != nil {
		return AlertChannel{}, fmt.Errorf("step-up: %w", err)
	}
	space, err := api.CreateSpace(ctx, rest.SpaceCreate{Name: AlertChannelName})
	if err != nil {
		return AlertChannel{}, fmt.Errorf("space: %w", err)
	}
	channel, err := api.CreateChannel(ctx, space.Id, rest.ChannelCreate{Kind: rest.ChannelCreateKindText, Name: AlertChannelName})
	if err != nil {
		return AlertChannel{}, fmt.Errorf("channel: %w", err)
	}
	if channel.MainThreadId == nil {
		return AlertChannel{}, errors.New("smoke: the alert channel has no main_thread_id")
	}
	app, err := api.InstallApp(ctx, map[string]any{
		"manifest": map[string]any{
			"schema_version": 1, "slug": alertAppSlug, "name": "Alertmanager", "description": "Posts the alerts of the instance.",
			"publisher": "Example Org", "version": "1.0.0", "min_api_version": "0.1",
			"scopes": map[string]any{"required": []string{scopeMessagesWrite}, "optional": []string{}},
		},
		"granted_scopes": []string{scopeMessagesWrite},
		"reason":         "Alerts of the instance",
	})
	if err != nil {
		return AlertChannel{}, fmt.Errorf("app: %w", err)
	}
	write := rest.Level("write")
	channelID := channel.Id
	if err := api.CreateGrant(ctx, rest.GrantCreate{
		SubjectKind: rest.GrantSubjectKind("app"), SubjectId: app, TargetKind: rest.TargetKind("channel"), TargetId: &channelID,
		Level: &write, Reason: "Alerts of the instance",
	}); err != nil {
		return AlertChannel{}, fmt.Errorf("grant: %w", err)
	}
	hook, err := api.CreateIncomingWebhook(ctx, channel.Id, rest.IncomingWebhookCreate{AppId: app, Name: "Alertmanager"})
	if err != nil {
		return AlertChannel{}, fmt.Errorf("webhook: %w", err)
	}
	if hook.Url == "" {
		return AlertChannel{}, errors.New("smoke: createIncomingWebhook returned no url")
	}
	return AlertChannel{ChannelID: channel.Id, ThreadID: *channel.MainThreadId, WebhookURL: hook.Url}, nil
}

// ExpectedAlert is one alert of the test rule the channel must show: its name, its severity label
// and its summary annotation.
type ExpectedAlert struct {
	Name, Severity, Summary string
}

// ParseExpectedAlert parses "<name>|<severity>|<summary>", the form of the flag --expect.
func ParseExpectedAlert(s string) (ExpectedAlert, error) {
	parts := strings.SplitN(s, "|", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ExpectedAlert{}, fmt.Errorf("smoke: expected alert %q is not <name>|<severity>|<summary>", s)
	}
	return ExpectedAlert{Name: parts[0], Severity: parts[1], Summary: parts[2]}, nil
}

// State is the state a message of the format alertmanager reports: firing or resolved.
type State string

// The two states of a notification of Alertmanager (11 section 8).
const (
	StateFiring   State = "firing"
	StateResolved State = "resolved"
)

// Matches reports whether content is the message of the format alertmanager for alert a in state
// st: the heading "**FIRING** <name>" or "**RESOLVED** <name>" and an item with the state, the
// name, the severity and the summary (internal/apps/domain/incoming, golden files).
func (a ExpectedAlert) Matches(content string, st State) bool {
	name := escapeMarkdown(a.Name)
	head := "**" + strings.ToUpper(string(st)) + "** " + name + " "
	item := "- **" + string(st) + "** " + name + ", severity " + escapeMarkdown(a.Severity) + ": " + escapeMarkdown(a.Summary)
	return strings.HasPrefix(content, head) && strings.Contains(content, item)
}

// AlertMessage is one message of the alert channel as the test records it: origin and content.
type AlertMessage struct {
	Origin    string `json:"origin"`
	ContentMd string `json:"content_md"`
}

// AwaitAlerts polls the main thread every interval until, for every alert of want, a message in
// state st exists, and returns all messages of the thread. Every message must have origin app: a
// message of an incoming webhook never triggers an agent (11 section 8, brief M7-20 requirement
// 14). The error names the missing alerts when ctx ends first.
func AwaitAlerts(ctx context.Context, api AlertREST, thread rest.Uuid, want []ExpectedAlert, st State, interval time.Duration, log io.Writer) ([]AlertMessage, error) {
	if log == nil {
		log = io.Discard
	}
	for {
		page, err := api.ListThreadMessages(ctx, thread)
		if err != nil {
			return nil, err
		}
		msgs := make([]AlertMessage, 0, len(page.Items))
		for _, m := range page.Items {
			msgs = append(msgs, AlertMessage{Origin: string(m.Origin), ContentMd: m.ContentMd})
			if m.Origin != rest.MessageOrigin("app") {
				return msgs, fmt.Errorf("smoke: a message of the alert channel has origin %q, want app", m.Origin)
			}
		}
		var missing []string
		for _, a := range want {
			if !anyMatch(msgs, a, st) {
				missing = append(missing, a.Name)
			}
		}
		if len(missing) == 0 {
			_, _ = fmt.Fprintf(log, "ok %s messages of %d alerts\n", st, len(want))
			return msgs, nil
		}
		select {
		case <-ctx.Done():
			return msgs, fmt.Errorf("smoke: no %s message for %s: %w", st, strings.Join(missing, ", "), ctx.Err())
		case <-time.After(interval):
		}
	}
}

// escapeMarkdown is the escape the server applies to payload values (11 section 8): a backslash
// before every ASCII punctuation or symbol character, so the expectation matches the rendered text.
func escapeMarkdown(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < utf8.RuneSelf && (unicode.IsPunct(r) || unicode.IsSymbol(r)) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func anyMatch(msgs []AlertMessage, a ExpectedAlert, st State) bool {
	for _, m := range msgs {
		if a.Matches(m.ContentMd, st) {
			return true
		}
	}
	return false
}

// InstallApp implements AlertREST.
func (c *HTTPClient) InstallApp(ctx context.Context, body map[string]any) (rest.Uuid, error) {
	var out struct {
		ID rest.Uuid `json:"id"`
	}
	err := c.do(ctx, call{op: "installApp", method: http.MethodPost, path: "/apps", body: body, idempotent: true, want: http.StatusCreated, out: &out})
	return out.ID, err
}

// CreateGrant implements AlertREST.
func (c *HTTPClient) CreateGrant(ctx context.Context, body rest.GrantCreate) error {
	return c.do(ctx, call{op: "createGrant", method: http.MethodPost, path: "/grants", body: body, idempotent: true, want: http.StatusCreated})
}

// CreateIncomingWebhook implements AlertREST.
func (c *HTTPClient) CreateIncomingWebhook(ctx context.Context, channelID rest.Uuid, body rest.IncomingWebhookCreate) (rest.IncomingWebhookCreated, error) {
	var out rest.IncomingWebhookCreated
	return out, c.do(ctx, call{op: "createIncomingWebhook", method: http.MethodPost, path: "/channels/" + channelID.String() + "/incoming-webhooks",
		body: body, idempotent: true, want: http.StatusCreated, out: &out})
}

// ListThreadMessages implements AlertREST.
func (c *HTTPClient) ListThreadMessages(ctx context.Context, threadID rest.Uuid) (rest.MessagePage, error) {
	var out rest.MessagePage
	q := url.Values{"limit": {strconv.Itoa(messagePageLimit)}}
	return out, c.do(ctx, call{op: "listThreadMessages", method: http.MethodGet, path: "/threads/" + threadID.String() + "/messages", query: q,
		want: http.StatusOK, out: &out})
}
