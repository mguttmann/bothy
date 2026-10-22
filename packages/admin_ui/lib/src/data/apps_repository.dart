// SPDX-License-Identifier: Apache-2.0

/// Apps, their outgoing webhook subscriptions and deliveries, and the incoming webhooks of a
/// channel over REST (docs/spec/01-product.md 5.2, 5.5; 11 3 to 8; brief M6-08 Design notes 1
/// to 5).
///
/// The types carry what the server answers, unchanged: whether an install, a scope, a host or
/// an event is allowed is the server's answer (`validateManifest`, `400 validation_failed`,
/// `400 webhook_host_not_allowed`), never a check of the client (15 1 principle 1). A manifest
/// travels as the JSON text the owner chose (a YAML file is converted to JSON first, 11 3.1), so
/// an unknown member reaches the server's closed schema instead of being dropped by a typed
/// model.
library;

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:yaml/yaml.dart';

import 'access_repository.dart';
import 'admin_repository.dart' show AdminPage;
import 'self_repository.dart';

/// The state of an installation (`AppInstallation.state`, 11 4.3).
enum AppInstallState {
  /// Installed and running.
  active,

  /// Switched off by `app.suspend`: tokens blocked, deliveries paused.
  suspended,

  /// Uninstalled: tokens and grants revoked, subscriptions disabled.
  uninstalled,

  /// A value of a newer server.
  unknown,
}

/// A grant the manifest asks for (`requested_grants`, 11 3.1): the owner picks the concrete
/// target, the manifest never sets one (11 4.1).
final class AppGrantAsk {
  /// Creates the request.
  const AppGrantAsk({
    required this.targetKind,
    required this.level,
    this.capabilities = const {},
    required this.purpose,
  });

  /// The kind of target the owner picks, for example a channel; null for a kind this build
  /// does not know.
  final AdminTargetKind? targetKind;

  /// The level (`read` or `write` for a valid manifest).
  final AccessLevel level;

  /// The capabilities (at most `upload` for a valid manifest).
  final Set<AccessCapability> capabilities;

  /// Why the app asks, in the publisher's words.
  final String purpose;

  /// A key that names the request in a diff of two manifests.
  String get key =>
      '${targetKind?.name}:${level.wire}:'
      '${(capabilities.map((c) => c.wire).toList()..sort()).join(',')}';
}

/// A command of the manifest (`commands`, 09 10).
final class AppCommandInfo {
  /// Creates the command.
  const AppCommandInfo(this.name, this.description);

  /// The name without the slash.
  final String name;

  /// The description.
  final String description;
}

/// An incoming webhook the manifest expects (`incoming_webhooks`).
final class AppIncomingAsk {
  /// Creates the entry.
  const AppIncomingAsk(this.name, this.purpose);

  /// The name.
  final String name;

  /// What it is for.
  final String purpose;
}

/// A manifest as the consent dialog shows it (`AppManifest`, 11 3.1), read from the JSON the
/// server validated or answered.
final class AppManifestInfo {
  /// Creates the manifest.
  const AppManifestInfo({
    required this.slug,
    required this.name,
    required this.description,
    required this.publisher,
    required this.version,
    this.homepageUrl,
    required this.minApiVersion,
    this.requiredScopes = const [],
    this.optionalScopes = const [],
    this.events = const [],
    this.deliveryMode,
    this.eventsUrl,
    this.interactionsUrl,
    this.commands = const [],
    this.components = const [],
    this.requestedGrants = const [],
    this.incomingWebhooks = const [],
    this.dataEgress,
  });

