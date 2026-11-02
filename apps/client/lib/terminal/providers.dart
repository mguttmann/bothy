// SPDX-License-Identifier: Apache-2.0

/// The terminal of the desktop app as providers (docs/spec/01-product.md 5.1 D11;
/// docs/spec/15-client-architecture.md 23; brief M4-36 Scope): the ports of `terminal_ui` over
/// the signed-in instance, the hosts of the group "Hosts" with their connection and the count of
/// own sessions, and the tabs of each host, one per own session.
///
/// Host data is never stored: it comes from REST and is reloaded after the host events of
/// `sync_engine` (D-M4-36-4). No permission logic: the hosts and profiles shown are what the
/// server returns (15 1 principle 1).
library;

import 'dart:async';

import 'package:api_client/api_client.dart'
    show Host, HostState, TerminalSession;
import 'package:api_client/api_client.dart' as rest show TerminalSessionState;
import 'package:auth/auth.dart' show Clock;
import 'package:domain/domain.dart' show AppConfig, AppConfigValues, HostId;
import 'package:flutter/widgets.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart' as sync;
import 'package:sync_engine/sync_engine.dart'
    show AppLifecycle, ConnectionPhase;
import 'package:terminal_ui/terminal_ui.dart';
import 'package:url_launcher/url_launcher.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import 'host_events_adapter.dart';
import 'pin_store_adapter.dart';
import 'reconnect.dart';
import 'terminal_api_adapter.dart';
import 'terminal_transport_io.dart';

part 'providers.g.dart';

/// The audit reason of `closeTerminalSession` when the person ends a session in the app.
const String terminalCloseReason = 'Ended in the terminal view';

/// Opens a confirmed link in the system browser (24 11.2; `LinkOpener` of `terminal_ui`).
final class SystemLinkOpener implements LinkOpener {
  /// Creates the opener; [launch] replaces `launchUrl` in tests.
  SystemLinkOpener({Future<bool> Function(Uri url)? launch})
    : _launch =
          launch ??
          ((url) => launchUrl(url, mode: LaunchMode.externalApplication));

  final Future<bool> Function(Uri url) _launch;

  @override
  Future<void> open(Uri target) async {
    await _launch(target);
  }
}

/// The ports of `terminal_ui` over one signed-in instance (15 23.1).
final class TerminalServices {
  /// Creates the services.
  TerminalServices({
    required this.api,
    required this.pins,
    required this.events,
    required this.engineEvents,
    required this.stepUp,
    required this.newTransport,
    required this.links,
    required this.clock,
    required this.config,
    required this.context,
    this.online = const Stream.empty(),
    this.lifecycle,
  });

  /// The REST side (without the interceptor that answers step-ups itself).
  final TerminalHostsApi api;

  /// The pins in `host_key_pins`.
  final HostKeyPinStore pins;

  /// The host events of the engine as the port of `terminal_ui`.
  final HostEvents events;

  /// The host events of the engine as they come.
  final Stream<sync.HostEvent> engineEvents;

  /// The step-up through the step-up device key (`session.stepUp.confirm`).
  final TerminalStepUp stepUp;

  /// A transport for one new tab.
  final TerminalTransport Function() newTransport;

  /// The opener of confirmed links.
  final LinkOpener links;

  /// The clock of the pins.
  final Clock clock;

  /// `AppConfig` (scrollback, font size, maximum duration).
  final AppConfig config;

  /// The context of the navigator the dialogs open over.
  final BuildContext? Function() context;

  /// Whether the engine's connection is live, one value per change: a dropped stream attaches
  /// again once it turns live (FIX-terminal-session-lifecycle).
  final Stream<bool> online;

  /// The app's life cycle: coming to the front attaches dropped streams again; null without.
  final AppLifecycle? lifecycle;
}

/// The terminal ports of the signed-in instance, or null before the login.
@Riverpod(keepAlive: true)
TerminalServices? terminalServices(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  final ports = ref.watch(appPortsProvider);
  final auth = services.auth;
  return TerminalServices(
    api: ApiTerminalServerApi(
      services.terminalApi,
      closeReason: terminalCloseReason,
    ),
    pins: LocalHostKeyPinStore(services.database.hostKeyPinsDao),
    events: EngineHostEvents(services.engine.hostEvents),
    engineEvents: services.engine.hostEvents,
    // The terminal's step-up must be fresh (08 6.3): only the step-up key gives it, so a device
    // without one (Windows) is refused before any request instead of asking for a password.
    stepUp: auth.stepUp.confirmFresh,
    newTransport: () => IoTerminalTransport(
      origin: services.instance.address,
      auth: auth,
      allowPlainHttpForTests: ports.allowPlainHttpForTests,
    ),
    links: SystemLinkOpener(),
    clock: ports.clock,
    config: ports.config,
    context: () => ports.navigatorKey.currentContext,
    online: services.engine.states.map((s) => s.phase == ConnectionPhase.live),
    lifecycle: ports.lifecycle,
  );
}

