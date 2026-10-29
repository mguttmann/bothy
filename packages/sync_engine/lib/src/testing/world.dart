// SPDX-License-Identifier: Apache-2.0

/// The in-memory world of the fake server (brief M4-17 Design note 3): instance, people with
/// devices and sessions, spaces, channels, threads, messages with `thread_seq`, the event log
/// with a global `seq` and a sync epoch, read states, drafts, settings and runs; since M4-19
/// also forums with tags, sidebar states and the instance logo; since M4-20 message
/// revisions, pins, templates, the capabilities of a channel view, main threads, threads from
/// a message, title, close, reopen, move and mark unread; since M4-22 the files of 12 2 and 3:
/// upload sessions with parts, completion, the scan result, thumbnails, transcripts and file
/// tickets; since M4-30 notification preferences and the hints of 13 as `notification.created`;
/// since M4-33 their REST writes with events, quiet hours rules, automatic following and the
/// channel export; since M4-23 the limits of the search handlers; since M4-36 the terminal
/// sessions and the host end of the device streams.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';
import 'package:domain/domain.dart' show EventCatalog, FractionalIndex;

import 'fake_agent.dart';
import 'fake_host.dart';

part 'world_files.dart';
part 'world_notify.dart';
part 'world_search.dart';
part 'world_terminal.dart';
part 'world_thread_view.dart';

/// A JSON object of the API.
typedef JsonMap = Map<String, Object?>;

/// A rejection of a write or read, answered as an RFC 9457 problem (06 5).
final class FakeProblem implements Exception {
  /// Creates the problem with [status] and [code]; [current] is the current object of a
  /// conflict, [reason] a refinement, [retryAfter] the wait of `429` and `503`.
  const FakeProblem(
    this.status,
    this.code, {
    this.current,
    this.reason,
    this.retryAfter,
    this.errors,
    this.extra = const {},
  });

  /// The HTTP status.
  final int status;

  /// The problem code of `x-problem-codes`.
  final String code;

  /// The current object (`412 version_conflict`).
  final JsonMap? current;

  /// The refinement of the code.
  final String? reason;

  /// The wait of `429` and `503`.
  final Duration? retryAfter;

  /// Field errors of `400 validation_failed`.
  final List<JsonMap>? errors;

  /// Further members of the problem body, for example `hint` or `attempts_remaining`.
  final JsonMap extra;

  @override
  String toString() => 'FakeProblem($status $code)';
}

/// One person or agent of the world.
final class FakePerson {
  /// Creates a principal; [kind] is `user` or `agent`.
  FakePerson({
    required this.id,
    required this.handle,
    required this.displayName,
    this.kind = 'user',
    this.loginName,
    this.password,
    this.totpCode,
    this.isOwner = false,
    this.newDevicesPending = false,
  });

  /// The principal ID.
  final String id;

  /// The handle.
  final String handle;

  /// The display name.
  final String displayName;

  /// `user` or `agent`.
  final String kind;

  /// The login name of a local account, or null.
  final String? loginName;

  /// The password of a local account.
  final String? password;

  /// The TOTP code the fake accepts (fixed: the fake has no clock-based TOTP).
  final String? totpCode;

  /// Whether the person is an owner (`addOwner` and `revokeOwner` change it).
  bool isOwner;

  /// `pending`, `active`, `disabled` or `deleted` (`UserStatus`); `disableUser` and
  /// `enableUser` change it.
  String status = 'active';

  /// The e-mail address of the profile, or null.
  String? email;

  /// When the person was created.
  String createdAt = '2026-10-01T00:00:00.000Z';

  /// When and why the person became an owner (`Owner.granted_at`, `Owner.reason`).
  String ownerGrantedAt = '2026-10-01T00:00:00.000Z';

  /// The reason of the owner row.
  String ownerReason = 'bootstrap';

  /// The linked external identities (`Identity`), by ID.
  final Map<String, JsonMap> identities = {};

  /// The passkeys (`Passkey`), by ID.
  final Map<String, JsonMap> passkeys = {};

  /// Whether a device that logs in with a password starts `pending` (08 5.2); false lets
  /// tests skip the approval, a simplification the README names.
  bool newDevicesPending;

  /// The language of the profile (`users.locale`), or null while the person has chosen none.
  String? locale;

  /// The IANA time zone of the profile (`users.time_zone`), or null.
  String? timeZone;

  /// The API `User`.
  JsonMap get user => {
    'id': id,
    'handle': handle,
    'display_name': displayName,
    'status': status,
    'is_owner': isOwner,
    'locale': ?locale,
    'time_zone': ?timeZone,
    'created_at': createdAt,
  };

  /// The API `PrincipalRef`.
  JsonMap get ref => {
    'id': id,
    'kind': kind,
    'handle': handle,
    'display_name': displayName,
  };
}

/// One device of a person (08 5).
final class FakeDevice {
  /// Creates a device.
  FakeDevice({
    required this.id,
    required this.userId,
    required this.name,
    required this.platform,
    required this.state,
    required this.jkt,
    required this.createdAt,
    this.appVersion,
  });

  /// The device ID the client created.
  final String id;

  /// The owner.
  final String userId;

  /// The display name.
  final String name;

  /// The `DevicePlatform`.
  final String platform;

  /// The app version.
  final String? appVersion;

  /// `pending`, `active` or `revoked`.
  String state;

  /// The thumbprint of the DPoP key (`cnf.jkt`).
  final String jkt;

  /// When it was registered.
  final String createdAt;

  /// When it was approved.
  String? approvedAt;

  /// When it was revoked.
  String? revokedAt;

  /// Why it was revoked.
  String? revokeReason;

  /// The public JWK of the `step_up` key (08 6.1), or null for a device without one.
  JsonMap? stepUpJwk;

  /// The time of the last authenticated request of the device.
  String? lastSeenAt;

  /// The verification code of a waiting device (08 5.2), derived from ID and key.
  String get verificationCode {
    final digest = sha256.convert(utf8.encode('$id|$jkt')).bytes;
    var n = 0;
    for (final b in digest.take(4)) {
      n = (n << 8) | b;
    }
    return (n % 1000000).toString().padLeft(6, '0');
  }

  /// The API `Device`; [self] adds the verification code of a waiting device.
  JsonMap toJson({bool self = false}) => {
    'id': id,
    'user_id': userId,
    'name': name,
    'platform': platform,
    'state': state,
    'app_version': ?appVersion,
    'key_thumbprint': jkt,
    if (self && state == 'pending') 'verification_code': verificationCode,
    'revoke_reason': ?revokeReason,
    'approved_at': ?approvedAt,
    'revoked_at': ?revokedAt,
    'created_at': createdAt,
    'last_seen_at': ?lastSeenAt,
  };
}

/// One login session with its tokens (08 7).
final class FakeSession {
  /// Creates a session.
  FakeSession({
    required this.id,
    required this.userId,
    required this.deviceId,
    required this.authMethod,
    required this.createdAt,
    required this.expiresAt,
    required this.jkt,
  });

  /// The session ID (`sid`).
  final String id;

  /// The person.
  final String userId;

  /// The device.
  final String deviceId;

  /// The `Session.auth_method`.
  final String authMethod;

  /// When it started.
  final String createdAt;

  /// When it ends.
  final String expiresAt;

  /// The thumbprint of the DPoP key of the family.
  final String jkt;

  /// The current access token.
  String accessToken = '';

  /// The current refresh token.
  String refreshToken = '';

  /// Earlier access tokens that are still valid until their expiry.
  final Set<String> olderAccessTokens = {};

  /// Whether the session was revoked.
  bool revoked = false;

  /// The app session that created this child session of an extension (08 7.4), or null.
  String? parentId;

  /// The `client_component` of a child session (`share_ext`), or null for an app session.
  String? clientComponent;

  /// When the session last confirmed with its step-up key (08 6.2), or null.
  DateTime? stepUpAt;

  /// The time of the last authenticated request of the session.
  String? lastSeenAt;

  /// The API `Session`; [current] marks the session of the request.
  JsonMap toJson({bool? current}) => {
    'id': id,
    'user_id': userId,
    'device_id': deviceId,
    'auth_method': authMethod,
    'created_at': createdAt,
    'expires_at': expiresAt,
    'last_seen_at': ?lastSeenAt,
    'client_component': ?clientComponent,
    'current': ?current,
  };
}

/// A device pairing (08 5.3).
final class FakePairing {
  /// Creates a pairing.
  FakePairing({
    required this.id,
    required this.code,
    required this.registration,
    required this.jkt,
    required this.expiresAt,
  });

  /// The pairing ID.
  final String id;

  /// The code the new device shows.
  final String code;

  /// The `DeviceRegistration` of the new device.
  final JsonMap registration;

  /// The thumbprint of the new device's DPoP key.
  final String jkt;

  /// When it expires.
  String expiresAt;

  /// `requested`, `approved`, `expired` or `completed`.
  String state = 'requested';

  /// The person who approved it.
  String? approvedBy;
}

/// A channel with the levels of the principals that see it.
final class FakeChannel {
  /// Creates the channel from its API `Channel`.
  FakeChannel(this.json, this.access);

  /// The capabilities (API `Capability`, for example `manage`) per principal ID next to its
  /// level; the channel view reports them (brief M4-20: actions follow the server's
  /// capabilities).
  final Map<String, Set<String>> capabilities = {};

  /// The API `Channel`.
  final JsonMap json;

  /// The level (`read`, `write`, `trigger`) per principal ID; absent principals do not see
  /// the channel. The fake has no permission logic beyond this (README).
  final Map<String, String> access;

  /// The channel ID.
  String get id => json['id']! as String;

  /// The space ID, or null for a direct message.
  String? get spaceId => json['space_id'] as String?;
}

/// One event of the log, durable (with `seq`) or ephemeral.
final class FakeEvent {
  /// Creates the event.
  const FakeEvent(this.envelope, {this.audience, this.devices});

  /// The envelope as sent (04 2.2).
  final JsonMap envelope;

  /// The only receiver of a personal event.
  final String? audience;

  /// The devices of [audience] an ephemeral event goes to (`notification.created`, 13 4.6),
  /// or null for every connection that may receive it.
  final Set<String>? devices;

  /// The global `seq`, or null for an ephemeral event.
  int? get seq => envelope['seq'] as int?;

  /// The event type.
  String get type => envelope['type']! as String;

  /// The scope.
  JsonMap get scope => envelope['scope']! as JsonMap;
}

/// A stored response of a write with `Idempotency-Key` (04 5.2).
final class StoredResponse {
  /// Creates the record.
  const StoredResponse(this.hash, this.status, this.body);

  /// The hash of the request.
  final String hash;

  /// The status of the first answer.
  final int status;

  /// The body of the first answer.
  final Object? body;
}

/// Called after a message was stored and its `message.created` appended; the scripted agent
/// of 04 16.5 listens here.
typedef MessageCreatedHook = void Function(JsonMap message, FakePerson author);

/// Called for every appended event, durable or ephemeral.
typedef EventHook = void Function(FakeEvent event);

/// Called after the run document [runId] changed from [baseVersion] to [version] by [ops]
/// (04 11.2); the gateway turns it into coalesced `run.updated` frames.
typedef RunPatchHook = void Function(
  String runId,
  int baseVersion,
  int version,
  List<JsonMap> ops,
);

/// The in-memory model of the fake server.
///
/// Every write appends the events of 05 section 6 in one step with a new `seq` and calls the
/// [EventHook]s, which push them to the gateways that may see them. Reads and writes check
/// only visibility per channel and the level of the principal; there is no other permission
/// logic (README).
///
/// Example:
/// ```dart
/// final world = FakeWorld();
/// final alex = world.addPerson(handle: 'alex', displayName: 'Alex');
/// final space = world.addSpace('Team');
/// final channel = world.addChannel(space, 'general', {alex.id: 'write'});
/// final thread = world.addThread(channel, 'Release notes', createdBy: alex.id);
/// world.createMessage(alex.id, thread, {'client_msg_id': id, 'content_md': 'Hello'});
/// ```
final class FakeWorld {
  /// Creates an empty world on [now] (UTC; default the system time).
  FakeWorld({DateTime Function()? now})
    : now = now ?? (() => DateTime.now().toUtc()) {
    instanceId = nextId();
    localProviderId = nextId();
    syncEpoch = nextId();
  }