  /// Reads the manifest from its JSON object [json]; a member of the wrong type reads as
  /// absent (the server's validation names it).
  ///
  /// Example: `AppManifestInfo.fromJson(jsonDecode(text) as Map<String, Object?>)`.
  factory AppManifestInfo.fromJson(Map<String, Object?> json) {
    String str(Object? v) => v is String ? v : '';
    String? opt(Object? v) => v is String ? v : null;
    List<String> strings(Object? v) => [
      if (v is List)
        for (final e in v)
          if (e is String) e,
    ];
    List<Map<String, Object?>> maps(Object? v) => [
      if (v is List)
        for (final e in v)
          if (e is Map) e.cast<String, Object?>(),
    ];
    final scopes = json['scopes'];
    final scopeMap = scopes is Map
        ? scopes.cast<String, Object?>()
        : const <String, Object?>{};
    final delivery = json['delivery'];
    final deliveryMap = delivery is Map
        ? delivery.cast<String, Object?>()
        : const <String, Object?>{};
    return AppManifestInfo(
      slug: str(json['slug']),
      name: str(json['name']),
      description: str(json['description']),
      publisher: str(json['publisher']),
      version: str(json['version']),
      homepageUrl: opt(json['homepage_url']),
      minApiVersion: str(json['min_api_version']),
      requiredScopes: strings(scopeMap['required']),
      optionalScopes: strings(scopeMap['optional']),
      events: strings(json['events']),
      deliveryMode: opt(deliveryMap['mode']),
      eventsUrl: opt(deliveryMap['events_url']),
      interactionsUrl: opt(deliveryMap['interactions_url']),
      commands: [
        for (final c in maps(json['commands']))
          AppCommandInfo(str(c['name']), str(c['description'])),
      ],
      components: strings(json['components']),
      requestedGrants: [
        for (final g in maps(json['requested_grants']))
          AppGrantAsk(
            targetKind: AdminTargetKind.values
                .where((k) => k.name == g['target_kind'])
                .firstOrNull,
            level:
                AccessLevel.values
                    .where((l) => l != AccessLevel.none && l.wire == g['level'])
                    .firstOrNull ??
                AccessLevel.none,
            capabilities: {
              for (final c in strings(g['capabilities']))
                ?AccessCapability.values.where((a) => a.wire == c).firstOrNull,
            },
            purpose: str(g['purpose']),
          ),
      ],
      incomingWebhooks: [
        for (final w in maps(json['incoming_webhooks']))
          AppIncomingAsk(str(w['name']), str(w['purpose'])),
      ],
      dataEgress: opt(json['data_egress']),
    );
  }

  /// The slug, unique in the instance.
  final String slug;

  /// The name (the display name of the app principal).
  final String name;

  /// The description.
  final String description;

  /// The publisher.
  final String publisher;

  /// The SemVer of the manifest.
  final String version;

  /// The homepage, for display only, or null.
  final String? homepageUrl;

  /// The lowest API version the app needs.
  final String minApiVersion;

  /// The scopes the installation must grant.
  final List<String> requiredScopes;

  /// The scopes the owner may grant.
  final List<String> optionalScopes;

  /// The event types the app may subscribe to.
  final List<String> events;

  /// `gateway` or `webhook`, or null without delivery.
  final String? deliveryMode;

  /// The address of the outgoing webhooks, or null.
  final String? eventsUrl;

  /// The address of synchronous requests, or null.
  final String? interactionsUrl;

  /// The commands.
  final List<AppCommandInfo> commands;

  /// The component types (`button`, `select`, `form`).
  final List<String> components;

  /// The grants the app asks for.
  final List<AppGrantAsk> requestedGrants;

  /// The incoming webhooks the app expects.
  final List<AppIncomingAsk> incomingWebhooks;

  /// The publisher's plain text on which data leave the instance and where to, or null.
  final String? dataEgress;

  /// The hosts data go to: the hosts of [eventsUrl] and [interactionsUrl], each once, in that
  /// order (11 4.1 "target hosts").
  List<String> get targetHosts => <String>{
    for (final url in [eventsUrl, interactionsUrl])
      if (url != null)
        if (Uri.tryParse(url)?.host case final String h when h.isNotEmpty) h,
  }.toList();
}

/// An app with its latest installation (`App`).
final class AppRecord {
  /// Creates the app.
  const AppRecord({
    required this.id,
    required this.slug,
    required this.displayName,
    required this.manifestVersion,
    required this.manifest,
    required this.installationId,
    required this.grantedScopes,
    required this.eventSubscriptions,
    required this.state,
    required this.installedAt,
    this.installedBy,
  });

  /// The app principal ID.
  final String id;

  /// The slug.
  final String slug;

  /// The display name.
  final String displayName;

  /// The version of the stored manifest.
  final String manifestVersion;

  /// The stored manifest.
  final AppManifestInfo manifest;

  /// The ID of the latest installation.
  final String installationId;

  /// The scopes the installation grants.
  final List<String> grantedScopes;

  /// The events the installation subscribes to.
  final List<String> eventSubscriptions;

  /// The state of the installation.
  final AppInstallState state;

  /// When it was installed.
  final DateTime installedAt;

  /// Who installed it, or null.
  final String? installedBy;
}

/// Why a manifest file could not be read before it reached the server.
enum ManifestFileProblem {
  /// The file is no UTF-8 text.
  notText,

