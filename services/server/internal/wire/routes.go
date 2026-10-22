// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/OWNER/bothy/services/server/api/routes"
	agentshttp "github.com/OWNER/bothy/services/server/internal/agents/adapters/httpapi"
	appshttp "github.com/OWNER/bothy/services/server/internal/apps/adapters/httpapi"
	audithttp "github.com/OWNER/bothy/services/server/internal/audit/adapters/httpapi"
	authzhttp "github.com/OWNER/bothy/services/server/internal/authz/adapters/httpapi"
	chathttp "github.com/OWNER/bothy/services/server/internal/chat/adapters/httpapi"
	fileshttp "github.com/OWNER/bothy/services/server/internal/files/adapters/httpapi"
	identityhttp "github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	instancehttp "github.com/OWNER/bothy/services/server/internal/instance/adapters/httpapi"
	messaginghttp "github.com/OWNER/bothy/services/server/internal/messaging/adapters/httpapi"
	notifyhttp "github.com/OWNER/bothy/services/server/internal/notify/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	runshttp "github.com/OWNER/bothy/services/server/internal/runs/adapters/httpapi"
	statshttp "github.com/OWNER/bothy/services/server/internal/stats/adapters/httpapi"
	synchttp "github.com/OWNER/bothy/services/server/internal/sync/adapters/httpapi"
	terminalhttp "github.com/OWNER/bothy/services/server/internal/terminal/adapters/httpapi"
)

// logKeyError is the attribute key of the joined error in the "route check failed" record
// (docs/DECISIONS.md M1-17/7). sloglint (M1-10) asks for constant keys; the value is unchanged.
const logKeyError = "error"

// Role is the role of a server process (02 "Roles of a process"): api serves REST, gateway
// and fan-out, worker runs jobs and consumers, all does both.
type Role string

// Process roles; the value is the argument of `bothy serve --role`.
const (
	RoleAPI    Role = "api"
	RoleWorker Role = "worker"
	RoleAll    Role = "all"
)

// Mux is the public mux the REST routes are mounted on; *http.ServeMux of the public listener
// implements it.
type Mux interface { // NOSONAR: names the public mux it stands for; see M1-17/33.
	Handle(pattern string, handler http.Handler)
}

