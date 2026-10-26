// SPDX-License-Identifier: Apache-2.0

/// Sharing on iOS and macOS, the app's side (brief M5-06 Design notes 3, 5 and 7; docs/spec/
/// 08-identity.md 7.4; 15-client-architecture.md 8, 9 and 12): the child session of the Share
/// Extension in the shared keychain, the settings and recent targets in the App Group, and the
/// takeover of the share inbox.
library;

import 'dart:async';

import 'package:api_client/api_client.dart' show ApiClient;
import 'package:api_client/api_client.dart' as api;
import 'package:auth/auth.dart' show Clock, SystemClock;
import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart' show listEquals;

import '../src/generated/share_apple_api.g.dart';
import 'share_target.dart';

export '../src/generated/share_apple_api.g.dart'
    show
        ExtensionSessionRecord,
        ShareAppleHostApi,
        ShareInboxEntryRecord,
        ShareInboxFileRecord,
        ShareSettingsRecord,
        ShareTargetRecord;

/// What `createExtensionSession` returned for the Share Extension: the child session with
/// its own refresh family on the same device key (08 7.4).
///
/// [toString] names no token, so a log line never carries one.
final class ExtensionSessionGrant {
  /// Creates the grant.
  const ExtensionSessionGrant({
    required this.sessionId,
    required this.accessToken,
    required this.accessExpiresIn,
    required this.refreshToken,
    required this.refreshExpiresAt,
  });

  /// The ID of the child session.
  final String sessionId;

  /// The access token of the child.
  final String accessToken;

  /// How long [accessToken] is valid from now (`expires_in`).
  final Duration accessExpiresIn;

  /// The refresh token of the child's own family, shown once.
  final String refreshToken;

  /// When [refreshToken] expires (`refresh_expires_at`).
  final DateTime refreshExpiresAt;

  @override
  String toString() => 'ExtensionSessionGrant(<redacted>)';
}

/// `createExtensionSession` with `client_component = share_ext` (08 7.4).
abstract interface class ExtensionSessions {
  /// Creates the child session of the calling app session; a new one replaces the earlier
  /// child of the same component on the server. Throws what the API client throws.
  Future<ExtensionSessionGrant> create();
}

/// [ExtensionSessions] over the generated API client of a signed-in session.
final class ApiExtensionSessions implements ExtensionSessions {
  /// Creates the adapter over [_api].
  ApiExtensionSessions(this._api);

  final ApiClient _api;

  @override
  Future<ExtensionSessionGrant> create() async {
    final response = await _api.getDevicesApi().createExtensionSession(
      extensionSessionCreate: api.ExtensionSessionCreate(
        (b) => b
          ..clientComponent =
              api.ExtensionSessionCreateClientComponentEnum.shareExt,
      ),
    );
    final tokens = response.data;
    if (tokens == null) {
      throw StateError('createExtensionSession answered without a body');
    }
    return ExtensionSessionGrant(
      sessionId: tokens.session.id,
      accessToken: tokens.accessToken,
      accessExpiresIn: Duration(seconds: tokens.expiresIn),
      refreshToken: tokens.refreshToken,
      refreshExpiresAt: tokens.refreshExpiresAt,
    );
  }
}

/// The signed-in account as sharing needs it.
final class ShareAccount {
  /// Creates the account.
  const ShareAccount({
    required this.instanceId,
    required this.serverUrl,
    required this.sessions,
    required this.recentTargetsMax,
    required this.recentTargets,
    required this.takeOver,
    this.limits,
  });

  /// The `instance_id`, the scope of the keychain item and of the App Group values.
  final String instanceId;

  /// The server address, for the extension's requests.
  final Uri serverUrl;

  /// The REST call that creates the child session.
  final ExtensionSessions sessions;

  /// `client.share.recent_targets` of `AppConfig`.
  final int recentTargetsMax;

  /// The limits of the instance (`getInstance`, D-W-115), or null while unknown.
  final InstanceLimits? limits;

  /// The recent targets from the local database, newest first, at most [recentTargetsMax];
  /// null while they cannot be known yet (the person is unknown before the first snapshot).
  final Future<List<ShareTarget>?> Function() recentTargets;

  /// Takes the share inbox of the App Group over into the outbox (`ShareTakeover.run`).
  final Future<void> Function() takeOver;
}

/// Sharing on iOS and macOS for the app (brief M5-06 Design notes 3, 5 and 7).
///
/// - Child session: after a login [signedIn] always creates a new child session of the
///   Share Extension (`createExtensionSession`, `client_component = share_ext`), at a start
///   and on a return to the foreground ([foreground]) only when the shared keychain has no
///   valid one (the extension deletes a revoked one). The tokens go straight to the shared
///   keychain through the runner; Dart never stores them. The app never refreshes the child
///   family, the extension never touches the app's (08 7.4).
/// - Settings: the instance, `client.share.recent_targets` and, once known, the instance
///   limits `file_max_upload_bytes` and `message_max_attachments` ([limitsChanged]).
/// - Recent targets: [refreshTargets] writes them when they changed; the wiring calls it at a
///   start, after every confirmed send and after every change of the stored channels
///   (`access.changed`).
/// - Inbox: [signedIn] and [foreground] take the share inbox over.
/// - [forget] deletes the child session and the shared data on every end of the account.
///
/// Every platform failure is swallowed: sharing is an addition, the app works without it and
/// the next start tries again. A build without the App Group ([ShareAppleHostApi.isAvailable]
/// false) does nothing.
///
/// Example:
/// ```dart
/// final share = AppleShare(host: ShareAppleHostApi());
/// await share.signedIn(account, afterLogin: true);
/// ```
final class AppleShare {
  /// Creates the glue over [host]; [clock] dates the expiry of the access token.
  AppleShare({required this.host, Clock clock = const SystemClock()})
    : _clock = clock; // ignore: prefer_initializing_formals