  /// The text is neither JSON nor YAML.
  notJsonOrYaml,

  /// The document is no object (a list or a scalar).
  notObject,
}

/// A manifest file the client could not read; the server never saw it.
final class ManifestFileError implements Exception {
  /// Creates the error.
  const ManifestFileError(this.problem);

  /// What is wrong.
  final ManifestFileProblem problem;

  @override
  String toString() => 'ManifestFileError(${problem.name})';
}

/// A manifest read from a file the owner picked (never from a URL, 11 4.1): the JSON text that
/// goes to the server and its decoded object.
final class ManifestUpload {
  const ManifestUpload._(this.fileName, this.json, this.data);

  /// Reads the file [fileName] with [bytes]: JSON as it is, YAML converted to JSON (11 3.1). A
  /// `.yaml` or `.yml` name is read as YAML, every other name as JSON first and as YAML when it
  /// is no JSON. A JSON file keeps its exact text, so the server hashes the bytes the owner
  /// chose (`apps.manifest_sha256`, D-M2-24-17).
  ///
  /// Throws [ManifestFileError] for a file that is no UTF-8, no JSON or YAML, or no object.
  ///
  /// Example: `ManifestUpload.read('repo-bridge.yaml', bytes).data['slug']`.
  static ManifestUpload read(String fileName, Uint8List bytes) {
    final String text;
    try {
      text = utf8.decode(bytes);
    } on FormatException {
      throw const ManifestFileError(ManifestFileProblem.notText);
    }
    final lower = fileName.toLowerCase();
    final yamlName = lower.endsWith('.yaml') || lower.endsWith('.yml');
    if (!yamlName) {
      try {
        final decoded = jsonDecode(text);
        if (decoded is! Map) {
          throw const ManifestFileError(ManifestFileProblem.notObject);
        }
        return ManifestUpload._(
          fileName,
          text,
          decoded.cast<String, Object?>(),
        );
      } on FormatException {
        // Not JSON: try YAML below.
      }
    }
    final Object? doc;
    try {
      doc = _plain(loadYaml(text));
    } on YamlException {
      throw const ManifestFileError(ManifestFileProblem.notJsonOrYaml);
    }
    if (doc is! Map<String, Object?>) {
      throw const ManifestFileError(ManifestFileProblem.notObject);
    }
    final String json;
    try {
      json = jsonEncode(doc);
    } on JsonUnsupportedObjectError {
      // NaN and infinity are YAML numbers that JSON cannot carry.
      throw const ManifestFileError(ManifestFileProblem.notJsonOrYaml);
    }
    return ManifestUpload._(fileName, json, doc);
  }

  /// Creates an upload of the object [data] (the built-in manifest of an "incoming app",
  /// brief Design note 5).
  factory ManifestUpload.ofObject(Map<String, Object?> data) =>
      ManifestUpload._('', jsonEncode(data), data);

  /// The name of the file, empty for a built-in manifest.
  final String fileName;

  /// The JSON text sent to the server.
  final String json;

  /// The decoded object.
  final Map<String, Object?> data;

  /// The manifest as the dialogs read it.
  AppManifestInfo get info => AppManifestInfo.fromJson(data);

  /// The plain Dart value of a YAML node: maps with string keys, lists and scalars. A key that
  /// is no string has no JSON form, so the file is refused ([ManifestFileError]).
  static Object? _plain(Object? node) => switch (node) {
    final YamlMap m when m.keys.every((k) => k is String) => <String, Object?>{
      for (final e in m.entries) e.key as String: _plain(e.value),
    },
    YamlMap() => throw const ManifestFileError(
      ManifestFileProblem.notJsonOrYaml,
    ),
    final YamlList l => [for (final e in l) _plain(e)],
    _ => node,
  };
}

/// The answer of `validateManifest`: valid, or the rules that failed with their pointer
/// relative to the manifest root (11 3.2; D-M2-24-3).
final class ManifestCheck {
  /// Creates the answer.
  const ManifestCheck({required this.valid, this.errors = const []});

  /// Whether every rule holds.
  final bool valid;

  /// The failed rules: pointer, code and the short English detail of the server.
  final List<ProblemFieldError> errors;
}

/// The state of an outgoing subscription (`WebhookSubscription.state`, 11 6, 7).
enum SubscriptionState {
  /// Delivering.
  active,

  /// Paused by the owner or by the delivery quota; rows keep being created.
  paused,

  /// Disabled: after `410`, after too many failures in a row, or after a gap in the event log.
  disabled,