// Handlers is the composite of every module's adapters/httpapi handler (02 "Layers within a module",
// rule 3): each module embeds the part of the generated strict server interface it implements,
// so the compiler proves that the interface is complete and that no operation is implemented
// twice. The compiler proves only the method set: the start (wire.Serve) fills every nil module
// handler with the one it builds from its dependencies before MountRoutes.
type Handlers struct {
	// Access implements listGroups, createGroup, getGroup, updateGroup, deleteGroup,
	// listGroupMembers, addGroupMember, removeGroupMember, listGrants, createGrant, getGrant and
	// revokeGrant (authz, M1-48).
	*authzhttp.Access
	// Agents implements listAgents, createAgent, getAgent, updateAgent, disableAgent, enableAgent,
	// setAgentBillingMode, setAllAgentsBillingMode, setCredentialHolder and setAgentToolClass
	// (agents, M1-51).
	*agentshttp.Agents
	// Commands implements registerCommand, updateCommand, removeCommand, getCommand and
	// listChannelCommands (agents, M2-16).
	*agentshttp.Commands
	// CommandExec implements invokeCommand (runs, M2-16).
	*runshttp.CommandExec
	// Spaces implements listSpaces, createSpace, getSpace, updateSpace, moveSpace, archiveSpace
	// and putMySidebarSpace (chat, M1-52).
	*chathttp.Spaces
	// Channels implements listSpaceChannels, createChannel, getChannel, updateChannel,
	// moveChannel, archiveChannel, listForumTags, createForumTag, updateForumTag, moveForumTag and
	// deleteForumTag (chat, M1-53).
	*chathttp.Channels
	// Threads implements listChannelThreads, getThread, setThreadTitle, closeThread, reopenThread
	// and setThreadTags (chat, M1-55) and moveThread (M1-62).
	*chathttp.Threads
	// Behavior implements getChannelBehavior, putChannelBehavior, listServedChannels and
	// getServedChannelConfig (chat, M2-02).
	*chathttp.Behavior
	// Messages implements createMessage and createThread (messaging, M1-56), listThreadMessages
	// and getMessage (M1-57), listMyMentions and the template operations (M1-60).
	*messaginghttp.Messages
	// Runs implements getRun, listThreadRuns, listRunSteps and listActiveRuns (runs, M2-05).
	*runshttp.Runs
	// Log implements listAuditEvents, exportAuditEvents and verifyAuditChain (audit, M1-50).
	*audithttp.Log
	// Devices implements listMyDevices, listUserDevices and getDevice (identity, M1-33).
	*identityhttp.Devices
	// People implements listUsers, createUser, getUser, updateUser, enableUser, listOwners,
	// addOwner, revokeOwner, getMe and updateMyProfile (identity, M1-31).
	*identityhttp.People
	// Keys implements getJwks, getWebhookKeys and rotateSigningKey (instance, M1-32, M2-26).
	*instancehttp.Keys
	// Admin implements getDiscovery, getInstance, listInstanceSettings, putInstanceSetting,
	// updateBranding and getProblemType (instance, M1-46), listFeatureFlags and putFeatureFlag
	// (M1-47).
	*instancehttp.Admin
	// CSPReports implements submitCspReport (instance, M6-03).
	*instancehttp.CSPReports
	// Sessions implements refreshToken, logout, listMySessions, listUserSessions and
	// createExtensionSession (identity, M1-34).
	*identityhttp.Sessions
	// StepUps implements createStepUpChallenge and completeStepUpWithDeviceKey (identity, M1-35).
	*identityhttp.StepUps
	// Providers implements listIdentityProviders, createIdentityProvider, getIdentityProvider,
	// updateIdentityProvider and deleteIdentityProvider (identity, M1-36).
	*identityhttp.Providers
	// Credentials implements getMyCredentials, getUserCredentials and unlinkIdentity (identity,
	// M1-36).
	*identityhttp.Credentials
	// Tokens implements listTokens, createToken and revokeToken (identity, M1-44).
	*identityhttp.Tokens
	// Enrollment implements redeemEnrollmentCode and createEnrollmentCode (identity, M1-37).
	*identityhttp.Enrollment
	// Approval implements approveDevice, beginPairing, approvePairing and completePairing
	// (identity, M1-38).
	*identityhttp.Approval
	// Passkeys implements beginPasskeyLogin, completePasskeyLogin, beginEnrollment,
	// completeEnrollment, beginPasskeyRegistration, completePasskeyRegistration, revokePasskey and
	// completeStepUpWithPasskey (identity, M1-40).
	*identityhttp.Passkeys
	// OIDC implements beginOidcLogin, handleOidcCallback, completeOidcLogin, beginIdentityLink and
	// completeIdentityLink (identity, M1-41).
	*identityhttp.OIDC
	// TrustedHeader implements loginWithTrustedHeader (identity, M1-43).
	*identityhttp.TrustedHeader
	// Lifecycle implements disableUser, revokeDevice and revokeSession (identity, M1-45).
	*identityhttp.Lifecycle
	// Hosts implements listHosts, createHost, getHost, updateHost and setHostState (terminal,
	// M2-39) and setHostLlmCredentials (M2-40).
	*terminalhttp.Hosts
	// TerminalSessions implements openTerminalSession, attachTerminalSession, listTerminalSessions,
	// getTerminalSession and closeTerminalSession (terminal, M2-44).
	*terminalhttp.TerminalSessions
	// Local implements loginWithPassword and setMyLocalCredential (identity, M1-39).
	*identityhttp.Local
	// Console implements getConsoleSession, endViewAs, stepUpWithPassword,
	// beginStepUpOidcReauth and completeStepUpOidcReauth (identity, M6-01).
	*identityhttp.Console
	// Apps implements listApps, installApp, validateManifest, getApp, upgradeAppManifest,
	// setAppScopes, suspendApp, resumeApp and uninstallApp (apps, M2-24).
	*appshttp.Apps
	// Webhooks implements listWebhookSubscriptions, createWebhookSubscription,
	// updateWebhookSubscription, deleteWebhookSubscription, listWebhookDeliveries and
	// redeliverWebhook (apps, M2-26).
	*appshttp.Webhooks
	// Incoming implements listIncomingWebhooks, createIncomingWebhook, revokeIncomingWebhook,
	// postIncomingWebhook and postIncomingWebhookWithBearer (apps, M2-27).
	*appshttp.Incoming
	// Personal implements listMyReadStates, putReadState, markThreadUnread, putThreadDraft,
	// deleteThreadDraft, putChannelDraft, deleteChannelDraft, listMyDrafts, listMySettings,
	// putMySetting, deleteMySetting and getMySidebar (sync, M1-61).
	*synchttp.Personal
	// Sync implements listEvents and getSnapshot (sync, M1-63).
	*synchttp.Sync
	// Preferences implements listMyNotificationPreferences, putNotificationPreference,
	// deleteNotificationPreference, listMyQuietHours, createQuietHoursRule, putQuietHoursRule and
	// deleteQuietHoursRule (notify, M3-18).
	*notifyhttp.Preferences
	// ClientMetrics implements submitClientMetrics (stats, M4-31).
	*statshttp.ClientMetrics
	// Files implements createUpload, uploadPart, getUpload, completeUpload, abortUpload and
	// getFile (files, M3-03).
	*fileshttp.Files
}

