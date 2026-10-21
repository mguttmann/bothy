// SPDX-License-Identifier: Apache-2.0

/// Local notifications of the desktop app (brief M4-30 Design note 2; docs/spec/
/// 15-client-architektur.md 10.3; docs/spec/13-push-benachrichtigungen.md 5.3).
///
/// [NotificationPresenter] is the port the router uses; [LocalNotificationPresenter] shows
/// toasts on Windows and notifications on macOS through `flutter_local_notifications`, and
/// [NoNotificationPresenter] shows nothing (tests and hosts without a notification service).
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io' show Platform;

import 'package:crypto/crypto.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';

import 'toast_content.dart';

/// Shows and replaces local notifications and reports clicks on them.
///
/// Example:
/// ```dart
/// await presenter.show(content, tag: 'thread:$id', silent: quiet, payload: '/t/$id');
/// presenter.clicks.listen(router.go);
/// ```
abstract interface class NotificationPresenter {
  /// Shows [content] in the slot [tag]: a later [show] with the same tag replaces it (13 4.5,
  /// one slot per thread). [silent] shows it without sound (quiet hours, 13 3.4). [payload] is
  /// the app path a click opens; it comes back through [clicks].
  ///
  /// Errors of the platform are logged without content and not thrown: a notification that
  /// cannot be shown must not stop the gateway connection.
  Future<void> show(
    ToastContent content, {
    required String tag,
    required bool silent,
    required String payload,
  });

  /// Removes the notification of slot [tag] where the platform allows it; without package
  /// identity on Windows it does nothing and logs one line (15 10.3).
  Future<void> cancelThread(String tag);

  /// The payloads of clicked notifications while the app runs.
  Stream<String> get clicks;

  /// The payload of the click that started the app, or null.
  Future<String?> launchPayload();

  /// Releases the platform plugin.
  Future<void> dispose();
}

/// A presenter that shows nothing: tests and builds without a notification service.
final class NoNotificationPresenter implements NotificationPresenter {
  /// Creates the presenter.
  const NoNotificationPresenter();

  @override
  Future<void> show(
    ToastContent content, {
    required String tag,
    required bool silent,
    required String payload,
  }) async {}

  @override
  Future<void> cancelThread(String tag) async {}

  @override
  Stream<String> get clicks => const Stream.empty();

  @override
  Future<String?> launchPayload() async => null;

  @override
  Future<void> dispose() async {}
}

/// [NotificationPresenter] over `flutter_local_notifications` for Windows and macOS.
///
/// It initializes on the first [show] with the app name of the build branding (M4-18) and, on
/// Windows, with the app user model ID [appUserModelId] and the activation GUID [guid] that an
/// app without package identity needs (D-M4-30-2). Without package identity (before the MSIX
/// of M7) Windows offers no `cancel` and no `getActiveNotifications`: [cancelThread] then logs
/// one line without content and does nothing, and a slot is replaced by showing the
/// notification again under the same ID. On macOS the permission is asked once at the first
/// notification, never awaited, so no dialog blocks the gateway path.
///
/// Example:
/// ```dart
/// final presenter = LocalNotificationPresenter(
///   appName: branding.appName,
///   appUserModelId: branding.bundleId,
///   guid: LocalNotificationPresenter.guidFor(branding.bundleId),
/// );
/// ```
final class LocalNotificationPresenter implements NotificationPresenter {
  /// Creates the presenter; [log] receives one line per notable step, never content;
  /// [plugin] replaces the platform plugin in tests; [hasPackageIdentity] overrides the check
  /// of the Windows package identity.
  LocalNotificationPresenter({
    required this.appName,
    required this.appUserModelId,
    required this.guid,
    void Function(String line)? log,
    FlutterLocalNotificationsPlugin? plugin,
    bool? hasPackageIdentity,
  }) : _log = log ?? _noLog,
       _plugin = plugin ?? FlutterLocalNotificationsPlugin(),
       _identity = hasPackageIdentity;

  /// The app name the notifications carry (`BuildBranding.appName`).
  final String appName;

  /// The app user model ID of Windows (the bundle identifier of the build).
  final String appUserModelId;

  /// The GUID of the Windows activation callback.
  final String guid;

  final void Function(String line) _log;
  final FlutterLocalNotificationsPlugin _plugin;
  final bool? _identity;
  final StreamController<String> _clicks = StreamController<String>.broadcast();
  Future<bool>? _ready;

