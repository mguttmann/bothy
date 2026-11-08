// SPDX-License-Identifier: Apache-2.0

// A personal archive (01 6.22 item 4, PROJ-ARCHIVE) changes only the sidebar: the archived
// channel stays a target of "Move thread" (01 6.7) and stays in the level tree of the
// notification settings.

import 'package:client/notifications_settings/providers.dart';
import 'package:client/thread/providers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import 'sidebar_archive_test.dart' show fill;

void main() {
  late List<String> move;
  late List<String> tree;

  Widget probe() => Consumer(
    builder: (context, ref, _) {
      move = [for (final c in ref.watch(visibleChannelsProvider)) c.name];
      tree = [
        for (final s
            in ref.watch(levelTreeProvider).value ?? const <LevelTreeSpace>[])
          for (final c in s.channels) c.channel.name,
      ];
      return const SizedBox();
    },
  );

  testWidgets('an archived project stays a move target', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(tester, probe());
    expect(move, contains('website'));
    await tester.runAsync(
      () => host.db.structureDao.putUserChannelHiddenLocally(
        channelId(2),
        hiddenAt: DateTime.utc(2026),
      ),
    );
    await host.settle(tester);
    expect(move, contains('website'));
    await host.dispose(tester);
  });

  testWidgets('an archived project stays in the level tree', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(tester, probe());
    await host.settle(tester);
    expect(tree, contains('website'));
    await tester.runAsync(
      () => host.db.structureDao.putUserChannelHiddenLocally(
        channelId(2),
        hiddenAt: DateTime.utc(2026),
      ),
    );
    await host.settle(tester);
    expect(tree, contains('website'));
    await host.dispose(tester);
  });
}