// routeBuilder builds and verifies the routes of one REST surface against a catalogue with the
// error handlers of opts; routes.Build bound to Handlers in production, the fixture surface in
// tests.
type routeBuilder func(opts routes.Options, cat routes.Catalog) ([]routes.Route, error)

// RouteOptions are the error handlers the generated server gets: the problem writers of the HTTP
// chain (M1-18), so binding, decoding and handler errors leave as application/problem+json.
func RouteOptions() routes.Options {
	return routes.Options{
		ErrorHandlerFunc:         httpserver.ValidationErrorHandler,
		RequestErrorHandlerFunc:  httpserver.MalformedRequestHandler,
		ResponseErrorHandlerFunc: httpserver.WriteProblem,
	}
}

// actionWebhookInPost is the action of the two post operations of the incoming webhooks (M2-27).
const actionWebhookInPost = "webhook_in.post"

// RouteOption adjusts the chain operation of a route before it is wrapped.
type RouteOption func(*httpserver.Operation)

// WebhookBodyLimit sets the body limit of the two post operations of the incoming webhooks to
// limit bytes (webhook_in.max_body_bytes, M2-27 design note 1 a) instead of
// http.max_json_body_bytes.
func WebhookBodyLimit(limit int64) RouteOption {
	return func(op *httpserver.Operation) {
		if op.Action == actionWebhookInPost {
			op.BodyLimit, op.BodyLimitKey = limit, keyWebhookInMaxBodyBytes
		}
	}
}

// HTTPOperation converts an operation of the route table into the operation type of the HTTP
// chain, which must not depend on the generated layer. The operations with action webhook_in.post
// keep their credential away from the authenticator (AnonymousCredentials, M2-27 design note 2):
// their token is no API token, and the token check of apps verifies it.
func HTTPOperation(op routes.Operation) httpserver.Operation {
	return httpserver.Operation{
		AnonymousCredentials: op.Action == actionWebhookInPost,
		ID:                   op.ID, Method: op.Method, Path: op.Path, Pattern: op.Pattern, Action: op.Action,
		RateLimitClass: op.RateLimitClass,
		Public:         op.Public, IdempotencyKey: op.IdempotencyKey,
		IfMatch:      httpserver.IfMatchMode(op.IfMatch),
		RequestBody:  httpserver.BodyKind(op.RequestBody),
		BodyRequired: op.BodyRequired,
		ETagStatuses: slices.Clone(op.ETagStatuses),
		Download:     op.Download,
	}
}

