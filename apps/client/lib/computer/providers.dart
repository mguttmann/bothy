// SPDX-License-Identifier: Apache-2.0

/// Computer use in the apps as providers (docs/spec/27-computer-use.md 11; docs/spec/
/// 15-client-architecture.md R-CL-18; brief CU-07 Scope): the owner's computer sessions, loaded
/// with `listComputerSessions` and kept current by `computer_session.changed` of the engine,
/// plus the stop and the switch-off.
///
/// The sessions live in memory only: the engine writes no table for them, and nothing here
/// stores, caches or requests an image (R-CL-18). Owner only (07 2.15 item 5): for a person who
/// is no owner the state stays [ComputerSessionsState.none] and no request goes out.
library;

import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart'
    show ComputerSessionChanged, HostEvent;

import '../app/providers/session.dart';
import '../notifications_settings/providers.dart' show isOwnerProvider;
import 'computer_api.dart';
import 'computer_status.dart';

part 'providers.g.dart';

/// The ports of computer use over one signed-in instance: the REST [api] and the host
/// [events] of the engine, where `computer_session.changed` arrives.
final class ComputerPorts {
  /// Creates the ports.
  const ComputerPorts({required this.api, required this.events});

  /// The REST operations.
  final ComputerApi api;

  /// The engine's host events.
  final Stream<HostEvent> events;
}

/// The computer ports of the signed-in instance, or null before the login; tests override it.
@Riverpod(keepAlive: true)
ComputerPorts? computerPorts(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  return ComputerPorts(
    api: ApiComputerApi(
      services.auth.api,
      newIdempotencyKey: services.ids.next,
    ),
    events: services.engine.hostEvents,
  );
}

/// The owner's computer sessions (27 11): the live ones and the ended ones this device saw
/// end, each ended one until [dismiss].
///
/// Example:
/// ```dart
/// final status = ref.watch(computerSessionsProvider).ofRun(runId);
/// await ref.read(computerSessionsProvider.notifier).stop(status!.sessionId);
/// ```
@Riverpod(keepAlive: true)
class ComputerSessions extends _$ComputerSessions {
  ComputerPorts? _ports;

  /// The sessions dismissed by the owner, so a replayed end does not show again.
  final Set<String> _dismissed = {};

  @override
  ComputerSessionsState build() {
    final ports = ref.watch(computerPortsProvider);
    final owner = ref.watch(isOwnerProvider).value ?? false;
    _ports = null;
    if (ports == null || !owner) return ComputerSessionsState.none;
    _ports = ports;
    final sub = ports.events.listen((e) {
      if (e is ComputerSessionChanged) _apply(ComputerStatus.fromEvent(e));
    });
    ref.onDispose(sub.cancel);
    // The first load runs after build, so it may set the state.
    scheduleMicrotask(() => unawaited(reload().catchError((Object _) {})));
    return ComputerSessionsState.none;
  }

  /// Loads the sessions again (`listComputerSessions`): the ones that are not closed become
  /// live; a live one the list names closed shows its end once. Throws the error of the call.
  Future<void> reload() async {
    final ports = _ports;
    if (ports == null) return;
    final sessions = await ports.api.listSessions();
    if (!ref.mounted || !identical(_ports, ports)) return;
    for (final s in sessions.reversed) {
      final status = ComputerStatus.fromSession(s);
      // A closed session the device never saw live stays history (27 11 item 1: once).
      if (status.closed && state.live[status.sessionId] == null) continue;
      _apply(status);
    }
  }

  /// Stops [sessionId] (`computer.stop`, 27 8.3) and shows its end once. Throws the error of
  /// the call (a refusal, no connection), and a [StateError] before the login or the owner
  /// check; the session then stays as it was.
  Future<void> stop(String sessionId) async {
    final ports = _ports ?? (throw StateError(_unavailable));
    final session = await ports.api.stop(sessionId);
    if (!ref.mounted || !identical(_ports, ports)) return;
    _apply(ComputerStatus.fromSession(session));
  }

  /// Switches the enablement of [hostId] off (`computer.device.disable`, 27 4 item 4); the
  /// end of its session arrives as an event. Throws the error of the call, and a [StateError]
  /// before the login or the owner check.
  Future<void> disableDevice(String hostId) async {
    final ports = _ports ?? (throw StateError(_unavailable));
    await ports.api.disableDevice(hostId);
  }

  /// Before the login or the owner check no request goes out, so a stop or a switch-off
  /// fails instead of reporting a success that never reached the server.
  static const String _unavailable = 'computer use is not available yet';

  /// Hides the ended [sessionId] for good.
  void dismiss(String sessionId) {
    _dismissed.add(sessionId);
    if (!state.ended.containsKey(sessionId)) return;
    state = ComputerSessionsState(
      live: state.live,
      ended: {...state.ended}..remove(sessionId),
    );
  }

  void _apply(ComputerStatus status) {
    final known = state.known(status.sessionId);
    if (known != null && status.version <= known.version) return;
    final live = {...state.live};
    final ended = {...state.ended};
    if (status.closed) {
      final wasLive = live.remove(status.sessionId) != null;
      if (wasLive && !_dismissed.contains(status.sessionId)) {
        ended[status.sessionId] = status;
      } else if (!wasLive) {
        return;
      }
    } else {
      live[status.sessionId] = status;
    }
    state = ComputerSessionsState(live: live, ended: ended);
  }
}