  /// The time source.
  final DateTime Function() now;

  int _ids = 0;

  /// The instance ID.
  late final String instanceId;

  /// The ID of the local login driver, which discovery lists like the server does (08 2).
  late final String localProviderId;

  /// The current sync epoch (04 7.5).
  late String syncEpoch;

  /// The product name the instance reports (`branding.product_name`).
  String productName = 'Chat';

  /// `min_client_versions` of discovery: one map per platform with `platform`, `min_version`
  /// and optionally `recommended_version`, as `ClientVersionRequirement` of the contract
  /// (18 9 item 2); a test raises them to exercise the app's version gate (brief M7-13).
  List<Map<String, Object?>> minClientVersions = [];

  /// `telemetry.dashboard_url` of the server: `getInstance` names it to owners only, null for
  /// no link (M6-06).
  String? telemetryDashboardUrl;

  /// `stats.time_zone` of the server, which `getInstance` reports to everyone (M6-06).
  String statsTimeZone = 'UTC';

  /// `security.app_lock_required` of the server, which `getInstance` reports to everyone in
  /// `app_lock` (M5-08); tests that need the enforced lock set it.
  bool appLockRequired = false;

  /// `client.app_lock_after` of the server in milliseconds, reported in `app_lock.after_ms`
  /// (M5-08); the server's default, tests lower it.
  int appLockAfterMs = 300000;

  /// The answers of the statistics operations (M6-06) by operation ID (`getUsageStats`,
  /// `getRunStats`, `getToolCallStats`, `getErrorStats`, `getCostStats`): the `buckets` and,
  /// for costs, the `currency`; an operation without an entry answers no buckets. The fake does
  /// not compute statistics; tests set the numbers a scope answers.
  ///
  /// The views of M6-16 add `getStatsLive`, `getStatsTimeline`, `getStatsSlo` and
  /// `getStatsBackends`: the whole body (for example a fixture of `bothy_viz`); without an
  /// entry they answer an empty body. `getStatsLive` gets the scope of the request.
  final Map<String, JsonMap> stats = {};

  /// `stats.live_enabled` (25 5.1 item 6): off, `getStatsLive` and `stats.subscribe` answer
  /// `feature_disabled`.
  bool statsLiveEnabled = true;

  /// `stats.timeline_max_range` (25 15): the longest range `getStatsTimeline` accepts.
  Duration statsTimelineMaxRange = const Duration(hours: 24);

  /// The components of the status page (M7-21) in their order, each an API
  /// `StatusComponent`; a component with `agent_id` is one of an agent (16 12).
  final List<JsonMap> statusComponents = [];

  /// The incidents and maintenance windows of the status page (M7-21), each an API
  /// `StatusIncident`, newest last.
  final List<JsonMap> statusIncidents = [];

  /// `status.public_enabled`: `getPublicStatus` answers `404` unless it is set (M7-21).
  bool statusPublicEnabled = false;

  /// The people who hold `agent.read` on an agent, by agent ID; owners read every agent. The
  /// status page shows the component of an agent only to them (16 12, M7-21).
  final Map<String, Set<String>> agentReaders = {};

  /// Whether [personId] sees the status component [c]: a component without an agent always,
  /// one of an agent only with `agent.read` on it (16 12).
  bool seesStatusComponent(String? personId, JsonMap c) {
    final agent = c['agent_id'] as String?;
    if (agent == null) return true;
    if (personId == null) return false;
    return (people[personId]?.isOwner ?? false) ||
        (agentReaders[agent]?.contains(personId) ?? false);
  }

  /// The API `StatusPage` for [personId], or for the public page when null: the visible
  /// components, the incidents with at least one visible component (with only the visible
  /// ones) or none at all, and `overall_state` the worst state of the components that belong
  /// to no agent (16 12 "As built" items 6 and 8).
  JsonMap statusPage(String? personId) {
    final visible = [
      for (final c in statusComponents)
        if (seesStatusComponent(personId, c)) c,
    ];
    final ids = {for (final c in visible) c['id']};
    return {
      'overall_state': statusOverallState(),
      'components': visible,
      'incidents': [
        for (final i in statusIncidents)
          if ((i['component_ids']! as List).isEmpty ||
              (i['component_ids']! as List).any(ids.contains))
            {
              ...i,
              'component_ids': [
                for (final id in i['component_ids']! as List)
                  if (ids.contains(id)) id,
              ],
            },
      ],
    };
  }

  /// The worst state of the components without an agent, in the order of the server
  /// (`operational`, `maintenance`, `degraded`, `partial_outage`, `major_outage`).
  String statusOverallState() {
    const order = [
      'operational',
      'maintenance',
      'degraded',
      'partial_outage',
      'major_outage',
    ];
    var worst = 0;
    for (final c in statusComponents) {
      if (c['agent_id'] != null) continue;
      final rank = order.indexOf(c['state']! as String);
      if (rank > worst) worst = rank;
    }
    return order[worst];
  }

  /// Adds a status component [name] at the end, without an event, and returns its ID; with
  /// [agentId] it is the component of that agent (`agent.presence`).
  String addStatusComponent(
    String name, {
    String state = 'operational',
    String description = '',
    String? healthSource,
    String? agentId,
  }) {
    final id = nextId();
    statusComponents.add({
      'id': id,
      'name': name,
      'description': description,
      'sort_key': statusComponents.length.toString().padLeft(6, '0'),
      'state': state,
      'health_source': ?(agentId != null ? 'agent.presence' : healthSource),
      'agent_id': ?agentId,
      'updated_at': stamp(),
    });
    return id;
  }

  /// Appends `status.updated` (16 12 item 6) for a change of [componentIds] or [incidentIds].
  void statusUpdated({
    List<String> componentIds = const [],
    List<String> incidentIds = const [],
  }) {
    append(
      'status.updated',
      resource: const {'kind': 'instance'},
      payload: {
        'overall_state': statusOverallState(),
        if (componentIds.isNotEmpty) 'changed_component_ids': componentIds,
        if (incidentIds.isNotEmpty) 'changed_incident_ids': incidentIds,
      },
    );
  }

  /// The bytes of the instance logo (`getBrandingLogo`), or null when none is set; discovery
  /// names its URL only when set.
  Uint8List? logo;

  /// The permission version of the instance; every access change raises it (04 8.2).
  int permissionVersion = 1;

  /// The `InstanceLimits` that `getInstance` reports (D-W-115); tests lower them.
  final JsonMap limits = {
    'message_max_content_bytes': 65536,
    'message_max_attachments': 10,
    'file_max_upload_bytes': 104857600,
    'thread_title_max_chars': 120,
    'voice_max_duration_ms': 600000,
  };

  /// The files of 03 `files` by ID with their bytes, upload session and transcript (M4-22).
  final Map<String, FakeFile> files = {};

  /// The settings of the file handlers (part size, scan, transcription, faults).
  final FakeFileSettings fileSettings = FakeFileSettings();

  /// The file tickets of `createFileTicket` by token (12 9.2).
  final Map<String, FakeFileTicket> fileTickets = {};

  /// The limits of the search handlers (`search.*`, 14 15); tests lower them.
  final FakeSearchSettings searchSettings = FakeSearchSettings();

  /// Events per catch-up page (`sync.catchup_page_size`).
  int catchupPageSize = 500;

  /// One `cursor` frame after this many skipped events (`sync.cursor_advance_every`).
  int cursorAdvanceEvery = 1000;

  /// A larger backlog yields `resync_required` `too_far_behind` (`sync.catchup_max_events`).
  int catchupMaxEvents = 50000;

  /// How long before its expiry a grant, a membership or an agent token shows in "Expiring
  /// soon" (`authz.expiry_reminder_lead`, 07 9.4 and 14); the default of the specification, a
  /// test may shorten it.
  Duration expiryReminderLead = const Duration(hours: 48);

  /// People and agents by ID.
  final Map<String, FakePerson> people = {};

  /// Groups (`Group` without `member_count`) by ID.
  final Map<String, JsonMap> groups = {};

  /// The members (`GroupMember`) of each group, by group and person ID.
  final Map<String, Map<String, JsonMap>> groupMembers = {};

  final Map<String, String> _membershipIds = {};

  /// The ID of the membership of [userId] in [groupId] (`group_memberships.id`, 03), which
  /// "Expiring soon" names but the API `GroupMember` does not carry, so [groupMembers] stays as
  /// the group handlers return it.
  ///
  /// Assigned with [nextId] on the first call and kept for the life of the world; unlike the
  /// server's new row, a member removed and added again keeps the ID (a simplification).
  String membershipId(String groupId, String userId) =>
      _membershipIds.putIfAbsent('$groupId|$userId', nextId);

  /// The grants by ID as the API returns them (`Grant`, brief M4-27); a revoked grant keeps
  /// its `revoked_at`.
  final Map<String, JsonMap> grants = {};

  /// The hosts of the terminal by ID (brief M4-37).
  final Map<String, FakeHost> hosts = {};

  /// The terminal sessions, the host link and the open device streams (brief M4-36).
  final FakeTerminal terminal = FakeTerminal();

  /// Simulates `bothy-host pair` on [hostId] (24 3.2 item 2): the host stores keys with
  /// [fingerprint] and the reported [names], becomes `confirming` and connected, and its key
  /// generation rises.
  void pairHost(
    String hostId,
    List<String> names, {
    String fingerprint =
        '7KQ2-9XTA-M4PB-1C8D-0000-0000-0000-0000-0000-0000-0000-0000-0000',
  }) {
    final h = hosts[hostId]!;
    h
      ..state = 'confirming'
      ..fingerprint = fingerprint
      ..keyGeneration += 1
      ..failures = 0
      ..reported = List.of(names)
      ..connected = true
      ..pairingCode = null
      ..version += 1;
  }

  /// People whose `enableUser` answers `409 i1_violation` (10 5.4), a switch of the tests.
  final Set<String> i1OnEnable = {};

  /// The last enrollment code issued per person (only the newest is valid, 08 3.2).
  final Map<String, String> enrollmentCodes = {};

  /// Devices by ID.
  final Map<String, FakeDevice> devices = {};

  /// Sessions by ID.
  final Map<String, FakeSession> sessions = {};

  /// Pairings by ID.
  final Map<String, FakePairing> pairings = {};

  /// Open step-up challenges (08 6.2) by ID: the random value and the session that asked.
  final Map<String, ({String challenge, String sessionId})> stepUpChallenges =
      {};

  /// Spaces (API `Space`) by ID.
  final Map<String, JsonMap> spaces = {};

  /// Channels by ID.
  final Map<String, FakeChannel> channels = {};

  /// Threads (API `Thread`) by ID.
  final Map<String, JsonMap> threads = {};

  /// Messages (API `Message`) by ID.
  final Map<String, JsonMap> messages = {};

  final Map<String, List<String>> _threadMessages = {};
  final Map<String, ({String id, String hash})> _clientMsgIds = {};

  /// Read states (API `ReadState`) by `<principal>|<thread>`.
  final Map<String, JsonMap> readStates = {};

  /// Drafts (API `Draft`) by `<principal>|<thread>`; the draft of a new post in a forum (no
  /// `thread_id`, 03 table drafts) by `<principal>|channel:<channel>`.
  final Map<String, JsonMap> drafts = {};

  /// Settings (API `UserSetting`) by principal and key.
  final Map<String, Map<String, JsonMap>> settings = {};

  /// Runs (API `Run`, the run document of 04 11.1) by ID.
  final Map<String, JsonMap> runs = {};

  /// Forum tags by ID (API `ForumTag`).
  final Map<String, JsonMap> forumTags = {};

  /// Sidebar states (API `SidebarState`) by `<principal>|<space>`.
  final Map<String, JsonMap> sidebarStates = {};

  /// Stored answers of `Idempotency-Key` writes by `<principal>|<key>`.
  final Map<String, StoredResponse> idempotency = {};

