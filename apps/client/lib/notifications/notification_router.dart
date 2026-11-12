// SPDX-License-Identifier: Apache-2.0

/// From a `notification.created` event to a toast or nothing (brief M4-30 Design note 1;
/// docs/spec/13-push-notifications.md 4.4, 4.5 and 5.3; docs/spec/01-product.md 10.3).
library;

import 'dart:async';
import 'dart:developer' as developer;

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
/// administration. A thread the person read, on this device or another one, loses the
/// notification this router showed for it ([onCountChange], 01 10.3).
///
/// Example:
/// ```dart
/// final router = NotificationRouter(
///   presenter: presenter,
///   texts: () => l,
///   appName: () => 'ExampleChat',
///   isOpenAndFocused: (t) => engine.focus.threads.contains(t) && !engine.focus.idle,
///   unreadOf: (t) => engine.catchUp.countsOf(t).unread,
///   navigate: goRouter.go,
/// );
/// engine.notifications.listen(router.onEvent);
/// engine.catchUp.countChanges.listen(router.onCountChange);
/// final clicks = router.listen();
/// ```
final class NotificationRouter {
  /// Creates the router.
  ///
  /// [presenter] shows the toasts; [texts] gives the interface texts of the current language;
  /// [appName] is the title when no channel name is known; [isOpenAndFocused] says whether a
  /// thread is open in the focused window that is not idle; [unreadOf] gives the unread
  /// messages of a thread as the engine counts them now; [navigate] opens an app path;
  /// [localChannelName] reads the channel name of a thread from the local database for the
  /// placeholder (01 10.2); [log] receives one line per notable step, never content, and is
  /// the developer log of the app unless given.
  NotificationRouter({
    required this.presenter,
    required this.texts,
    required this.appName,
    required this.isOpenAndFocused,
    required this.unreadOf,
    required this.navigate,
    Future<String?> Function(ThreadId thread)? localChannelName,
    void Function(String line)? log,
  }) : _localChannelName = localChannelName ?? _noChannel,
       _log = log ?? _developerLog;

  /// Shows the toasts.
  final NotificationPresenter presenter;

  /// The interface texts of the current language.
  final AppLocalizations Function() texts;

  /// The title without a known channel name.
  final String Function() appName;

  /// Whether a thread is open in the focused window that is not idle.
  final bool Function(ThreadId thread) isOpenAndFocused;

  /// The unread messages of a thread as the engine counts them now (04 14).
  final int Function(ThreadId thread) unreadOf;

  /// Opens an app path, for example `/t/<thread>?m=<message>`.
  final void Function(String path) navigate;

  final Future<String?> Function(ThreadId thread) _localChannelName;
  final void Function(String line) _log;

  /// The slots of threads this router filled and has not removed since, each with whether
  /// its thread had an unread message while the slot was filled: only then a count of zero
  /// says that the person read the thread ([onCountChange]).
  final Map<String, bool> _shown = {};

  static Future<String?> _noChannel(ThreadId _) async => null;
  static void _developerLog(String line) =>
      developer.log(line, name: 'notifications');

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
    return thread == null ? 'kind:${n.kind.code}' : _threadTag(thread);
  }

  static String _threadTag(ThreadId thread) => 'thread:${thread.value}';

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
    final tag = tagOf(n);
    if (thread != null) _shown[tag] = unreadOf(thread) > 0;
    await presenter.show(
      toastContentFor(
        n,
        texts: texts(),
        appName: appName(),
        localChannelName: local,
      ),
      tag: tag,
      silent: n.quiet,
      payload: target,
    );
  }

  /// The engine reported the counts of [thread]: removes the notification of its slot where
  /// the platform allows it ([NotificationPresenter.cancelThread]) once the person has read
  /// the thread, on this device or on another one (01 10.3).
  ///
  /// The read is the unread count that falls to zero after the thread had an unread message
  /// while its slot was filled; the deletion of its last unread messages lowers the count the
  /// same way and removes the notification too. A report of the counts alone is no read: an
  /// edit by another person and every snapshot of `ready` report them too. So a notification
  /// that showed while nothing was unread, as that of a run that waits for the person before
  /// its agent wrote a message (13 2.1), stays until the thread gets an unread message and
  /// loses it again. Only a slot this router filled since the app started is removed, and
  /// each once; `security` and `admin` have no thread and stay.
  Future<void> onCountChange(ThreadId thread) async {
    final tag = _threadTag(thread);
    final hadUnread = _shown[tag];
    if (hadUnread == null) return;
    if (unreadOf(thread) > 0) {
      _shown[tag] = true;
      return;
    }
    if (!hadUnread) return;
    _shown.remove(tag);
    await presenter.cancelThread(tag);
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
