# admin_ui

Administration screens for the desktop app, the console and the emergency actions on mobile.

Status: own devices and sessions (M4-25), the administration of people, groups and waiting
devices (M4-26), grants with preview, "Who has access?" and "View as person" (M4-27), the
audit log (M4-29), the hosts of the terminal with pairing and session users (M4-37) and the
agents with their backend switch, the guided flow and their tokens (M4-28), what the
console needs to host the administration: the body without navigation and the stored view of
"View as person" (M6-04), the lists "Permission hints" and "Expiring soon" (M6-07), and the
apps with their consent, lifecycle and outgoing webhooks and the incoming webhooks per channel
(M6-08), and the instance settings, branding, login providers and feature flags for owners
(M6-09), the import sources of the web console with their owner steps (M8-08), the
statistics screen A7 (M6-06), the status page with its owner forms (M7-21), and the emergency
actions on the phone (M5-09). Kind: Flutter package, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## What it offers

The same screens serve the desktop app (Settings, Devices and sessions), the console (M6-01)
and the emergency actions on mobile (M5, R-PR-07). They talk REST only and use no local
database, because the console has neither a gateway nor a local store (15 3.1).

| Symbol | Purpose |
|---|---|
| `SelfRepository`, `ApiSelfRepository` | the own devices (`listMyDevices`, `getMe` for "this device") and sessions (`listMySessions`), `approveDevice`, `approvePairing`, `revokeDevice`, `revokeSession` through the client of the signed-in `AuthSession`; failures as `SelfActionFailed` with the server's `ApiProblem` (null without an answer), a cancelled step-up as `StepUpCancelled` |
| `MyDevicesView`, `DeviceTile` | the device list with platform, name, app version, last activity and "This device"; "Enter code" for a waiting device, "Add device" with the pairing code, "Revoke" with one confirmation; no hardware label, because `Device` has no hardware field |
| `MySessionsView` | the sessions with device, login method and last use; "End session" with one confirmation and never a step-up |
| `ConfirmDeviceDialog`, `AddDeviceDialog`, `RevokeConfirmDialog` | the code dialogs (digits only; pairing codes normalized like `normalizeCode` of `auth`) and the one confirmation of a destructive action (01 6.7) |
| `StepUpDialog`, `StepUpDialogPrompt` | the explanation before the system asks for biometrics or the device password; `StepUpDialogPrompt` is the `StepUpPrompt` of `auth` that the apps pass to their `AuthSession` |