/// Attaches the dropped streams of every host's tabs again when the app is online again or in
/// front (FIX-terminal-session-lifecycle, D-TSL-5); null before the login.
@Riverpod(keepAlive: true)
TerminalReconnector? terminalReconnector(Ref ref) {
  final services = ref.watch(terminalServicesProvider);
  if (services == null) return null;
  final r = TerminalReconnector(
    online: services.online,
    lifecycle: services.lifecycle,
  );
  ref.onDispose(r.dispose);
  return r;
}

/// The person's own user ID as the engine's last snapshot named it; null before it.
@Riverpod(keepAlive: true)
String? terminalSelfUserId(Ref ref) {
  ref.watch(connectionStateProvider);
  return ref.watch(signedInServicesProvider)?.engine.catchUp.me?.value;
}

/// The hosts of the group "Hosts" (01 D11): every host the server lists for the person, its
/// connection and the count of the person's own sessions that are not closed.
@immutable
final class HostsOverview {
  /// Creates the overview.
  const HostsOverview({
    required this.hosts,
    required this.ownSessions,
    required this.connected,
  });

  /// The hosts in the server's order.
  final List<Host> hosts;

  /// The own sessions that are not closed, per host ID.
  final Map<String, List<TerminalSession>> ownSessions;

  /// The connection per host ID, from `connected` and `host.presence_changed`.
  final Map<String, bool> connected;

  /// The host [hostId], or null when the person does not see it.
  Host? host(String hostId) => hosts.where((h) => h.id == hostId).firstOrNull;

  /// The count of own sessions on [hostId].
  int sessionCount(String hostId) => ownSessions[hostId]?.length ?? 0;

  /// Whether [hostId] is connected.
  bool isConnected(String hostId) => connected[hostId] ?? false;

  /// A copy with the connection of [hostId] set to [value].
  HostsOverview withConnection(String hostId, bool value) => HostsOverview(
    hosts: hosts,
    ownSessions: ownSessions,
    connected: {...connected, hostId: value},
  );
}

/// Loads the overview through REST: `listHosts` and `listTerminalSessions`, keeping the
/// sessions of [selfUserId] that are not closed.
Future<HostsOverview> loadHostsOverview(
  TerminalHostsApi api,
  String? selfUserId,
) async {
  final hosts = await api.listHosts();
  final sessions = await api.listSessions();
  final own = <String, List<TerminalSession>>{};
  for (final s in sessions) {
    if (s.userId != selfUserId || s.state == rest.TerminalSessionState.closed) {
      continue;
    }
    own.putIfAbsent(s.hostId, () => []).add(s);
  }
  return HostsOverview(
    hosts: hosts,
    ownSessions: own,
    connected: {for (final h in hosts) h.id: h.connected},
  );
}

/// The hosts of the person, reloaded after every host event (D-M4-36-4); null before the
/// login.
@Riverpod(keepAlive: true)
class HostsList extends _$HostsList {
  bool _reloading = false;
  bool _again = false;

  @override
  Future<HostsOverview?> build() async {
    final services = ref.watch(terminalServicesProvider);
    final me = ref.watch(terminalSelfUserIdProvider);
    if (services == null) return null;
    final sub = services.engineEvents.listen((e) {
      if (e is sync.HostPresenceChanged) {
        final current = state.value;
        if (current != null) {
          state = AsyncData(current.withConnection(e.hostId, e.connected));
        }
        return;
      }
      unawaited(reload());
    });
    ref.onDispose(sub.cancel);
    return loadHostsOverview(services.api, me);
  }

  /// Loads the hosts again; a call while a load runs loads once more after it.
  Future<void> reload() async {
    final services = ref.read(terminalServicesProvider);
    if (services == null) return;
    if (_reloading) {
      _again = true;
      return;
    }
    _reloading = true;
    try {
      do {
        _again = false;
        final me = ref.read(terminalSelfUserIdProvider);
        final next = await AsyncValue.guard(
          () => loadHostsOverview(services.api, me),
        );
        if (!ref.mounted) return;
        // A failed reload keeps the hosts shown before.
        if (next.hasValue || !state.hasValue) state = next;
      } while (_again);
    } finally {
      _reloading = false;
    }
  }
}

