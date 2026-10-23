// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// AppWay is how an app receives its commands, interactions and autocomplete requests
// (docs/spec/11-extensions.md section 9). The way follows the manifest of the installation, not
// the presence of a gateway connection, so it is the same on every api node (OQ-07).
type AppWay string

// The ways to an app.
const (
	// AppWayGateway is an app without delivery.interactions_url in its manifest: the durable event
	// reaches it through its gateway connection, the REST catch-up or a webhook subscription.
	AppWayGateway AppWay = "gateway"
	// AppWayInteractionsURL is a POST to delivery.interactions_url, signed like an outgoing
	// webhook and answered within app.interaction_timeout.
	AppWayInteractionsURL AppWay = "interactions_url"
	// AppWayInactive is an app whose installation is not active (suspended, uninstalled or none).
	AppWayInactive AppWay = "inactive"
)

// AppPost names one stored durable event (command.invoked or interaction.created) whose owning
// app is asked through its interaction address.
type AppPost struct {
	// AppID is the owning app (the audience of the event); EventID the stored event.
	AppID, EventID ids.ID
	// ReplyThreadID is the thread a 200 reply message goes to; zero when there is none (a command
	// in a forum channel without thread_id).
	ReplyThreadID ids.ID
}

// AppAsk is one autocomplete request for an app (ephemeral, never stored).
type AppAsk struct {
	AppID ids.ID
	// Event is command.autocomplete_requested without invocation token.
	Event eventlog.EphemeralEvent
	// Deadline ends the request: the earlier of the end of the person's wait
	// (command.autocomplete_timeout) and now plus app.interaction_timeout.
	Deadline time.Time
}

// The closed list of AppOutcome.Reason: why a request to an app was not answered.
const (
	// AppReasonInactive: the installation is not active; no request was sent.
	AppReasonInactive = "inactive"
	// AppReasonNoURL: the manifest names no interactions_url; no request was sent.
	AppReasonNoURL = "no_url"
	// AppReasonDenied: the re-check with the rights of the moment refused; no request was sent.
	AppReasonDenied = "denied"
	// AppReasonTimeout: no answer within app.interaction_timeout or the deadline.
	AppReasonTimeout = "timeout"
	// AppReasonTransport: the connection failed for another reason.
	AppReasonTransport = "transport"
	// AppReasonBlocked: the host is not in webhook.allowed_hosts or the egress client refused the
	// address.
	AppReasonBlocked = "blocked"
	// AppReasonRedirect: the app answered with a 3xx; redirects are never followed.
	AppReasonRedirect = "redirect"
	// AppReasonHTTPStatus: any status other than 200 and 202 (and 3xx).
	AppReasonHTTPStatus = "http_status"
	// AppReasonTooLarge: the answer exceeds http.max_json_body_bytes.
	AppReasonTooLarge = "too_large"
	// AppReasonInvalidReply: a 200 whose body is no valid MessageCreate or AutocompleteAnswer.
	AppReasonInvalidReply = "invalid_reply"
	// AppReasonNoThread: a 200 with a message, but the request has no thread to write it to.
	AppReasonNoThread = "no_thread"
	// AppReasonReplyRefused: a 200 with a message that the create path refused.
	AppReasonReplyRefused = "reply_refused"
)

// AppReasons is the closed list of AppOutcome.Reason in the order of the specification.
var AppReasons = []string{
	AppReasonInactive, AppReasonNoURL, AppReasonDenied, AppReasonTimeout, AppReasonTransport, AppReasonBlocked, AppReasonRedirect,
	AppReasonHTTPStatus, AppReasonTooLarge, AppReasonInvalidReply, AppReasonNoThread, AppReasonReplyRefused,
}

// AppOutcome is the result of one request to an app. Exactly one of Answered, Deferred and a
// non-empty Reason holds.
type AppOutcome struct {
	// Answered: 200 in time, and its reply was used (a message written, or the choices of Ask).
	Answered bool
	// Deferred: 202 in time; the app answers later through the API (respondToInteraction,
	// answerAutocomplete or createMessage).
	Deferred bool
	// Reason is one code of AppReasons when neither holds.
	Reason string
	// Choices are the suggestions of an answered Ask; nil otherwise.
	Choices []Choice
}

// ErrAfterRequest marks a fault of the server after the request reached the app (writing the reply
// failed with kind internal or unavailable). Post wraps such a fault with it; every other error of
// Post happened before anything was sent. The job runs.app_dispatch returns a fault before the
// request, so the job system runs it again, and ends after one with the outcome transport, because
// nothing is repeated once the app got the request (09 section 1 point 5; 11 section 9).
var ErrAfterRequest = errors.New("runs: a server fault after the request to the app")

// AppDispatcher is the source port through which runs reaches the owning app of a command, a
// component or an autocomplete request (docs/spec/02-architecture.md section 4.1 point 4): runs
// declares it, the module apps implements it (apps/adapters/appdispatch), because only apps reads
// the manifest, the event log and the egress client of the interaction address. Errors are
// returned only for faults of the server (database, programming); every answer of the app is an
// AppOutcome.
//
// Example:
//
//	way, err := d.Way(ctx, appID)
//	if err == nil && way == runs.AppWayInteractionsURL {
//		out, err = d.Post(ctx, runs.AppPost{AppID: appID, EventID: eventID, ReplyThreadID: thread})
//	}
type AppDispatcher interface {
	// Way reports how the app is reached now: inactive without an active installation,
	// interactions_url when the manifest declares delivery.interactions_url, gateway otherwise.
	Way(ctx context.Context, appID ids.ID) (AppWay, error)
	// Post sends the stored event p.EventID to the interaction address of p.AppID, outside every
	// transaction, and writes a 200 reply as a message of the app into p.ReplyThreadID. A fault
	// after the request was sent wraps ErrAfterRequest; any other error is a fault before it.
	Post(ctx context.Context, p AppPost) (AppOutcome, error)
	// Ask sends the autocomplete request a to the interaction address of a.AppID and returns the
	// choices of a valid 200 answer.
	Ask(ctx context.Context, a AppAsk) (AppOutcome, error)
}
