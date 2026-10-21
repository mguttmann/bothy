// SPDX-License-Identifier: Apache-2.0

/// The wiring of the local notifications (brief M4-30; docs/spec/13-push-benachrichtigungen.md
/// 5.3): the events of the signed-in engine go through the [NotificationRouter] to the
/// presenter of the ports, and the targets of clicked notifications go to the app's router.
library;

import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../settings/settings_providers.dart';
import 'notification_router.dart';

part 'providers.g.dart';

/// The app paths of clicked notifications, for the app's router.
final class DesktopNotifications {
  /// Creates the value over [targets].
  const DesktopNotifications(this.targets);

  /// The app path of every clicked notification, for example `/t/<thread>?m=<message>`.
  final Stream<String> targets;
}

/// Shows `notification.created` of the signed-in engine as local notifications, on Windows and
/// on macOS while the app is connected (13 5.3, D-M4-30-5); the app widget listens to its
/// [DesktopNotifications.targets].
@Riverpod(keepAlive: true)
DesktopNotifications desktopNotifications(Ref ref) {
  final ports = ref.watch(appPortsProvider);
  final targets = StreamController<String>.broadcast();
  final router = NotificationRouter(
    presenter: ports.notifications,
    texts: () => lookupAppLocalizations(
      resolveAppLocale(
        userLocale: ref.read(profileControllerProvider).value?.locale,
        systemLocales: WidgetsBinding.instance.platformDispatcher.locales,
      ),
    ),
    appName: () =>
        ref.read(currentInstanceProvider)?.productName ??
        ports.branding.appName,
    // Open in the focused window that is not idle: the focus the app sends (04 6.2).
    isOpenAndFocused: (thread) {
      final engine = ref.read(syncEngineProvider);
      return engine != null &&
          !engine.focus.idle &&
          engine.focus.threads.contains(thread);
    },
    navigate: targets.add,
    localChannelName: (thread) async {
      final db = ref.read(localDatabaseProvider);
      final t = await db?.threadsDao.byId(thread);
      if (db == null || t == null) return null;
      return (await db.structureDao.channelById(t.channelId))?.name;
    },
  );
  final clicks = router.listen();
  StreamSubscription<NotificationCreated>? events;
  void bind(SyncEngine? engine) {
    unawaited(events?.cancel());
    events = engine?.notifications.listen((n) => unawaited(router.onEvent(n)));
  }

  ref.listen(syncEngineProvider, (_, engine) => bind(engine));
  bind(ref.read(syncEngineProvider));
  ref.onDispose(() {
    unawaited(events?.cancel());
    unawaited(clicks.cancel());
    unawaited(targets.close());
  });
  return DesktopNotifications(targets.stream);
}