| `AdminShell`, `AdminSection`, `AdminLocation` | the administration of 01 5.2 A1: the navigation (only entries with a screen are listed; "Devices" carries the count of waiting devices) and the open screen; the host maps `AdminLocation.path` to its own paths (`/admin/...` on desktop) |
| `AdminRepository`, `ApiAdminRepository` | people (`listUsers` with `q` and `status`, `getUser`, `createUser`, `disableUser`, `enableUser`, `createEnrollmentCode`), credentials (`getUserCredentials`, `unlinkIdentity`, `revokePasskey`), devices and sessions of a person (`listUserDevices`, `listUserSessions`, `approveDevice`, `revokeDevice`, `revokeSession`), owners (`listOwners`, `addOwner`, `revokeOwner`), groups (`listGroups`, `getGroup`, `createGroup`, `updateGroup`, `deleteGroup`, `listGroupMembers`, `addGroupMember`, `removeGroupMember`) and the waiting devices, assembled from `listUsers`, `listUserDevices` (pending) and `listUserSessions` (origin), because the API has no instance-wide list |
| `PeopleList`, `PersonDetail`, `CreatePersonDialog`, `EnrollmentCodeView`, `CredentialsSection`, `OwnersSection` | A1 People: list with search, state and group filter (the group filter works on the loaded page, with a note), "Add person" with the code shown once, a person's detail, owners with the server's `last_owner` text |
| `GroupsList`, `GroupDetail`, `MemberEditor` | groups: create, rename, delete, members with an optional expiry (`YYYY-MM-DD`) |
| `WaitingDevicesView`, `ApproveDeviceDialog` | the queue of waiting devices with person, platform, time and origin; approve with the code the device shows, reject (revoke) |
| `AccessRepository`, `ApiAccessRepository` | grants and access over REST: `listSpaces` and `listSpaceChannels` (targets), `listAgents`, `previewGrant`, `createGrant`, `listGrants`, `revokeGrant`, `explainSpaceAccess`, `explainChannelAccess`, `viewAsUser`, `listPermissionHints` and `listExpiringAccess` (`access.hints.read`, no step-up); `AdminRepository` includes these calls |
| `GrantWizard`, `PreviewPanel` | A2: who (person, group, agent, search by name), where (instance, space, channel), level, capabilities (preselectable, with an own `title`, for "Extend" of M6-07), last day and required reason; every change previews the full request, the panel shows the server's `text` unchanged, the effects per principal, I1 and SR8 ("Switch the agent to organization first."), the hints and a delegation denial; "Confirm" only with a current, allowed preview and a reason, then the step-up |
| `GrantsList`, `RevokeGrantDialog` | the grants of a target (`/grants/<target key>`) with "Revoke": takes effect immediately, required reason |
| `AdminBody` | the screen of one `AdminLocation` without the navigation: the audit log gets the body's height, every other screen scrolls with padding; `maxWidth` is 880 by default and null fills the width (the browser); `onChanged` runs after every change; `AdminShell` puts it next to its navigation |
| `AdminViewAs`, `ViewAsBanner`, `ViewAsStructure`, `notVisibleSpaces` | the view-as hook (`WhoHasAccess`, `PersonDetail`, `GrantsList` and `AdminBody` hand the chosen person to the host instead of opening the dialog; `PersonDetail` then shows "View as this person"), the full-width banner with name, time left as `mm:ss` and "End view" (the host passes `now` every second), the structure the person sees as text only (spaces with their channels indented, the spaces of the owner the person does not see, the note that the console shows structure and access only) and the names of those spaces |
| `WhoHasAccess`, `SourceChain`, `ViewAsPerson` | A3 and A4: level, capabilities and source chain per principal (owner, direct, group, inherited, participant, operator, via agent with H1, blocked by a guard rule; disabled principals struck through), and the read-only snapshot of a person in a clear frame with its end |
| `AuditRepository`, `ApiAuditRepository` | the audit log of A6 (part of `AdminRepository`): `listAuditEvents` with its filters (`action`, `actor_id`, `decision`, `resource_kind`, `resource_id`, `from`, `to`) and cursor, `verifyAuditChain` with or without a `seq` range, `exportAuditEvents` as the server's byte stream; a streamed export asks one entry of the range first, so the step-up and a `403` arrive on a JSON answer |
| `AuditLogView`, `AuditFilters`, `AuditEntryDetail` | A6 Audit: the virtualized list (time in the person's time zone, who, action with its label where `l10n` has one, result, resource, reason), the filters (action from the loaded IDs or typed, person, result, resource kind and ID, days `YYYY-MM-DD`), "Load more" with the cursor, the details of an entry (correlation ID, deny reason, details, hashes); nothing changes or deletes an entry |
| `AuditChainCheckButton`, `AuditChainPanel` | "Check chain": without a filter from the last checkpoint, with a filter over the `seq` span of the listed entries; "Chain checked up to no. n: unchanged" or the first broken entry with its reason in the danger style, the range, the checkpoints compared and the warnings |
| `AuditExportBar`, `AuditSaveTarget`, `auditNdjsonToCsv`, `auditCsvCell` | "Export" writes the NDJSON stream unchanged, "Export as CSV" converts it on the device (columns `occurred_at` to `prev_hash`, RFC 4180, an apostrophe before a cell starting with `=`, `+`, `-` or `@`); the host passes the save dialog (`AuditSaveTarget`), without one there are no export buttons |
| `HostsRepository`, `ApiHostsRepository` | A8 over REST: `getMe` (`is_owner` lists "Hosts"), `listHosts`, `getHost`, `createHost`, `issueHostPairingCode`, `confirmHost` (`FingerprintMismatch` carries `attempts_remaining`), `updateHost`, `setHostLlmCredentials`, `setHostState`, `listAuditEvents` (the person of an ended assignment, step-up) and `listTerminalSessions`; `AdminRepository` includes these calls |
| `HostsList`, `HostDetail` | A8 for owners: the hosts and one host with every field of the first wireframe; the check results ask the audit of the host only for a named session user without a valid assignment |
| `PairingWizard`, `CreateHostDialog`, `FingerprintConfirm`, `LlmLoginChoice`, `ServerHostChoice`, `SessionUserRows` | the three steps of pairing: create, the code in the command `bothy-host pair --server <origin> --code <code>` with its remaining time and "New code", then the fingerprint prefix, the LLM login (I6 explained), the server host and the session users from the reported names only |
| `LlmLoginSection`, `ServerHostSection`, `DoctorResults`, `HostGrantsSection`, `SessionUsersSection`, `HostStateActions` | the parts of the host page: change of the declaration with `i6_violation` explained, declaration and report of the server host, every check entry with names and people, the grants with the server's preview and a "Not effective" badge, assign and end session users (`session_user_taken` and `session_user_not_on_host` explained), disable, enable, re-pair, retire |
| `AgentsRepository`, `ApiAgentsRepository` | A5 over REST (part of `AdminRepository`): `listAgents`, `getAgent` (configured and reported mode, holder, `billing_modes_supported`, evidence and the server's `warnings` codes, unchanged), the operated targets from `Space.default_operator_agent_id` and `Channel.operator.effective_agent_id`, `setAgentBillingMode`, `setAllAgentsBillingMode`, `setCredentialHolder`, the proposal of "All to Personal" from the newest `agent.billing_mode.set_all` in the audit (`listAuditEvents`, step-up), `listTokens`, `createToken`, `revokeToken`; a `409 i1_violation` carries its `conflicts` in `SelfActionFailed.conflicts`; `createGrant`, `addGroupMember` and `addOwner` take `resolve_i1` |
| `AgentsList`, `AgentPage`, `AgentModeBadge` | A5: the badge "Personal" or "Organization" with icon and text, the number of server warnings, an agent's page with adapter, presence (a stream of the host, else `getAgent`), configured next to reported mode ("follows the configured mode per run" without one), one text per server warning code of 10 8 with its consequence and the switch hint, the operated spaces and channels and the holder |
| `SwitchModeDialog`, `HolderDialog`, `SwitchAllDialog`, `I1Explanation` | the switch of one agent (reason required; to organization clears the holder, to personal needs one), the change of the holder, the switch of several agents (all to organization with a reason; all to personal with a holder and an explicit, editable list proposed from the last global switch; one violating agent changes nothing, BR-08), and the explanation of an I1 rejection with the principals the caller may read and a count of the others |
| `AgentTokens`, `TokenSecretView` | the agent's API tokens: prefix, validity, last use, DPoP binding, revoked ones; create with step-up and the DPoP key thumbprint of the adapter, the secret shown once and never kept, revoke after one confirmation without step-up |
| `GuidedSwitch`, `GuidedFlow`, `GuidedResult` | the guided flow of 10 11 in the grant preview and on a `409 i1_violation` of adding a group member or an owner: the explanation, "Switch to Organization and grant" with a reason, one request with `resolve_i1`, and the result with the warning while the adapter still reports the old mode |
| `AppsRepository`, `ApiAppsRepository`, `ManifestUpload` | apps and webhooks over REST (part of `AdminRepository`): `listApps`, `getApp`, `validateManifest`, `installApp`, `upgradeAppManifest`, `setAppScopes`, `suspendApp`, `resumeApp`, `uninstallApp`, `listWebhookSubscriptions`, `createWebhookSubscription`, `updateWebhookSubscription`, `deleteWebhookSubscription`, `listWebhookDeliveries`, `redeliverWebhook`, `getWebhookKeys`, `listIncomingWebhooks`, `createIncomingWebhook`, `revokeIncomingWebhook`; a manifest file is read as JSON (its exact text) or YAML (converted to JSON) and sent as text, so the server's closed schema sees every member |
| `AppsList`, `InstallAppDialog`, `ConsentView` | the installed apps and the install from a manifest file the host picks (`ManifestPicker`): the server's validation, the consent in plain text (scopes with their meaning, events, target hosts, commands, components, requested grants with a target each, `data_egress`), `installApp`, the server's preview of each grant, then the grants and the app token, shown once with the key ID for receivers; a failed step shows the progress with "Retry" and "Uninstall" |
| `AppPage`, `UpdateAppDialog`, `AppLifecycleDialog` | one app: consent summary, its tokens, the update with the diff and the new consent (`setAppScopes`), suspend, resume and uninstall with the effects of 11 4.3 before the confirmation |
| `AppSubscriptionsSection`, `DeliveriesView`, `WebhookKeysCard` | outgoing subscriptions within the granted events and `https` addresses, their state and the reason of a disabled one, the HMAC secret once, the deliveries with status, attempts, next attempt and last status code (never a body), "Redeliver", and the public keys of the `v1a` signatures |
| `IncomingWebhooksView`, `CreateIncomingDialog` | the incoming webhooks of a chosen channel (the channel never enters a path), create with an existing app or, for owners, the composed "incoming app" (`installApp` with a built-in manifest, `createGrant` write on the channel, `createIncomingWebhook`), the address with its token shown once, revoke |
| `OnceOnlySecret` | the one dialog for every secret shown once (app and agent tokens, incoming webhook addresses, HMAC secrets): kept only in its state, dropped on close, not announced to screen readers until focused, never logged or stored |
| `AccessHintKind`, `AccessHint`, `AccessHintPage`, `ExpiringKind`, `ExpiringEntry`, `ExpiringPage`, `AccessPageLabels` | the answers of `permissionHints` (`listPermissionHints`, 07 9.5: kind, space and cursor) and `expiringAccess` (`listExpiringAccess`, 07 9.4: space and cursor) of `AccessRepository`, unchanged, with the names of each page (`labels`), joined over "Load more" |
| `ImportRepository`, `ApiImportRepository` | the import over REST (part of `AdminRepository`): `listImports`, `getImport`, `createImport` (the mapping sent as the plain JSON the dialog converted), `startImport`, `pauseImport`, `cancelImport`, `completeImport`, `setImportLead`, `listImportItems` (every page), and `getChannel` and `setChannelOperator` (`If-Match`, `resolve_i1`) for the operator step of 22 13.3 |
| `ImportSourceSnapshot`, `ImportController` | one `ImportSource`, the payload of `getImport` and of `import.progress`: state, the numbers of `stats.server` and `stats.tool` (a text in `stats` is never shown), the mapped channels with Discord ID, Bothy name, planned operator and lead; the controller keeps the newest snapshot per source (no timer) and joins the mapping with the posted channel items, the channels and the agents into `ImportChannelRow`s |
| `ImportListPage`, `ImportCreateDialog`, `mappingJsonOf`, `mappingPathOf` | the sources with kind, state and progress; "Add source" with a name and the YAML mapping file (strict: a key named twice is an error), the server's errors at their places in the file (`channels[2].planned_operator`) |
| `ImportSourcePage`, `ImportChannelTable`, `ImportConfirmDialog`, `ImportOperatorDialog` | one source: the state steps with their confirmation and step-up (completion only while every imported channel is led here, with the retention warning of 22 13.5), counters, failed items, one row per mapped channel with "Lead here" (the order of 22 13.3, then the planned operator with I1 and the guided switch) and "Discord leads" (then the offer to remove the operator); the page reads again after each step, on refresh and when the tab becomes visible. Web only: `AdminSection.dataImport` is not listed by the desktop shell |
| `PermissionHintsView`, `ExpiringAccessView`, `AccessListView` | the places `/hints` and `/expiring` of `AdminSection` (M6-07): the scope filter (the instance, owners only, or one space) and, for hints, the kind; per entry the kind as icon and text and the sentence from the templates of `l10n` ("Sam can learn content of Marketing/Intern through agent Texter."); "Explain" opens "Who has access?" of the channel or space in a dialog; a grant has "Revoke" and, with an end, "Extend": the `GrantWizard` with subject, target, level and capabilities of the old grant, an empty reason and no last day, then `createGrant` and after it `revokeGrant` of the old grant with the new reason (one step-up within `auth.step_up_max_age`); an agent token opens the agent's page with its tokens ("Renew token"), an app token has no screen here yet and says how to renew it (a new token, then revoking the old one, 08 12); a membership opens its group; a personal agent opens its page |
| `hintLines`, `hintKindLabel`, `hintKindGlyph`, `expiringText`, `expiringKindLabel`, `expiringKindGlyph`, `grantSubjectName`, `grantLevelText`, `grantTarget`, `isAppToken` | the texts and icons of both lists, from `l10n` and the labels of the page only |
| `InstanceRepository`, `ApiInstanceRepository`, `InstanceVersionConflict` | the instance administration over REST (part of `AdminRepository`): `listInstanceSettings`, `putInstanceSetting`, `getInstance` (branding and the flags evaluated for the caller), `updateBranding`, `uploadBrandingLogo`, `deleteBrandingLogo`, `getBrandingLogo` (a `404` is "no preview yet"), `listFeatureFlags`, `putFeatureFlag` (the rollout is sent unchanged), `listIdentityProviders`, `createIdentityProvider`, `updateIdentityProvider` (only the changed members), `deleteIdentityProvider`, and the OIDC redirect URI of the server; setting values and provider configurations travel as plain JSON; a versioned write that loses against another owner ends in `InstanceVersionConflict` with the newer state (from the problem's `current`, or read again) |
| `InstanceView`, `InstanceTab` | the place `/instance` of `AdminSection` (01 5.5 "Instance, branding"), listed for owners: settings, branding, login providers and feature flags; `instanceChanges` opens the desktop's `instance.updated` events, after which settings and flags load again |
| `InstanceSettingsSection`, `SettingEditorDialog`, `instanceSettingSchemas`, `SettingSchema`, `SettingKind` | every setting with its kind and allowed values from the configuration reference section 6 (the table here is held equal to it by a test), value, version and who changed it when; a typed editor (a choice for a closed list, text otherwise) that states "from, to", confirms and sends `If-Match`; the server's `400` at the input, a `412` with the newer value next to the input; branding keys point to the branding screen, an unknown key is read-only with the note to use the API |
| `BrandingSection`, `AccentPicker`, `AccentPreview`, `LogoPicker`, `accentPaletteKeys` | product name (the server's `400` at the field: too long, control or bidi character; no counter of its own), the accent from exactly the seven palette keys with a swatch each and a preview of the accent roles in light and dark built with `buildDsTheme` (no input takes a color), the logo uploaded through the host's file dialog, shown from the server's preview once checked, removed; `onBrandingChanged` lets the console read discovery again |
| `ProvidersSection`, `ProviderEditorDialog`, `RedirectUriView` | the login providers: list with driver and state, add OIDC, proxy header or local accounts, change, enable, disable and delete; the OIDC form has the fields of 08 2.6, the redirect URI to register with "Copy", and secret references written only (masked, empty keeps the stored reference, a stored reference is never shown, the confirmation names changed members, never values); the proxy header form states the loopback rule and shows the server's `409` with its hint; disable and delete say that the provider's sessions end at once (R-ID-12) |
| `FlagsSection`, `FlagSwitchDialog` | the flags with key, description, kind, the state evaluated for the signed-in person and the stored value with default and rollout; the switch states the change, takes the required reason and keeps the rollout; a `412` shows the newer state and changes nothing; orphaned flags are read-only |
| `AdminStatsSource`, `ApiAdminStatsSource`, `StatsQuery`, `StatsBucketSize`, `StatsCostGrouping`, `StatsSpace`, `requiredCurrencies`, `metricsOf` | the port of A7 (part of `AdminRepository`): one typed call per statistics operation of M6-05 (`getUsageStats`, `getRunStats`, `getToolCallStats`, `getErrorStats`, `getCostStats` with `group_by` and `currency`), the spaces of the scope selector (`listSpaces`), the agent names (`listAgents`) and `telemetry_dashboard_url` and `stats_time_zone` of `getInstance` (`StatsInstance`); the currencies a `currency_required` answer names |
| `StatsScreen`, `StatsController`, `StatsRange`, `StatsDays`, `StatsPreset`, `StatsData`, `StatsPart`, `StatsRemovedScope` | the place `/statistics` of `AdminSection` (01 5.2 A7, 5.5): range presets and a custom range on whole buckets of the server (the 24-hour preset on UTC hours, the others on calendar days of `stats_time_zone`; hours up to `statsHourlyUpTo`, days up to `statsDailyUpTo`, weeks above; presentation, the server bounds the range with `stats.max_range`), the scope selector ("All spaces" and the spaces; the first query decides, a `403` or `404` removes the entry with a note), the run tile with the comparison against the previous range of the same length (two `getRunStats` calls), the cost tiles per billing mode, a currency choice after `currency_required`, and the telemetry link through the host's `openExternal` (system browser or new tab, never a WebView) |
| `StatsSection`, `StatsSectionData`, `StatsBarRow`, `StatsBarPart`, `StatsTexts`, `statsBarModel`, `runsSection`, `durationSection`, `failuresSection`, `toolsSection`, `errorsSection`, `costSection`, `costSums`, `usageSection`, `runTotal` | the sections of A7 drawn with the `BarMark` and `MetricTileMark` of `bothy_viz`, each with "As table" (`VizTable`, the same values); counters and amounts of one mode are summed over the buckets, rates, percentiles and counts of people never; a value in `below_min_group` reads "fewer than the minimum group"; no section groups by person |
| `StatusScreen`, `StatusController`, `StatusSource`, `LiveStatusSource`, `PollingStatusSource`, `StatusUnreachable`, `StatusComponentRow`, `StatusIncidentCard`, `StatusStateChip`, `statusStateLook` | the status page of 16 12 (M7-21) for the desktop, the phones and the console: the overall state, the components with their states (word, icon and a state role; maintenance in `info`, never as an outage), the open incidents, the maintenance windows (incidents of kind `maintenance`), the history and the maintenance mode (`overall_state` `maintenance`); owner texts through `DsMarkdown`. The `StatusSource` port gives the page and its changes: the apps `getStatus`, `status.updated` and the connection state, the console `getStatus` every `client.status_poll_interval`. While the connection state reports the server unreachable or a read gets no answer, the page shows "Server unreachable" and hides every state until a new answer comes (D-M7-21-2); the place `/status` of `AdminSection` is listed for owners |
| `StatusRepository`, `ApiStatusRepository`, `StatusComponentForm`, `StatusIncidentForm`, `StatusUpdateForm`, `statusComponentInput`, `statusNameProblem` | the seven operations of the status page (part of `AdminRepository`) and the owner forms; each form builds the generated request body of its operation, and a change of a component sends the name and only what changed; the forms show only when `getMe` says the caller is an owner (`status.manage`) |
| `EmergencyHome`, `PersonEmergencyView`, `EmergencyConfirmSheet`, `emergencyStepUpLabel`, `emergencyFailure` | the emergency actions on the phone (01 3.7, 5.3 M13; R-PR-07), and nothing else of the administration: the people with the server's search, one person with their devices ("Revoke") and "Disable person" when the host says the caller is an owner (`owner`, from `getMe`), and the grants the server lists for that person (`grantsOf`, `listGrants` by subject) with "Revoke". Each action opens a bottom sheet that says it takes effect at once (07 9.5) and whose button names the step-up ("Confirm with Face ID" on iOS, fingerprint or screen lock elsewhere); a cancelled step-up says that nothing changed, and `409 last_owner` is explained, never prevented on the client. Phone layouts over `AdminRepository` and `AdminActionState`, no second data layer |
| `ConfirmChange`, `ProblemBanner`, `adminFailure`, `CodeText` | the one confirmation before a change (danger style for destructive ones), the server's problem texts (`one_time_secret` and `i1_violation` explained), the code view of onboarding and enrollment codes |

Step-up comes only from the server: a changing call carries its purpose text
(`stepUpPurposeExtra`), and the step-up interceptor of `auth` asks when the server answers
`401 step_up_required` (05: `device.approve`, `device.revoke`, `host.manage`, `audit.read`, `agent.billing_mode.set`,
`agent.billing_mode.set_all`, `agent.credential_holder.set`, `token.create`,
`instance.settings.update`, `instance.branding.update`, `feature_flag.update` and
`identity_provider.manage` have step-up,
`session.revoke` and `token.revoke` have none). A `403` hides the refused action for that item and shows the
translated reason; the screens hold no permission logic (15 1 principle 1).

## Embedding the administration

The desktop app hosts `AdminShell` at `/admin` (`apps/client/lib/admin/admin_route.dart`) and
passes the system file dialog as the `ManifestPicker` of the apps and the `LogoPicker` of the
instance logo, and the `instance.updated` events of its local store as `instanceChanges`; the
console passes a file input of the browser for both and reads discovery again after a branding
change (`onBrandingChanged`), because it has no live updates:
it builds `ApiAdminRepository` over the client of the signed-in session and maps
`AdminLocation` to `/admin/<section>/<id>`. The console (M6-04) passes its own repository and
maps the same locations to its routes: it hosts `AdminBody` inside its own shell, with
`maxWidth: null` so the screens use the full browser width, and passes `onViewAs`. With the
hook "Who has access?" and a person's detail only hand over the person; the console calls
`viewAsUser` itself, keeps the stored view of its session and shows `ViewAsBanner` on every
page and `ViewAsStructure` instead of channel content, because it has no content routes (ADR
0028). The desktop passes no hook and keeps the dialog with `ViewAsPerson`. Every screen also works on its own with a repository
in its constructor. Every changing action asks once (`ConfirmChange`) and then sends the call;
the step-up dialog appears when the server answers `401 step_up_required` (05 2.4: every
administration action here has step-up except `session.revoke`). Reading, checking and
exporting the audit log have step-up too (`audit.read`, `audit.verify`). The desktop app passes
no `saveTarget` yet, so it shows the audit log without the export buttons until a host wires a
save dialog (docs/DECISIONS.md D-M4-29-3).

## The page "Live" (M6-16)

`LiveScreen` hosts the views of `bothy_viz` for the desktop app (D12, TV mode D13), the phone (M10)
and the console (01 5.5): one tab per view with its table view, a scope selector (all spaces or
one space of `listSpaces`), TV mode over the whole window where the form factor allows it, and
"Open in Grafana" only when `getInstance` names `telemetry_dashboard_url`. The host passes its
live source (the gateway subscription in the apps, polling in the console), `ApiStatsQuerySource`
over its own client, an `AdminStatsSource` for the names and the link, and what a run card does
(docs/decisions.d/M6-16.md). The page knows no transport and no permission.

## Dependencies

Allowed members (docs/spec/15-client-architecture.md, 3.1; `tools/arch-check/rules.yaml`): `design_system`, `auth`, `api_client`, `bothy_viz` (arrows of the diagram), and through them `domain`, `l10n`.
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- no native source (rule 4).

## Layout

| Path | Content |
|---|---|
| `lib/admin_ui.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `lib/src/data/` | `self_repository.dart`, `admin_repository.dart` |
| `lib/src/self/` | the device and session views and their dialogs |
| `lib/src/shell/`, `lib/src/people/`, `lib/src/groups/`, `lib/src/devices/`, `lib/src/common/` | the administration of M4-26 |
| `lib/src/audit/` | the audit log of M4-29 |
| `lib/src/step_up/` | the step-up dialog |
| `lib/src/grants/`, `lib/src/access/`, `lib/src/data/access_repository.dart` | the permission screens of M4-27 |
| `lib/src/hosts/`, `lib/src/data/hosts_repository.dart` | screen A8 of M4-37 |
| `lib/src/agents/`, `lib/src/data/agents_repository.dart` | screen A5 of M4-28 and the guided flow |
| `lib/src/apps/`, `lib/src/incoming_webhooks/`, `lib/src/secrets/`, `lib/src/data/apps_repository.dart` | apps, outgoing and incoming webhooks and the once-only secret of M6-08 |
| `test/apps/`, `test/incoming_webhooks/`, `test/secrets/` | the app screens, subscriptions, deliveries, incoming webhooks and the once-only test over the app fake (`test/admin/apps_fake.dart`) |
| `lib/src/emergency/`, `test/emergency/` | the emergency actions on the phone of M5-09 and their tests over the fake repository |
| `lib/src/hints/` | "Permission hints" and "Expiring soon" of M6-07 (07 9.4, 9.5) |
| `lib/src/instance/`, `lib/src/data/instance_repository.dart` | the instance administration of M6-09 |
| `lib/src/live/`, `test/live/` | the page "Live" and the REST source of the views of M6-16, and their tests |
| `test/instance/` | settings, branding, accent picker, providers, flags, the step-up of every write and the REST source over the instance fake (`test/admin/instance_fake.dart`) |
| `test/hints/` | both lists, "Extend" with one step-up, "Explain", the navigation entries and the REST source of the two lists |
| `test/agents/` | agent page, switches, switch of all agents, tokens, guided flow and the REST source over the agent fake (`test/admin/agents_fake.dart`) |
| `test/hosts/` | pairing, I6, session users, check results, grants on a host and the host page over the host fake (`test/admin/hosts_fake.dart`) |
| `test/grants/`, `test/access/` | the wizard, revoke, "Who has access?" and "View as person"; the fake reads the example of 07 9.3 at run time |
| `test/self/`, `test/admin/`, `test/audit/` | widget tests over fake repositories that behave like the server behind the step-up interceptor |
| `test/goldens/` | goldens (`ci/` with Ahem, `macos/` with the bundled fonts) |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
