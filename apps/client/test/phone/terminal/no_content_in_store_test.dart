// SPDX-License-Identifier: Apache-2.0

// R-CL-17 on the phone (brief M5-12 fixed requirement 16; 15 23.4): a canary in the output of
// the host, in the input the person types through the soft keyboard and in the scrollback that
// a reattach replays appears in no table of the local database (the pins of `host_key_pins` are
// written there by the same flow), in no log line and in no client metric.

import 'dart:convert';

import 'package:client/app/providers/ports.dart';
import 'package:client/terminal/pin_store_adapter.dart';
import 'package:client/terminal/providers.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity;
import 'package:terminal_ui/terminal_ui.dart';

import '../../terminal/support.dart';
import 'support.dart';

const String _canary = 'CANARYTERM5c12';

void main() {
  testWidgets('terminal content reaches no table, no log line and no metric', (
    tester,
  ) async {
    final logs = <String>[];
    final previousPrint = debugPrint;
    debugPrint = (message, {wrapWidth}) => logs.add(message ?? '');
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    try {
      final db = (await tester.runAsync(() async => openInMemoryForTests()))!;
      addTearDown(() => tester.runAsync(db.close));
      final h = await PhoneTerminalHarness.pump(
        tester,
        services: (w) {
          final base = w.services();
          return TerminalServices(
            api: base.api,
            pins: LocalHostKeyPinStore(db.hostKeyPinsDao),
            events: base.events,
            engineEvents: base.engineEvents,
            stepUp: base.stepUp,
            newTransport: base.newTransport,
            links: base.links,
            clock: base.clock,
            config: base.config,
            context: base.context,
          );
        },
      );
      final c = await h.openShell(tester);
      await tester.runAsync(
        () => h.world.host.sendData(utf8.encode('$_canary output\r\n')),
      );
      await pumpUntil(
        tester,
        () => c.screen.visibleText().contains('$_canary output'),
      );
      // The person types the canary on the soft keyboard of the phone.
      await tester.tap(find.byType(TerminalScreenView));
      await tester.pump();
      final base = tester.testTextInput.editingState!['text'] as String;
      for (final ch in _canary.split('')) {
        tester.testTextInput.updateEditingValue(
          TextEditingValue(
            text: '$base$ch',
            selection: TextSelection.collapsed(offset: base.length + 1),
          ),
        );
      }
      await pumpUntil(
        tester,
        () => utf8.decode(h.world.host.input).contains(_canary),
      );
      // A detach in the background and the reattach with the replayed scrollback.
      h.lifecycle.set(AppActivity.background);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
      h.lifecycle.set(AppActivity.foreground);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
      await tester.runAsync(
        () => h.world.host.sendReplay(utf8.encode('$_canary replay\r\n')),
      );
      await pumpUntil(
        tester,
        () => c.screen.visibleText().contains('$_canary replay'),
      );

      final rows = (await tester.runAsync(() async {
        final out = <String>[];
        for (final table in db.allTables) {
          final result = await db
              .customSelect('SELECT * FROM "${table.actualTableName}"')
              .get();
          for (final row in result) {
            out.add('${table.actualTableName}: ${row.data}');
          }
        }
        return out;
      }))!;
      expect(
        rows.where((r) => r.startsWith('host_key_pins')),
        isNotEmpty,
        reason: 'the flow wrote its pin, so the scan saw a written table',
      );
      for (final row in rows) {
        expect(row, isNot(contains(_canary)), reason: row);
      }
      final metrics = jsonEncode([
        for (final e
            in h.container.read(appPortsProvider).measurements.metrics.take())
          e.toJson(),
      ]);
      expect(metrics, isNot(contains(_canary)));
      for (final line in logs) {
        expect(line, isNot(contains(_canary)), reason: line);
      }
      await h.close(tester);
    } finally {
      debugPrint = previousPrint;
      debugDefaultTargetPlatformOverride = null;
    }
  });
}
