// SPDX-License-Identifier: Apache-2.0

// The last route selection (brief M4-19 Design note 5) is kept outside the refresh of the
// providers that notified it (PL/59): main run 40691988401 failed `forum_test.dart` on macOS
// with `Tried to rebuild newChatRowsProvider(...) multiple times in the same frame`, because the
// desktop shell's listener of the route selection set it while a refresh ran in which a
// provider that watches it (newChatRows, M4-41) had already been rebuilt.

import 'package:client/sidebar/providers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

/// A dependency of the route and of the rows, so both rebuild in one refresh.
class _Tick extends Notifier<int> {
  @override
  int build() => 0;

  void bump() => state++;
}

final _tick = NotifierProvider<_Tick, int>(_Tick.new);

/// The route selection: a new value on every build.
final _route = FutureProvider<RouteSelection>((ref) async {
  ref.watch(_tick);
  return RouteSelection(newChat: true, sentEntry: 'e${ref.watch(_tick)}');
});

/// A provider of the sidebar that watches the last selection, like newChatRows.
final _rows = Provider<String?>((ref) {
  ref.watch(_tick);
  return ref.watch(lastRouteSelectionProvider).sentEntry;
});

/// The tree above the rows, like channelsOfSpace.
final _tree = Provider<String?>((ref) => ref.watch(_rows));

void main() {
  test('a selection kept from a listener during a refresh rebuilds its watchers once', () async {
    final c = ProviderContainer();
    addTearDown(c.dispose);
    // The rows subscribe to the tick before the route, so the refresh rebuilds them first.
    final tree = c.listen(_tree, (_, _) {});
    final first = await c.read(_route.future);
    // As the desktop shell does (desktop_shell.dart).
    c.listen(_route, (_, next) {
      if (next.value case final value?) {
        c.read(lastRouteSelectionProvider.notifier).keep(value);
      }
    });

    c.read(_tick.notifier).bump();
    await pumpEventQueue();

    final latest = c.read(_route).value;
    expect(latest, isNot(same(first)));
    expect(c.read(lastRouteSelectionProvider), same(latest));
    expect(tree.read(), latest!.sentEntry);
  });
}
