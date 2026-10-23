// SPDX-License-Identifier: Apache-2.0

// The app and webhook calls of the fake repository (brief M6-08 Tests), behaving like the server
// of 11 4 to 8: `validateManifest` stores nothing, an install creates neither a grant nor a token
// (D-M2-24-8) but one subscription for a webhook delivery (D-M2-26-15), an upgrade keeps only
// scopes and events the new manifest still names (11 4.2), suspend blocks and uninstall revokes
// the incoming webhooks and disables the subscriptions (11 4.3), a subscription takes only
// granted events and https addresses whose host is allowed, and the address of a created
// incoming webhook holds its token once.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';

/// The app and webhook calls and whether 05 gives their action step-up (`app.install`,
/// `app.scopes.update`, `app.suspend`, `app.resume`, `app.uninstall`, `webhook_out.manage`,
/// `webhook_in.create`, `webhook_in.revoke`; `webhook_out.redeliver` has none).
const Map<String, bool> appsStepUpOf = {
  'installApp': true,
  'upgradeManifest': true,
  'setScopes': true,
  'suspendApp': true,
  'resumeApp': true,
  'uninstallApp': true,
  'createSubscription': true,
  'updateSubscription': true,
  'deleteSubscription': true,
  'redeliver': false,
  'createIncomingWebhook': true,
  'revokeIncomingWebhook': true,
};

/// The host the fake server allows for webhooks (`webhook.allowed_hosts`).
const String allowedHookHost = 'bridge.example.com';

/// The manifest of "Repo Bridge" (11 3.3) as an object; [version], [optional], [events] and
/// [grants] vary it for the update tests.
Map<String, Object?> repoBridgeManifest({
  String version = '1.2.0',
  List<String> required = const ['messages:write', 'commands:write'],
  List<String> optional = const ['messages:read', 'events:subscribe'],
  List<String> events = const ['message.created'],
  String eventsUrl = 'https://bridge.example.com/hooks/events',
  List<Map<String, Object?>> grants = const [
    {
      'target_kind': 'channel',
      'level': 'write',
      'capabilities': <String>[],
      'purpose': 'Create posts per ticket',
    },
  ],
}) => {
  'schema_version': 1,
  'slug': 'repo-bridge',
  'name': 'Repo Bridge',
  'description': 'Brings events from a code hosting service into a forum.',
  'publisher': 'Example Org',
  'version': version,
  'homepage_url': 'https://bridge.example.com',
  'min_api_version': '0.2',
  'scopes': {'required': required, 'optional': optional},
  'events': events,
  'delivery': {'mode': 'webhook', 'events_url': eventsUrl},
  'commands': [
    {'name': 'issue', 'description': 'Create an issue'},
  ],
  'components': ['button', 'form'],
  'requested_grants': grants,
  'data_egress':
      'Messages of the granted channels go to bridge.example.com if '
      'messages:read is granted.',
};