  /// The durable event log in `seq` order; [dropEventsUpTo] removes old entries.
  final List<FakeEvent> log = [];

  /// The `seq` of the newest durable event.
  int headSeq = 0;

  /// The smallest `seq` still in the log after a retention cut, or `headSeq + 1`.
  int minRetainedSeq = 1;

  /// Listeners of appended events (the gateway).
  final List<EventHook> eventHooks = [];

  /// Listeners of created messages (the scripted agents), called after `message.created`.
  final List<MessageCreatedHook> messageHooks = [];

  /// Listeners of run document patches (the gateway).
  final List<RunPatchHook> runPatchHooks = [];

  // -------------------------------------------------------------------------------------------
  // Notifications (13; M4-30, world_notify.dart)

  /// The stored preferences (API `NotificationPreference`) by `<user>|<kind>|<target>`.
  final Map<String, JsonMap> _notificationPrefs = {};

  /// The quiet hours rules (API `QuietHoursRule`) per person by rule ID (13 3.4). The fake
  /// stores and lists them and sends `quiet_hours.updated`; it does not evaluate them, the
  /// tests list the people inside quiet hours in [quietHours].
  final Map<String, Map<String, JsonMap>> quietHoursRules = {};

  /// `notify.user_default_level`, the last step of the effective level (13 3.1, 17).
  String userDefaultLevel = 'mentions';

  /// `notify.default_preview_mode`, the preview mode of a person without the setting
  /// `notify.preview_mode` (13 6.4).
  String defaultPreviewMode = 'full';

  /// `notify.preview_max_chars`, the length of `excerpt` in characters (13 6.4).
  int previewMaxChars = 240;

  /// The people inside quiet hours right now; their hints carry `quiet: true`. The fake does
  /// not evaluate the rules of [quietHoursRules] (13 3.4), this is a switch of the tests.
  final Set<String> quietHours = {};

  /// Whether [createMessage] calls [notify] for every new message (not for a replay); off so
  /// tests without hints see no extra frames.
  bool notifyOnMessages = false;

  /// The devices of a person that have an open, live gateway connection; the gateway sets
  /// it, without one no device has a route (13 4.6).
  Set<String> Function(String userId) connectedDevices = _noDevices;

  static Set<String> _noDevices(String userId) => const {};

  /// Every hint [notify] created, in order.
  final List<FakeNotification> notifications = [];

  /// The recipients [notify] decided against: `level` (the effective level is too low) or
  /// `no_route` (no active device with a live connection, 13 4.6 item 4).
  final List<({String userId, String reason})> notificationSkips = [];

  /// Stores the preference of [userId] for the target [targetKind] (`thread`, `channel` or
  /// `space`) [targetId] at [level] (`all`, `mentions` or `mute`), muted until [mutedUntil]
  /// when given (13 3.1). A later call replaces it.
  ///
  /// A setup step of the tests: no checks of the target and no `notification_pref.updated`
  /// event; the REST path is [putNotificationPref]. Throws an [ArgumentError] for another
  /// target kind or level.
  ///
  /// Example:
  /// ```dart
  /// world.setNotificationPref(sam.id, 'channel', general, 'all');
  /// ```
  void setNotificationPref(
    String userId,
    String targetKind,
    String targetId,
    String level, {
    DateTime? mutedUntil,
  }) => _setPref(userId, targetKind, targetId, level, mutedUntil);

  /// Removes the preference of [userId] for [targetKind] [targetId]; nothing happens when
  /// there is none. No event, as for [setNotificationPref].
  void deleteNotificationPref(
    String userId,
    String targetKind,
    String targetId,
  ) => _notificationPrefs.remove('$userId|$targetKind|$targetId');

  /// The preferences of [userId] on targets it can still read, ordered by target kind, then
  /// target ID (`listMyNotificationPreferences`).
  List<JsonMap> notificationPrefsOf(String userId) => _prefsOf(userId);

  /// Sets the level of [userId] on [targetKind] [targetId] from a `NotificationPreferencePut`
  /// [body] and appends `notification_pref.updated` (`putNotificationPreference`, 13 3.1).
  ///
  /// A write that changes nothing keeps the version and appends nothing. Throws [FakeProblem]:
  /// `404` for a target the person cannot read (or of another kind), `409 conflict` with
  /// `target_archived` for an archived target, `400 validation_failed` for a level outside
  /// the list.
  JsonMap putNotificationPref(
    String userId,
    String targetKind,
    String targetId,
    JsonMap body,
  ) => _putPref(userId, targetKind, targetId, body);

  /// Deletes the level of [userId] on [targetKind] [targetId]: `notification_pref.updated`
  /// with `deleted: true` and the last version plus one; nothing when there is no row
  /// (`deleteNotificationPreference`). Throws `404` for a target the person cannot read.
  void removeNotificationPref(
    String userId,
    String targetKind,
    String targetId,
  ) => _removePref(userId, targetKind, targetId);

  /// The quiet hours rules of [userId], ascending by ID (`listMyQuietHours`).
  List<JsonMap> quietHoursOf(String userId) => _rulesOf(userId);

  /// Adds a rule of [userId] from a `QuietHoursRuleInput` [body] and appends the complete
  /// list in `quiet_hours.updated` (`createQuietHoursRule`, 13 3.4). Throws `400
  /// validation_failed` for weekdays outside 1 to 127 or a time that is not `HH:MM`.
  JsonMap createQuietHoursRule(String userId, JsonMap body) =>
      _createRule(userId, body);

  /// Replaces rule [ruleId] of [userId] with [body]; equal values keep the version and append
  /// nothing (`putQuietHoursRule`). Throws `404` for an unknown rule, `400` as for
  /// [createQuietHoursRule].
  JsonMap putQuietHoursRule(String userId, String ruleId, JsonMap body) =>
      _putRule(userId, ruleId, body);

  /// Deletes rule [ruleId] of [userId] and appends the remaining list
  /// (`deleteQuietHoursRule`). Throws `404` for an unknown rule.
  void deleteQuietHoursRule(String userId, String ruleId) =>
      _deleteRule(userId, ruleId);

  /// The effective level (`all`, `mentions` or `mute`) of [userId] for [threadId] (13 3.1):
  /// the first existing value of the own preference for the thread, the channel and the
  /// space, the channel's `default_notification_level`, `all` in a DM channel, the setting
  /// `notify.default_level` and [userDefaultLevel]. A selected preference with `muted_until`
  /// after [now] gives `mute`.
  ///
  /// Example:
  /// ```dart
  /// expect(world.effectiveLevel(sam.id, thread), 'mentions');
  /// ```
  String effectiveLevel(String userId, String threadId) =>
      _effectiveLevel(userId, threadId);

  /// Decides the hints of the message [messageId] (13 2) and delivers them (13 4.6, 6.3);
  /// returns the hints created by this call, which [notifications] also records.
  ///
  /// No hint for `silent` messages, `origin` `import` or `system`, or message kind `run` or
  /// `system`. Candidates are the active people (not agents) who see the channel, never the
  /// author: `dm` in a DM channel and `mention` for a mentioned person need `mentions` or
  /// `all`, `message` needs `all`. Each hint goes as the ephemeral `notification.created`
  /// with the preview of the recipient's `notify.preview_mode` (13 6.4) to the live
  /// connections of the recipient's active devices ([connectedDevices]); a recipient
  /// without one gets nothing. Both kinds of refusal land in [notificationSkips].
  /// Simplified: no runs, no bundling (`count` is 1), no pushes. Throws an
  /// [ArgumentError] for an unknown message.
  ///
  /// Example:
  /// ```dart
  /// final message = world.createMessage(alex.id, thread, body).message;
  /// final hints = world.notify(message['id']! as String);
  /// ```
  List<FakeNotification> notify(String messageId) => _notify(messageId);

  /// Checks before a message is stored: the first non-null answer is its `trigger_state`
  /// (`received` for a message that triggers an agent, 04 16.5).
  final List<String? Function(JsonMap message, FakePerson author)>
  triggerChecks = [];

  // -------------------------------------------------------------------------------------------
  // IDs and time

  /// A new UUIDv7-shaped ID; deterministic per world, so faults and assertions repeat.
  String nextId() {
    final n = ++_ids;
    final hex = n.toRadixString(16).padLeft(15, '0');
    return '0192f4a0-7b1c-7${hex.substring(0, 3)}-8${hex.substring(3, 6)}-'
        '000${hex.substring(6)}';
  }

  /// The current time as an RFC 3339 timestamp with milliseconds and `Z`.
  String stamp([DateTime? at]) {
    final t = (at ?? now()).toUtc();
    String two(int v) => v.toString().padLeft(2, '0');
    return '${t.year.toString().padLeft(4, '0')}-${two(t.month)}-${two(t.day)}'
        'T${two(t.hour)}:${two(t.minute)}:${two(t.second)}'
        '.${t.millisecond.toString().padLeft(3, '0')}Z';
  }

  // -------------------------------------------------------------------------------------------
  // Building the world

  /// Adds a person or agent and returns it.
  FakePerson addPerson({
    required String handle,
    required String displayName,
    String kind = 'user',
    String? loginName,
    String? password,
    String? totpCode,
    bool isOwner = false,
    bool newDevicesPending = false,
    String? id,
  }) {
    final p = FakePerson(
      id: id ?? nextId(),
      handle: handle,
      displayName: displayName,
      kind: kind,
      loginName: loginName,
      password: password,
      totpCode: totpCode,
      isOwner: isOwner,
      newDevicesPending: newDevicesPending,
    );
    people[p.id] = p;
    return p;
  }

  /// Adds a space and returns its ID.
  String addSpace(String name) {
    final id = nextId();
    spaces[id] = {
      'id': id,
      'name': name,
      'sort_key': 'a${spaces.length}',
      'version': 1,
      'created_at': stamp(),
    };
    return id;
  }

  /// Adds a text channel to [spaceId] that the principals of [access] see with their level,
  /// and its main thread; returns the channel ID.
  String addChannel(String spaceId, String name, Map<String, String> access) {
    final id = nextId();
    channels[id] = FakeChannel({
      'id': id,
      'space_id': spaceId,
      'kind': 'text',
      'name': name,
      'topic': '',
      'sort_key': 'a${channels.length}',
      'operator': _inheritOperator(spaceId),
      'read_only': false,
      'project_bound': false,
      'version': 1,
      'settings_version': 1,
      'created_at': stamp(),
    }, Map.of(access));
    return id;
  }

  /// Adds a thread to [channelId] and returns its ID.
  String addThread(
    String channelId,
    String title, {
    required String createdBy,
  }) {
    final id = nextId();
    threads[id] = {
      'id': id,
      'channel_id': channelId,
      'kind': 'thread',
      'title': title,
      'title_source': 'manual',
      'state': 'open',
      'last_message_seq': 0,
      'last_activity_at': stamp(),
      'message_count': 0,
      'created_by': createdBy,
      'created_at': stamp(),
      'version': 1,
      'tag_ids': <Object?>[],
    };
    _threadMessages[id] = [];
    return id;
  }

  /// Adds a forum to [spaceId] that the principals of [access] see with their level, sorted by
  /// [defaultSort] (`forum_default_sort`); returns the channel ID.
  String addForum(
    String spaceId,
    String name,
    Map<String, String> access, {
    String defaultSort = 'last_activity',
  }) {
    final id = addChannel(spaceId, name, access);
    channels[id]!.json
      ..['kind'] = 'forum'
      ..['forum_default_sort'] = defaultSort;
    return id;
  }

  /// Creates ([bound] true) or deletes the project binding of [channelId] as the server does
  /// (03 `project_bindings`, R-DM-54): `project_bound` follows, `channels.version` rises and
  /// `channel.updated` with the channel goes to everyone who sees it. The fake keeps no binding
  /// row, only the derived flag; as on the server, a call that leaves the flag as it is writes
  /// nothing (an identical binding raises no version).
  void setProjectBound(String channelId, {required bool bound}) {
    final channel = channels[channelId]!;
    if (channel.json['project_bound'] == bound) return;
    final json = channel.json
      ..['project_bound'] = bound
      ..['version'] = (channel.json['version']! as int) + 1;
    append(
      'channel.updated',
      resource: {
        'kind': 'channel',
        'id': channelId,
        'version': json['version'],
      },
      scope: {'space_id': ?channel.spaceId, 'channel_id': channelId},
      payload: {'channel': json},
    );
  }