  /// A value of a newer server.
  unknown,
}

/// An outgoing webhook subscription of an app (`WebhookSubscription`).
final class AppSubscription {
  /// Creates the subscription.
  const AppSubscription({
    required this.id,
    required this.url,
    required this.eventTypes,
    required this.state,
    required this.consecutiveFailures,
    required this.signatureSchemes,
    required this.signingKeyKid,
    required this.createdAt,
    this.disabledAt,
  });

  /// The subscription ID.
  final String id;

  /// The target address.
  final String url;

  /// The event types.
  final List<String> eventTypes;

  /// The state.
  final SubscriptionState state;

  /// The failed attempts in a row.
  final int consecutiveFailures;

  /// `v1a` and, with HMAC, `v1`.
  final List<String> signatureSchemes;

  /// The key that signed the last accepted delivery.
  final String signingKeyKid;

  /// When it was created.
  final DateTime createdAt;

  /// When it was disabled, or null.
  final DateTime? disabledAt;
}

/// A created or changed subscription with the HMAC secret that is shown once, if any.
final class SubscriptionSaved {
  /// Creates the result.
  const SubscriptionSaved(this.subscription, {this.hmacSecret});

  /// The subscription.
  final AppSubscription subscription;

  /// The `whsec_` secret, returned once with HMAC, or null.
  final String? hmacSecret;
}

/// The status of a delivery (`WebhookDeliveryStatus`).
enum DeliveryStatus {
  /// Waiting for its attempt.
  pending,

  /// Answered with `2xx`.
  succeeded,

  /// Ended by `410`.
  failed,

  /// Given up (retry window over, no longer authorized, the event left the log).
  dead,

  /// A value of a newer server.
  unknown,
}

/// One delivery of a subscription (`WebhookDelivery`); the response body is never stored
/// (11 7), so it is never shown.
final class AppDelivery {
  /// Creates the delivery.
  const AppDelivery({
    required this.id,
    required this.eventId,
    required this.eventSeq,
    this.eventType,
    required this.attempt,
    required this.status,
    this.responseCode,
    this.lastError,
    this.nextAttemptAt,
    required this.createdAt,
    this.deliveredAt,
  });

  /// The delivery ID.
  final String id;

  /// The event ID (the `webhook-id` header).
  final String eventId;

  /// The sequence number of the event.
  final int eventSeq;

  /// The event type, or null once the event left the event log.
  final String? eventType;

  /// The attempts so far.
  final int attempt;

  /// The status.
  final DeliveryStatus status;

  /// The last HTTP status code, or null.
  final int? responseCode;

  /// The code of the last error (`timeout`, `gone`, `event_expired`, ...), or null.
  final String? lastError;

  /// When the next attempt is due, or null.
  final DateTime? nextAttemptAt;

  /// When the delivery was created.
  final DateTime createdAt;

  /// When it succeeded, or null.
  final DateTime? deliveredAt;
}

/// A public key of the `v1a` webhook signatures (`WebhookKey`).
final class WebhookKeyInfo {
  /// Creates the key.
  const WebhookKeyInfo({
    required this.kid,
    required this.publicKey,
    required this.state,
    this.notAfter,
  });

  /// The key ID.
  final String kid;

  /// The public key with the prefix `whpk_`.
  final String publicKey;

  /// `next`, `active` or `retiring`.
  final String state;

  /// When a retiring key ends, or null.
  final DateTime? notAfter;
}

/// An active incoming webhook of a channel (`IncomingWebhook`); never its token.
final class IncomingWebhookInfo {
  /// Creates the webhook.
  const IncomingWebhookInfo({
    required this.id,
    required this.appId,
    required this.channelId,
    this.threadId,
    required this.name,
    required this.createdAt,
    this.lastUsedAt,
  });

  /// The webhook ID.
  final String id;

  /// The app it writes as.
  final String appId;

  /// The channel.
  final String channelId;

  /// The fixed thread, or null.
  final String? threadId;

  /// The name.
  final String name;

  /// When it was created.
  final DateTime createdAt;

  /// When it was last used, or null.
  final DateTime? lastUsedAt;
}

/// A created incoming webhook with its address, which holds the token and is shown once
/// (11 8.1; 08 12).
final class IncomingWebhookCreatedInfo {
  /// Creates the result.
  const IncomingWebhookCreatedInfo(this.webhook, this.url);

  /// The webhook.
  final IncomingWebhookInfo webhook;

  /// The absolute address with the token as its last segment.
  final String url;
}