// MountRoutes is start step 7 (02 "Start"): for the roles api and all it builds the routes of the
// enabled operations from h with routes.Build, which checks every registered pattern, action and
// rate-limit class against cat (05 rule 3, AZ-P1), and mounts each route at its pattern on mux.
// The role worker serves no REST API and mounts nothing.
//
// Every route runs through chain (02 "Request path"): chain.Wrap puts the per-route stages
// around the generated handler, whose error handlers are the problem writers (RouteOptions).
// When api.cors_allowed_origins names origins, OPTIONS <path> answers the CORS preflight for every
// enabled path. chain must not be nil for the roles api and all.
//
// cat is the closed catalogue of authz (authz/app.Catalog, M1-25), so there is one definition of
// a known action; the start checks the route bindings against the same catalogue first
// (startRoutes). log nil means slog.Default().
//
// On a failed check MountRoutes logs "route check failed" at level error with the joined error,
// mounts nothing and returns the error. Contract with the start of `bothy serve` (M1-11): it
// calls this step before it opens the public listener and exits with code 1 on the error, so the
// process never reports ready. A route whose pattern
// conflicts with a pattern already on mux fails the step too; the routes mounted before it stay
// on mux, which is harmless because the start fails. An unknown role is an error as well.
//
// Example:
//
//	if err := wire.MountRoutes(role, publicMux, handlers, authzCatalog, chain, log); err != nil {
//		return err // exit code 1, never ready
//	}
func MountRoutes(role Role, mux Mux, h Handlers, cat routes.Catalog, chain *httpserver.Chain, log *slog.Logger, opts ...RouteOption) error {
	return mountRoutes(role, mux, func(opts routes.Options, c routes.Catalog) ([]routes.Route, error) {
		return routes.Build(h, opts, c)
	}, cat, chain, log, opts...)
}

// mountRoutes is MountRoutes with the builder as a parameter, so tests run the start step on the
// generator's fixture surface.
func mountRoutes(role Role, mux Mux, build routeBuilder, cat routes.Catalog, chain *httpserver.Chain, log *slog.Logger, opts ...RouteOption) error {
	switch role {
	case RoleWorker:
		return nil
	case RoleAPI, RoleAll:
	default:
		return fmt.Errorf("wire: unknown role %q (want %s, %s or %s)", role, RoleAPI, RoleWorker, RoleAll)
	}
	if log == nil {
		log = slog.Default()
	}
	if chain == nil {
		return errors.New("wire: no HTTP chain for the public routes")
	}
	rs, err := build(RouteOptions(), cat)
	if err == nil {
		rs = wrapRoutes(chain, rs, opts...)
		// A scratch mux first: routes that conflict with each other fail before the real mux
		// holds any of them.
		err = mount(http.NewServeMux(), rs)
	}
	if err == nil {
		err = mount(mux, rs)
	}
	if err != nil {
		log.Error("route check failed", slog.Any(logKeyError, err))
		return fmt.Errorf("route check failed: %w", err)
	}
	return nil
}

// mount registers the routes on mux and turns the panic of net/http for a conflicting pattern
// into an error naming the operation. A conflict with a pattern that was on mux before (mounted
// by another start step) can leave the routes before it on mux; the start fails then, so the
// server never serves them.
func mount(mux Mux, rs []routes.Route) (err error) {
	var current routes.Operation
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("wire: operation %s (%s) cannot be mounted: %v", current.ID, current.Pattern, p)
		}
	}()
	for _, r := range rs {
		current = r.Operation
		mux.Handle(r.Operation.Pattern, r.Handler)
	}
	return nil
}

// wrapRoutes returns the routes with the chain around each handler, plus one preflight route per
// enabled path when CORS origins are configured. Preflight routes carry the pattern
// "OPTIONS <path>" and an Operation with only Pattern and Method set.
func wrapRoutes(chain *httpserver.Chain, rs []routes.Route, opts ...RouteOption) []routes.Route {
	out := make([]routes.Route, 0, len(rs))
	methods := map[string][]string{}
	var paths []string
	for _, r := range rs {
		op := HTTPOperation(r.Operation)
		for _, o := range opts {
			o(&op)
		}
		out = append(out, routes.Route{Operation: r.Operation, Handler: chain.Wrap(op, r.Handler)})
		if _, seen := methods[r.Operation.Path]; !seen {
			paths = append(paths, r.Operation.Path)
		}
		methods[r.Operation.Path] = append(methods[r.Operation.Path], r.Operation.Method)
	}
	if !chain.CORSEnabled() {
		return out
	}
	slices.Sort(paths)
	for _, p := range paths {
		pattern := http.MethodOptions + " " + p
		out = append(out, routes.Route{
			Operation: routes.Operation{Method: http.MethodOptions, Path: p, Pattern: pattern},
			Handler:   chain.Preflight(p, methods[p]),
		})
	}
	return out
}
