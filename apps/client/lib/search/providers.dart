// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of the search view D7 (brief M4-23 Scope, docs/spec/15-client-architecture.md
/// 4: one provider per query): the server and the local search port, the principals, spaces and
/// channels for the notation and the filter menus, the connection, and the controller that holds
/// one [SearchSession]. Widgets never query the database themselves (R-CL-03).
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import 'local_search.dart';
import 'search_controller.dart';
import 'search_port.dart';

part 'providers.g.dart';

/// The server search of the signed-in instance (`search`), or null while signed out.
@riverpod
SearchPort? searchPort(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  return services == null ? null : ApiSearchPort(services.auth.api);
}

/// The offline search over the local index (14 6), or null while no database is open.
@riverpod
LocalSearchPort? localSearchPort(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null ? null : LocalSearch(db);
}

/// The stored people and agents by display name: `from:` and the "From" menu.
@riverpod
Stream<List<Principal>> searchPrincipals(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return Stream.value(const []);
  return db.principalsDao.watchAll().map(
    (all) => [...all]
      ..sort(
        (a, b) =>
            a.displayName.toLowerCase().compareTo(b.displayName.toLowerCase()),
      ),
  );
}

/// The stored spaces in their order: the "Space" menu.
@riverpod
Stream<List<Space>> searchSpaces(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null ? Stream.value(const []) : db.structureDao.watchSpaces();
}

/// The stored channels of [spaceId] in their order.
@riverpod
Stream<List<Channel>> searchSpaceChannels(Ref ref, String spaceId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.structureDao.watchChannels(SpaceId.parse(spaceId));
}

/// The stored channels of every space, space by space: `in:` and the "Channel" menu. The
/// database holds only what the person may see (15 1 principle 1).
@riverpod
List<Channel> searchChannels(Ref ref) {
  final spaces = ref.watch(searchSpacesProvider).value ?? const <Space>[];
  return [
    for (final s in spaces)
      ...?ref.watch(searchSpaceChannelsProvider(s.id.value)).value,
  ];
}

/// Whether the engine's connection is live (15 7.2); any other phase counts as offline for the
/// search (brief M4-23 Design note 1).
@riverpod
bool searchOnline(Ref ref) =>
    ref.watch(connectionStateProvider).value?.phase == ConnectionPhase.live;

/// The search of D7, or with [channelId] and [threadId] the in-thread search of M4-24, which
/// sends both with every request.
///
/// Example:
/// ```dart
/// final search = ref.read(searchQueryControllerProvider().notifier);
/// search.setInput('from:sam deploy');
/// await search.submit();
/// ```
@riverpod
class SearchQueryController extends _$SearchQueryController {
  late SearchSession _session;

  @override
  SearchState build({String? channelId, String? threadId}) {
    final session = SearchSession(
      server: ref.watch(searchPortProvider),
      local: ref.watch(localSearchPortProvider),
      principals: () => ref.read(searchPrincipalsProvider).value ?? const [],
      channels: () => ref.read(searchChannelsProvider),
      now: () => ref.read(appPortsProvider).clock.now(),
      live: () => ref.read(searchOnlineProvider),
      localLimit: ref.watch(appConfigProvider).quickSwitchMaxResults,
      emit: (s) => state = s,
      scopeChannel: channelId == null ? null : ChannelId.parse(channelId),
      scopeThread: threadId == null ? null : ThreadId.parse(threadId),
    );
    // Kept subscribed, so a submit reads current values without rebuilding the search.
    ref
      ..listen(searchPrincipalsProvider, (_, _) {})
      ..listen(searchChannelsProvider, (_, _) {})
      ..listen(searchOnlineProvider, (previous, next) {
        if (next && previous != true) unawaited(session.reconnected());
      })
      ..onDispose(session.dispose);
    _session = session;
    return session.state;
  }

  /// Stores the typed text.
  void setInput(String input) => _session.setInput(input);

  /// Stores the filter column and, after a first search, searches again.
  Future<void> setFilters(SearchFilters filters) =>
      _session.setFilters(filters);

  /// Searches the typed text with the filters (server and local at the same time).
  Future<void> submit() => _session.submit();

  /// Loads the next server page ("Load more").
  Future<void> loadMore() => _session.loadMore();
}