/// The tabs of the terminal view of one host: one [TerminalController] per own session
/// (01 D11), the selected one, the font size of the view and the search in the scrollback.
///
/// The tabs live as long as the login, so leaving the view keeps the streams; the logout
/// disposes them with the services. A session the server lists for the person becomes a
/// detached tab with "Reattach" (`TerminalController.adopt`); a session the server closed ends
/// its tab, which "Close" removes (FIX-terminal-session-lifecycle).
final class TerminalTabs extends ChangeNotifier
    implements TerminalReconnectTarget {
  /// Creates the tabs of [hostId].
  TerminalTabs({
    required this.hostId,
    required this.services,
    required this.selfUserId,
  }) : fontSize = services.config.terminalFontSizeDefault.toDouble() {
    _changes = services.events.watch(hostId).listen((change) {
      if (change is TerminalSessionUpdated && !change.closed) {
        unawaited(refresh());
      }
    });
  }

  /// The host.
  final String hostId;

  /// The ports.
  final TerminalServices services;

  /// The person's own user ID.
  final String selfUserId;

  final List<TerminalController> _tabs = [];
  late final StreamSubscription<HostChange> _changes;
  int _selected = 0;
  bool _disposed = false;

  /// The smallest and largest font size the shortcuts reach.
  static const double minFontSize = 8;

  /// The largest font size the shortcuts reach.
  static const double maxFontSize = 40;

  /// The font size of the view in logical pixels (`client.terminal.font_size_default` until
  /// the person changes it, 01 8).
  double fontSize;

  /// The tabs, in order.
  List<TerminalController> get tabs => List.unmodifiable(_tabs);

  /// The index of the selected tab.
  int get selected => _selected;

  /// The selected tab, or null without tabs.
  TerminalController? get current =>
      _tabs.isEmpty ? null : _tabs[_selected.clamp(0, _tabs.length - 1)];

  /// The label of each tab: profile and running number per profile (`terminalTabLabel`).
  List<(String profile, int number)> get labels {
    final counts = <String, int>{};
    return [
      for (final t in _tabs)
        () {
          final profile = t.state.profile ?? '';
          final n = (counts[profile] ?? 0) + 1;
          counts[profile] = n;
          return (profile, n);
        }(),
    ];
  }

  TerminalController _controller() => TerminalController(
    api: services.api,
    transport: services.newTransport(),
    pins: services.pins,
    stepUp: services.stepUp,
    links: services.links,
    clock: services.clock,
    prompts: DialogTerminalPrompts(services.context, selfUserId: selfUserId),
    selfUserId: selfUserId,
    config: services.config,
    events: services.events,
  );

  /// Loads the host and the person's sessions on it, adds a detached tab for each own session
  /// that is not closed and has no tab yet, and ends the tab of a session the server closed
  /// meanwhile (the app slept and missed its event; `TerminalController.reconcile`).
  Future<void> refresh() async {
    final Host host;
    final List<TerminalSession> sessions;
    try {
      host = await services.api.getHost(hostId);
      sessions = await services.api.listSessions(hostId: hostId);
    } on Object {
      // The next host event or opening of the view tries again; the tabs stay as they are.
      return;
    }
    if (_disposed) return;
    final byId = {for (final s in sessions) s.id: s};
    for (final t in _tabs) {
      if (byId[t.state.sessionId] case final s?) t.reconcile(s);
    }
    // A tab whose opening still waits for the server has no session ID yet; its session
    // would show up twice, so the next refresh adds what is new.
    if (_tabs.any(_opening)) return;
    final known = {for (final t in _tabs) ?t.state.sessionId};
    var added = false;
    for (final s in sessions) {
      if (s.userId != selfUserId ||
          s.state == rest.TerminalSessionState.closed ||
          known.contains(s.id)) {
        continue;
      }
      final c = _controller()..adopt(host: host, session: s);
      c.addListener(notifyListeners);
      _tabs.add(c);
      added = true;
    }
    if (added) notifyListeners();
  }

  /// Opens a new session with [profile] in a new tab and selects it: the dialog "Open
  /// terminal", the step-up and the stream run in its controller. Tabs whose opening failed or
  /// was cancelled before a session existed go first.
  Future<void> newSession(
    String profile, {
    int cols = 80,
    int rows = 24,
  }) async {
    for (final t in _tabs.where(_withoutSession).toList()) {
      _tabs.remove(t);
      t
        ..removeListener(notifyListeners)
        ..dispose();
    }
    final c = _controller()..addListener(notifyListeners);
    _tabs.add(c);
    _selected = _tabs.length - 1;
    notifyListeners();
    await c.open(
      host: HostId.parse(hostId),
      profile: profile,
      cols: cols,
      rows: rows,
    );
  }

  static bool _opening(TerminalController t) =>
      t.state.sessionId == null &&
      (t.state.phase == TerminalPhase.preparing ||
          t.state.phase == TerminalPhase.connecting);

  static bool _withoutSession(TerminalController t) =>
      t.state.sessionId == null &&
      (t.state.phase == TerminalPhase.failed ||
          t.state.phase == TerminalPhase.idle);

  /// The maximum duration of the person's sessions on [host] for the header:
  /// `terminal.owner_account_max_session_duration` in the owner account mode (the person's own
  /// assignment is the owner account, ADR 0073), else `terminal.max_session_duration`.
  Duration maxSessionDuration(Host host) =>
      ownOwnerAccount(host, selfUserId) != null
      ? services.config.ownerAccountMaxSessionDuration
      : services.config.maxSessionDuration;

  /// The rows of the session list (FIX-terminal-session-lifecycle): every tab with a session,
  /// named by [label], with its start, since when it is detached, "Reattach" for a detached one
  /// and a tap that selects its tab.
  List<TerminalSessionEntry> entries(
    String Function(String profile, int n) label,
  ) {
    final names = labels;
    return [
      for (final (i, t) in _tabs.indexed)
        if (t.state.sessionId != null &&
            t.state.phase != TerminalPhase.ended &&
            t.state.phase != TerminalPhase.blocked)
          TerminalSessionEntry(
            label: label(names[i].$1, names[i].$2),
            phase: t.state.phase,
            startedAt: t.state.openedAt,
            detachedAt: t.state.phase == TerminalPhase.detached
                ? t.state.detachedAt
                : null,
            onReattach: t.state.phase == TerminalPhase.detached
                ? () {
                    select(_tabs.indexOf(t));
                    unawaited(t.attach());
                  }
                : null,
            onSelect: () => select(_tabs.indexOf(t)),
          ),
    ];
  }

  /// Removes the tab of [tab] and disposes it; the selection stays on the tab next to it.
  /// Used for an ended tab ("Close") and after "End" (FIX-terminal-session-lifecycle).
  void close(TerminalController tab) {
    final i = _tabs.indexOf(tab);
    if (i < 0) return;
    _tabs.removeAt(i);
    tab
      ..removeListener(notifyListeners)
      ..dispose();
    if (_selected >= _tabs.length) {
      _selected = _tabs.isEmpty ? 0 : _tabs.length - 1;
    }
    if (!_disposed) notifyListeners();
  }

  /// "End": ends the session of [tab] on the host and closes its tab.
  Future<void> end(TerminalController tab) async {
    await tab.end();
    close(tab);
  }

  /// The detach causes of a stream that dropped while the tab was attached: those attach again
  /// by themselves once the app is back (D-TSL-5). A detach by the person, another device
  /// (`4103`), a stale token (`4101`, needs a step-up) or a refused login stay.
  static const Set<DetachCause> _reconnectable = {
    DetachCause.connectionLost,
    DetachCause.serverRestart,
    DetachCause.slowConsumer,
    DetachCause.hostUnavailable,
  };

  @override
  void reconnectLost() {
    for (final t in List.of(_tabs)) {
      if (t.state.phase == TerminalPhase.detached &&
          _reconnectable.contains(t.state.detachCause)) {
        unawaited(t.attach());
      }
    }
  }

  /// Selects tab [index].
  void select(int index) {
    if (index < 0 || index >= _tabs.length || index == _selected) return;
    _selected = index;
    notifyListeners();
  }

  /// Selects the next tab, wrapping around.
  void next() {
    if (_tabs.length > 1) select((_selected + 1) % _tabs.length);
  }

  /// Selects the previous tab, wrapping around.
  void previous() {
    if (_tabs.length > 1) {
      select((_selected - 1 + _tabs.length) % _tabs.length);
    }
  }

  /// Makes the font larger ([step] positive) or smaller, within [minFontSize] and
  /// [maxFontSize].
  void changeFont(double step) {
    final next = (fontSize + step).clamp(minFontSize, maxFontSize);
    if (next == fontSize) return;
    fontSize = next;
    notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    unawaited(_changes.cancel());
    for (final t in _tabs) {
      t
        ..removeListener(notifyListeners)
        ..dispose();
    }
    _tabs.clear();
    super.dispose();
  }
}

/// The tabs of host [hostId], kept for the login; null before the login or before the engine
/// named the person.
@Riverpod(keepAlive: true)
TerminalTabs? terminalTabs(Ref ref, String hostId) {
  final services = ref.watch(terminalServicesProvider);
  final me = ref.watch(terminalSelfUserIdProvider);
  if (services == null || me == null) return null;
  final tabs = TerminalTabs(hostId: hostId, services: services, selfUserId: me);
  final reconnector = ref.watch(terminalReconnectorProvider);
  reconnector?.register(tabs);
  ref.onDispose(() {
    reconnector?.unregister(tabs);
    tabs.dispose();
  });
  return tabs;
}

/// Whether [host] is disabled (shown as "disabled" in the group, 01 D11).
bool hostDisabled(Host host) => host.state == HostState.disabled;
