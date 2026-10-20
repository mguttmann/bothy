# admin_ui

Administration screens for the desktop app, the console and the emergency actions on mobile.

Status: own devices and sessions (M4-25), the administration of people, groups and waiting
devices (M4-26), grants with preview, "Who has access?" and "View as person" (M4-27), the
audit log (M4-29) and the hosts of the terminal with pairing and session users (M4-37); agents
(M4-28) and the console (M6) follow. Kind: Flutter package, a member of the Dart workspace at
the repository root (`resolution: workspace`, `publish_to: none`).

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
| `AccessRepository`, `ApiAccessRepository` | grants and access over REST: `listSpaces` and `listSpaceChannels` (targets), `listAgents`, `previewGrant`, `createGrant`, `listGrants`, `revokeGrant`, `explainSpaceAccess`, `explainChannelAccess`, `viewAsUser`; `AdminRepository` includes these calls |
| `GrantWizard`, `PreviewPanel` | A2: who (person, group, agent, search by name), where (instance, space, channel), level, capabilities, last day and required reason; every change previews the full request, the panel shows the server's `text` unchanged, the effects per principal, I1 and SR8 ("Switch the agent to organization first."), the hints and a delegation denial; "Confirm" only with a current, allowed preview and a reason, then the step-up |
| `GrantsList`, `RevokeGrantDialog` | the grants of a target (`/grants/<target key>`) with "Revoke": takes effect immediately, required reason |
| `WhoHasAccess`, `SourceChain`, `ViewAsPerson` | A3 and A4: level, capabilities and source chain per principal (owner, direct, group, inherited, participant, operator, via agent with H1, blocked by a guard rule; disabled principals struck through), and the read-only snapshot of a person in a clear frame with its end |
| `AuditRepository`, `ApiAuditRepository` | the audit log of A6 (part of `AdminRepository`): `listAuditEvents` with its filters (`action`, `actor_id`, `decision`, `resource_kind`, `resource_id`, `from`, `to`) and cursor, `verifyAuditChain` with or without a `seq` range, `exportAuditEvents` as the server's byte stream; a streamed export asks one entry of the range first, so the step-up and a `403` arrive on a JSON answer |
| `AuditLogView`, `AuditFilters`, `AuditEntryDetail` | A6 Audit: the virtualized list (time in the person's time zone, who, action with its label where `l10n` has one, result, resource, reason), the filters (action from the loaded IDs or typed, person, result, resource kind and ID, days `YYYY-MM-DD`), "Load more" with the cursor, the details of an entry (correlation ID, deny reason, details, hashes); nothing changes or deletes an entry |
| `AuditChainCheckButton`, `AuditChainPanel` | "Check chain": without a filter from the last checkpoint, with a filter over the `seq` span of the listed entries; "Chain checked up to no. n: unchanged" or the first broken entry with its reason in the danger style, the range, the checkpoints compared and the warnings |
| `AuditExportBar`, `AuditSaveTarget`, `auditNdjsonToCsv`, `auditCsvCell` | "Export" writes the NDJSON stream unchanged, "Export as CSV" converts it on the device (columns `occurred_at` to `prev_hash`, RFC 4180, an apostrophe before a cell starting with `=`, `+`, `-` or `@`); the host passes the save dialog (`AuditSaveTarget`), without one there are no export buttons |
| `HostsRepository`, `ApiHostsRepository` | A8 over REST: `getMe` (`is_owner` lists "Hosts"), `listHosts`, `getHost`, `createHost`, `issueHostPairingCode`, `confirmHost` (`FingerprintMismatch` carries `attempts_remaining`), `updateHost`, `setHostLlmCredentials`, `setHostState`, `listAuditEvents` (the person of an ended assignment, step-up) and `listTerminalSessions`; `AdminRepository` includes these calls |
| `HostsList`, `HostDetail` | A8 for owners: the hosts and one host with every field of the first wireframe; the check results ask the audit of the host only for a named session user without a valid assignment |
| `PairingWizard`, `CreateHostDialog`, `FingerprintConfirm`, `LlmLoginChoice`, `ServerHostChoice`, `SessionUserRows` | the three steps of pairing: create, the code in the command `bothy-host pair --server <origin> --code <code>` with its remaining time and "New code", then the fingerprint prefix, the LLM login (I6 explained), the server host and the session users from the reported names only |
| `LlmLoginSection`, `ServerHostSection`, `DoctorResults`, `HostGrantsSection`, `SessionUsersSection`, `HostStateActions` | the parts of the host page: change of the declaration with `i6_violation` explained, declaration and report of the server host, every check entry with names and people, the grants with the server's preview and a "Not effective" badge, assign and end session users (`session_user_taken` and `session_user_not_on_host` explained), disable, enable, re-pair, retire |
| `ConfirmChange`, `ProblemBanner`, `adminFailure`, `CodeText` | the one confirmation before a change (danger style for destructive ones), the server's problem texts (`one_time_secret` and `i1_violation` explained), the code view of onboarding and enrollment codes |

Step-up comes only from the server: a changing call carries its purpose text
(`stepUpPurposeExtra`), and the step-up interceptor of `auth` asks when the server answers
`401 step_up_required` (05: `device.approve`, `device.revoke`, `host.manage` and `audit.read`
have step-up, `session.revoke` has none). A `403` hides the refused action for that item and shows the
translated reason; the screens hold no permission logic (15 1 principle 1).

## Embedding the administration

The desktop app hosts `AdminShell` at `/admin` (`apps/client/lib/admin/admin_route.dart`):
it builds `ApiAdminRepository` over the client of the signed-in session and maps
`AdminLocation` to `/admin/<section>/<id>`. The console (M6-04) passes its own repository and
maps the same locations to its routes. Every screen also works on its own with a repository
in its constructor. Every changing action asks once (`ConfirmChange`) and then sends the call;
the step-up dialog appears when the server answers `401 step_up_required` (05 2.4: every
administration action here has step-up except `session.revoke`). Reading, checking and
exporting the audit log have step-up too (`audit.read`, `audit.verify`). The desktop app passes
no `saveTarget` yet, so it shows the audit log without the export buttons until a host wires a
save dialog (docs/DECISIONS.md D-M4-29-3).

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
| `test/hosts/` | pairing, I6, session users, check results, grants on a host and the host page over the host fake (`test/admin/hosts_fake.dart`) |
| `test/grants/`, `test/access/` | the wizard, revoke, "Who has access?" and "View as person"; the fake reads the example of 07 9.3 at run time |
| `test/self/`, `test/admin/`, `test/audit/` | widget tests over fake repositories that behave like the server behind the step-up interceptor |
| `test/goldens/` | goldens (`ci/` with Ahem, `macos/` with the bundled fonts) |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