/// The calls of apps and webhooks; a changing call carries the [purpose] text the step-up
/// dialog shows when the server answers `401 step_up_required` (05: `app.install`,
/// `app.scopes.update`, `app.suspend`, `app.resume`, `app.uninstall`, `webhook_out.manage`,
/// `webhook_in.create` and `webhook_in.revoke` have step-up). The repository never asks for a
/// step-up on its own.
///
/// Errors: [SelfActionFailed] with the server's problem (a `400 validation_failed` of an
/// install carries the failed rules in `ApiProblem.errors`), [StepUpCancelled] when the person
/// cancels the step-up.
abstract interface class AppsRepository {
  /// Every installed app (`listApps`, all pages).
  Future<List<AppRecord>> apps();

  /// One app with its latest installation (`getApp`).
  Future<AppRecord> app(String appId);

  /// Checks [upload] (`validateManifest`): stores nothing.
  Future<ManifestCheck> validateManifest(ManifestUpload upload);

  /// Installs [upload] with [grantedScopes] (every required and the chosen optional ones) and
  /// [eventSubscriptions] (`installApp`); creates neither a grant nor a token (D-M2-24-8).
  Future<AppRecord> installApp(
    ManifestUpload upload, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
    required String purpose,
  });

  /// Replaces the manifest of [appId] by [upload] with the same slug (`upgradeAppManifest`);
  /// [grantedScopes] and [eventSubscriptions] are subsets of the current ones (11 4.2).
  Future<AppRecord> upgradeManifest(
    String appId,
    ManifestUpload upload, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
    required String purpose,
  });

  /// Changes the granted scopes and subscriptions of [appId] (`setAppScopes`, the new
  /// consent of 11 4.2).
  Future<AppRecord> setScopes(
    String appId, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
    required String purpose,
  });

  /// Suspends [appId] (`suspendApp`).
  Future<void> suspendApp(String appId, {required String purpose});

  /// Resumes [appId] (`resumeApp`).
  Future<void> resumeApp(String appId, {required String purpose});

  /// Uninstalls [appId] with [reason] (`uninstallApp`).
  Future<void> uninstallApp(
    String appId, {
    required String reason,
    required String purpose,
  });

  /// The outgoing subscriptions of [appId] (`listWebhookSubscriptions`).
  Future<List<AppSubscription>> subscriptions(String appId);

  /// Creates a subscription of [appId] (`createWebhookSubscription`); with [enableHmac] the
  /// answer carries the secret once.
  Future<SubscriptionSaved> createSubscription(
    String appId, {
    required String url,
    required List<String> eventTypes,
    bool enableHmac = false,
    required String purpose,
  });

  /// Changes [subscriptionId] (`updateWebhookSubscription`); only the given members change.
  Future<SubscriptionSaved> updateSubscription(
    String subscriptionId, {
    String? url,
    List<String>? eventTypes,
    SubscriptionState? state,
    bool rotateHmacSecret = false,
    required String purpose,
  });

  /// Deletes [subscriptionId] (`deleteWebhookSubscription`).
  Future<void> deleteSubscription(
    String subscriptionId, {
    required String purpose,
  });

  /// One page of the deliveries of [subscriptionId], newest first (`listWebhookDeliveries`).
  Future<AdminPage<AppDelivery>> deliveries(
    String subscriptionId, {
    String? cursor,
  });

  /// Makes [deliveryId] due again (`redeliverWebhook`).
  Future<AppDelivery> redeliver(String deliveryId, {required String purpose});

  /// The public keys of the `v1a` signatures (`getWebhookKeys`).
  Future<List<WebhookKeyInfo>> webhookKeys();

  /// The active incoming webhooks of [channelId] (`listIncomingWebhooks`).
  Future<List<IncomingWebhookInfo>> incomingWebhooks(String channelId);

  /// Creates an incoming webhook of [channelId] that writes as [appId]
  /// (`createIncomingWebhook`); the answer's address holds the token, shown once.
  Future<IncomingWebhookCreatedInfo> createIncomingWebhook(
    String channelId, {
    required String appId,
    required String name,
    String? threadId,
    required String purpose,
  });

  /// Revokes [webhookId] (`revokeIncomingWebhook`); the next post answers `404`.
  Future<void> revokeIncomingWebhook(
    String webhookId, {
    required String purpose,
  });
}

