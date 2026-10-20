// SPDX-License-Identifier: Apache-2.0

// R-CL-17, SEC-TERM-1: a canary in the output of the host, in the input of the person and in
// a tampered frame appears in no log record (debugPrint, print, FlutterError) and in no
// exception text or state of the controller.

import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';

const String _canary = 'CANARY-7f3a9c';

void main() {
  test(
    'the canary reaches the screen and the host, but no log and no state text',
    () async {
      final logs = <String>[];
      final previous = debugPrint;
      debugPrint = (message, {wrapWidth}) => logs.add(message ?? '');
      final errors = <String>[];
      final previousError = FlutterError.onError;
      FlutterError.onError = (d) => errors.add(d.toString());
      addTearDown(() {
        debugPrint = previous;
        FlutterError.onError = previousError;
      });

      await runZonedGuarded(
        () async {
          final h = await Harness.create();
          await h.open();
          await h.connected();
          await h.host.sendData(utf8.encode('$_canary output\r\n'));
          h.controller.input(utf8.encode('$_canary input\r'));
          await h.settle();
          expect(h.controller.screen.visibleText(), contains(_canary));
          expect(utf8.decode(h.host.input), contains(_canary));
          // A frame that cannot be decoded: its bytes must not show up in the block either.
          await h.host.sendRawEncrypted(
            Uint8List.fromList([0x0b, ...utf8.encode(_canary)]),
          );
          await h.until((s) => s.phase == TerminalPhase.blocked);
          expect(h.controller.state.toString(), isNot(contains(_canary)));
        },
        (error, stack) => errors.add('$error'),
        zoneSpecification: ZoneSpecification(
          print: (self, parent, zone, line) => logs.add(line),
        ),
      );
      expect(logs.where((l) => l.contains(_canary)), isEmpty);
      expect(errors.where((e) => e.contains(_canary)), isEmpty);
      expect(errors, isEmpty);
    },
  );
}
