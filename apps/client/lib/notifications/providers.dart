// SPDX-License-Identifier: Apache-2.0

/// The wiring of the local notifications (brief M4-30; docs/spec/13-push-notifications.md
/// 4.6 and 5.3): the events of the signed-in engine go through the [NotificationRouter] to the
/// presenter of the ports on the desktop and to the in-app banner on the phone, and the
/// targets of clicked notifications go to the app's router.
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../settings/settings_providers.dart';
import 'in_app_banner.dart';
import 'notification_router.dart';

part 'providers.g.dart';

/// The app paths of clicked notifications and tapped banners, for the app's router.
final class DesktopNotifications {
  /// Creates the value over [targets].
  const DesktopNotifications(this.targets);

  /// The app path of every clicked notification, for example `/t/<thread>?m=<message>`.
  final Stream<String> targets;
}

/// The banners of the phone app, or null on the desktop: a phone is connected only while it is
/// in front, so it shows the hints of the gateway inside the app instead of as notifications
/// of the system (13 4.6 item 1; docs/decisions.d/AUD-client-app-01.md). The app widget lays
/// them over its content with an `InAppBannerHost`.
@Riverpod(keepAlive: true)
InAppBanners? inAppBanners(Ref ref) {
  if (!ref.watch(appPortsProvider).formFactor.isPhone) return null;
  final banners = InAppBanners();
  ref.onDispose(() => unawaited(banners.dispose()));
  return banners;
}

/// Shows `notification.created` of the signed-in engine while the app is connected: as local
/// notifications on Windows and macOS (13 5.3, D-M4-30-5), as the in-app banner on iOS and
/// Android ([inAppBanners], with the same rules of the [NotificationRouter]), and removes the
/// notification or the banner of a thread once the person has read it (01 10.3); the app
/// widget listens to its [DesktopNotifications.targets].
@Riverpod(keepAlive: true)
DesktopNotifications desktopNotifications(Ref ref) {
  final ports = ref.watch(appPortsProvider);
  final banners = ref.watch(inAppBannersProvider);
  final targets = StreamController<String>.broadcast();
  final router = NotificationRouter(
    presenter: banners ?? ports.notifications,
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
  StreamSubscription<ThreadId>? reads;
  void bind(SyncEngine? engine) {
    unawaited(events?.cancel());
    unawaited(reads?.cancel());
    events = engine?.notifications.listen((n) => unawaited(router.onEvent(n)));
    // A thread without anything unread, read here or on another device (`read_state.updated`),
    // loses its notification or its banner (01 10.3).
    reads = engine?.catchUp.countChanges.listen((thread) {
      if (engine.catchUp.countsOf(thread).unread == 0) {
        unawaited(router.onRead(thread));
      }
    });
    // A banner of the account that ended names its content: it goes with the engine.
    if (engine == null) banners?.clear();
  }

  ref.listen(syncEngineProvider, (_, engine) => bind(engine));
  bind(ref.read(syncEngineProvider));
  ref.onDispose(() {
    unawaited(events?.cancel());
    unawaited(reads?.cancel());
    unawaited(clicks.cancel());
    unawaited(targets.close());
  });
  return DesktopNotifications(targets.stream);
}