/// [AppsRepository] over the generated client of `api_client`; the three calls that carry a
/// manifest send its JSON text as chosen.
///
/// Example:
/// ```dart
/// final apps = ApiAppsRepository(session.api, newIdempotencyKey: ids.next);
/// final check = await apps.validateManifest(ManifestUpload.read(name, bytes));
/// ```
final class ApiAppsRepository implements AppsRepository {
  /// Creates the repository over `api` (the client of the signed-in session, so DPoP, refresh
  /// and the step-up interceptor apply); [newIdempotencyKey] returns a fresh UUIDv7.
  ApiAppsRepository(this._api, {required String Function() newIdempotencyKey})
    : _newKey = newIdempotencyKey;

  final ApiClient _api;
  final String Function() _newKey;

  AppsApi get _apps => _api.getAppsApi();
  WebhooksApi get _hooks => _api.getWebhooksApi();

  Map<String, Object?> _purpose(String text) => {stepUpPurposeExtra: text};

  /// The security requirement of the app operations as the generated client sends it, so the
  /// auth interceptors sign the requests sent with a JSON text too.
  static const Map<String, Object?> _secure = {
    'secure': <Map<String, String>>[
      {
        'type': 'apiKey',
        'name': 'consoleSession',
        'keyName': '__Host-console_session',
        'where': '',
      },
      {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
      {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
    ],
  };

  /// Sends [body] (a JSON text) with [method] to [path] and returns the decoded answer.
  Future<Object?> _sendJson(
    String method,
    String path,
    String body, {
    String? purpose,
    bool idempotent = false,
  }) async {
    final response = await _api.dio.request<Object>(
      path,
      data: body,
      options: Options(
        method: method,
        contentType: 'application/json',
        headers: {if (idempotent) 'Idempotency-Key': _newKey()},
        extra: {..._secure, if (purpose != null) ..._purpose(purpose)},
      ),
    );
    final data = response.data;
    return data is String ? jsonDecode(data) : data;
  }

  /// The JSON text of an `AppInstall` with [manifest] inserted unchanged.
  static String _installBody(
    String manifest, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
  }) =>
      '{"manifest":$manifest,'
      '"granted_scopes":${jsonEncode(grantedScopes)},'
      '"event_subscriptions":${jsonEncode(eventSubscriptions)},'
      '"reason":${jsonEncode(reason)}}';

  AppRecord _appOfJson(Object? json) =>
      _record(_api.serializers.deserializeWith(App.serializer, json)!);

  @override
  Future<List<AppRecord>> apps() => adminCall(() async {
    final out = <AppRecord>[];
    String? cursor;
    do {
      final page = (await _apps.listApps(cursor: cursor)).data!;
      out.addAll(page.items.map(_record));
      cursor = page.nextCursor;
    } while (cursor != null);
    return out;
  });

  @override
  Future<AppRecord> app(String appId) =>
      adminCall(() async => _record((await _apps.getApp(appId: appId)).data!));

  @override
  Future<ManifestCheck> validateManifest(ManifestUpload upload) =>
      adminCall(() async {
        final json = await _sendJson(
          'POST',
          '/app-manifests/validate',
          upload.json,
        );
        final v = _api.serializers.deserializeWith(
          ManifestValidation.serializer,
          json,
        )!;
        return ManifestCheck(
          valid: v.valid,
          errors: [
            for (final e in v.errors)
              ProblemFieldError(
                pointer: e.pointer,
                code: e.code,
                detail: e.detail,
              ),
          ],
        );
      });

  @override
  Future<AppRecord> installApp(
    ManifestUpload upload, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
    required String purpose,
  }) => adminCall(
    () async => _appOfJson(
      await _sendJson(
        'POST',
        '/apps',
        _installBody(
          upload.json,
          grantedScopes: grantedScopes,
          eventSubscriptions: eventSubscriptions,
          reason: reason,
        ),
        purpose: purpose,
        idempotent: true,
      ),
    ),
  );