  /// Adds the tag [name] to forum [channelId] with the category [color]; returns its ID.
  String addForumTag(
    String channelId,
    String name, {
    String? color,
    bool restricted = false,
  }) {
    final id = nextId();
    final order = forumTags.values.where((t) => t['channel_id'] == channelId);
    forumTags[id] = {
      'id': id,
      'channel_id': channelId,
      'name': name,
      'color': ?color,
      'sort_key': 'a${order.length}',
      'restricted': restricted,
    };
    return id;
  }

  /// The tags of forum [channelId] in their order.
  List<JsonMap> tagsOf(String channelId) =>
      forumTags.values.where((t) => t['channel_id'] == channelId).toList()
        ..sort(
          (a, b) =>
              (a['sort_key']! as String).compareTo(b['sort_key']! as String),
        );

  // -------------------------------------------------------------------------------------------
  // Visibility

  /// The level of [principalId] in [channelId], or null when it does not see it.
  String? levelOf(String principalId, String channelId) =>
      channels[channelId]?.access[principalId];

  /// Whether [principalId] sees [channelId].
  bool seesChannel(String principalId, String channelId) =>
      levelOf(principalId, channelId) != null;

  /// Whether [principalId] sees at least one channel of [spaceId].
  bool seesSpace(String principalId, String spaceId) => channels.values.any(
    (c) => c.spaceId == spaceId && c.access.containsKey(principalId),
  );

  /// Whether [principalId] sees [threadId].
  bool seesThread(String principalId, String threadId) {
    final t = threads[threadId];
    return t != null && seesChannel(principalId, t['channel_id']! as String);
  }

  /// Whether [principalId] may receive [event] (04 7.2, 05 6): a personal event only its
  /// audience, a channel event those who see the channel, a space event those who see a
  /// channel of the space, an instance event everyone.
  bool mayReceive(String principalId, FakeEvent event) {
    if (event.audience != null) return event.audience == principalId;
    final channel = event.scope['channel_id'] as String?;
    if (channel != null) return seesChannel(principalId, channel);
    final space = event.scope['space_id'] as String?;
    if (space != null) return seesSpace(principalId, space);
    return true;
  }

  /// The thread [threadId] when [principalId] sees it; else `404 not_found`.
  JsonMap visibleThread(String principalId, String threadId) {
    if (!seesThread(principalId, threadId)) {
      throw const FakeProblem(404, 'not_found');
    }
    return threads[threadId]!;
  }

  /// The message [messageId] when [principalId] sees its thread; else `404 not_found`.
  JsonMap visibleMessage(String principalId, String messageId) {
    final m = messages[messageId];
    if (m == null || !seesThread(principalId, m['thread_id']! as String)) {
      throw const FakeProblem(404, 'not_found');
    }
    return m;
  }

  // -------------------------------------------------------------------------------------------
  // Views

  /// The API `SpaceView` of [spaceId] for [principalId]: the highest level of its channels.
  JsonMap spaceView(String principalId, String spaceId) => {
    'space': spaces[spaceId],
    'level': _highest([
      for (final c in channels.values)
        if (c.spaceId == spaceId && c.access[principalId] != null)
          c.access[principalId]!,
    ]),
    'capabilities': <Object?>[],
  };

  /// The API `ChannelView` of [channelId] for [principalId].
  JsonMap channelView(String principalId, String channelId) => {
    'channel': channels[channelId]!.json,
    'level': levelOf(principalId, channelId),
    'capabilities': [
      ...?channels[channelId]!.capabilities[principalId]?.toList()?..sort(),
    ],
  };

  /// Whether [principalId] holds the capability `manage` in [channelId].
  bool manages(String principalId, String channelId) =>
      channels[channelId]?.capabilities[principalId]?.contains('manage') ??
      false;

  static String _highest(List<String> levels) => levels.contains('trigger')
      ? 'trigger'
      : levels.contains('write')
      ? 'write'
      : 'read';

  /// The visible spaces of [principalId], as `SpaceView`s.
  List<JsonMap> spaceViews(String principalId) => [
    for (final id in spaces.keys)
      if (seesSpace(principalId, id)) spaceView(principalId, id),
  ];

  /// The visible channels of [principalId], optionally of one space, as `ChannelView`s.
  List<JsonMap> channelViews(String principalId, {String? spaceId}) => [
    for (final c in channels.values)
      if (c.access.containsKey(principalId) &&
          (spaceId == null || c.spaceId == spaceId))
        channelView(principalId, c.id),
  ];

  /// The messages of [threadId] in `thread_seq` order.
  List<JsonMap> messagesOf(String threadId) => [
    for (final id in _threadMessages[threadId] ?? const <String>[])
      messages[id]!,
  ];

  /// The bound of `active_threads` per channel in the fake: the default of
  /// `sync.snapshot_threads_per_channel` (04 18). The fake has no `sync.snapshot_thread_window`:
  /// every open thread counts as active within it.
  static const snapshotThreadsPerChannel = 20;

  /// The API `active_threads` of [principalId] (04 6.3 item 4, R-SY-28): per visible channel, in
  /// the order of the channels, the open threads of kind `thread` with an active run or in
  /// [unread], plus the newest others until the channel holds [snapshotThreadsPerChannel];
  /// newest activity first.
  List<JsonMap> activeThreads(String principalId, List<JsonMap> unread) {
    final pinned = {for (final u in unread) u['thread_id']};
    bool kept(JsonMap t) =>
        pinned.contains(t['id']) || activeRunOf(t['id']! as String) != null;
    return [
      for (final c in channels.values)
        if (c.access.containsKey(principalId))
          ..._boundedThreads(_openThreads(principalId, c.id), kept),
    ];
  }

  /// The open threads of kind `thread` of [channelId] that [principalId] sees, newest activity
  /// first.
  List<JsonMap> _openThreads(String principalId, String channelId) =>
      [
        for (final t in threads.values)
          if (t['channel_id'] == channelId &&
              t['kind'] == 'thread' &&
              t['state'] == 'open' &&
              seesThread(principalId, t['id']! as String))
            t,
      ]..sort(
        (a, b) => (b['last_activity_at']! as String).compareTo(
          a['last_activity_at']! as String,
        ),
      );

  /// Every thread of [open] that [kept] holds, plus the others in their order until the list has
  /// [snapshotThreadsPerChannel] entries.
  static List<JsonMap> _boundedThreads(
    List<JsonMap> open,
    bool Function(JsonMap) kept,
  ) {
    var room = snapshotThreadsPerChannel - open.where(kept).length;
    return [
      for (final t in open)
        if (kept(t) || room-- > 0) t,
    ];
  }

  /// The API `Snapshot` of [principalId] (04 6.3 item 4, 04 7.3).
  JsonMap snapshot(String principalId) {
    final person = people[principalId]!;
    final unread = unreadThreads(principalId);
    return {
      'head_seq': headSeq,
      'permission_version': permissionVersion,
      'sync_epoch': syncEpoch,
      'principal': person.ref,
      'is_owner': person.isOwner,
      'spaces': spaceViews(principalId),
      'channels': channelViews(principalId),
      'sidebar': sidebarOf(principalId),
      'unread_threads': unread,
      'active_runs': [
        for (final r in runs.values)
          if (_active(r) &&
              seesChannel(principalId, r['channel_id']! as String))
            runSummary(r),
      ],
      'active_threads': activeThreads(principalId, unread),
      'agents': <Object?>[],
      'settings': [...?settings[principalId]?.values],
      'drafts': [
        for (final MapEntry(:key, :value) in drafts.entries)
          if (key.startsWith('$principalId|'))
            {
              'id': value['id'],
              'thread_id': ?value['thread_id'],
              'channel_id': value['channel_id'],
              'version': value['version'],
            },
      ],
      'features': <Object?>[],
    };
  }

  /// The messages of [threadId] after [lastRead] that count as unread for [principalId]:
  /// written by someone else and not deleted.
  List<JsonMap> _unread(String principalId, String threadId, int lastRead) => [
    for (final m in messagesOf(threadId))
      if ((m['thread_seq']! as int) > lastRead &&
          m['deleted_at'] == null &&
          (m['author']! as JsonMap)['id'] != principalId)
        m,
  ];

  static int _mentions(String principalId, List<JsonMap> messages) => messages
      .where((m) => (m['mention_ids']! as List<Object?>).contains(principalId))
      .length;

  /// The API `UnreadThread`s of [principalId]: every visible thread with unread messages, with
  /// the counts after its read state (04 14).
  List<JsonMap> unreadThreads(String principalId) => [
    for (final t in threads.values)
      if (seesThread(principalId, t['id']! as String))
        ...() {
          final id = t['id']! as String;
          final lastRead =
              (readStates['$principalId|$id']?['last_read_seq'] as int?) ?? 0;
          final unread = _unread(principalId, id, lastRead);
          return [
            if (unread.isNotEmpty)
              {
                'thread_id': id,
                'channel_id': t['channel_id'],
                'unread_count': unread.length,
                'mention_count': _mentions(principalId, unread),
                'last_message_seq': t['last_message_seq'],
                'last_read_seq': lastRead,
              },
          ];
        }(),
  ];

  // -------------------------------------------------------------------------------------------
  // The event log

  /// Appends a durable event of [type] about [resource] in [scope] with [payload], raising
  /// `seq`, and calls the [eventHooks]; returns it.
  ///
  /// [audience] makes it personal (`audience: self`); [actor] names who triggered it.
  FakeEvent append(
    String type, {
    required JsonMap resource,
    required JsonMap payload,
    JsonMap scope = const {},
    String? actor,
    String? audience,
    String? clientMsgId,
  }) {
    // 04 9.3: `resource.version` is present exactly for the types the catalogue marks, so the
    // fake cannot report a foreign version that would make a client drop later changes.
    final info = EventCatalog.lookup(type);
    if (info == null) {
      throw ArgumentError.value(type, 'type', 'not in the event catalogue');
    }
    if (info.carriesResourceVersion && !resource.containsKey('version')) {
      throw ArgumentError.value(type, 'type', 'needs resource.version');
    }
    final res = info.carriesResourceVersion
        ? resource
        : ({...resource}..remove('version'));
    final envelope = <String, Object?>{
      'id': nextId(),
      'seq': ++headSeq,
      'type': type,
      'schema_version': 1,
      'occurred_at': stamp(),
      if (actor != null)
        'actor': {'id': actor, 'kind': people[actor]?.kind ?? 'user'},
      'scope': scope,
      'resource': res,
      if (audience != null) 'audience': 'self',
      'payload': payload,
      'client_msg_id': ?clientMsgId,
      'permission_version': permissionVersion,
    };
    // A copy: the world keeps changing its objects, a logged event never changes.
    final event = FakeEvent(_copy(envelope), audience: audience);
    log.add(event);
    for (final hook in List.of(eventHooks)) {
      hook(event);
    }
    return event;
  }

  /// Publishes an ephemeral event (no `seq`, never in the log) to the [eventHooks].
  ///
  /// [audience] makes it personal; [devices] names the devices of [audience] whose live
  /// connections get it (the gateway sends an ephemeral event only with both).
  FakeEvent publishEphemeral(
    String type, {
    required JsonMap resource,
    required JsonMap payload,
    JsonMap scope = const {},
    String? audience,
    Set<String>? devices,
  }) {
    final event = FakeEvent(
      _copy({
        'id': nextId(),
        'type': type,
        'schema_version': 1,
        'occurred_at': stamp(),
        'scope': scope,
        'resource': resource,
        if (audience != null) 'audience': 'self',
        'payload': payload,
      }),
      audience: audience,
      devices: devices == null ? null : Set.unmodifiable(devices),
    );
    for (final hook in List.of(eventHooks)) {
      hook(event);
    }
    return event;
  }

  static JsonMap _copy(JsonMap value) =>
      jsonDecode(jsonEncode(value)) as JsonMap;

