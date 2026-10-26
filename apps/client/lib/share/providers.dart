// SPDX-License-Identifier: Apache-2.0

/// The wiring of sharing (brief M5-06 Design notes 3 to 7): the signed-in account goes to
/// [AppleShare] on iOS and macOS with its triggers, and the share the Android runner received
/// waits in [PendingShare] for the route `/share`.
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as wire show Thread;
import 'package:auth/auth.dart' show SignedIn;
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/bootstrap.dart';
import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../files/providers.dart';
import '../onboarding/onboarding_controller.dart';
import 'share_android.dart';
import 'share_apple.dart';
import 'share_outbox.dart';
import 'share_takeover.dart';
import 'share_targets.dart';

part 'providers.g.dart';

/// [AppleShare] of the build kept in step with the signed-in account, or null where the build
/// has none (Windows, Android, tests).
///
/// The account is handed over once per signed-in services: after a login (the session's
/// `SignedIn` event) the child session is always created anew, at a start with a stored
/// session only when the shared keychain has none. The recent targets are written again when
/// the connection becomes live (the person is known from the snapshot), after every change of
/// the outbox (a confirmed send) and of the stored channels (`access.changed`); the
/// `share_inbox` rows of confirmed or discarded entries go on every change of the outbox. A
/// return to the foreground creates a missing child session and takes the inbox over, and the
/// instance limits follow `getInstance`.
@Riverpod(keepAlive: true)
AppleShare? appleShare(Ref ref) {
  final ports = ref.watch(appPortsProvider);
  final share = ports.appleShare;
  if (share == null) return null;
  final subscriptions = <StreamSubscription<Object?>>[];
  StreamSubscription<Object?>? authEvents;
  InstanceServices? watched;
  InstanceServices? handed;
  ShareAccount? account;
  var loggedIn = false;

  void stop() {
    for (final s in subscriptions) {
      unawaited(s.cancel());
    }
    subscriptions.clear();
  }

  // A login emits `SignedIn` on the session of the services shown on E2; a start with a stored
  // session does not. The event may come after the services were handed over, so then the
  // account is handed over again as one after a login.
  void watchLogin(InstanceServices? services) {
    if (identical(services, watched)) return;
    unawaited(authEvents?.cancel());
    authEvents = null;
    watched = services;
    loggedIn = false;
    if (services == null) return;
    authEvents = services.auth.events.listen((event) {
      if (event is! SignedIn) return;
      loggedIn = true;
      final current = account;
      if (identical(handed, services) && current != null) {
        unawaited(share.signedIn(current, afterLogin: true));
      }
    });
  }

  void sync() {
    final services = ref.read(signedInServicesProvider);
    if (identical(services, handed)) return;
    stop();
    handed = services;
    account = null;
    if (services == null) return;
    final db = services.database;
    final engine = services.engine;
    final source = ShareTargetSource(db, loadThread: threadLoaderOf(services));
    final max = ports.config.shareRecentTargets;
    final takeover = ShareTakeover(
      host: share.host,
      instanceId: services.instance.instanceId,
      outbox: ShareOutbox(
        outbox: engine.outbox,
        entries: db.outboxDao,
        outgoing: () async => ref.read(outgoingFolderProvider),
      ),
      db: db,
    );
    final current = ShareAccount(
      instanceId: services.instance.instanceId,
      serverUrl: services.instance.address,
      sessions: ApiExtensionSessions(services.auth.api),
      recentTargetsMax: max,
      recentTargets: () async {
        final me = engine.catchUp.me;
        return me == null ? null : source.recent(me: me, max: max);
      },
      takeOver: () async {
        await takeover.run();
        await takeover.prune();
      },
    );
    account = current;
    unawaited(share.signedIn(current, afterLogin: loggedIn));
    subscriptions
      ..add(
        engine.states.listen((s) {
          if (s.phase == ConnectionPhase.live) {
            unawaited(share.refreshTargets());
          }
        }),
      )
      ..add(
        engine.outbox.depth.listen((_) {
          unawaited(share.refreshTargets());
          unawaited(takeover.prune().catchError((Object _) {}));
        }),
      )
      ..add(source.changes().listen((_) => unawaited(share.refreshTargets())))
      ..add(
        db.instanceDao.watch().listen(
          (instance) => unawaited(share.limitsChanged(instance?.limits)),
        ),
      )
      ..add(
        ports.lifecycle.changes.listen((activity) {
          if (activity == AppActivity.foreground) unawaited(share.foreground());
        }),
      );
  }

  ref
    ..listen(
      onboardingControllerProvider.select((s) => s.services),
      (_, services) => watchLogin(services),
      fireImmediately: true,
    )
    ..listen(signedInServicesProvider, (_, _) => sync())
    ..onDispose(() {
      stop();
      unawaited(authEvents?.cancel());
    });
  sync();
  return share;
}

/// Stores a thread of [services] that the local database lacks, as `getThread` returns it, by
/// the version rule (`storeConfirmedThread`), so a recent target can show its title; throws
/// what the API client throws (`404` for a thread the person cannot see).
Future<void> Function(ThreadId thread) threadLoaderOf(
  InstanceServices services,
) => (thread) async {
  final api = services.auth.api;
  final found = (await api.getThreadsApi().getThread(threadId: thread.value))
      .data;
  if (found == null) return;
  final json = api.serializers.serializeWith(wire.Thread.serializer, found);
  if (json is Map) {
    await services.engine.catchUp.storeConfirmedThread(
      json.cast<String, Object?>(),
    );
  }
};

/// The share the Android runner received last, until the route `/share` sent or cancelled it.
@Riverpod(keepAlive: true)
class PendingShare extends _$PendingShare {
  @override
  SharedContent? build() => null;

  /// Keeps [content] for the sheet; a newer share replaces an older one, whose cache copies
  /// go.
  void put(SharedContent content) {
    final previous = state;
    if (previous != null && previous.id != content.id) {
      unawaited(AndroidShare.discard(previous));
    }
    state = content;
  }

  /// The sheet is done with [content].
  void done(SharedContent content) {
    if (identical(state, content)) state = null;
  }
}