  /// The runner's side: the shared keychain, the App Group values and the share inbox.
  final ShareAppleHostApi host;

  ShareAppleHostApi get _host => host;
  final Clock _clock;
  Future<void> _targets = Future<void>.value();
  ShareAccount? _account;
  InstanceLimits? _limits;
  List<ShareTarget>? _written;
  Future<void>? _creating;

  /// The account whose child session this app created since the account was handed over.
  ShareAccount? _createdFor;

  /// The account signed in: after a login ([afterLogin]) or at a start with a stored
  /// session. Writes the settings, creates the child session (always after a login, else
  /// when missing), takes the inbox over and writes the recent targets.
  Future<void> signedIn(
    ShareAccount account, {
    required bool afterLogin,
  }) async {
    // The same account again (a login reported after the handover) keeps what is known.
    if (!identical(_account, account)) {
      _account = account;
      _limits = account.limits;
      _written = null;
    }
    if (!await _available()) return;
    await _guard(_writeSettings);
    await _ensureSession(account, afterLogin: afterLogin);
    await _guard(account.takeOver);
    await refreshTargets();
  }

  /// The app came to the foreground: creates the child session again when it is missing
  /// and takes the inbox over.
  Future<void> foreground() async {
    final account = _account;
    if (account == null || !await _available()) return;
    await _ensureSession(account, afterLogin: false);
    await _guard(account.takeOver);
  }

  /// The instance limits changed (read from `getInstance`); writes the settings again.
  Future<void> limitsChanged(InstanceLimits? limits) async {
    if (_account == null || limits == _limits) return;
    _limits = limits;
    if (!await _available()) return;
    await _guard(_writeSettings);
  }

  /// Reads the recent targets of the account and writes them when they differ from the ones
  /// written last; one write at a time, so an older list never overwrites a newer one.
  Future<void> refreshTargets() => _targets = _targets.then((_) => _refresh());

  Future<void> _refresh() async {
    final account = _account;
    if (account == null || !await _available()) return;
    final targets = await _guardValue(account.recentTargets);
    if (targets == null || !identical(_account, account)) return;
    if (_written != null && listEquals(_written, targets)) return;
    await _guard(() async {
      await _host.writeRecentTargets([for (final t in targets) t.toRecord()]);
      _written = List.of(targets);
    });
  }

  /// Every end of the account of [instanceId] (logout, `4012`, a revoked device): deletes
  /// the child session of the shared keychain and the settings and recent targets of the App
  /// Group.
  Future<void> forget(String instanceId) async {
    if (_account?.instanceId == instanceId) {
      _account = null;
      _limits = null;
      _written = null;
      _createdFor = null;
    }
    if (!await _available()) return;
    await _guard(() => _host.deleteExtensionSession(instanceId));
    await _guard(_host.clearShareData);
  }

  Future<bool> _available() async =>
      await _guardValue(_host.isAvailable) ?? false;

  Future<void> _writeSettings() async {
    final account = _account;
    if (account == null) return;
    final limits = _limits;
    await _host.writeShareSettings(
      ShareSettingsRecord(
        instanceId: account.instanceId,
        recentTargetsMax: account.recentTargetsMax,
        fileMaxUploadBytes: limits?.fileMaxUploadBytes,
        messageMaxAttachments: limits?.messageMaxAttachments,
      ),
    );
  }

  /// Creates the child session of [account] after a login ([afterLogin]) or when the keychain
  /// has no valid one; one creation at a time.
  Future<void> _ensureSession(
    ShareAccount account, {
    required bool afterLogin,
  }) async {
    while (_creating != null) {
      await _creating;
    }
    if (!identical(_account, account)) return;
    // Decided only now, after a creation that ran: a child created since the handover was
    // created after the login already, so a login reported late creates no second one.
    final force = afterLogin && !identical(_createdFor, account);
    final creating = _createOnce(account, force: force);
    _creating = creating;
    try {
      await creating;
    } finally {
      _creating = null;
    }
  }

  Future<void> _createOnce(
    ShareAccount account, {
    required bool force,
  }) => _guard(() async {
    final now = _clock.now();
    if (!force &&
        await _host.hasExtensionSession(
          account.instanceId,
          now.millisecondsSinceEpoch,
        )) {
      return;
    }
    final grant = await account.sessions.create();
    // The account may have ended while the call ran: then nothing is stored.
    if (!identical(_account, account)) return;
    await _host.saveExtensionSession(
      ExtensionSessionRecord(
        instanceId: account.instanceId,
        serverUrl: account.serverUrl.toString(),
        sessionId: grant.sessionId,
        accessToken: grant.accessToken,
        accessExpiresAtMillis: now
            .add(grant.accessExpiresIn)
            .millisecondsSinceEpoch,
        refreshToken: grant.refreshToken,
        refreshExpiresAtMillis: grant.refreshExpiresAt.millisecondsSinceEpoch,
      ),
    );
    _createdFor = account;
  });

  static Future<void> _guard(FutureOr<void> Function() body) async {
    try {
      await body();
    } on Object {
      // Sharing is an addition; the next start tries again (see the class comment).
    }
  }

  static Future<T?> _guardValue<T>(Future<T> Function() body) async {
    try {
      return await body();
    } on Object {
      return null;
    }
  }
}