  static void _noLog(String _) {}

  /// A stable GUID for [bundleId]: the first 16 bytes of its SHA-256 with the version and
  /// variant bits of a name-based UUID, so every build of one branding uses the same
  /// activation GUID without a new branding key (D-M4-30-2).
  ///
  /// Example: `LocalNotificationPresenter.guidFor('com.example.chat')`.
  static String guidFor(String bundleId) {
    final b = sha256
        .convert(utf8.encode('notifications:$bundleId'))
        .bytes
        .sublist(0, 16);
    b[6] = (b[6] & 0x0f) | 0x50;
    b[8] = (b[8] & 0x3f) | 0x80;
    String hex(int from, int to) => [
      for (final x in b.sublist(from, to)) x.toRadixString(16).padLeft(2, '0'),
    ].join();
    return '${hex(0, 4)}-${hex(4, 6)}-${hex(6, 8)}-${hex(8, 10)}-${hex(10, 16)}';
  }

  /// The notification ID of slot [tag]: a 31-bit FNV-1a hash, the same for the same tag.
  static int idOf(String tag) {
    var h = 0x811c9dc5;
    for (final unit in utf8.encode(tag)) {
      h = ((h ^ unit) * 0x01000193) & 0xffffffff;
    }
    return h & 0x7fffffff;
  }

  bool get _hasIdentity =>
      _identity ?? (!Platform.isWindows || MsixUtils.hasPackageIdentity());

  Future<bool> _init() => _ready ??= () async {
    try {
      final ok = await _plugin.initialize(
        settings: InitializationSettings(
          macOS: const DarwinInitializationSettings(
            requestAlertPermission: false,
            requestBadgePermission: false,
            requestSoundPermission: false,
          ),
          windows: WindowsInitializationSettings(
            appName: appName,
            appUserModelId: appUserModelId,
            guid: guid,
          ),
        ),
        onDidReceiveNotificationResponse: (r) {
          final payload = r.payload;
          if (payload != null && !_clicks.isClosed) _clicks.add(payload);
        },
      );
      if (Platform.isMacOS) {
        // Asked once and never awaited: the answer only decides whether macOS shows them.
        unawaited(
          _plugin
              .resolvePlatformSpecificImplementation<
                MacOSFlutterLocalNotificationsPlugin
              >()
              ?.requestPermissions(alert: true, sound: true)
              .then<void>((_) {}, onError: (Object _) {}),
        );
      }
      if (!_hasIdentity) {
        _log('notifications: no package identity, cancel and history skipped');
      }
      return ok ?? false;
    } on Object catch (e) {
      _log('notifications: initialize failed (${e.runtimeType})');
      return false;
    }
  }();

  @override
  Future<void> show(
    ToastContent content, {
    required String tag,
    required bool silent,
    required String payload,
  }) async {
    if (!await _init()) return;
    try {
      await _plugin.show(
        id: idOf(tag),
        title: content.title,
        body: content.body,
        notificationDetails: NotificationDetails(
          macOS: DarwinNotificationDetails(
            presentSound: !silent,
            threadIdentifier: tag,
            interruptionLevel: silent
                ? InterruptionLevel.passive
                : InterruptionLevel.active,
          ),
          windows: WindowsNotificationDetails(
            audio: silent ? WindowsNotificationAudio.silent() : null,
          ),
        ),
        payload: payload,
      );
    } on Object catch (e) {
      _log('notifications: show failed (${e.runtimeType})');
    }
  }

  @override
  Future<void> cancelThread(String tag) async {
    if (!await _init()) return;
    if (!_hasIdentity) {
      _log('notifications: cancel skipped without package identity');
      return;
    }
    try {
      await _plugin.cancel(id: idOf(tag));
    } on Object catch (e) {
      _log('notifications: cancel failed (${e.runtimeType})');
    }
  }

  @override
  Stream<String> get clicks => _clicks.stream;

  @override
  Future<String?> launchPayload() async {
    if (!await _init()) return null;
    try {
      final details = await _plugin.getNotificationAppLaunchDetails();
      if (details == null || !details.didNotificationLaunchApp) return null;
      return details.notificationResponse?.payload;
    } on Object catch (e) {
      _log('notifications: launch details failed (${e.runtimeType})');
      return null;
    }
  }

  @override
  Future<void> dispose() => _clicks.close();
}
