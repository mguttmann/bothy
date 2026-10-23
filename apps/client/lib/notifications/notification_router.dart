// SPDX-License-Identifier: Apache-2.0

/// From a `notification.created` event to a toast or nothing (brief M4-30 Design note 1;
/// docs/spec/13-push-notifications.md 4.4, 4.5 and 5.3; docs/spec/01-product.md 10.3).
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import 'notification_presenter.dart';
import 'toast_content.dart';

/// Decides per event whether a toast shows and with which slot, and opens the target of a
/// clicked toast.
///
/// The server alone decides who is notified (R-PN-01); the router only drops the event for the
/// thread that is open in the focused, not idle window (01 10.3 row "all"). The server
/// normally sends nothing then (13 4.4), so this guards the race between a focus change and an
/// event already on its way (D-M4-30-1). A thread's notifications share one slot (13 4.5);
/// `security` and `admin` have one fixed slot per kind and open the devices settings and the
/// administration.
///
/// Example:
/// ```dart
/// final router = NotificationRouter(
///   presenter: presenter,
///   texts: () => l,
///   appName: () => 'ExampleChat',
///   isOpenAndFocused: (t) => engine.focus.threads.contains(t) && !engine.focus.idle,
///   navigate: goRouter.go,
/// );
/// engine.notifications.listen(router.onEvent);
/// final clicks = router.listen();
/// ```
final class NotificationRouter {
  /// Creates the router.
  ///
  /// [presenter] shows the toasts; [texts] gives the interface texts of the current language;
  /// [appName] is the title when no channel name is known; [isOpenAndFocused] says whether a
  /// thread is open in the focused window that is not idle; [navigate] opens an app path;
  /// [localChannelName] reads the channel name of a thread from the local database for the
  /// placeholder (01 10.2); [log] receives one line per notable step, never content.
  NotificationRouter({
    required this.presenter,
    required this.texts,
    required this.appName,
    required this.isOpenAndFocused,
    required this.navigate,
    Future<String?> Function(ThreadId thread)? localChannelName,
    void Function(String line)? log,
  }) : _localChannelName = localChannelName ?? _noChannel,
       _log = log ?? _noLog;

  /// Shows the toasts.
  final NotificationPresenter presenter;

  /// The interface texts of the current language.
  final AppLocalizations Function() texts;

  /// The title without a known channel name.
  final String Function() appName;

  /// Whether a thread is open in the focused window that is not idle.
  final bool Function(ThreadId thread) isOpenAndFocused;

  /// Opens an app path, for example `/t/<thread>?m=<message>`.
  final void Function(String path) navigate;

  final Future<String?> Function(ThreadId thread) _localChannelName;
  final void Function(String line) _log;

  static Future<String?> _noChannel(ThreadId _) async => null;
  static void _noLog(String _) {}

  /// The app path a click on [n] opens (15 11): `/t/<thread>?m=<message>` for a thread,
  /// `/settings/devices` for `security`, `/admin` for `admin`; null for an unknown kind
  /// without a thread.
  static String? targetOf(NotificationCreated n) {
    final thread = n.threadId;
    if (thread != null) {
      final message = n.messageId;
      return message == null
          ? '/t/${thread.value}'
          : '/t/${thread.value}?m=${message.value}';
    }
    return switch (n.kind) {
      HintKind.security => '/settings/devices',
      HintKind.admin => '/admin',
      _ => null,
    };
  }

  /// The slot of [n]: one per thread, one per kind without a thread (13 4.5).
  static String tagOf(NotificationCreated n) {
    final thread = n.threadId;
    return thread == null ? 'kind:${n.kind.code}' : 'thread:${thread.value}';
  }

  /// Shows the toast of [n], or nothing for the thread that is open in the focused window.
  Future<void> onEvent(NotificationCreated n) async {
    final thread = n.threadId;
    if (thread != null && isOpenAndFocused(thread)) {
      _log('notification ${n.nid} dropped: thread open and focused');
      return;
    }
    final target = targetOf(n);
    if (target == null) {
      _log('notification ${n.nid} dropped: kind ${n.kind.code} without target');
      return;
    }
    String? local;
    if (thread != null && n.channelName == null) {
      try {
        local = await _localChannelName(thread);
      } on Object catch (e) {
        // The name is only the fallback of the placeholder; a closed database (Quit, sign-out)
        // must not drop the toast. The line names no content.
        _log('notification ${n.nid}: no local channel name (${e.runtimeType})');
      }
    }
    await presenter.show(
      toastContentFor(
        n,
        texts: texts(),
        appName: appName(),
        localChannelName: local,
      ),
      tag: tagOf(n),
      silent: n.quiet,
      payload: target,
    );
  }

  /// Opens [payload] when it is an app path; anything else is ignored.
  void onClick(String payload) {
    if (!payload.startsWith('/') || payload.startsWith('//')) {
      _log('notification click ignored: not an app path');
      return;
    }
    navigate(payload);
  }

  /// Opens the click that started the app, then every click while it runs; cancel the
  /// subscription to stop.
  StreamSubscription<String> listen() {
    unawaited(
      presenter.launchPayload().then((p) {
        if (p != null) onClick(p);
      }),
    );
    return presenter.clicks.listen(onClick);
  }
}