/// The app and webhook calls of the fake; [change] and [refused] are those of the repository.
mixin FakeApps {
  /// Records the change [op] and asks for the step-up as the server would.
  Future<void> change(String op, String args, String purpose);

  /// Throws the refusal of [op], if any.
  void refused(String op);

  /// The apps by ID.
  final Map<String, AppRecord> appsById = {};

  /// The subscriptions per app.
  final Map<String, List<AppSubscription>> subscriptionsByApp = {};

  /// The deliveries per subscription, newest first.
  final Map<String, List<AppDelivery>> deliveriesBySubscription = {};

  /// The active incoming webhooks per channel.
  final Map<String, List<IncomingWebhookInfo>> incomingByChannel = {};

  /// The answer of `validateManifest` for an upload, or null for valid.
  ManifestCheck? Function(ManifestUpload upload)? manifestCheck;

  /// The uploads `validateManifest` saw, in order.
  final List<ManifestUpload> validated = [];

  /// The secrets this fake handed out once (tokens of incoming webhooks, HMAC secrets).
  final List<String> handedOut = [];

  int _apps = 0;
  int _subs = 0;
  int _hooks = 0;

  AppRecord _withState(
    AppRecord a, {
    AppInstallState? state,
    AppManifestInfo? manifest,
    String? manifestVersion,
    List<String>? grantedScopes,
    List<String>? eventSubscriptions,
  }) => AppRecord(
    id: a.id,
    slug: a.slug,
    displayName: manifest?.name ?? a.displayName,
    manifestVersion: manifestVersion ?? a.manifestVersion,
    manifest: manifest ?? a.manifest,
    installationId: a.installationId,
    grantedScopes: grantedScopes ?? a.grantedScopes,
    eventSubscriptions: eventSubscriptions ?? a.eventSubscriptions,
    state: state ?? a.state,
    installedAt: a.installedAt,
    installedBy: a.installedBy,
  );

  AppSubscription _sub(
    AppSubscription s, {
    SubscriptionState? state,
    String? url,
    List<String>? eventTypes,
    DateTime? disabledAt,
  }) => AppSubscription(
    id: s.id,
    url: url ?? s.url,
    eventTypes: eventTypes ?? s.eventTypes,
    state: state ?? s.state,
    consecutiveFailures: s.consecutiveFailures,
    signatureSchemes: s.signatureSchemes,
    signingKeyKid: s.signingKeyKid,
    createdAt: s.createdAt,
    disabledAt: disabledAt ?? s.disabledAt,
  );

  /// Installs an app without the dialogs (the setup of a test).
  AppRecord seedApp(
    Map<String, Object?> manifest, {
    List<String>? grantedScopes,
    List<String> eventSubscriptions = const [],
    AppInstallState state = AppInstallState.active,
  }) {
    final info = AppManifestInfo.fromJson(manifest);
    final id = 'app-${info.slug}';
    final app = AppRecord(
      id: id,
      slug: info.slug,
      displayName: info.name,
      manifestVersion: info.version,
      manifest: info,
      installationId: 'inst-$id',
      grantedScopes: grantedScopes ?? info.requiredScopes,
      eventSubscriptions: eventSubscriptions,
      state: state,
      installedAt: DateTime.utc(2026, 10, 18, 9),
      installedBy: 'alex',
    );
    appsById[id] = app;
    return app;
  }

  /// Answers every app (`listApps`).
  Future<List<AppRecord>> apps() async {
    refused('apps');
    return [
      for (final a in appsById.values)
        if (a.state != AppInstallState.uninstalled) a,
    ];
  }

  /// Answers one app (`getApp`).
  Future<AppRecord> app(String appId) async {
    refused('app');
    return appsById[appId] ??
        (throw const SelfActionFailed(
          ApiProblem(status: 404, code: 'not_found'),
        ));
  }

  /// Checks a manifest; stores nothing.
  Future<ManifestCheck> validateManifest(ManifestUpload upload) async {
    refused('validateManifest');
    validated.add(upload);
    return manifestCheck?.call(upload) ?? const ManifestCheck(valid: true);
  }

  /// Installs; one subscription for a webhook delivery (D-M2-26-15).
  Future<AppRecord> installApp(
    ManifestUpload upload, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
    required String purpose,
  }) async {
    final info = upload.info;
    await change(
      'installApp',
      '${info.slug} ${grantedScopes.join(',')} ${eventSubscriptions.join(',')} '
          '$reason',
      purpose,
    );
    final id = 'app-${info.slug}-${_apps++}';
    final app = AppRecord(
      id: id,
      slug: info.slug,
      displayName: info.name,
      manifestVersion: info.version,
      manifest: info,
      installationId: 'inst-$id',
      grantedScopes: grantedScopes,
      eventSubscriptions: eventSubscriptions,
      state: AppInstallState.active,
      installedAt: DateTime.utc(2026, 10, 18, 9),
      installedBy: 'alex',
    );
    appsById[id] = app;
    final url = info.eventsUrl;
    if (info.deliveryMode == 'webhook' &&
        url != null &&
        eventSubscriptions.isNotEmpty) {
      subscriptionsByApp
          .putIfAbsent(id, () => [])
          .add(
            AppSubscription(
              id: 'sub-${_subs++}',
              url: url,
              eventTypes: eventSubscriptions,
              state: SubscriptionState.active,
              consecutiveFailures: 0,
              signatureSchemes: const ['v1a'],
              signingKeyKid: 'whk-2026-10',
              createdAt: DateTime.utc(2026, 10, 18, 9),
            ),
          );
    }
    return app;
  }

  /// Replaces the manifest; scopes and events the new one drops end at once (11 4.2).
  Future<AppRecord> upgradeManifest(
    String appId,
    ManifestUpload upload, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
    required String purpose,
  }) async {
    final info = upload.info;
    await change(
      'upgradeManifest',
      '$appId ${info.version} ${grantedScopes.join(',')} '
          '${eventSubscriptions.join(',')} $reason',
      purpose,
    );
    final app = appsById[appId]!;
    final named = {...info.requiredScopes, ...info.optionalScopes};
    return appsById[appId] = _withState(
      app,
      manifest: info,
      manifestVersion: info.version,
      grantedScopes: [
        for (final s in grantedScopes)
          if (named.contains(s) && app.grantedScopes.contains(s)) s,
      ],
      eventSubscriptions: [
        for (final e in eventSubscriptions)
          if (info.events.contains(e) && app.eventSubscriptions.contains(e)) e,
      ],
    );
  }

  /// The new consent (`app.scopes.update`).
  Future<AppRecord> setScopes(
    String appId, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
    required String purpose,
  }) async {
    await change(
      'setScopes',
      '$appId ${grantedScopes.join(',')} ${eventSubscriptions.join(',')} '
          '$reason',
      purpose,
    );
    return appsById[appId] = _withState(
      appsById[appId]!,
      grantedScopes: grantedScopes,
      eventSubscriptions: eventSubscriptions,
    );
  }

  /// Suspends: tokens blocked, deliveries paused (11 4.3).
  Future<void> suspendApp(String appId, {required String purpose}) async {
    await change('suspendApp', appId, purpose);
    appsById[appId] = _withState(
      appsById[appId]!,
      state: AppInstallState.suspended,
    );
  }

  /// Resumes.
  Future<void> resumeApp(String appId, {required String purpose}) async {
    await change('resumeApp', appId, purpose);
    appsById[appId] = _withState(
      appsById[appId]!,
      state: AppInstallState.active,
    );
  }

  /// Uninstalls: subscriptions disabled, incoming webhooks revoked (11 4.3).
  Future<void> uninstallApp(
    String appId, {
    required String reason,
    required String purpose,
  }) async {
    await change('uninstallApp', '$appId $reason', purpose);
    appsById[appId] = _withState(
      appsById[appId]!,
      state: AppInstallState.uninstalled,
    );
    final subs = subscriptionsByApp[appId] ?? [];
    for (final (i, s) in subs.indexed) {
      subs[i] = _sub(
        s,
        state: SubscriptionState.disabled,
        disabledAt: DateTime.utc(2026, 10, 18, 10),
      );
    }
    for (final list in incomingByChannel.values) {
      list.removeWhere((w) => w.appId == appId);
    }
  }

  /// The subscriptions of [appId].
  Future<List<AppSubscription>> subscriptions(String appId) async {
    refused('subscriptions');
    return List.of(subscriptionsByApp[appId] ?? const []);
  }

  void _checkSubscription(String appId, String url, List<String> events) {
    final app = appsById[appId]!;
    final uri = Uri.tryParse(url);
    if (uri == null || uri.scheme != 'https') {
      throw const SelfActionFailed(
        ApiProblem(
          status: 400,
          code: 'validation_failed',
          errors: [ProblemFieldError(pointer: '/url', code: 'invalid')],
        ),
      );
    }
    if (uri.host != allowedHookHost) {
      throw const SelfActionFailed(
        ApiProblem(status: 400, code: 'webhook_host_not_allowed'),
      );
    }
    for (final (i, e) in events.indexed) {
      if (!app.eventSubscriptions.contains(e)) {
        throw SelfActionFailed(
          ApiProblem(
            status: 400,
            code: 'validation_failed',
            errors: [
              ProblemFieldError(
                pointer: '/event_types/$i',
                code: 'not_granted',
              ),
            ],
          ),
        );
      }
    }
  }

  /// Creates a subscription; with HMAC the secret is in the answer only.
  Future<SubscriptionSaved> createSubscription(
    String appId, {
    required String url,
    required List<String> eventTypes,
    bool enableHmac = false,
    required String purpose,
  }) async {
    await change(
      'createSubscription',
      '$appId $url ${eventTypes.join(',')} hmac=$enableHmac',
      purpose,
    );
    _checkSubscription(appId, url, eventTypes);
    final n = _subs++;
    final sub = AppSubscription(
      id: 'sub-$n',
      url: url,
      eventTypes: eventTypes,
      state: SubscriptionState.active,
      consecutiveFailures: 0,
      signatureSchemes: ['v1a', if (enableHmac) 'v1'],
      signingKeyKid: 'whk-2026-10',
      createdAt: DateTime.utc(2026, 10, 18, 9, 30),
    );
    subscriptionsByApp.putIfAbsent(appId, () => []).add(sub);
    final secret = enableHmac ? 'whsec_EXAMPLE${n}HmacSecretShownOnce' : null;
    if (secret != null) handedOut.add(secret);
    return SubscriptionSaved(sub, hmacSecret: secret);
  }

  /// Changes a subscription.
  Future<SubscriptionSaved> updateSubscription(
    String subscriptionId, {
    String? url,
    List<String>? eventTypes,
    SubscriptionState? state,
    bool rotateHmacSecret = false,
    required String purpose,
  }) async {
    await change(
      'updateSubscription',
      '$subscriptionId ${url ?? ''} ${eventTypes?.join(',') ?? ''} '
          '${state?.name ?? ''} rotate=$rotateHmacSecret',
      purpose,
    );
    for (final MapEntry(key: appId, value: subs)
        in subscriptionsByApp.entries) {
      for (final (i, s) in subs.indexed) {
        if (s.id != subscriptionId) continue;
        _checkSubscription(appId, url ?? s.url, eventTypes ?? s.eventTypes);
        subs[i] = _sub(s, state: state, url: url, eventTypes: eventTypes);
        return SubscriptionSaved(subs[i]);
      }
    }
    throw const SelfActionFailed(ApiProblem(status: 404, code: 'not_found'));
  }

  /// Deletes a subscription.
  Future<void> deleteSubscription(
    String subscriptionId, {
    required String purpose,
  }) async {
    await change('deleteSubscription', subscriptionId, purpose);
    for (final subs in subscriptionsByApp.values) {
      subs.removeWhere((s) => s.id == subscriptionId);
    }
  }

  /// Deliveries per page of `deliveries` (an offset cursor).
  int deliveryPageSize = 50;

  /// The `listWebhookDeliveries` requests: subscription and cursor.
  final List<String> deliveryRequests = [];

  /// The deliveries of a subscription, newest first, [deliveryPageSize] per page.
  Future<AdminPage<AppDelivery>> deliveries(
    String subscriptionId, {
    String? cursor,
  }) async {
    deliveryRequests.add('$subscriptionId ${cursor ?? ''}'.trim());
    refused('deliveries');
    final all = deliveriesBySubscription[subscriptionId] ?? const [];
    final start = int.tryParse(cursor ?? '') ?? 0;
    final end = (start + deliveryPageSize).clamp(0, all.length);
    return AdminPage(
      all.sublist(start.clamp(0, all.length), end),
      nextCursor: end < all.length ? '$end' : null,
    );
  }

  /// Makes a delivery due again (no step-up).
  Future<AppDelivery> redeliver(
    String deliveryId, {
    required String purpose,
  }) async {
    await change('redeliver', deliveryId, purpose);
    for (final list in deliveriesBySubscription.values) {
      for (final (i, d) in list.indexed) {
        if (d.id != deliveryId) continue;
        return list[i] = AppDelivery(
          id: d.id,
          eventId: d.eventId,
          eventSeq: d.eventSeq,
          eventType: d.eventType,
          attempt: d.attempt,
          status: DeliveryStatus.pending,
          nextAttemptAt: DateTime.utc(2026, 10, 18, 11),
          createdAt: DateTime.utc(2026, 10, 18, 11),
        );
      }
    }
    throw const SelfActionFailed(ApiProblem(status: 404, code: 'not_found'));
  }

  /// One active key.
  Future<List<WebhookKeyInfo>> webhookKeys() async {
    refused('webhookKeys');
    return const [
      WebhookKeyInfo(
        kid: 'whk-2026-10',
        publicKey:
            'whpk_MCowBQYDK2VwAyEAEXAMPLEPublicKeyOfTheWebhookSignatures=',
        state: 'active',
      ),
    ];
  }

  /// The active incoming webhooks of a channel.
  Future<List<IncomingWebhookInfo>> incomingWebhooks(String channelId) async {
    refused('incomingWebhooks');
    return List.of(incomingByChannel[channelId] ?? const []);
  }

  /// Creates an incoming webhook; the address holds the token once.
  Future<IncomingWebhookCreatedInfo> createIncomingWebhook(
    String channelId, {
    required String appId,
    required String name,
    String? threadId,
    required String purpose,
  }) async {
    await change('createIncomingWebhook', '$channelId $appId $name', purpose);
    final app = appsById[appId];
    if (app == null || app.state != AppInstallState.active) {
      throw const SelfActionFailed(
        ApiProblem(
          status: 400,
          code: 'validation_failed',
          errors: [
            ProblemFieldError(pointer: '/app_id', code: 'app_not_installed'),
          ],
        ),
      );
    }
    final n = _hooks++;
    final hook = IncomingWebhookInfo(
      id: 'hook-$n',
      appId: appId,
      channelId: channelId,
      threadId: threadId,
      name: name,
      createdAt: DateTime.utc(2026, 10, 18, 9, 45),
    );
    incomingByChannel.putIfAbsent(channelId, () => []).add(hook);
    final token = 'bothy_whi_EXAMPLE${n}CanaryTokenShownOnce000000000_2WwD3e';
    handedOut.add(token);
    return IncomingWebhookCreatedInfo(
      hook,
      'https://chat.example.com/api/v1/hooks/hook-$n/$token',
    );
  }

  /// Revokes an incoming webhook.
  Future<void> revokeIncomingWebhook(
    String webhookId, {
    required String purpose,
  }) async {
    await change('revokeIncomingWebhook', webhookId, purpose);
    for (final list in incomingByChannel.values) {
      list.removeWhere((w) => w.id == webhookId);
    }
  }
}
