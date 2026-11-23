// SPDX-License-Identifier: Apache-2.0

// IndexedDB of the terminal pins in a real browser: run with
// `flutter test --platform chrome test/web` from apps/console (`make console-web-test`).
// `make test` runs test/ on the VM and skips this file (TestOn browser).
@TestOn('browser')
library;

import 'dart:async';
import 'dart:js_interop';

import 'package:console/terminal/ports.dart';
import 'package:console/terminal/ports_web.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:web/web.dart' as web;

/// Opens the pin database the way another tab would and keeps the connection open.
Future<web.IDBDatabase> _holdOpen() {
  final request = web.window.indexedDB.open(pinDatabaseName, 1);
  final done = Completer<web.IDBDatabase>();
  request.onupgradeneeded = ((web.Event _) {
    final db = request.result as web.IDBDatabase;
    if (!db.objectStoreNames.contains(pinStoreName)) {
      db.createObjectStore(pinStoreName);
    }
  }).toJS;
  request.onsuccess = ((web.Event _) {
    done.complete(request.result as web.IDBDatabase);
  }).toJS;
  return done.future;
}

void main() {
  test(
    'a record survives the round trip and the deletion removes it',
    () async {
      final pins = IndexedDbPinDatabase(web.window.indexedDB);
      await pins.write('host-1', {'fingerprint': 'SHA256:abc'});
      expect(await pins.read('host-1'), {'fingerprint': 'SHA256:abc'});
      await pins.delete();
      expect(await pins.read('host-1'), isNull);
    },
  );

  test(
    'a deletion that another connection blocks fails instead of waiting',
    () async {
      final pins = IndexedDbPinDatabase(web.window.indexedDB);
      await pins.write('host-1', {'fingerprint': 'SHA256:abc'});
      final other = await _holdOpen();
      try {
        await expectLater(
          pins.delete().timeout(const Duration(seconds: 5)),
          throwsA(isA<StateError>()),
        );
      } finally {
        other.close();
      }
      // The browser deletes the database once the other connection closed.
      await Future<void>.delayed(const Duration(milliseconds: 100));
      expect(await pins.read('host-1'), isNull);
    },
  );
}