  @override
  Future<AppRecord> upgradeManifest(
    String appId,
    ManifestUpload upload, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
    required String purpose,
  }) => adminCall(
    () async => _appOfJson(
      await _sendJson(
        'PUT',
        '/apps/${Uri.encodeComponent(appId)}/manifest',
        _installBody(
          upload.json,
          grantedScopes: grantedScopes,
          eventSubscriptions: eventSubscriptions,
          reason: reason,
        ),
        purpose: purpose,
      ),
    ),
  );

  @override
  Future<AppRecord> setScopes(
    String appId, {
    required List<String> grantedScopes,
    required List<String> eventSubscriptions,
    required String reason,
    required String purpose,
  }) => adminCall(
    () async => _record(
      (await _apps.setAppScopes(
        appId: appId,
        appScopesPut: AppScopesPut(
          (b) => b
            ..grantedScopes.replace(grantedScopes.map(_scope))
            ..eventSubscriptions.replace(eventSubscriptions)
            ..reason = reason,
        ),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  AppScope _scope(String wire) =>
      _api.serializers.deserializeWith(AppScope.serializer, wire)!;

  @override
  Future<void> suspendApp(String appId, {required String purpose}) => adminCall(
    () => _apps.suspendApp(
      appId: appId,
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    ),
  );

  @override
  Future<void> resumeApp(String appId, {required String purpose}) => adminCall(
    () => _apps.resumeApp(
      appId: appId,
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    ),
  );

  @override
  Future<void> uninstallApp(
    String appId, {
    required String reason,
    required String purpose,
  }) => adminCall(
    () => _apps.uninstallApp(
      appId: appId,
      reasonBody: ReasonBody((b) => b..reason = reason),
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    ),
  );

  @override
  Future<List<AppSubscription>> subscriptions(String appId) => adminCall(
    () async =>
        (await _hooks.listWebhookSubscriptions(appId: appId)).data!.items
            .map(_subscription)
            .toList(),
  );

  @override
  Future<SubscriptionSaved> createSubscription(
    String appId, {
    required String url,
    required List<String> eventTypes,
    bool enableHmac = false,
    required String purpose,
  }) => adminCall(() async {
    final saved = (await _hooks.createWebhookSubscription(
      appId: appId,
      webhookSubscriptionCreate: WebhookSubscriptionCreate(
        (b) => b
          ..url = url
          ..eventTypes.replace(eventTypes)
          ..enableHmac = enableHmac ? true : null,
      ),
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    )).data!;
    return SubscriptionSaved(
      _subscription(saved.subscription),
      hmacSecret: saved.hmacSecret,
    );
  });

  @override
  Future<SubscriptionSaved> updateSubscription(
    String subscriptionId, {
    String? url,
    List<String>? eventTypes,
    SubscriptionState? state,
    bool rotateHmacSecret = false,
    required String purpose,
  }) => adminCall(() async {
    final saved = (await _hooks.updateWebhookSubscription(
      subscriptionId: subscriptionId,
      webhookSubscriptionUpdate: WebhookSubscriptionUpdate((b) {
        b
          ..url = url
          ..state = switch (state) {
            SubscriptionState.active =>
              WebhookSubscriptionUpdateStateEnum.active,
            SubscriptionState.paused =>
              WebhookSubscriptionUpdateStateEnum.paused,
            _ => null,
          }
          ..rotateHmacSecret = rotateHmacSecret ? true : null;
        if (eventTypes != null) b.eventTypes.replace(eventTypes);
      }),
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    )).data!;
    return SubscriptionSaved(
      _subscription(saved.subscription),
      hmacSecret: saved.hmacSecret,
    );
  });

  @override
  Future<void> deleteSubscription(
    String subscriptionId, {
    required String purpose,
  }) => adminCall(
    () => _hooks.deleteWebhookSubscription(
      subscriptionId: subscriptionId,
      extra: _purpose(purpose),
    ),
  );

  @override
  Future<AdminPage<AppDelivery>> deliveries(
    String subscriptionId, {
    String? cursor,
  }) => adminCall(() async {
    final page = (await _hooks.listWebhookDeliveries(
      subscriptionId: subscriptionId,
      cursor: cursor,
    )).data!;
    return AdminPage(
      page.items.map(_delivery).toList(),
      nextCursor: page.nextCursor,
    );
  });

  @override
  Future<AppDelivery> redeliver(String deliveryId, {required String purpose}) =>
      adminCall(
        () async => _delivery(
          (await _hooks.redeliverWebhook(
            deliveryId: deliveryId,
            idempotencyKey: _newKey(),
            extra: _purpose(purpose),
          )).data!,
        ),
      );

  @override
  Future<List<WebhookKeyInfo>> webhookKeys() => adminCall(
    () async => [
      for (final k
          in (await _api.getDiscoveryApi().getWebhookKeys()).data!.keys)
        WebhookKeyInfo(
          kid: k.kid,
          publicKey: k.publicKey,
          state: k.state.name,
          notAfter: k.notAfter,
        ),
    ],
  );

  @override
  Future<List<IncomingWebhookInfo>> incomingWebhooks(String channelId) =>
      adminCall(
        () async =>
            (await _hooks.listIncomingWebhooks(channelId: channelId))
                .data!
                .items
                .map(_incoming)
                .toList(),
      );

  @override
  Future<IncomingWebhookCreatedInfo> createIncomingWebhook(
    String channelId, {
    required String appId,
    required String name,
    String? threadId,
    required String purpose,
  }) => adminCall(() async {
    final created = (await _hooks.createIncomingWebhook(
      channelId: channelId,
      incomingWebhookCreate: IncomingWebhookCreate(
        (b) => b
          ..appId = appId
          ..name = name
          ..threadId = threadId,
      ),
      idempotencyKey: _newKey(),
      extra: _purpose(purpose),
    )).data!;
    return IncomingWebhookCreatedInfo(_incoming(created.webhook), created.url);
  });

  @override
  Future<void> revokeIncomingWebhook(
    String webhookId, {
    required String purpose,
  }) => adminCall(
    () => _hooks.revokeIncomingWebhook(
      webhookId: webhookId,
      extra: _purpose(purpose),
    ),
  );

  /// The [AppRecord] of the API's [a]: the serializers of the client give the manifest back as
  /// its JSON object and the scopes as their wire names.
  AppRecord _record(App a) {
    final serializers = _api.serializers;
    final manifest = serializers.serializeWith(
      AppManifest.serializer,
      a.manifest,
    );
    String scope(AppScope s) =>
        serializers.serializeWith(AppScope.serializer, s)! as String;
    final i = a.installation;
    return AppRecord(
      id: a.id,
      slug: a.slug,
      displayName: a.displayName,
      manifestVersion: a.manifestVersion,
      manifest: AppManifestInfo.fromJson(
        manifest is Map ? manifest.cast<String, Object?>() : const {},
      ),
      installationId: i.id,
      grantedScopes: [
        for (final s in i.grantedScopes)
          if (s != AppScope.unknownDefaultOpenApi) scope(s),
      ],
      eventSubscriptions: i.eventSubscriptions.toList(),
      state: switch (i.state) {
        AppInstallationStateEnum.active => AppInstallState.active,
        AppInstallationStateEnum.suspended => AppInstallState.suspended,
        AppInstallationStateEnum.uninstalled => AppInstallState.uninstalled,
        _ => AppInstallState.unknown,
      },
      installedAt: i.installedAt,
      installedBy: i.installedBy,
    );
  }

  static AppSubscription _subscription(WebhookSubscription s) =>
      AppSubscription(
        id: s.id,
        url: s.url,
        eventTypes: s.eventTypes.toList(),
        state: switch (s.state) {
          WebhookSubscriptionStateEnum.active => SubscriptionState.active,
          WebhookSubscriptionStateEnum.paused => SubscriptionState.paused,
          WebhookSubscriptionStateEnum.disabled => SubscriptionState.disabled,
          _ => SubscriptionState.unknown,
        },
        consecutiveFailures: s.consecutiveFailures,
        signatureSchemes: [
          for (final scheme in s.signatureSchemes)
            if (scheme !=
                WebhookSubscriptionSignatureSchemesEnum.unknownDefaultOpenApi)
              scheme.name,
        ],
        signingKeyKid: s.signingKeyKid,
        createdAt: s.createdAt,
        disabledAt: s.disabledAt,
      );

  static AppDelivery _delivery(WebhookDelivery d) => AppDelivery(
    id: d.id,
    eventId: d.eventId,
    eventSeq: d.eventSeq,
    eventType: d.eventType,
    attempt: d.attempt,
    status: switch (d.status) {
      WebhookDeliveryStatus.pending => DeliveryStatus.pending,
      WebhookDeliveryStatus.succeeded => DeliveryStatus.succeeded,
      WebhookDeliveryStatus.failed => DeliveryStatus.failed,
      WebhookDeliveryStatus.dead => DeliveryStatus.dead,
      _ => DeliveryStatus.unknown,
    },
    responseCode: d.responseCode,
    lastError: d.lastError,
    nextAttemptAt: d.nextAttemptAt,
    createdAt: d.createdAt,
    deliveredAt: d.deliveredAt,
  );

  static IncomingWebhookInfo _incoming(IncomingWebhook w) =>
      IncomingWebhookInfo(
        id: w.id,
        appId: w.appId,
        channelId: w.channelId,
        threadId: w.threadId,
        name: w.name,
        createdAt: w.createdAt,
        lastUsedAt: w.lastUsedAt,
      );
}