  /// The events with `after < seq <= upTo` still in the log.
  Iterable<FakeEvent> eventsAfter(int after, {int? upTo}) =>
      log.where((e) => e.seq! > after && e.seq! <= (upTo ?? headSeq));

  /// Removes the events up to [seq] from the log, as `sync.event_retention` does, so a cursor
  /// below them gets `resync_required` with `retention` (04 7.3).
  void dropEventsUpTo(int seq) {
    log.removeWhere((e) => e.seq! <= seq);
    minRetainedSeq = seq + 1;
  }

  /// Starts a new time line as the final step of a restore does (04 7.5): a new epoch, so a
  /// client with the old one gets `resync_required` with `epoch_changed`.
  void changeEpoch() => syncEpoch = nextId();

  // -------------------------------------------------------------------------------------------
  // Messages

  static final RegExp _uuid7 = RegExp(
    r'^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
  );

  /// Creates a message of [authorId] in [threadId] from a `MessageCreate` [body] (04 5.1):
  /// returns the message and whether it was a replay of the same `client_msg_id`.
  ///
  /// Throws [FakeProblem]: `404` for a thread the author does not see, `403 forbidden` below
  /// `write`, `403 channel_read_only`, `400 validation_failed` for a `client_msg_id` that is
  /// not a UUIDv7, `422 idempotency_key_reuse` for the same ID with a different body.
  ({JsonMap message, bool replay}) createMessage(
    String authorId,
    String threadId,
    JsonMap body, {
    String origin = 'human',
  }) {
    final thread = visibleThread(authorId, threadId);
    final channelId = thread['channel_id']! as String;
    final channel = channels[channelId]!;
    final level = channel.access[authorId];
    if (level == 'read') throw const FakeProblem(403, 'forbidden');
    if (channel.json['read_only'] == true) {
      throw const FakeProblem(403, 'channel_read_only');
    }
    final cmid = body['client_msg_id']! as String;
    if (!_uuid7.hasMatch(cmid)) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/client_msg_id', 'code': 'invalid_format'},
        ],
      );
    }
    // The canonical request: thread, content, reply reference, attachments (04 5.1 item 3).
    final hash = sha256
        .convert(
          utf8.encode(
            jsonEncode([
              threadId,
              body['content_md'] ?? '',
              body['reply_to_message_id'] ?? '',
              body['attachment_file_ids'] ?? const <Object?>[],
            ]),
          ),
        )
        .toString();
    final known = _clientMsgIds['$authorId|$cmid'];
    if (known != null) {
      if (known.hash != hash) {
        throw const FakeProblem(422, 'idempotency_key_reuse');
      }
      return (message: messages[known.id]!, replay: true);
    }
    final author = people[authorId]!;
    if (thread['state'] == 'closed') {
      _reopen(threadId, authorId);
      _systemLine(threadId, 'thread.reopened', authorId);
    }
    final seq = (thread['last_message_seq']! as int) + 1;
    final id = nextId();
    final mentions = [
      ...?(body['mention_ids'] as List<Object?>?)?.cast<String>(),
      // 03 content format item 4: the server resolves `@handle` tokens of principals that
      // see the channel; the fake does it only when a test asks (M4-21).
      if (resolveMentions && body['mention_ids'] == null)
        ..._handlesIn(body['content_md'] as String? ?? '', channelId),
    ];
    final attachments = _attachable(authorId, channelId, [
      ...?(body['attachment_file_ids'] as List<Object?>?)?.cast<String>(),
    ], kind: body['kind'] as String? ?? 'text');
    final message = <String, Object?>{
      'id': id,
      'thread_id': threadId,
      'channel_id': channelId,
      'thread_seq': seq,
      'author': author.ref,
      'client_msg_id': cmid,
      'origin': origin,
      'kind': body['kind'] ?? 'text',
      'content_md': body['content_md'] ?? '',
      'content_format': 1,
      'reply_to_message_id': ?body['reply_to_message_id'],
      'silent': body['silent'] ?? false,
      'attachments': attachments,
      'mention_ids': mentions,
      'reactions': <Object?>[],
      'pinned': false,
      'components': <Object?>[],
      'version': 1,
      'created_at': stamp(),
    };
    messages[id] = message;
    _attach(id, attachments);
    _threadMessages[threadId]!.add(id);
    _clientMsgIds['$authorId|$cmid'] = (id: id, hash: hash);
    thread
      ..['last_message_seq'] = seq
      ..['message_count'] = (thread['message_count']! as int) + 1
      ..['last_activity_at'] = message['created_at'];
    for (final check in List.of(triggerChecks)) {
      final state = check(message, author);
      if (state == null) continue;
      message
        ..['trigger_state'] = state
        ..['trigger_state_at'] = message['created_at'];
      break;
    }
    append(
      'message.created',
      resource: {'kind': 'message', 'id': id, 'version': message['version']},
      scope: _scopeOf(threadId),
      payload: {'message': message},
      actor: authorId,
      clientMsgId: cmid,
    );
    for (final hook in List.of(messageHooks)) {
      hook(message, author);
    }
    // 13 3.2: a person who writes in a thread (not the main thread) follows it.
    if (origin == 'human') _follow(authorId, threadId);
    if (notifyOnMessages) notify(id);
    if (body['clear_draft'] == true) deleteDraft(authorId, threadId);
    return (message: message, replay: false);
  }

  /// Applies a `MessageUpdate` [body] to [messageId] of [principalId] when [expectedVersion]
  /// matches (04 13.3, 06 `If-Match`).
  ///
  /// Throws [FakeProblem]: `404`, `403 forbidden` for another author, `410 message_deleted`,
  /// `412 version_conflict` with the current revision.
  JsonMap updateMessage(
    String principalId,
    String messageId,
    int expectedVersion,
    JsonMap body,
  ) {
    final m = visibleMessage(principalId, messageId);
    if (m['deleted_at'] != null) {
      throw const FakeProblem(410, 'message_deleted');
    }
    if ((m['author']! as JsonMap)['id'] != principalId) {
      throw const FakeProblem(403, 'forbidden');
    }
    if (m['version'] != expectedVersion) {
      throw FakeProblem(412, 'version_conflict', current: Map.of(m));
    }
    final earlier = revisions.putIfAbsent(messageId, () => []);
    earlier.add({
      'revision': earlier.length + 1,
      'content_md': m['content_md'],
      'edited_by': principalId,
      'created_at': m['edited_at'] ?? m['created_at'],
    });
    if (body.containsKey('content_md')) m['content_md'] = body['content_md'];
    if (body.containsKey('mention_ids')) m['mention_ids'] = body['mention_ids'];
    m
      ..['version'] = (m['version']! as int) + 1
      ..['edited_at'] = stamp();
    _messageUpdated(m, principalId);
    return m;
  }

  /// Deletes [messageId] as a tombstone (03 6.5); a second delete changes nothing (idempotent
  /// `204`). Throws `404` or `403 forbidden` for another author's message.
  void deleteMessage(String principalId, String messageId) {
    final m = visibleMessage(principalId, messageId);
    if (m['deleted_at'] != null) return;
    if ((m['author']! as JsonMap)['id'] != principalId) {
      throw const FakeProblem(403, 'forbidden');
    }
    final at = stamp();
    m
      ..['content_md'] = ''
      ..['attachments'] = <Object?>[]
      ..['mention_ids'] = <Object?>[]
      ..['reactions'] = <Object?>[]
      ..['components'] = <Object?>[]
      ..remove('my_reactions')
      ..['deleted_at'] = at
      ..['version'] = (m['version']! as int) + 1;
    append(
      'message.deleted',
      resource: {'kind': 'message', 'id': messageId, 'version': m['version']},
      scope: _scopeOf(m['thread_id']! as String),
      payload: {
        'message_id': messageId,
        'thread_id': m['thread_id'],
        'thread_seq': m['thread_seq'],
        'deleted_at': at,
        'deleted_by': principalId,
      },
      actor: principalId,
    );
  }

  final Map<String, Set<String>> _reactions = {};

  /// The earlier revisions (API `MessageRevision`) of each edited message, oldest first.
  final Map<String, List<JsonMap>> revisions = {};

  /// Who pinned a message and when (API `Pin`), by message ID.
  final Map<String, JsonMap> pins = {};

  /// Templates (API `Template`) by ID; a personal template also names its owner in
  /// [templateOwners].
  final Map<String, JsonMap> templates = {};

  /// The owner of each personal template, by template ID.
  final Map<String, String> templateOwners = {};

  /// Adds or removes the reaction [emoji] of [principalId] on [messageId]; a repeat changes
  /// nothing. Throws `404`, `410 message_deleted` when adding to a deleted message.
  void react(
    String principalId,
    String messageId,
    String emoji, {
    required bool add,
  }) {
    final m = visibleMessage(principalId, messageId);
    if (add && m['deleted_at'] != null) {
      throw const FakeProblem(410, 'message_deleted');
    }
    final key = '$messageId|$emoji';
    final who = _reactions.putIfAbsent(key, () => {});
    if (add ? !who.add(principalId) : !who.remove(principalId)) return;
    m['reactions'] = [
      for (final e in _reactions.entries)
        if (e.key.startsWith('$messageId|') && e.value.isNotEmpty)
          {
            'emoji': e.key.substring(messageId.length + 1),
            'count': e.value.length,
          },
    ];
    // Reactions never raise the message version (04 9.3); the payload carries the count.
    append(
      add ? 'reaction.added' : 'reaction.removed',
      resource: {'kind': 'message', 'id': messageId},
      scope: _scopeOf(m['thread_id']! as String),
      payload: {
        'message_id': messageId,
        'thread_id': m['thread_id'],
        'emoji': emoji,
        'principal_id': principalId,
        'count': who.length,
      },
      actor: principalId,
    );
  }

  /// Sets `trigger_state` of [messageId] and appends `message.updated` (04 16.5).
  void setTriggerState(String messageId, String state, {String? actor}) {
    final m = messages[messageId]!;
    m
      ..['trigger_state'] = state
      ..['trigger_state_at'] = stamp()
      ..['version'] = (m['version']! as int) + 1;
    _messageUpdated(m, actor);
  }

  /// Whether [createMessage] resolves `@handle` tokens of the content into `mention_ids`
  /// when the body carries none, as the server does (03 content format item 4); off by
  /// default, so tests of earlier tickets keep their messages.
  bool resolveMentions = false;

  List<String> _handlesIn(String text, String channelId) {
    final handles = {
      for (final m in RegExp(r'@([a-z0-9_.-]+)').allMatches(text.toLowerCase()))
        m.group(1)!,
    };
    return [
      for (final p in people.values)
        if (handles.contains(p.handle.toLowerCase()) &&
            seesChannel(p.id, channelId))
          p.id,
    ];
  }

  /// Merges [fields] into [messageId], raises its version and appends `message.updated`
  /// (the anchor's `run_id` and the components of a card, M4-21).
  JsonMap updateMessageFields(
    String messageId,
    JsonMap fields, {
    String? actor,
  }) {
    final m = messages[messageId]!;
    m
      ..addAll(fields)
      ..['version'] = (m['version']! as int) + 1;
    _messageUpdated(m, actor);
    return m;
  }

  void _messageUpdated(JsonMap m, String? actor) => append(
    'message.updated',
    resource: {'kind': 'message', 'id': m['id'], 'version': m['version']},
    scope: _scopeOf(m['thread_id']! as String),
    payload: {'message': m},
    actor: actor,
  );

  JsonMap _scopeOf(String threadId) {
    final channelId = threads[threadId]!['channel_id']! as String;
    return {
      'space_id': ?channels[channelId]!.spaceId,
      'channel_id': channelId,
      'thread_id': threadId,
    };
  }

  // -------------------------------------------------------------------------------------------
  // Personal state

  /// Sets the read state of [principalId] in [threadId] and appends `read_state.updated`.
  JsonMap putReadState(
    String principalId,
    String threadId,
    int lastRead, {
    String? deviceId,
  }) {
    final thread = visibleThread(principalId, threadId);
    final key = '$principalId|$threadId';
    final previous = readStates[key];
    final state = <String, Object?>{
      'thread_id': threadId,
      'channel_id': thread['channel_id'],
      'last_read_seq': lastRead,
      'mention_count': _mentions(
        principalId,
        _unread(principalId, threadId, lastRead),
      ),
      'version': ((previous?['version'] as int?) ?? 0) + 1,
      'updated_by_device_id': ?deviceId,
      'updated_at': stamp(),
    };
    readStates[key] = state;
    append(
      'read_state.updated',
      resource: {'kind': 'thread', 'id': threadId, 'version': state['version']},
      scope: _scopeOf(threadId),
      payload: {'read_state': state},
      actor: principalId,
      audience: principalId,
    );
    return state;
  }

  /// Stores the draft of [principalId] in [threadId] from a `DraftPut` [body]; returns the
  /// `DraftPutResult` (`overwrote_newer` as [_overwroteNewer], 04 14).
  JsonMap putDraft(
    String principalId,
    String threadId,
    JsonMap body, {
    String? deviceId,
  }) {
    final thread = visibleThread(principalId, threadId);
    final key = '$principalId|$threadId';
    final previous = drafts[key];
    final base = body['base_version'] as int?;
    final previousVersion = (previous?['version'] as int?) ?? 0;
    final draft = <String, Object?>{
      'id': previous?['id'] ?? nextId(),
      'thread_id': threadId,
      'channel_id': thread['channel_id'],
      'content_md': body['content_md'],
      'reply_to_message_id': ?body['reply_to_message_id'],
      'file_ids': body['file_ids'] ?? <Object?>[],
      'version': previousVersion + 1,
      'updated_by_device_id': ?deviceId,
      'updated_at': stamp(),
    };
    drafts[key] = draft;
    append(
      'draft.updated',
      resource: {
        'kind': 'draft',
        'id': draft['id'],
        'version': draft['version'],
      },
      scope: _scopeOf(threadId),
      payload: {'draft': draft},
      actor: principalId,
      audience: principalId,
    );
    return {
      'draft': draft,
      'overwrote_newer': _overwroteNewer(previous, base, deviceId, body),
    };
  }

  /// `overwrote_newer` of a draft write as the server decides it (04 14, addendum of
  /// 2026-10-29): a stored draft [previous] whose version the writer has not seen ([base]
  /// absent or older), written by another device than [deviceId], with other text or files
  /// than the write [body]; an empty write never reports it.
  static bool _overwroteNewer(
    JsonMap? previous,
    int? base,
    String? deviceId,
    JsonMap body,
  ) {
    if (previous == null) return false;
    final content = body['content_md'] as String? ?? '';
    final files = (body['file_ids'] as List<Object?>?) ?? const <Object?>[];
    if (content.trim().isEmpty && files.isEmpty) return false;
    if (base != null && base >= (previous['version']! as int)) return false;
    final device = previous['updated_by_device_id'];
    if (deviceId != null && device == deviceId) return false;
    final previousFiles =
        (previous['file_ids'] as List<Object?>?) ?? const <Object?>[];
    return previous['content_md'] != content ||
        previousFiles.length != files.length ||
        [for (var i = 0; i < files.length; i++) previousFiles[i] == files[i]]
            .contains(false);
  }

  /// Deletes the draft of [principalId] in [threadId]; nothing happens without one.
  void deleteDraft(String principalId, String threadId) {
    visibleThread(principalId, threadId);
    final draft = drafts.remove('$principalId|$threadId');
    if (draft == null) return;
    append(
      'draft.deleted',
      resource: {'kind': 'draft', 'id': draft['id']},
      scope: _scopeOf(threadId),
      payload: {
        'draft_id': draft['id'],
        'channel_id': draft['channel_id'],
        'thread_id': threadId,
      },
      actor: principalId,
      audience: principalId,
    );
  }

  /// Stores the draft of a new post of [principalId] in the forum [channelId] from a
  /// `DraftPut` [body] (`putChannelDraft`, 04 14, 03 table drafts with `thread_id` NULL) and
  /// returns the `DraftPutResult`. The version and `overwrote_newer` follow [putDraft]; a draft
  /// without content (only white space) and without `file_ids` deletes the row like
  /// [deleteChannelDraft] and answers the empty final state with the last version plus one (1
  /// without a row), as the server's `putDraft` does. [deviceId] is the writing device.
  ///
  /// Throws [FakeProblem]: `404` for a channel the principal does not see, `403 forbidden`
  /// below `write` (as [createThread]), `400 validation_failed` for a channel that is not a
  /// forum (the server's `NotAForum`) and for a `reply_to_message_id`.
  JsonMap putChannelDraft(
    String principalId,
    String channelId,
    JsonMap body, {
    String? deviceId,
  }) {
    _draftForum(principalId, channelId);
    if (body['reply_to_message_id'] != null) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/reply_to_message_id', 'code': 'not_allowed'},
        ],
      );
    }
    final key = '$principalId|channel:$channelId';
    final previous = drafts[key];
    final base = body['base_version'] as int?;
    final previousVersion = (previous?['version'] as int?) ?? 0;
    final content = body['content_md'] as String? ?? '';
    final files = body['file_ids'] as List<Object?>? ?? <Object?>[];
    final empty = content.trim().isEmpty && files.isEmpty;
    final draft = <String, Object?>{
      'id': previous?['id'] ?? nextId(),
      'channel_id': channelId,
      'content_md': empty ? '' : content,
      'file_ids': empty ? <Object?>[] : files,
      'version': previousVersion + 1,
      'updated_by_device_id': ?deviceId,
      'updated_at': stamp(),
    };
    if (empty) {
      deleteChannelDraft(principalId, channelId);
    } else {
      drafts[key] = draft;
      append(
        'draft.updated',
        resource: {
          'kind': 'draft',
          'id': draft['id'],
          'version': draft['version'],
        },
        scope: _channelScopeOf(channelId),
        payload: {'draft': draft},
        actor: principalId,
        audience: principalId,
      );
    }
    return {
      'draft': draft,
      'overwrote_newer': _overwroteNewer(previous, base, deviceId, body),
    };
  }

  /// Deletes the draft of a new post of [principalId] in the forum [channelId]
  /// (`deleteChannelDraft`) and appends `draft.deleted` without `thread_id`; nothing happens
  /// without one. Throws [FakeProblem] as [putChannelDraft] for the channel.
  void deleteChannelDraft(String principalId, String channelId) {
    _draftForum(principalId, channelId);
    _dropChannelDraft(principalId, channelId);
  }

  /// Checks that [principalId] may keep a draft of a new post in [channelId]: a forum it sees
  /// with at least `write`. Throws [FakeProblem] as [putChannelDraft].
  void _draftForum(String principalId, String channelId) {
    final channel = channels[channelId];
    if (channel == null || !seesChannel(principalId, channelId)) {
      throw const FakeProblem(404, 'not_found');
    }
    if (channel.access[principalId] == 'read') {
      throw const FakeProblem(403, 'forbidden');
    }
    if (channel.json['kind'] != 'forum') {
      throw const FakeProblem(400, 'validation_failed');
    }
  }

  /// Removes the channel draft of [principalId] in [channelId] and appends `draft.deleted`
  /// when one existed (the server's `deleteDraft` and `OnMessagePosted` with `CreatedThread`).
  void _dropChannelDraft(String principalId, String channelId) {
    final draft = drafts.remove('$principalId|channel:$channelId');
    if (draft == null) return;
    append(
      'draft.deleted',
      resource: {'kind': 'draft', 'id': draft['id']},
      scope: _channelScopeOf(channelId),
      payload: {'draft_id': draft['id'], 'channel_id': channelId},
      actor: principalId,
      audience: principalId,
    );
  }

  JsonMap _channelScopeOf(String channelId) => {
    'space_id': ?channels[channelId]!.spaceId,
    'channel_id': channelId,
  };

  /// Stores the setting [key] of [principalId] and appends `settings.updated`.
  JsonMap putSetting(
    String principalId,
    String key,
    Object? value, {
    String? deviceId,
  }) {
    final mine = settings.putIfAbsent(principalId, () => {});
    final setting = <String, Object?>{
      'key': key,
      'value': value,
      'version': ((mine[key]?['version'] as int?) ?? 0) + 1,
      'updated_by_device_id': ?deviceId,
      'updated_at': stamp(),
    };
    mine[key] = setting;
    _settingEvent(principalId, setting, deleted: false);
    return setting;
  }

  /// Deletes the setting [key] of [principalId]; `404` when it does not exist.
  void deleteSetting(String principalId, String key) {
    final setting = settings[principalId]?.remove(key);
    if (setting == null) throw const FakeProblem(404, 'not_found');
    setting['version'] = (setting['version']! as int) + 1;
    _settingEvent(principalId, setting, deleted: true);
  }

  void _settingEvent(
    String principalId,
    JsonMap setting, {
    required bool deleted,
  }) => append(
    'settings.updated',
    resource: {
      'kind': 'user',
      'id': principalId,
      'version': setting['version'],
    },
    payload: {'setting': setting, if (deleted) 'deleted': true},
    actor: principalId,
    audience: principalId,
  );

  /// The sidebar states of [principalId] for the spaces it sees (API `SidebarState`).
  List<JsonMap> sidebarOf(String principalId) => [
    for (final MapEntry(:key, :value) in sidebarStates.entries)
      if (key.startsWith('$principalId|') &&
          seesSpace(principalId, value['space_id']! as String))
        value,
  ];

  /// Stores the sidebar state of [principalId] for [spaceId] from a `SidebarStatePut` [body]
  /// and appends the personal `sidebar.updated` (04 14: last write wins per space).
  ///
  /// An absent `sort_key` keeps the stored one, `use_global_order` removes it. Throws
  /// [FakeProblem]: `404` for a space the principal does not see, `400 validation_failed` for
  /// a `sort_key` outside the fractional index (`/sort_key`, `invalid`, as the server's
  /// `fracindex` check of M1-52) or together with `use_global_order`.
  JsonMap putSidebar(
    String principalId,
    String spaceId,
    JsonMap body, {
    String? deviceId,
  }) {
    if (!spaces.containsKey(spaceId) || !seesSpace(principalId, spaceId)) {
      throw const FakeProblem(404, 'not_found');
    }
    final key = body['sort_key'] as String?;
    final global = body['use_global_order'] == true;
    if (key != null && global) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/use_global_order', 'code': 'not_allowed'},
        ],
      );
    }
    if (key != null && !FractionalIndex.isValid(key)) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/sort_key', 'code': 'invalid'},
        ],
      );
    }
    final slot = '$principalId|$spaceId';
    final previous = sidebarStates[slot];
    final sortKey = global ? null : key ?? previous?['sort_key'];
    final state = <String, Object?>{
      'space_id': spaceId,
      'collapsed': body['collapsed'] == true,
      'sort_key': ?sortKey,
      'version': ((previous?['version'] as int?) ?? 0) + 1,
      'updated_by_device_id': ?deviceId,
    };
    sidebarStates[slot] = state;
    append(
      'sidebar.updated',
      resource: {'kind': 'space', 'id': spaceId},
      scope: {'space_id': spaceId},
      payload: {'state': state},
      actor: principalId,
      audience: principalId,
    );
    return state;
  }

  /// Creates a forum post of [principalId] in [channelId] from a `ThreadCreate` [body]: the
  /// thread, `thread.created`, then its first message through [createMessage]; returns the
  /// API `ThreadCreated`.
  ///
  /// Throws [FakeProblem]: `404` for a channel the principal does not see, `403 forbidden`
  /// below `write`, `400 validation_failed` for a forum post without `first_message`
  /// (`required_for_forum_post`, as the server of M1-56), with a tag that is not a tag of the
  /// forum, and for a thread from a message (`parent_message_id`), which the fake does not
  /// model. A post without a title gets `title_source = auto`; `first_message.clear_draft`
  /// deletes the principal's channel draft of the forum with `draft.deleted`.
  JsonMap createThread(String principalId, String channelId, JsonMap body) {
    final channel = channels[channelId];
    if (channel == null || !seesChannel(principalId, channelId)) {
      throw const FakeProblem(404, 'not_found');
    }
    if (channel.access[principalId] == 'read') {
      throw const FakeProblem(403, 'forbidden');
    }
    FakeProblem invalid(String pointer, String code) => FakeProblem(
      400,
      'validation_failed',
      errors: [
        {'pointer': pointer, 'code': code},
      ],
    );
    if (channel.json['kind'] != 'forum' || body['parent_message_id'] != null) {
      throw invalid('/parent_message_id', 'not_supported_by_fake');
    }
    final first = body['first_message'] as JsonMap?;
    if (first == null) {
      throw invalid('/first_message', 'required_for_forum_post');
    }
    // An empty title is allowed (the server's CheckThreadTitle); the post then waits for its
    // auto title (`title_source = auto`).
    final title = (body['title'] as String?)?.trim() ?? '';
    final tagIds = [...?(body['tag_ids'] as List<Object?>?)?.cast<String>()];
    for (final tag in tagIds) {
      if (forumTags[tag]?['channel_id'] != channelId) {
        throw invalid('/tag_ids', 'unknown_tag');
      }
    }
    final id = addThread(channelId, title, createdBy: principalId);
    final thread = threads[id]!
      ..['tag_ids'] = tagIds
      ..['title_source'] = title.isEmpty ? 'auto' : 'manual';
    append(
      'thread.created',
      resource: {'kind': 'thread', 'id': id, 'version': thread['version']},
      scope: _scopeOf(id),
      payload: {'thread': thread},
      actor: principalId,
    );
    // `clear_draft` of the first message clears the channel draft of the forum, not a draft of
    // the new thread (the server's OnMessagePosted with CreatedThread).
    final message = createMessage(
      principalId,
      id,
      {...first}..remove('clear_draft'),
    ).message;
    if (first['clear_draft'] == true) _dropChannelDraft(principalId, channelId);
    return {'thread': thread, 'first_message': message};
  }

  /// Marks [threadId] done (`thread.close`) and appends `thread.closed`.
  void closeThread(String threadId, {String? actor}) {
    final thread = threads[threadId]!;
    thread
      ..['state'] = 'closed'
      ..['closed_at'] = stamp()
      ..['version'] = (thread['version']! as int) + 1;
    if (actor != null) thread['closed_by'] = actor;
    append(
      'thread.closed',
      resource: {
        'kind': 'thread',
        'id': threadId,
        'version': thread['version'],
      },
      scope: _scopeOf(threadId),
      payload: {'thread': thread},
      actor: actor,
    );
  }

  /// Writes the system line [code] (`kind = system`, `origin = system`) into [threadId] with
  /// [authorId] as its author and appends `message.created` (03: the author of
  /// `thread.reopened` is the person whose message reopened the thread).
  void _systemLine(String threadId, String code, String authorId) {
    final thread = threads[threadId]!;
    final seq = (thread['last_message_seq']! as int) + 1;
    final id = nextId();
    final message = <String, Object?>{
      'id': id,
      'thread_id': threadId,
      'channel_id': thread['channel_id'],
      'thread_seq': seq,
      'author': people[authorId]!.ref,
      'client_msg_id': nextId(),
      'origin': 'system',
      'kind': 'system',
      'system_code': code,
      'content_md': '',
      'content_format': 1,
      'silent': true,
      'attachments': <Object?>[],
      'mention_ids': <Object?>[],
      'reactions': <Object?>[],
      'pinned': false,
      'components': <Object?>[],
      'version': 1,
      'created_at': stamp(),
    };
    messages[id] = message;
    _threadMessages[threadId]!.add(id);
    thread
      ..['last_message_seq'] = seq
      ..['message_count'] = (thread['message_count']! as int) + 1;
    append(
      'message.created',
      resource: {'kind': 'message', 'id': id, 'version': 1},
      scope: _scopeOf(threadId),
      payload: {'message': message},
    );
  }

  /// Opens the closed [threadId] again and appends `thread.reopened` (01 6.1: a new message
  /// reopens a done thread).
  void _reopen(String threadId, String actor) {
    final thread = threads[threadId]!
      ..['state'] = 'open'
      ..remove('closed_at')
      ..remove('closed_by');
    thread['version'] = (thread['version']! as int) + 1;
    append(
      'thread.reopened',
      resource: {
        'kind': 'thread',
        'id': threadId,
        'version': thread['version'],
      },
      scope: _scopeOf(threadId),
      payload: {'thread': thread},
      actor: actor,
    );
  }

  // -------------------------------------------------------------------------------------------
  // Access

  /// Gives [principalId] [level] in [channelId], or removes its access with a null [level],
  /// and appends the personal `access.changed` of 04 8.2 with the new permission version.
  ///
  /// A removed channel that was the principal's last one in its space also removes the space;
  /// a first channel in a space also adds the space. Nothing happens when nothing changes.
  void setAccess(String principalId, String channelId, String? level) {
    final channel = channels[channelId]!;
    final previous = channel.access[principalId];
    if (previous == level) return;
    final space = channel.spaceId;
    final sawSpace = space != null && seesSpace(principalId, space);
    if (level == null) {
      channel.access.remove(principalId);
    } else {
      channel.access[principalId] = level;
    }
    final seesSpaceNow = space != null && seesSpace(principalId, space);
    permissionVersion++;
    append(
      'access.changed',
      resource: {'kind': 'principal', 'id': principalId},
      payload: {
        'permission_version': permissionVersion,
        'added': [
          if (previous == null)
            {'kind': 'channel', 'id': channelId, 'level': level},
          if (!sawSpace && seesSpaceNow)
            {'kind': 'space', 'id': space, 'level': level},
        ],
        'removed': [
          if (level == null) {'kind': 'channel', 'id': channelId},
          if (sawSpace && !seesSpaceNow) {'kind': 'space', 'id': space},
        ],
        'changed': [
          if (previous != null && level != null)
            {
              'kind': 'channel',
              'id': channelId,
              'level': level,
              'previous_level': previous,
            },
        ],
      },
      audience: principalId,
    );
  }

  // -------------------------------------------------------------------------------------------
  // Runs

  static const Set<String> _endStates = {'completed', 'failed', 'cancelled'};

  bool _active(JsonMap run) => !_endStates.contains(run['state']);

  /// The active run of [threadId], or null.
  JsonMap? activeRunOf(String threadId) {
    for (final r in runs.values) {
      if (r['thread_id'] == threadId && _active(r)) return r;
    }
    return null;
  }

  /// Creates a queued run of [agentId] for [triggeringMessageId] and appends
  /// `run.state_changed`; returns the run document.
  JsonMap createRun({
    required String agentId,
    required String triggeringMessageId,
    required String triggeredBy,
  }) {
    final m = messages[triggeringMessageId]!;
    final id = nextId();
    final run = <String, Object?>{
      'id': id,
      'thread_id': m['thread_id'],
      'channel_id': m['channel_id'],
      'agent_id': agentId,
      'trigger_kind': 'message',
      'triggering_message_id': triggeringMessageId,
      'triggered_by': triggeredBy,
      'state': 'queued',
      'version': 1,
      'billing_mode_expected': 'organization',
      'metrics': <String, Object?>{},
      'cost': {'amount_micros': 0, 'currency': 'EUR', 'is_estimate': true},
      'steps': <String, Object?>{},
      'followups': <String, Object?>{},
      'queued_at': stamp(),
    };
    runs[id] = run;
    _runStateEvent(run, triggerState: m['trigger_state'] as String?);
    return run;
  }

  /// Moves [runId] to [state] (`running`, `completed`, `failed`, `cancelled`) and appends
  /// `run.state_changed` with the summary (04 11.3 item 1).
  void setRunState(
    String runId,
    String state, {
    String? triggerState,
    String? reason,
  }) {
    final run = runs[runId]!;
    run
      ..['state'] = state
      ..['version'] = (run['version']! as int) + 1;
    if (reason != null) run['state_reason'] = reason;
    if (state == 'running') run['started_at'] = stamp();
    if (_endStates.contains(state)) run['finished_at'] = stamp();
    _runStateEvent(run, triggerState: triggerState);
  }

  void _runStateEvent(JsonMap run, {String? triggerState}) => append(
    'run.state_changed',
    resource: {'kind': 'run', 'id': run['id'], 'version': run['version']},
    scope: _scopeOf(run['thread_id']! as String),
    payload: {
      'run': runSummary(run),
      'triggering_message_id': ?run['triggering_message_id'],
      'trigger_state': ?triggerState,
    },
  );

  /// The API `RunSummary` of [run].
  JsonMap runSummary(JsonMap run) => {
    for (final k in const [
      'id',
      'thread_id',
      'channel_id',
      'agent_id',
      'triggered_by',
      'state',
      'state_reason',
      'version',
      'billing_mode_expected',
      'billing_mode_used',
      'status_line',
      'summary',
      'cost',
      'duration_ms',
      'queued_at',
      'started_at',
      'finished_at',
      'stall_warned_at',
    ])
      if (run[k] != null) k: run[k],
    'step_count': (run['steps']! as JsonMap).length,
  };

  /// Applies the RFC 6902 [ops] (`add`, `replace`, `remove`) to the document of [runId],
  /// raises its version and returns the previous version (04 11.2).
  int patchRun(String runId, List<JsonMap> ops) {
    final run = runs[runId]!;
    final base = run['version']! as int;
    for (final op in ops) {
      _applyOp(run, op);
    }
    run['version'] = base + 1;
    for (final hook in List.of(runPatchHooks)) {
      hook(runId, base, base + 1, [for (final op in ops) _copy(op)]);
    }
    return base;
  }

  static void _applyOp(JsonMap doc, JsonMap op) {
    final parts = (op['path']! as String)
        .split('/')
        .skip(1)
        .map((p) => p.replaceAll('~1', '/').replaceAll('~0', '~'))
        .toList();
    Object? node = doc;
    for (final p in parts.take(parts.length - 1)) {
      node = node is List<Object?> ? node[int.parse(p)] : (node! as JsonMap)[p];
    }
    final last = parts.last;
    final kind = op['op'];
    if (node is List<Object?>) {
      final index = last == '-' ? node.length : int.parse(last);
      switch (kind) {
        case 'add':
          node.insert(index, op['value']);
        case 'replace':
          node[index] = op['value'];
        case 'remove':
          node.removeAt(index);
      }
      return;
    }
    final map = node! as JsonMap;
    if (kind == 'remove') {
      map.remove(last);
    } else {
      map[last] = op['value'];
    }
  }

  // -------------------------------------------------------------------------------------------
  // Devices and sessions

  /// Revokes [sessionId] with [reason] and appends the personal `session.revoked` (05 6).
  void revokeSession(String sessionId, {String reason = 'admin'}) {
    final s = sessions[sessionId];
    if (s == null || s.revoked) return;
    s.revoked = true;
    append(
      'session.revoked',
      resource: {'kind': 'auth_session', 'id': sessionId},
      payload: {'session_id': sessionId, 'reason': reason},
      audience: s.userId,
    );
    // Revoking an app session revokes the child sessions of its extensions (08 7.4).
    for (final child
        in sessions.values.where((c) => c.parentId == sessionId).toList()) {
      revokeSession(child.id, reason: reason);
    }
  }

  /// Revokes [deviceId] with [reason] (`revoked_by_owner`, `verification_failed`,
  /// `pending_expired`, `pairing_expired`) and every session on it.
  void revokeDevice(String deviceId, {String reason = 'revoked_by_owner'}) {
    final d = devices[deviceId]!;
    d
      ..state = 'revoked'
      ..revokeReason = reason
      ..revokedAt = stamp();
    for (final s
        in sessions.values.where((s) => s.deviceId == deviceId).toList()) {
      revokeSession(s.id, reason: 'device_revoked');
    }
  }

  /// Approves the waiting device [deviceId] (08 5.2).
  void approveDevice(String deviceId) {
    devices[deviceId]!
      ..state = 'active'
      ..approvedAt = stamp();
  }

  /// Lets the pending device [deviceId] expire (`auth.device_pending_ttl`, 08 5.2).
  void expirePendingDevice(String deviceId) =>
      revokeDevice(deviceId, reason: 'pending_expired');

  /// Rejects the pending device [deviceId] as a wrong verification code does.
  void rejectPendingDevice(String deviceId) =>
      revokeDevice(deviceId, reason: 'verification_failed');

  /// Disables the person [userId] as 08 11 does: status `disabled`, every session revoked
  /// (`user_disabled`), every device revoked (`user_disabled`).
  void disablePerson(String userId) {
    people[userId]!.status = 'disabled';
    for (final d in devices.values.where((d) => d.userId == userId).toList()) {
      if (d.state != 'revoked') revokeDevice(d.id, reason: 'user_disabled');
    }
    for (final s in sessions.values.where((s) => s.userId == userId).toList()) {
      revokeSession(s.id, reason: 'user_disabled');
    }
  }

  /// Adds a passkey named [name] to [userId] and returns its ID.
  String addPasskey(String userId, {required String name}) {
    final id = nextId();
    people[userId]!.passkeys[id] = {
      'id': id,
      'name': name,
      'backup_eligible': true,
      'created_at': stamp(),
    };
    return id;
  }

  /// Links an OIDC identity with [subject] to [userId] and returns its ID.
  String addIdentity(String userId, {required String subject}) {
    final id = nextId();
    people[userId]!.identities[id] = {
      'id': id,
      'provider_id': localProviderId,
      'provider_kind': 'oidc',
      'subject': subject,
      'issuer': 'https://login.example.com',
      'linked_at': stamp(),
    };
    return id;
  }

  /// Lets the pairing [pairingId] expire (`auth.pairing_code_ttl`, 08 5.3).
  void expirePairing(String pairingId) {
    final p = pairings[pairingId]!;
    p
      ..state = 'expired'
      ..expiresAt = stamp();
  }

  // -------------------------------------------------------------------------------------------
  // Agents, operators and I1 (brief M4-28)

  /// The agent rows by principal ID. An agent person added with [addPerson] alone has none
  /// until [agentOf] creates it with the defaults (organization), so the worlds of earlier
  /// tickets keep working.
  final Map<String, FakeAgent> agents = {};

  /// Adds an agent person (through [addPerson], kind `agent`) and its [FakeAgent] with the
  /// given configured and reported values, as if its first heartbeat had just arrived; returns
  /// the [FakeAgent].
  ///
  /// [credentialHolderId] is stored as given, so a test can build a world that already breaks
  /// I1 (10 5.4) and repair it through the API.
  FakeAgent addAgent({
    required String handle,
    required String displayName,
    String billingMode = 'organization',
    String? credentialHolderId,
    String? reportedBillingMode,
    List<String>? billingModesSupported = const ['organization', 'personal'],
    String adapterKind = 'hermes',
    String state = 'online',
    String keyType = 'api_key',
  }) {
    final p = addPerson(
      handle: handle,
      displayName: displayName,
      kind: 'agent',
    );
    final a = FakeAgent(
      id: p.id,
      billingMode: billingMode,
      credentialHolderId: credentialHolderId,
      reportedBillingMode: reportedBillingMode,
      billingModesSupported: billingModesSupported == null
          ? null
          : List.of(billingModesSupported),
      adapterKind: adapterKind,
      state: state,
      keyType: keyType,
    )..lastHeartbeatAt = stamp();
    a.mismatchAtHeartbeat =
        reportedBillingMode != null && reportedBillingMode != billingMode;
    agents[a.id] = a;
    return a;
  }

  /// The [FakeAgent] of [agentId]: the stored one, or a new one with the defaults for an agent
  /// person without a row; null for an unknown ID or a principal that is not an agent.
  FakeAgent? agentOf(String agentId) {
    final p = people[agentId];
    if (p == null || p.kind != 'agent') return null;
    return agents.putIfAbsent(agentId, () => FakeAgent(id: agentId));
  }

  /// The `operator` of a channel in [spaceId] that inherits: the default operator of the space
  /// as `effective_agent_id`, absent without one (API `ChannelOperator`).
  JsonMap _inheritOperator(String spaceId) => {
    'mode': 'inherit',
    'effective_agent_id': ?spaces[spaceId]?['default_operator_agent_id'],
  };

  /// Makes [agentId] the default operator of [spaceId] (`default_operator_agent_id`), or
  /// removes it for null, and refreshes the `operator.effective_agent_id` of the space's
  /// channels that inherit (03 `channels`, 10 5.2 point 1).
  void setSpaceOperator(String spaceId, String? agentId) {
    final space = spaces[spaceId]!;
    if (agentId == null) {
      space.remove('default_operator_agent_id');
    } else {
      space['default_operator_agent_id'] = agentId;
    }
    for (final c in channels.values) {
      final op = c.json['operator'] as JsonMap?;
      if (c.spaceId == spaceId && op?['mode'] == 'inherit') {
        c.json['operator'] = _inheritOperator(spaceId);
      }
    }
  }

  /// Makes [agentId] the fixed operator of [channelId] (`mode: agent`), or lets the channel
  /// inherit the default of its space again for null.
  void setChannelOperator(String channelId, String? agentId) {
    final c = channels[channelId]!;
    c.json['operator'] = agentId == null
        ? _inheritOperator(c.spaceId!)
        : {'mode': 'agent', 'agent_id': agentId, 'effective_agent_id': agentId};
  }

  /// The fake's heartbeat of [agentId] (09 4.8): changes the reported values that are given
  /// and stamps `last_heartbeat_at`.
  ///
  /// A null [reportedBillingMode] leaves the reported mode as it is; [clearReported] removes it
  /// (an adapter that follows the configured mode per run, 10 8).
  void agentHeartbeat(
    String agentId, {
    String? reportedBillingMode,
    List<String>? billingModesSupported,
    String? keyType,
    String? state,
    bool clearReported = false,
  }) {
    final a = agentOf(agentId)!;
    if (clearReported) a.reportedBillingMode = null;
    if (reportedBillingMode != null) {
      a.reportedBillingMode = reportedBillingMode;
    }
    if (billingModesSupported != null) {
      a.billingModesSupported = List.of(billingModesSupported);
    }
    if (keyType != null) a.keyType = keyType;
    if (state != null) a.state = state;
    a.mismatchAtHeartbeat =
        a.reportedBillingMode != null && a.reportedBillingMode != a.billingMode;
    a.lastHeartbeatAt = stamp();
  }

  /// The codes of `AgentRuntimeStatus.warnings` for [a] as the server computes them at each
  /// heartbeat (10 8, 09 4.8), only while the runtime is `online` or `degraded`.
  ///
  /// The fake counts `billing.mismatch_alert_after` as one heartbeat: `billing_mode_mismatch`
  /// shows once a heartbeat saw the reported mode differ from the configured one
  /// ([FakeAgent.mismatchAtHeartbeat]); right after a switch only `billing_mode_matches` is
  /// false, as on the server. There is no `tool_class_mismatch`, because the fake has
  /// no reported tool class.
  List<String> agentWarnings(FakeAgent a) {
    if (a.state != 'online' && a.state != 'degraded') return const [];
    final reported = a.reportedBillingMode;
    final supported = a.billingModesSupported;
    return [
      if (a.mismatchAtHeartbeat &&
          reported != null &&
          reported != a.billingMode)
        'billing_mode_mismatch',
      if (reported == null &&
          supported != null &&
          !supported.contains(a.billingMode))
        'billing_mode_unsupported',
      if (a.keyType == 'unknown') 'billing_evidence_unknown',
    ];
  }

  /// The API `Agent` of [a]: its person's handle, name and status, the configured policy
  /// (`credential_holder_id` only with `personal`) and the presence (`AgentRuntimeStatus`) with
  /// the reported values, `billing_evidence`, `billing_mode_matches` (only with a reported mode)
  /// and the [agentWarnings].
  JsonMap agentJson(FakeAgent a) {
    final p = people[a.id]!;
    final reported = a.reportedBillingMode;
    return {
      'id': a.id,
      'handle': p.handle,
      'display_name': p.displayName,
      'status': switch (p.status) {
        'active' => 'active',
        'deleted' => 'deleted',
        _ => 'disabled',
      },
      'adapter_kind': a.adapterKind,
      'description': a.description,
      'billing_mode': a.billingMode,
      if (a.billingMode == 'personal')
        'credential_holder_id': ?a.credentialHolderId,
      'billing_mode_changed_at': ?a.billingModeChangedAt,
      'tool_class': a.toolClass,
      'presence': {
        'state': a.state,
        'reported_billing_mode': ?reported,
        'last_heartbeat_at': ?a.lastHeartbeatAt,
        if (a.billingModesSupported case final modes?)
          'billing_modes_supported': List.of(modes),
        if (reported != null) 'billing_mode_matches': reported == a.billingMode,
        'billing_evidence': {
          'provider': 'anthropic',
          'key_type': a.keyType,
          'method': ?a.evidenceMethod,
        },
        'warnings': agentWarnings(a),
      },
      'operator_config': <String, Object?>{},
      'version': a.version,
    };
  }

  /// The I1 trigger set of [agentId] (10 5.2 and 5.3, simplified): the active people who may
  /// trigger it, in the order of [people].
  ///
  /// Targets are the channels whose `operator.effective_agent_id` is the agent and every
  /// channel of a space whose default operator it is; their spaces count for space grants.
  /// Triggerers are the active owners, the people with `trigger` in the access list of a target
  /// channel, and the active members (membership not expired) of groups and the people that
  /// hold a valid grant (not revoked, not expired) with `trigger` on a target channel, its
  /// space or the instance. The fake has no thread operators (10 5.2 point 2).
  List<String> agentTriggerers(String agentId) {
    final targets = [
      for (final c in channels.values)
        if ((c.json['operator'] as JsonMap?)?['effective_agent_id'] ==
                agentId ||
            spaces[c.spaceId]?['default_operator_agent_id'] == agentId)
          c,
    ];
    final targetIds = {for (final c in targets) c.id};
    final targetSpaces = {
      for (final c in targets) ?c.spaceId,
      for (final s in spaces.values)
        if (s['default_operator_agent_id'] == agentId) s['id']! as String,
    };
    final t = now();
    bool valid(Object? expiresAt) =>
        expiresAt == null || DateTime.parse(expiresAt as String).isAfter(t);
    final found = <String>{
      for (final p in people.values)
        if (p.isOwner) p.id,
      for (final c in targets)
        for (final MapEntry(:key, :value) in c.access.entries)
          if (value == 'trigger') key,
    };
    for (final g in grants.values) {
      final target = g['target_id'];
      final onTarget = switch (g['target_kind']) {
        'instance' => true,
        'space' => targetSpaces.contains(target),
        'channel' => targetIds.contains(target),
        _ => false,
      };
      if (g['revoked_at'] != null ||
          !valid(g['expires_at']) ||
          g['level'] != 'trigger' ||
          !onTarget) {
        continue;
      }
      final subject = g['subject_id']! as String;
      if (g['subject_kind'] == 'user') found.add(subject);
      if (g['subject_kind'] == 'group') {
        for (final MapEntry(:key, :value)
            in (groupMembers[subject] ?? {}).entries) {
          if (valid(value['expires_at'])) found.add(key);
        }
      }
    }
    return [
      for (final p in people.values)
        if (found.contains(p.id) && p.kind == 'user' && p.status == 'active')
          p.id,
    ];
  }

  /// The I1 conflicts of the world (10 5.4): for every active agent on `personal` whose
  /// trigger set ([agentTriggerers]) is not exactly its credential holder, the other
  /// principals of the set, by agent ID in the order of [agents].
  ///
  /// As on the server a missing holder, or a holder who cannot trigger the agent, is a
  /// conflict too, then with an empty list.
  Map<String, List<String>> i1Conflicts() {
    final out = <String, List<String>>{};
    for (final a in agents.values) {
      if (a.billingMode != 'personal' || people[a.id]?.status != 'active') {
        continue;
      }
      final set = agentTriggerers(a.id);
      final holder = a.credentialHolderId;
      if (holder != null && set.length == 1 && set.single == holder) continue;
      out[a.id] = [
        for (final p in set)
          if (p != holder) p,
      ];
    }
    return out;
  }
}
